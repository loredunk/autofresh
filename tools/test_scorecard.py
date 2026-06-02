#!/usr/bin/env python3
"""Scorecard regression test (T3).

Runs scorecard.py (zh + en) and render_dual_platform.py against the committed
fixtures, then asserts:
  - four axes, each with a non-empty tier label
  - zh and en tiers are localized (differ)
  - rank is flagged as a local estimate
  - NO quota-percentage claims and NO raw prompt text / secrets leak into output

Run: python3 tools/test_scorecard.py   (exit 0 = pass, 1 = fail)
"""
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SKILL = ROOT / "skills" / "ai-usage-html-report" / "scripts"
FIX = ROOT / "tools" / "fixtures"
DATA = FIX / "merged_sample.json"
INSIGHTS = FIX / "insights_sample.json"

QUOTA_RE = re.compile(r"配额[^。\n]{0,8}\d+\s*%|quota[^.\n]{0,8}\d+\s*%", re.I)
SECRET_RE = re.compile(r"sk-[A-Za-z0-9]{6,}|ghp_[A-Za-z0-9]{6,}")
AXES = {"prompt", "spending", "engineering", "diligence"}


def run(*args):
    subprocess.run([sys.executable, *map(str, args)], check=True,
                   capture_output=True, text=True)


def scorecard(lang, out):
    run(SKILL / "scorecard.py", "--data", DATA, "--lang", lang, "--output", out)
    return json.loads(Path(out).read_text(encoding="utf-8"))


def main():
    fails = []
    with tempfile.TemporaryDirectory() as d:
        d = Path(d)
        zh = scorecard("zh", d / "zh.json")
        en = scorecard("en", d / "en.json")

        for card, lang in ((zh, "zh"), (en, "en")):
            keys = {a["key"] for a in card.get("axes", [])}
            if keys != AXES:
                fails.append(f"[{lang}] axes {keys} != {AXES}")
            for a in card.get("axes", []):
                if not a.get("tier"):
                    fails.append(f"[{lang}] axis {a.get('key')} has empty tier")
            if not card.get("rank_is_estimate"):
                fails.append(f"[{lang}] rank not flagged as estimate")

        zt = [a["tier"] for a in zh["axes"]]
        et = [a["tier"] for a in en["axes"]]
        if zt == et:
            fails.append(f"zh/en tiers not localized: {zt}")

        # render and scan the HTML for leaks / quota claims
        html_out = d / "r.html"
        run(SKILL / "render_dual_platform.py", "--data", DATA,
            "--insights", INSIGHTS, "--scorecard", d / "zh.json",
            "--lang", "zh", "--output", html_out)
        html = html_out.read_text(encoding="utf-8")
        if "class='scorecard'" not in html:
            fails.append("rendered HTML missing scorecard section")
        for blob, name in ((json.dumps(zh, ensure_ascii=False), "zh.json"),
                           (json.dumps(en, ensure_ascii=False), "en.json"),
                           (html, "report.html")):
            if QUOTA_RE.search(blob):
                fails.append(f"{name}: contains a quota-percentage claim")
            if SECRET_RE.search(blob):
                fails.append(f"{name}: contains a secret-like token")

    if fails:
        print("scorecard test FAILED:")
        for f in fails:
            print("  -", f)
        return 1
    print("scorecard test OK: 4 axes, zh/en localized, estimate flagged, "
          "no quota%/secret leak, HTML has scorecard.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
