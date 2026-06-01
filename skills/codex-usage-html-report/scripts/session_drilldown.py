#!/usr/bin/env python3
import argparse
import json
import os
import re
import sqlite3
from collections import Counter
from datetime import datetime, timedelta, timezone
from pathlib import Path


ZERO_TOKENS = {
    "input": 0,
    "cached_input": 0,
    "output": 0,
    "reasoning_output": 0,
    "total": 0,
}


def main():
    parser = argparse.ArgumentParser(
        description="List high-token Codex sessions, optionally extracting user prompts for one selected session."
    )
    parser.add_argument("--codex-home", default=os.environ.get("CODEX_HOME") or str(Path.home() / ".codex"))
    parser.add_argument("--date", help="Local date YYYY-MM-DD")
    parser.add_argument("--since", help="Local start date YYYY-MM-DD through today")
    parser.add_argument("--days", type=int, default=1, help="Last N local days including today")
    parser.add_argument("--repo", help="Repo name/path substring to filter")
    parser.add_argument("--session-id", help="Exact or substring session id filter")
    parser.add_argument("--rollout", help="Specific rollout JSONL path")
    parser.add_argument("--top", type=int, default=20)
    parser.add_argument("--include-user-prompts", action="store_true")
    parser.add_argument("--prompt-char-limit", type=int, default=1200)
    args = parser.parse_args()

    if args.include_user_prompts and not (args.session_id or args.rollout):
        parser.error("--include-user-prompts requires --session-id or --rollout")

    loc = datetime.now().astimezone().tzinfo
    start, end, label = resolve_window(args, loc)
    threads = discover_threads(Path(args.codex_home), args.rollout)

    sessions = []
    for thread in threads:
        if args.session_id and args.session_id not in (thread.get("id") or ""):
            continue
        repo = repo_key(thread)
        if args.repo and args.repo.lower() not in " ".join(
            [repo, thread.get("cwd") or "", thread.get("git_origin_url") or ""]
        ).lower():
            continue
        parsed = parse_rollout(thread, start, end, loc, args.include_user_prompts, args.prompt_char_limit)
        if not parsed:
            continue
        sessions.append(parsed)

    sessions.sort(key=lambda s: (s["tokens"]["total"], s["tools"]["total_calls"]), reverse=True)
    if args.top > 0:
        sessions = sessions[: args.top]

    output = {
        "generated_for": label,
        "codex_home": str(Path(args.codex_home).expanduser()),
        "privacy": {
            "includes_user_prompts": bool(args.include_user_prompts),
            "includes_system_prompts": False,
            "prompt_scope": "one selected session" if args.include_user_prompts else "none",
        },
        "sessions": sessions,
    }
    print(json.dumps(output, ensure_ascii=False, indent=2))


def resolve_window(args, loc):
    today = datetime.now(loc).replace(hour=0, minute=0, second=0, microsecond=0)
    if args.date:
        start = datetime.strptime(args.date, "%Y-%m-%d").replace(tzinfo=loc)
        return start, start + timedelta(days=1), args.date
    if args.since:
        start = datetime.strptime(args.since, "%Y-%m-%d").replace(tzinfo=loc)
        return start, today + timedelta(days=1), f"{args.since} to {today.date().isoformat()}"
    days = max(1, args.days)
    start = today - timedelta(days=days - 1)
    return start, today + timedelta(days=1), f"last {days} days"


def discover_threads(codex_home, rollout):
    if rollout:
        return [{"rollout_path": str(Path(rollout).expanduser())}]

    threads = read_state_threads(codex_home)
    if not threads:
        threads = [{"rollout_path": str(p)} for p in sorted((codex_home / "sessions").glob("**/rollout-*.jsonl"))]

    seen = set()
    out = []
    for thread in threads:
        path = thread.get("rollout_path")
        if not path or path in seen:
            continue
        if "subagent" in (thread.get("source") or "") or "thread_spawn" in (thread.get("source") or ""):
            continue
        seen.add(path)
        out.append(thread)
    return out


