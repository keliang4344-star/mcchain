#!/usr/bin/env python3
"""高信号 secret 扫描（无第三方依赖）。

用途：CI / 发布门禁在 gitleaks 不可用时仍能拦截最危险的凭据泄漏：
  - PEM 私钥块（RSA / EC / OPENSSH / PRIVATE KEY）
  - BIP39 助记词赋值（变量名含 MNEMONIC 且值为 20+ 个单词）
  - GitHub PAT（ghp_ / github_pat_）
  - AWS Access Key（AKIA...）
  - Slack / Google API key 常见前缀

退出码：0 = 未发现；1 = 命中（打印 path:line，绝不打印命中的明文）。
用法：python scripts/secret_scan.py [--root .]
"""
import argparse
import os
import re
import sys

PATTERNS = [
    re.compile(r"-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----"),
    re.compile(r"(?i)\b[A-Z0-9_]*(MNEMONIC|SEED_PHRASE)[A-Z0-9_]*\s*[:=]\s*['\"][a-z]+(\s+[a-z]+){11,}"),
    re.compile(r"\bghp_[A-Za-z0-9]{30,}\b"),
    re.compile(r"\bgithub_pat_[A-Za-z0-9_]{40,}\b"),
    re.compile(r"\bAKIA[0-9A-Z]{16}\b"),
    re.compile(r"\bxox[baprs]-[A-Za-z0-9-]{10,}\b"),
    re.compile(r"\bAIza[0-9A-Za-z_-]{35}\b"),
]

SKIP_DIRS = {".git", "build", "vendor", "node_modules", "__pycache__", "docs/generated"}
SKIP_EXTS = {".png", ".jpg", ".jpeg", ".gif", ".pdf", ".wasm", ".sum", ".zip", ".tar", ".gz"}
# 允许出现在模板/示例中的占位写法（避免把文档示例当泄漏）。
ALLOW_MARKERS = ("example", "placeholder", "your_", "<", "redacted", "xxxx")


def should_skip(path, root):
    rel = os.path.relpath(path, root).replace("\\", "/")
    parts = rel.split("/")
    for d in SKIP_DIRS:
        if d in parts:
            return True
    return os.path.splitext(path)[1].lower() in SKIP_EXTS


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", default=".")
    args = ap.parse_args()
    root = os.path.abspath(args.root)

    hits = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in filenames:
            path = os.path.join(dirpath, fn)
            if should_skip(path, root):
                continue
            try:
                with open(path, encoding="utf-8", errors="ignore") as f:
                    lines = f.readlines()
            except OSError:
                continue
            for i, line in enumerate(lines, 1):
                low = line.lower()
                if any(m in low for m in ALLOW_MARKERS):
                    continue
                if any(p.search(line) for p in PATTERNS):
                    # 只输出位置，不输出明文。
                    hits.append(f"{os.path.relpath(path, root)}:{i}")

    if hits:
        print("SECRET SCAN FAILED (potential credentials):")
        for h in hits[:50]:
            print("  -", h)
        return 1
    print("SECRET SCAN OK")
    return 0


if __name__ == "__main__":
    sys.exit(main())
