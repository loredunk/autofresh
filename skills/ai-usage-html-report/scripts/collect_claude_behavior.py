#!/usr/bin/env python3
"""Collect a Claude Code *behavior* profile from local ~/.claude/projects/**/*.jsonl
transcripts, mirroring the ccoach/codexreport JSON field shapes so the dual
renderer can show both platforms symmetrically.

Pure python3 standard library, zero pip, offline, no network.

Window filtering (by record timestamp, converted to local tz):
  --since YYYY-MM-DD   from this local day through today (inclusive)
  --days  N            last N local days including today
  --date  YYYY-MM-DD   a single local day
  (default: full history)

Scope (--scope, default global):
  global   one aggregate across everything (back-compatible default)
  project  also emit projects[] (per-project behavior, keyed by cwd basename)
  session  also emit sessions_detail[] (per-session behavior)

Output (JSON to --output or stdout), field names aligned with
internal/codexreport JSON tags where they overlap:

  generated_for, timezone, sessions, tokens{input,cached_input,output,total},
  cache_hit_rate,
  tools{shell_calls, web_searches, file_changes, total_calls, top_commands[],
        by_name[], categories{}},
  repos[]{repo, branches, sessions, tokens, tool_calls},
  hours[]{hour, tokens, count},
  git_habits{command_count, top_subcommands[]},
  project_management{build_test_commands[], file_change_types[]},
  languages[]{name, files},
  sources{entrypoints[], permission_modes[], subagent_calls, subagent_share},
  prompt_signals{prompts, avg_len, structured_ratio, file_ref_ratio,
                 constraint_ratio, correction_rate}

PRIVACY (hard rules):
  - never emit prompt text, thinking text, tool_result content, or file contents
  - user prompt text is read transiently to derive NUMERIC signals only; it is
    never stored, never emitted, and assistant replies are never read
  - Bash command -> first token only (or git subcommand); never full command line
  - repo -> basename of cwd (or git repo name); never absolute path text
  - file_path -> extension only; never the path
"""
import argparse
import datetime
import json
import os
import re
from collections import Counter, defaultdict
from pathlib import Path


# --- git subcommands we recognise (privacy-safe, fixed vocabulary) ---
GIT_SUBCMDS = {
    "add", "commit", "push", "pull", "fetch", "diff", "status", "log",
    "checkout", "branch", "merge", "rebase", "stash", "show", "reset",
    "clone", "switch", "restore", "tag", "cherry-pick", "revert",
    "rev-parse", "remote", "init", "blame",
}
# git subcommands surfaced as "git_habits" focus (task spec list)
GIT_HABIT_FOCUS = {"add", "commit", "push", "diff", "status", "log",
                   "checkout", "branch"}

# build/test/CI tokens (first word of a Bash command)
BUILD_TEST_FIRST = {
    "go", "npm", "pnpm", "yarn", "npx", "pytest", "make", "cargo", "gradle",
    "mvn", "cmake", "xcodebuild", "swift", "tox", "jest", "vitest", "ruff",
    "mypy", "eslint", "tsc", "bun", "deno", "dotnet", "rake", "bundle",
    "ctest", "ninja", "gcc", "clang", "phpunit", "rspec",
}

# tool categorisation buckets
SHELL_TOOLS = {"Bash"}
WEB_TOOLS = {"WebFetch", "WebSearch"}
FILE_TOOLS = {"Edit", "Write", "Read", "NotebookEdit"}
SEARCH_TOOLS = {"Glob", "Grep", "ToolSearch"}

# --- prompt-quality signal vocabularies (privacy-safe: matched, never stored) ---
CONSTRAINT_WORDS_EN = ("must", "should", "don't", "do not", "only", "never",
                       "ensure", "require", "avoid", "without", "acceptance")
CONSTRAINT_WORDS_ZH = ("必须", "应该", "不要", "不能", "只", "确保", "需要",
                       "避免", "禁止", "验收", "务必")
CORRECTION_STARTS_EN = ("actually", "instead", "wait", "no,", "no ", "not ",
                        "sorry", "oops", "rather")
CORRECTION_STARTS_ZH = ("不对", "重来", "不是", "错了", "应该是", "改成",
                        "其实", "等等", "算了")
SECRET_RE = re.compile(
    r"(sk-[A-Za-z0-9]{6,}|ghp_[A-Za-z0-9]{6,}|AKIA[0-9A-Z]{10,}"
    r"|xox[baprs]-[A-Za-z0-9-]{6,})")
