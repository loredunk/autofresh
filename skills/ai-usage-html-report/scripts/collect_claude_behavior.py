#!/usr/bin/env python3
"""Collect a Claude Code *behavior* profile from local ~/.claude/projects/**/*.jsonl
transcripts, mirroring the autofresh/codexreport JSON field shapes so the dual
renderer can show both platforms symmetrically.

Pure python3 standard library, zero pip, offline, no network.

Window filtering (by record timestamp, converted to local tz):
  --since YYYY-MM-DD   from this local day through today (inclusive)
  --days  N            last N local days including today
  --date  YYYY-MM-DD   a single local day
  (default: full history)

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
  sources{entrypoints[], permission_modes[], subagent_calls, subagent_share}

PRIVACY (hard rules):
  - never emit prompt text, thinking text, tool_result content, or file contents
  - Bash command -> first token only (or git subcommand); never full command line
  - repo -> basename of cwd (or git repo name); never absolute path text
  - file_path -> extension only; never the path
"""
import argparse
import datetime
import json
import os
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


def collect(projects_dir, start, end, tz):
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

                    inp = c.get("input") or {}
                    if name in SHELL_TOOLS:
                        shell_calls += 1
                        cat["shell"] += 1
                        ft = first_token(inp.get("command", ""))
                        if ft:
                            bash_first[ft] += 1
                            if ft == "git":
                                gs = git_subcommand(inp.get("command", ""))
                                if gs:
                                    git_sub[gs] += 1
                            elif ft in BUILD_TEST_FIRST:
                                build_test[ft] += 1
                    elif name in WEB_TOOLS:
                        web_searches += 1
                        cat["web"] += 1
                    elif name in FILE_TOOLS:
                        file_changes += 1
                        cat["file"] += 1
                        ext = ext_of(inp.get("file_path", ""))
                        if ext:
                            file_ext[ext] += 1
                            kind = "write" if name in ("Edit", "Write",
                                                       "NotebookEdit") else "read"
                            file_change_types[f"{kind}:{ext}"] += 1
                    elif name in SEARCH_TOOLS:
                        cat["search"] += 1
                    elif name.startswith("mcp__"):
                        cat["mcp"] += 1
                    else:
                        cat["other"] += 1

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

    return dict(
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
    )


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--projects-dir",
                    default=os.path.expanduser("~/.claude/projects"))
    ap.add_argument("--since", default="")
    ap.add_argument("--days", type=int, default=0)
    ap.add_argument("--date", default="")
    ap.add_argument("--output", default="")
    a = ap.parse_args()

    tz = datetime.datetime.now().astimezone().tzinfo
    tzname = datetime.datetime.now(tz).strftime("%Z") or "local"
    start, end, label = parse_window(a, tz)

    result = collect(a.projects_dir, start, end, tz)
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
              f"window={label}")
    else:
        print(out)


if __name__ == "__main__":
    main()
