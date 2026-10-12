"""Generate docs/generated/events.md from actual source code.

Scans x/*/keeper and x/*/types for sdk.NewEvent("...") literals and
types/events.go constants (EventType* = "..."), so the doc can never drift
from code. Run:  python scripts/gen_events_doc.py
CI: scripts/check_docs_consistency.py verifies the committed doc matches.
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, "docs", "generated", "events.md")


def collect():
    events = {}
    xdir = os.path.join(ROOT, "x")
    for root, _dirs, files in os.walk(xdir):
        for fn in files:
            if not fn.endswith(".go"):
                continue
            path = os.path.join(root, fn)
            with open(path, encoding="utf-8", errors="ignore") as f:
                s = f.read()
            for m in re.finditer(r'NewEvent\(\s*\n?\s*"([^"]+)"', s):
                events.setdefault(m.group(1).split(".")[0], []).append(m.group(1))
    # x/liquidstaking/types/events.go constants
    evfile = os.path.join(xdir, "liquidstaking", "types", "events.go")
    if os.path.exists(evfile):
        with open(evfile, encoding="utf-8") as f:
            s = f.read()
        for m in re.finditer(r'EventType\w+\s*=\s*"([^"]+)"', s):
            events.setdefault("liquidstaking", []).append(m.group(1))
    for k in events:
        events[k] = sorted(set(events[k]))
    return events


def main():
    events = collect()
    total = sum(len(v) for v in events.values())
    os.makedirs(os.path.dirname(OUT), exist_ok=True)
    with open(OUT, "w", encoding="utf-8", newline="") as f:
        f.write("# MobileChain 链上事件完整清单（由脚本生成，禁止手改）\n\n")
        f.write(f"> 生成命令：`python scripts/gen_events_doc.py`。本文件由 CI 门禁校验，与代码漂移会直接失败。\n\n")
        f.write(f"共 **{total} 类事件**，按模块聚合：\n\n")
        f.write("| 模块 | 事件 |\n|---|---|\n")
        for mod in sorted(events):
            for ev in events[mod]:
                f.write(f"| {mod} | `{ev}` |\n")
    print(f"generated {OUT}: {total} events")


if __name__ == "__main__":
    main()