LIST_RE = re.compile(r"(^|\n)\s*([-*•]|\d+[.)])\s+")
FILE_REF_RE = re.compile(r"@[\w./\-]+")

# file extension -> language label
EXT_LANG = {
    "py": "Python", "go": "Go", "js": "JavaScript", "jsx": "JavaScript",
    "ts": "TypeScript", "tsx": "TypeScript", "rs": "Rust", "java": "Java",
    "kt": "Kotlin", "swift": "Swift", "c": "C", "h": "C/C++ Header",
    "cc": "C++", "cpp": "C++", "cxx": "C++", "hpp": "C++ Header",
    "rb": "Ruby", "php": "PHP", "cs": "C#", "sh": "Shell", "bash": "Shell",
    "zsh": "Shell", "fish": "Shell", "html": "HTML", "htm": "HTML",
    "css": "CSS", "scss": "CSS", "less": "CSS", "md": "Markdown",
    "json": "JSON", "yaml": "YAML", "yml": "YAML", "toml": "TOML",
    "xml": "XML", "sql": "SQL", "vue": "Vue", "svelte": "Svelte",
    "dart": "Dart", "scala": "Scala", "lua": "Lua", "r": "R",
    "ipynb": "Jupyter", "txt": "Text", "cfg": "Config", "ini": "Config",
    "env": "Config", "gradle": "Gradle", "proto": "Protobuf",
}


def parse_window(args, tz):
    """Return (start_local_date, end_local_date, label) or (None, None, label)."""
    today = datetime.datetime.now(tz).date()
    if args.date:
        d = datetime.date.fromisoformat(args.date)
        return d, d, args.date
    if args.since:
        d = datetime.date.fromisoformat(args.since)
        return d, today, f"{args.since} 至 {today.isoformat()}"
    if args.days and args.days > 0:
        start = today - datetime.timedelta(days=args.days - 1)
        return start, today, f"{start.isoformat()} 至 {today.isoformat()}"
    return None, None, "全部历史"


def parse_ts(ts):
    """ISO UTC like 2026-06-01T14:37:03.474Z -> aware datetime, or None."""
    if not isinstance(ts, str) or not ts:
        return None
    s = ts.replace("Z", "+00:00")
    try:
        dt = datetime.datetime.fromisoformat(s)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=datetime.timezone.utc)
    return dt


def first_token(cmd):
    """First word of a shell command, safely (privacy: never full command)."""
    if not isinstance(cmd, str):
        return ""
    cmd = cmd.strip()
    # strip leading env assignments like FOO=bar cmd
    for part in cmd.split():
        if "=" in part and not part.startswith(("-", "/", ".")):
            # could be VAR=val prefix; skip it
            name = part.split("=", 1)[0]
            if name and name.replace("_", "").isalnum() and name.upper() == name:
                continue
        # strip path prefix, keep basename of the executable
        tok = part.rsplit("/", 1)[-1]
        return tok
    return ""


def git_subcommand(cmd):
    """Extract the git subcommand from a Bash command, privacy-safe."""
    if not isinstance(cmd, str):
        return None
    toks = cmd.strip().split()
    seen_git = False
    for t in toks:
        base = t.rsplit("/", 1)[-1]
        if not seen_git:
            if base == "git":
                seen_git = True
            continue
        # first non-flag token after git
        if t.startswith("-"):
            continue
        sub = base.lower()
        if sub in GIT_SUBCMDS:
            return sub
        return None  # unknown subcommand: don't leak it
    return None


def ext_of(file_path):
    """Extension (lowercase, no dot) of a path. Privacy: only the extension."""
    if not isinstance(file_path, str) or not file_path:
        return ""
    base = file_path.rsplit("/", 1)[-1]
    if "." not in base:
        return ""
    return base.rsplit(".", 1)[-1].lower()


def repo_name(cwd):
    """Basename of cwd; privacy-safe repo label."""
    if not isinstance(cwd, str) or not cwd:
        return "(unknown)"
    name = cwd.rstrip("/").rsplit("/", 1)[-1]
    return name or "(unknown)"


def user_text(msg):
    """Concatenate user-authored text from a Claude Code user message.

    Read transiently to derive numeric signals; NEVER stored or emitted. Only
    `type == "text"` blocks (or a plain string) count as human-typed; tool_result
    / image / other blocks are ignored, so this never reads tool output.
    """
    content = msg.get("content")
    if isinstance(content, str):
        return content
    if not isinstance(content, list):
        return ""
    parts = []
    for c in content:
        if isinstance(c, str):
            parts.append(c)
        elif isinstance(c, dict) and c.get("type") == "text":
            t = c.get("text")
            if isinstance(t, str):
                parts.append(t)
    return "\n".join(parts)


