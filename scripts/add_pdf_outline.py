# -*- coding: utf-8 -*-
"""为白皮书 PDF 添加目录书签（outline）。

用法：python scripts/add_pdf_outline.py <pdf> <source_markdown>

从正典 Markdown 取标题层级，逐条在 PDF 页面文本里定位首次出现的页面，
按 h1 > h2 > h3 建三级书签。定位不到的条目自动跳过并计入报告。
"""
import os
import re
import sys

from pypdf import PdfReader, PdfWriter

MAX_TITLE = 60


def norm(text):
    return re.sub(r"\s+", "", text)


def md_headings(path):
    """返回 [(level, title)]，跳过代码块内的 # 行。"""
    out = []
    in_code = False
    for line in open(path, encoding="utf-8"):
        s = line.rstrip("\n")
        if s.strip().startswith("```"):
            in_code = not in_code
            continue
        if in_code:
            continue
        m = re.match(r"^(#{1,4})\s+(.+?)\s*$", s)
        if m:
            title = re.sub(r"（补全）\s*$", "", m.group(2)).strip()
            out.append((len(m.group(1)), title))
    return out


def main(pdf_path, md_path):
    reader = PdfReader(pdf_path)
    pages = [norm(p.extract_text() or "") for p in reader.pages]

    headings = md_headings(md_path)

    # 逐条定位：优先长匹配，避免「第一章」误配到「第十一章」
    located = []
    cursor = 0
    for level, title in headings:
        key = norm(title)
        # 逐步放宽：整标题 -> 前 12 字 -> 前 8 字
        cands = [key]
        if len(key) > 12:
            cands.append(key[:12])
        if len(key) > 8:
            cands.append(key[:8])
        found = None
        for c in cands:
            for i in range(cursor, len(pages)):
                if c and c in pages[i]:
                    found = i
                    break
            if found is not None:
                break
        if found is None:
            # 兜底：全局搜（不限制游标，容忍目录/交叉引用造成的乱序）
            for c in cands:
                for i, ptxt in enumerate(pages):
                    if c and c in ptxt:
                        found = i
                        break
                if found is not None:
                    break
        if found is not None:
            located.append((level, title, found))
            cursor = found

    writer = PdfWriter()
    writer.append(reader)

    stack = {}
    added = 0
    for level, title, page in located:
        parent = stack.get(level - 1)
        label = title if len(title) <= MAX_TITLE else title[: MAX_TITLE - 1] + "…"
        try:
            item = writer.add_outline_item(label, page, parent=parent)
            added += 1
        except Exception as exc:  # noqa: BLE001
            print("  跳过 %r: %s" % (title, exc))
            continue
        stack[level] = item
        for deeper in (level + 1, level + 2, level + 3, level + 4):
            stack.pop(deeper, None)

    with open(pdf_path, "wb") as fh:
        writer.write(fh)

    print("书签：定位 %d / 标题总数 %d，写入 %d 条" % (len(located), len(headings), added))
    have = {t for _, t, _ in located}
    miss = [t for _, t in headings if t not in have]
    if miss:
        print("未定位（无书签）：")
        for t in miss[:15]:
            print("   -", t)
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit("用法: python scripts/add_pdf_outline.py <pdf> <source_markdown>")
    sys.exit(main(sys.argv[1], sys.argv[2]))
