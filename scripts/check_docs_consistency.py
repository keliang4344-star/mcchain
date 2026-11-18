"""Docs consistency gate. Run: python scripts/check_docs_consistency.py

Fails (exit 1) when any known-stale string or version drift appears in docs/README.
Also regenerates docs/generated/events.md and compares it to the committed copy.
"""
import os
import re
import shutil
import subprocess
import sys
import tempfile

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
    # 已废弃的链标识 / 工具链 / 费用口径（回归防护）
    "--chain-id mcchain-1", "make proto-gen", "0.001umc",
]

LINK_RE = re.compile(r"(?<!!)\[[^\]]+\]\(([^)]+)\)")


def strip_code_fences(text):
    return re.sub(r"```.*?```", "", text, flags=re.DOTALL)


def check_links(errors):
    """校验 Markdown 中的相对链接目标存在（跳过 http/mailto/anchor）。"""
    for root, _dirs, files in os.walk(ROOT):
        norm = root.replace("\\", "/")
        if "/.git" in norm or "node_modules" in norm or "/build" in norm:
            continue
        for fn in files:
            if not fn.endswith(".md"):
                continue
            path = os.path.join(root, fn)
            with open(path, encoding="utf-8", errors="ignore") as f:
                text = strip_code_fences(f.read())
            for target in LINK_RE.findall(text):
                target = target.strip().split()[0] if target.strip() else ""
                if not target or target.startswith(("http://", "https://", "mailto:", "#")):
                    continue
                rel = target.split("#", 1)[0]
                if not rel:
                    continue
                dest = os.path.normpath(os.path.join(os.path.dirname(path), rel))
                if not os.path.exists(dest):
                    errors.append(f"{os.path.relpath(path, ROOT)}: broken link -> {target}")


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
    # 提交策略：docs/generated/ 不入库（CI 生成 + artifact 归档，见 ci.yml
    # docs-generated job）。本地若保留副本则必须与代码一致，否则报错——
    # 既保证「文档永不漂移」，又避免每次事件变更都要手工提交生成物。
    gen = os.path.join(ROOT, "scripts", "gen_events_doc.py")
    committed = os.path.join(ROOT, "docs", "generated", "events.md")
    if os.path.exists(gen):
        tmp = tempfile.mkdtemp(prefix="mcchain-docs-")
        gen_out = os.path.join(tmp, "events.md")
        try:
            subprocess.run([sys.executable, gen, "--out", gen_out], check=True)
            if not os.path.exists(gen_out) or os.path.getsize(gen_out) == 0:
                errors.append("docs/generated/events.md generation produced empty output")
            if os.path.exists(committed):
                with open(gen_out, encoding="utf-8") as f1, open(committed, encoding="utf-8") as f2:
                    if f1.read() != f2.read():
                        errors.append("docs/generated/events.md drifted from code (run gen_events_doc.py)")

            # 事件契约文档的总数/分模块数量必须与生成结果一致。
            with open(gen_out, encoding="utf-8") as f:
                generated = f.read()
            gen_rows = re.findall(r"^\| ([a-z]+) \| `", generated, re.MULTILINE)
            gen_total = len(gen_rows)
            per_module = {}
            for mod in gen_rows:
                per_module[mod] = per_module.get(mod, 0) + 1

            contract = os.path.join(ROOT, "docs", "sdk_event_contract.md")
            if os.path.exists(contract):
                with open(contract, encoding="utf-8") as f:
                    text = f.read()
                m = re.search(r"链上共 \*\*(\d+) 类事件\*\*", text)
                if not m:
                    errors.append("docs/sdk_event_contract.md: missing event total declaration")
                elif int(m.group(1)) != gen_total:
                    errors.append(
                        f"docs/sdk_event_contract.md: declares {m.group(1)} events, generator produced {gen_total}")
                for mod, count in re.findall(r"^\| ([a-z]+) \| (\d+) \|", text, re.MULTILINE):
                    if mod in per_module and int(count) != per_module[mod]:
                        errors.append(
                            f"docs/sdk_event_contract.md: module {mod} declares {count}, generator produced {per_module[mod]}")
        finally:
            shutil.rmtree(tmp, ignore_errors=True)

    # 4) 文档元数据扫描：禁止 AIGC/生成器水印元数据进入仓库。
    #    这类 front matter 与项目无关，且会在文档渲染、法务审查与对外发布时
    #    造成归属混淆；新增文档必须从模板里彻底去掉。
    for root, _dirs, files in os.walk(ROOT):
        norm = root.replace("\\", "/")
        if "/.git" in norm or "docs/generated" in norm:
            continue
        for fn in files:
            if not fn.endswith(".md"):
                continue
            path = os.path.join(root, fn)
            with open(path, encoding="utf-8", errors="ignore") as f:
                head = f.read(4096)
            if re.search(r"^\s*AIGC\s*:", head, re.MULTILINE) or "ContentProducer:" in head:
                errors.append(
                    f"{os.path.relpath(path, ROOT)}: contains AIGC metadata front matter (remove it)")

    # 5) 相对链接存在性（防止 README 指向未入库/已删除文件）。
    check_links(errors)

    if errors:
        print("DOCS CONSISTENCY FAILED:")
        for e in errors:
            print("  -", e)
        sys.exit(1)
    print("DOCS CONSISTENCY OK")


if __name__ == "__main__":
    main()