def new_prompt_acc():
    return dict(count=0, len_sum=0, structured=0, file_ref=0,
                constraint=0, correction=0)


def prompt_acc_update(acc, text):
    """Update prompt-quality counters from one user message. `text` is local
    and transient: we derive booleans/length only and never keep it."""
    if not isinstance(text, str):
        return
    text = text.strip()
    if not text:
        return
    # ignore obvious command-output / system-injected pastes with secrets only
    acc["count"] += 1
    acc["len_sum"] += len(text)
    low = text.lower()
    if "```" in text or LIST_RE.search(text):
        acc["structured"] += 1
    if FILE_REF_RE.search(text):
        acc["file_ref"] += 1
    if any(w in low for w in CONSTRAINT_WORDS_EN) or \
       any(w in text for w in CONSTRAINT_WORDS_ZH):
        acc["constraint"] += 1
    if low.startswith(CORRECTION_STARTS_EN) or \
       any(text.startswith(w) for w in CORRECTION_STARTS_ZH):
        acc["correction"] += 1


def prompt_signals(acc):
    n = acc["count"]
    if not n:
        return dict(prompts=0, avg_len=0, structured_ratio=0.0,
                    file_ref_ratio=0.0, constraint_ratio=0.0,
                    correction_rate=0.0)
    return dict(
        prompts=n,
        avg_len=round(acc["len_sum"] / n, 1),
        structured_ratio=round(acc["structured"] / n, 4),
        file_ref_ratio=round(acc["file_ref"] / n, 4),
        constraint_ratio=round(acc["constraint"] / n, 4),
        correction_rate=round(acc["correction"] / n, 4),
    )


def new_group():
    return dict(sessions=set(), tokens=0, cache_read=0, input=0,
                tool_calls=0, cat=Counter(), git=Counter(),
                prompt=new_prompt_acc(), first=None, last=None, repo=None)


def group_touch_ts(g, dt):
    if dt is None:
        return
    if g["first"] is None or dt < g["first"]:
        g["first"] = dt
    if g["last"] is None or dt > g["last"]:
        g["last"] = dt


def group_emit(key, g, is_session):
    denom = g["cache_read"] + g["input"]
    chr_ = round(g["cache_read"] / denom, 4) if denom else 0.0
    dur = 0
    if g["first"] and g["last"]:
        dur = int((g["last"] - g["first"]).total_seconds())
    out = dict(
        tokens=g["tokens"],
        tool_calls=g["tool_calls"],
        cache_hit_rate=chr_,
        categories=dict(g["cat"]),
        git_top=[dict(command=k, count=v) for k, v in g["git"].most_common(6)],
        prompt_signals=prompt_signals(g["prompt"]),
    )
    if is_session:
        out["session_id"] = key
        out["repo"] = g["repo"] or "(unknown)"
        out["duration_seconds"] = dur
    else:
        out["repo"] = key
        out["sessions"] = len(g["sessions"])
    return out


