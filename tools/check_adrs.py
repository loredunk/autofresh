#!/usr/bin/env python3
"""Docs lint (T3): ADR numbering/status sanity + relative-link resolution.

Run: python3 tools/check_adrs.py   (exit 0 = ok, 1 = problems found)
"""
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCS = ROOT / "docs"
ADR = DOCS / "adr"
LINK_RE = re.compile(r"\[[^\]]+\]\(([^)]+)\)")
STATUS_RE = re.compile(r"状态[:：]")


def check_adrs(errors):
    seen = {}
    for fp in sorted(ADR.glob("*.md")):
        m = re.match(r"(\d{4})-.*\.md$", fp.name)
        if not m:
            errors.append(f"ADR filename not NNNN-*.md: {fp.name}")
            continue
        num = m.group(1)
        if num in seen:
            errors.append(f"duplicate ADR number {num}: {fp.name} & {seen[num]}")
        seen[num] = fp.name
        text = fp.read_text(encoding="utf-8")
        if not STATUS_RE.search(text):
            errors.append(f"{fp.name}: missing 状态 field")
    if not seen:
        errors.append("no ADR files found")


def check_links(errors):
    for fp in sorted(DOCS.rglob("*.md")):
        for target in LINK_RE.findall(fp.read_text(encoding="utf-8")):
            t = target.strip()
            if t.startswith(("http://", "https://", "#", "mailto:")):
                continue
            t = t.split("#", 1)[0].split("?", 1)[0]
            if not t:
                continue
            if not (fp.parent / t).resolve().exists():
                errors.append(f"{fp.relative_to(ROOT)}: broken link -> {target}")


def main():
    errors = []
    check_adrs(errors)
    check_links(errors)
    if errors:
        print("docs lint FAILED:")
        for e in errors:
            print("  -", e)
        return 1
    print("docs lint OK: ADR numbering/status valid, all relative links resolve.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