def read_state_threads(codex_home):
    dbs = sorted(codex_home.glob("state_*.sqlite"), key=lambda p: p.stat().st_mtime, reverse=True)
    for db_path in dbs:
        conn = None
        try:
            conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
            conn.row_factory = sqlite3.Row
            cols = [r["name"] for r in conn.execute("pragma table_info(threads)")]
            wanted = [
                "id",
                "rollout_path",
                "created_at",
                "source",
                "git_branch",
                "git_origin_url",
                "cwd",
                "model",
            ]
            select = ", ".join(c for c in wanted if c in cols)
            if not select or "rollout_path" not in cols:
                continue
            rows = conn.execute(f"select {select} from threads").fetchall()
            return [dict(row) for row in rows]
        except Exception:
            continue
        finally:
            if conn is not None:
                conn.close()
    return []


def parse_rollout(thread, start, end, loc, include_prompts, prompt_limit):
    path = Path(thread.get("rollout_path") or "").expanduser()
    try:
        fh = path.open("r", encoding="utf-8")
    except OSError:
        return None

    session_id = thread.get("id") or ""
    cwd = thread.get("cwd") or ""
    source = thread.get("source") or ""
    branch = thread.get("git_branch") or ""
    model = thread.get("model") or ""
    origin = thread.get("git_origin_url") or ""
    tokens = dict(ZERO_TOKENS)
    tools = {"shell_calls": 0, "web_searches": 0, "file_changes": 0, "total_calls": 0}
    commands = Counter()
    prompts = []
    first_seen = None
    last_seen = None
    baseline = None

    with fh:
        for raw in fh:
            try:
                line = json.loads(raw)
            except Exception:
                continue
            ts = parse_ts(line.get("timestamp"), loc)
            payload = line.get("payload") or {}
            if not isinstance(payload, dict):
                payload = {}
            line_type = line.get("type") or ""

            if line_type == "session_meta":
                session_id = session_id or payload.get("id") or ""
                cwd = cwd or payload.get("cwd") or ""
                source = source or payload.get("source") or ""
                if "subagent" in json.dumps(payload) or "thread_spawn" in json.dumps(payload):
                    return None
            elif line_type == "turn_context":
                model = payload.get("model") or model

            if line_type == "event_msg":
                event_type = payload.get("type")
                if event_type == "token_count" and payload.get("info"):
                    cur = token_usage(payload["info"].get("total_token_usage") or {})
                    if baseline is None:
                        baseline = cur
                        continue
                    delta = token_delta(cur, baseline)
                    baseline = cur
                    if delta["total"] < 0 or not in_window(ts, start, end):
                        continue
                    add_tokens(tokens, delta)
                    first_seen, last_seen = update_span(first_seen, last_seen, ts)
                elif event_type == "patch_apply_end" and in_window(ts, start, end):
                    tools["file_changes"] += len(payload.get("changes") or {})
                    first_seen, last_seen = update_span(first_seen, last_seen, ts)

            if line_type == "response_item" and in_window(ts, start, end):
                item_type = payload.get("type")
                if item_type in ("function_call", "local_shell_call", "custom_tool_call", "web_search_call"):
                    tools["total_calls"] += 1
                    first_seen, last_seen = update_span(first_seen, last_seen, ts)
                if item_type == "function_call":
                    name = payload.get("name")
                    if name in ("exec_command", "local_shell_call", "shell"):
                        tools["shell_calls"] += 1
                        cmd = command_from_args(payload.get("arguments") or "")
                        if cmd:
                            commands[cmd.split()[0]] += 1
                    elif name == "web_search_call":
                        tools["web_searches"] += 1
                elif item_type == "web_search_call":
                    tools["web_searches"] += 1

            if include_prompts and in_window(ts, start, end):
                text = extract_user_prompt(line_type, payload)
                if text:
                    prompts.append({"timestamp": ts.isoformat(), "text": redact(text)[:prompt_limit]})

    if tokens["total"] == 0 and tools["total_calls"] == 0 and not prompts:
        return None

    result = {
        "session_id": session_id,
        "repo": repo_key({"cwd": cwd, "git_origin_url": origin}),
        "cwd": cwd,
        "source": source_key(source),
        "branch": branch,
        "model": model,
        "rollout_path": str(path),
        "first_seen": first_seen.isoformat() if first_seen else "",
        "last_seen": last_seen.isoformat() if last_seen else "",
        "duration_seconds": int((last_seen - first_seen).total_seconds()) if first_seen and last_seen else 0,
        "tokens": tokens,
        "tools": tools,
        "top_commands": [{"command": c, "count": n} for c, n in commands.most_common(8)],
        "prompt_count": len(prompts),
    }
    if include_prompts:
        result["user_prompts"] = prompts
    return result