def collect(projects_dir, start, end, tz, scope="global"):
    sessions = set()
    tok = dict(input=0, cached_input=0, output=0, total=0)
    cache_read_total = 0

    tool_by_name = Counter()
    shell_calls = web_searches = file_changes = total_calls = 0
    bash_first = Counter()
    git_sub = Counter()
    build_test = Counter()
    cat = Counter()  # shell / web / file / search / other

    repos = defaultdict(lambda: dict(sessions=set(), tool_calls=0, tokens=0,
                                     branches=set()))
    hours = defaultdict(lambda: dict(tokens=0, count=0))
    file_ext = Counter()
    file_change_types = Counter()  # Edit/Write vs Read kinds by ext

    entrypoints = Counter()
    permission_modes = Counter()
    subagent_calls = 0
    record_count = 0

    prompt_glob = new_prompt_acc()
    groups = defaultdict(new_group)  # keyed by sid or repo when scope != global

    files = sorted(Path(projects_dir).rglob("*.jsonl"))
    for fp in files:
        try:
            fh = fp.open("r", encoding="utf-8", errors="replace")
        except OSError:
            continue
        with fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    r = json.loads(line)
                except ValueError:
                    continue
                rtype = r.get("type")
                # window filter on timestamp -> local date
                dt = parse_ts(r.get("timestamp"))
                if dt is not None and (start is not None):
                    ld = dt.astimezone(tz).date()
                    if ld < start or ld > end:
                        continue
                elif dt is None and start is not None:
                    # records without timestamp (mode/ai-title/etc) only
                    # contribute when in full-history mode
                    continue

                sid = r.get("sessionId")
                cwd = r.get("cwd")
                branch = r.get("gitBranch")
                ep = r.get("entrypoint")
                pm = r.get("permissionMode") or (
                    r.get("permissionMode") if rtype == "permission-mode" else None)
                if rtype == "permission-mode":
                    pm = r.get("permissionMode")

                if sid:
                    sessions.add(sid)
                if cwd:
                    rp = repos[repo_name(cwd)]
                    if sid:
                        rp["sessions"].add(sid)
                    if branch:
                        rp["branches"].add(branch)
                if ep:
                    entrypoints[ep] += 1
                if pm:
                    permission_modes[pm] += 1
                if r.get("isSidechain") or r.get("agentId"):
                    subagent_calls += 1

                record_count += 1

                # --- scope grouping key ---
                gkey = None
                if scope == "session" and sid:
                    gkey = sid
                elif scope == "project" and cwd:
                    gkey = repo_name(cwd)
                g = groups[gkey] if gkey is not None else None
                if g is not None:
                    group_touch_ts(g, dt)
                    if sid:
                        g["sessions"].add(sid)
                    if g["repo"] is None and cwd:
                        g["repo"] = repo_name(cwd)

                # --- user prompts: derive numeric signals only (never store) ---
                if rtype == "user":
                    txt = user_text(r.get("message") or {})
                    prompt_acc_update(prompt_glob, txt)
                    if g is not None:
                        prompt_acc_update(g["prompt"], txt)
                    continue

                if rtype != "assistant":
                    continue

                msg = r.get("message") or {}
                usage = msg.get("usage") or {}
                in_t = int(usage.get("input_tokens", 0) or 0)
                out_t = int(usage.get("output_tokens", 0) or 0)
                cr = int(usage.get("cache_read_input_tokens", 0) or 0)
                cc = int(usage.get("cache_creation_input_tokens", 0) or 0)
                msg_total = in_t + out_t + cr + cc
                tok["input"] += in_t
                tok["output"] += out_t
                tok["cached_input"] += cr
                tok["total"] += msg_total
                cache_read_total += cr

                if cwd:
                    repos[repo_name(cwd)]["tokens"] += msg_total
                if g is not None:
                    g["tokens"] += msg_total
                    g["cache_read"] += cr
                    g["input"] += in_t

                if dt is not None:
                    h = dt.astimezone(tz).hour
                    hours[h]["tokens"] += msg_total
                    hours[h]["count"] += 1

                for c in (msg.get("content") or []):
                    if not isinstance(c, dict):
                        continue
                    if c.get("type") != "tool_use":
                        continue
                    name = c.get("name") or "unknown"
                    tool_by_name[name] += 1
                    total_calls += 1
                    if cwd:
                        repos[repo_name(cwd)]["tool_calls"] += 1
                    if g is not None:
                        g["tool_calls"] += 1

                    inp = c.get("input") or {}
                    if name in SHELL_TOOLS:
                        shell_calls += 1
                        cat["shell"] += 1
                        if g is not None:
                            g["cat"]["shell"] += 1
                        ft = first_token(inp.get("command", ""))
                        if ft:
                            bash_first[ft] += 1
                            if ft == "git":
                                gs = git_subcommand(inp.get("command", ""))
                                if gs:
                                    git_sub[gs] += 1
                                    if g is not None:
                                        g["git"][gs] += 1
                            elif ft in BUILD_TEST_FIRST:
                                build_test[ft] += 1
                    elif name in WEB_TOOLS:
                        web_searches += 1
                        cat["web"] += 1
                        if g is not None:
                            g["cat"]["web"] += 1
                    elif name in FILE_TOOLS:
                        file_changes += 1
                        cat["file"] += 1
                        if g is not None:
                            g["cat"]["file"] += 1
                        ext = ext_of(inp.get("file_path", ""))
                        if ext:
                            file_ext[ext] += 1
                            kind = "write" if name in ("Edit", "Write",
                                                       "NotebookEdit") else "read"
                            file_change_types[f"{kind}:{ext}"] += 1
                    elif name in SEARCH_TOOLS:
                        cat["search"] += 1
                        if g is not None:
                            g["cat"]["search"] += 1
                    elif name.startswith("mcp__"):
                        cat["mcp"] += 1
                        if g is not None:
                            g["cat"]["mcp"] += 1
                    else:
                        cat["other"] += 1
                        if g is not None:
                            g["cat"]["other"] += 1

    # ---- assemble repos list (codexreport RepoReport shape subset) ----
    repos_out = []
    for name, d in repos.items():
        repos_out.append(dict(
            repo=name,
            branches=sorted(d["branches"]),
            sessions=len(d["sessions"]),
            tokens=d["tokens"],
            tool_calls=d["tool_calls"],
        ))
    repos_out.sort(key=lambda x: (-x["tokens"], -x["tool_calls"]))

    hours_out = [dict(hour=h, tokens=hours[h]["tokens"], count=hours[h]["count"])
                 for h in range(24) if h in hours]
    hours_out.sort(key=lambda x: x["hour"])

    # languages: aggregate ext -> language label, count files (file ops)
    lang_files = Counter()
    for ext, n in file_ext.items():
        lang_files[EXT_LANG.get(ext, ext.upper())] += n
    languages_out = [dict(name=k, files=v)
                     for k, v in lang_files.most_common()]

    denom = cache_read_total + tok["input"]
    chr_ = (cache_read_total / denom) if denom else 0.0

    result = dict(
        sessions=len(sessions),
        records=record_count,
        tokens=tok,
        cache_hit_rate=round(chr_, 4),
        tools=dict(
            shell_calls=shell_calls,
            web_searches=web_searches,
            file_changes=file_changes,
            total_calls=total_calls,
            top_commands=[dict(command=k, count=v)
                          for k, v in bash_first.most_common(10)],
            by_name=[dict(name=k, count=v)
                     for k, v in tool_by_name.most_common(15)],
            categories=dict(cat),
        ),
        repos=repos_out,
        hours=hours_out,
        git_habits=dict(
            command_count=int(bash_first.get("git", 0)),
            top_subcommands=[dict(command=k, count=v)
                             for k, v in git_sub.most_common(12)
                             if k in GIT_HABIT_FOCUS or True],
        ),
        project_management=dict(
            build_test_commands=[dict(command=k, count=v)
                                 for k, v in build_test.most_common(12)],
            file_change_types=[dict(type=k, count=v)
                               for k, v in file_change_types.most_common(12)],
        ),
        languages=languages_out,
        sources=dict(
            entrypoints=[dict(name=k, count=v)
                         for k, v in entrypoints.most_common()],
            permission_modes=[dict(name=k, count=v)
                              for k, v in permission_modes.most_common()],
            subagent_calls=subagent_calls,
            subagent_share=round(subagent_calls / record_count, 4)
            if record_count else 0.0,
        ),
        prompt_signals=prompt_signals(prompt_glob),
    )

    if scope == "project":
        projects = [group_emit(k, g, is_session=False)
                    for k, g in groups.items()]
        projects.sort(key=lambda x: -x["tokens"])
        result["scope"] = "project"
        result["projects"] = projects
    elif scope == "session":
        detail = [group_emit(k, g, is_session=True)
                  for k, g in groups.items()]
        detail.sort(key=lambda x: -x["tokens"])
        result["scope"] = "session"
        result["sessions_detail"] = detail
    else:
        result["scope"] = "global"

    return result


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--projects-dir",
                    default=os.path.expanduser("~/.claude/projects"))
    ap.add_argument("--since", default="")
    ap.add_argument("--days", type=int, default=0)
    ap.add_argument("--date", default="")
    ap.add_argument("--scope", choices=["global", "project", "session"],
                    default="global")
    ap.add_argument("--output", default="")
    a = ap.parse_args()

    tz = datetime.datetime.now().astimezone().tzinfo
    tzname = datetime.datetime.now(tz).strftime("%Z") or "local"
    start, end, label = parse_window(a, tz)

    result = collect(a.projects_dir, start, end, tz, scope=a.scope)
    result["platform"] = "Claude Code"
    result["generated_for"] = label
    result["timezone"] = tzname
    result["source"] = "~/.claude/projects/**/*.jsonl (本地解析)"

    out = json.dumps(result, indent=2, ensure_ascii=False)
    if a.output:
        Path(a.output).write_text(out, encoding="utf-8")
        print(f"wrote {a.output}")
        print(f"  sessions={result['sessions']} "
              f"tool_calls={result['tools']['total_calls']} "
              f"repos={len(result['repos'])} "
              f"prompts={result['prompt_signals']['prompts']} "
              f"scope={a.scope} window={label}")
    else:
        print(out)


if __name__ == "__main__":
    main()
