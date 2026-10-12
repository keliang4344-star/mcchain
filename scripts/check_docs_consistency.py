"""Docs consistency gate. Run: python scripts/check_docs_consistency.py

Fails (exit 1) when any known-stale string or version drift appears in docs/README.
Also regenerates docs/generated/events.md and compares it to the committed copy.
"""
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# (path, must_contain[]) -> hard fail if absent
MUST_HAVE = [
    ("README.md", ["keliang4344-star", "ibc-go v7.3.0", "55%", "15%", "12%", "13%", "5%"]),
    ("README_EN.md", ["keliang4344-star", "ibc-go v7.3.0", "55%", "15%", "12%", "13%", "5%"]),
    ("docs/tokenomics.md", ["5500", "1500", "1200", "1300", "500"]),
    ("DEVELOPMENT.md", ["v0.47.14", "gogoproto@v1.7.0", "cosmos-proto@v1.0.0-beta.5"]),
    ("docs/sdk_event_contract.md", ["v0.47.14", "v0.37.6"]),
    ("docs/mobile_sdk_integration.md", ["v0.47.14", "v0.37.6"]),
    ("docs/VERSIONS.md", ["v7.3.0", "v0.47.14"]),
]

# stale strings that must NOT appear anywhere in docs/README (unless commented with DOC-EXCEPT)
STALE = [
    "v0.47.3", "v0.37.1", "ibc-go/v7 v7.1", "IBC v7.1.0",
    "0.0001umc", "/mcchain/phonenode/v1/", "/mcchain/depin/v1/", "/mcchain/tokenomics/v1/",
    "mcchain.phonenode.v1.Query", "押金 30k MC", "投票期 2 weeks", "ignite scaffold",
    "github.com/mcchain/mcchain",
]


def main():
    errors = []
    # 1) must-have
    for rel, needles in MUST_HAVE:
        path = os.path.join(ROOT, *rel.split("/"))
        if not os.path.exists(path):
            errors.append(f"MISSING FILE: {rel}")
            continue
        with open(path, encoding="utf-8", errors="ignore") as f:
            s = f.read()
        for n in needles:
            if n not in s:
                errors.append(f"{rel}: missing '{n}'")
    # 2) stale
    for root, _dirs, files in os.walk(ROOT):
        if "docs/generated" in root.replace("\\", "/"):
            continue
        for fn in files:
            if not fn.endswith(".md"):
                continue
            path = os.path.join(root, fn)
            with open(path, encoding="utf-8", errors="ignore") as f:
                s = f.read()
            for st in STALE:
                # allow explicit exceptions like "已从 x 升级"
                if st in s:
                    errors.append(f"{os.path.relpath(path, ROOT)}: stale '{st}'")
    # 3) regenerated events doc drift
    gen = os.path.join(ROOT, "scripts", "gen_events_doc.py")
    committed = os.path.join(ROOT, "docs", "generated", "events.md")
    if os.path.exists(gen):
        subprocess.run([sys.executable, gen], check=True)
        if not os.path.exists(committed):
            errors.append("docs/generated/events.md not committed")
        else:
            status = subprocess.run(["git", "diff", "--quiet", "--", "docs/generated/events.md"],
                                    cwd=ROOT, capture_output=True)
            if status.returncode != 0:
                errors.append("docs/generated/events.md drifted from code (run gen_events_doc.py)")

    if errors:
        print("DOCS CONSISTENCY FAILED:")
        for e in errors:
            print("  -", e)
        sys.exit(1)
    print("DOCS CONSISTENCY OK")


if __name__ == "__main__":
    main()