def parse_ts(value, loc):
    if not value:
        return datetime.min.replace(tzinfo=timezone.utc).astimezone(loc)
    value = value.replace("Z", "+00:00")
    try:
        return datetime.fromisoformat(value).astimezone(loc)
    except ValueError:
        return datetime.min.replace(tzinfo=timezone.utc).astimezone(loc)


def in_window(ts, start, end):
    return start <= ts < end


def update_span(first, last, ts):
    if first is None or ts < first:
        first = ts
    if last is None or ts > last:
        last = ts
    return first, last


def token_usage(raw):
    return {
        "input": int(raw.get("input_tokens") or 0),
        "cached_input": int(raw.get("cached_input_tokens") or 0),
        "output": int(raw.get("output_tokens") or 0),
        "reasoning_output": int(raw.get("reasoning_output_tokens") or 0),
        "total": int(raw.get("total_tokens") or 0),
    }


def token_delta(cur, prev):
    return {k: cur[k] - prev[k] for k in ZERO_TOKENS}


def add_tokens(total, delta):
    for key in ZERO_TOKENS:
        total[key] += delta[key]


def command_from_args(raw):
    try:
        args = json.loads(raw)
    except Exception:
        return ""
    if isinstance(args, dict):
        if isinstance(args.get("cmd"), str):
            return " ".join(args["cmd"].split())
        if isinstance(args.get("command"), list):
            return " ".join(str(x) for x in args["command"])
    return ""


def extract_user_prompt(line_type, payload):
    role = payload.get("role")
    if isinstance(payload.get("author"), dict):
        role = role or payload["author"].get("role")
    if role == "user":
        return collect_text(payload.get("content") or payload.get("text") or payload.get("message") or payload)
    if line_type in ("user_message", "user_prompt", "input_message"):
        return collect_text(payload.get("content") or payload.get("text") or payload.get("message") or payload)
    if line_type == "response_item" and payload.get("type") == "message" and payload.get("role") == "user":
        return collect_text(payload.get("content") or payload)
    return ""


def collect_text(value):
    if isinstance(value, str):
        return " ".join(value.split())
    if isinstance(value, list):
        return " ".join(filter(None, (collect_text(v) for v in value)))
    if isinstance(value, dict):
        parts = []
        for key in ("text", "input_text", "content", "message"):
            if key in value:
                parts.append(collect_text(value[key]))
        return " ".join(filter(None, parts))
    return ""


def redact(text):
    text = re.sub(r"sk-[A-Za-z0-9_-]{12,}", "sk-REDACTED", text)
    text = re.sub(r"(?i)(api[_-]?key|token|secret|password|authorization)\s*[:=]\s*\S+", r"\1=REDACTED", text)
    return text


def repo_key(thread):
    origin = (thread.get("git_origin_url") or "").rstrip("/").removesuffix(".git")
    if origin:
        return re.split(r"[/ :]", origin)[-1] or "(unknown)"
    cwd = thread.get("cwd") or ""
    return Path(cwd).name if cwd else "(unknown)"


def source_key(source):
    s = (source or "").lower()
    if "vscode" in s or "ide" in s:
        return "plugin"
    if "codex-app" in s or "desktop" in s or s == "app":
        return "codex-app"
    if "cli" in s or "terminal" in s:
        return "cli"
    return source or "(unknown)"


if __name__ == "__main__":
    main()
