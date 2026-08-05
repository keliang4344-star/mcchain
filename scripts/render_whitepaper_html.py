#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Render WHITEPAPER_CN.md into docs/whitepaper.html.

The previous HTML carrier was produced by a converter that emitted table rows
out of order (every <tr> was flushed at the end of the document), which broke
the whole appendix.  This renderer keeps the existing visual shell (the <head>
style block and the sidebar chrome) and regenerates both the table of contents
and the body from the canonical Chinese whitepaper, so the HTML can never drift
from the Markdown again.

Usage:  python scripts/render_whitepaper_html.py
"""

import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "WHITEPAPER_CN.md")
DST = os.path.join(ROOT, "docs", "whitepaper.html")

# Everything above this marker in the current HTML is the reusable shell.
SHELL_END = '  <div class="sidebar-header">\n    <a href="index.html">&larr; 返回首页</a>\n  </div>\n'

RAW_PASSTHROUGH = ('<div align="center">', "</div>")


def esc(text):
    return text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def inline(text):
    """Inline markdown -> HTML.  Code spans are protected from further passes."""
    out = esc(text)
    spans = []

    def stash(match):
        spans.append(match.group(1))
        return "\x00%d\x00" % (len(spans) - 1)

    out = re.sub(r"`([^`]+)`", stash, out)
    out = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", out)
    out = re.sub(r"(?<![\*\w])\*([^*\n]+)\*(?!\*)", r"<em>\1</em>", out)
    out = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", r'<a href="\2">\1</a>', out)
    out = re.sub(r"\x00(\d+)\x00", lambda m: "<code>%s</code>" % spans[int(m.group(1))], out)
    return out


def clean_title(text):
    """Display text of a heading: drop editing markers, keep the prose."""
    return re.sub(r"（补全）\s*$", "", text).strip()


def slug(text):
    """GitHub-compatible anchor, matching the anchors already published."""
    s = re.sub(r"\*\*(.+?)\*\*", r"\1", text)
    s = re.sub(r"`([^`]+)`", r"\1", s)
    s = s.lower().strip()
    s = re.sub(r"[\s\u3000]+", "-", s)
    s = re.sub(r"[^\w\u4e00-\u9fff\-]", "", s, flags=re.UNICODE)
    return s


def render(md_lines):
    body = []
    toc = []
    i = 0
    n = len(md_lines)

    def flush_paragraph(buf):
        if buf:
            body.append("<p>%s</p>\n" % inline(" ".join(buf).strip()))
            del buf[:]

    para = []

    while i < n:
        line = md_lines[i].rstrip("\n")
        stripped = line.strip()

        # blank line -> paragraph break
        if not stripped:
            flush_paragraph(para)
            i += 1
            continue

        # raw html passthrough (the centred cover block)
        if stripped in RAW_PASSTHROUGH:
            flush_paragraph(para)
            body.append(stripped + "\n\n")
            i += 1
            continue

        # horizontal rule
        if re.fullmatch(r"-{3,}", stripped):
            flush_paragraph(para)
            body.append("<hr>\n")
            i += 1
            continue

        # fenced code block
        if stripped.startswith("```"):
            flush_paragraph(para)
            i += 1
            code = []
            while i < n and not md_lines[i].strip().startswith("```"):
                code.append(md_lines[i].rstrip("\n"))
                i += 1
            i += 1  # closing fence
            body.append("<pre><code>%s\n</code></pre>\n\n" % esc("\n".join(code)))
            continue

        # heading
        m = re.match(r"^(#{1,4})\s+(.*)$", stripped)
        if m:
            flush_paragraph(para)
            level = len(m.group(1))
            title = clean_title(m.group(2))
            anchor = slug(title)
            body.append('<h%d id="%s">%s</h%d>\n\n' % (level, anchor, inline(title), level))
            toc.append((level, anchor, title))
            i += 1
            continue

        # table
        if stripped.startswith("|"):
            flush_paragraph(para)
            rows = []
            while i < n and md_lines[i].strip().startswith("|"):
                rows.append(md_lines[i].strip())
                i += 1

            def cells(row):
                parts = row.strip().strip("|").split("|")
                return [c.strip() for c in parts]

            header = cells(rows[0])
            data = rows[2:] if len(rows) > 1 and re.fullmatch(r"[\s|:\-]+", rows[1]) else rows[1:]
            body.append('<div class="table-wrap"><table>\n')
            body.append("<tr>%s</tr>\n" % "".join("<th>%s</th>" % inline(c) for c in header))
            for row in data:
                body.append("<tr>%s</tr>\n" % "".join("<td>%s</td>" % inline(c) for c in cells(row)))
            body.append("</table></div>\n\n")
            continue

        # blockquote
        if stripped.startswith(">"):
            flush_paragraph(para)
            quote = []
            while i < n and md_lines[i].strip().startswith(">"):
                quote.append(re.sub(r"^>\s?", "", md_lines[i].strip()))
                i += 1
            body.append("<blockquote>\n")
            chunk = []
            for q in quote:
                if q.strip():
                    chunk.append(q.strip())
                elif chunk:
                    body.append("<p>%s</p>\n" % inline(" ".join(chunk)))
                    chunk = []
            if chunk:
                body.append("<p>%s</p>\n" % inline(" ".join(chunk)))
            body.append("</blockquote>\n\n")
            continue

        # unordered list
        if re.match(r"^[-*]\s+", stripped):
            flush_paragraph(para)
            body.append("<ul>\n")
            while i < n and re.match(r"^[-*]\s+", md_lines[i].strip()):
                item = re.sub(r"^[-*]\s+", "", md_lines[i].strip())
                body.append("<li>%s</li>\n" % inline(item))
                i += 1
            body.append("</ul>\n\n")
            continue

        # ordered list
        if re.match(r"^\d+\.\s+", stripped):
            flush_paragraph(para)
            body.append("<ol>\n")
            while i < n and re.match(r"^\d+\.\s+", md_lines[i].strip()):
                item = re.sub(r"^\d+\.\s+", "", md_lines[i].strip())
                body.append("<li>%s</li>\n" % inline(item))
                i += 1
            body.append("</ol>\n\n")
            continue

        para.append(stripped)
        i += 1

    flush_paragraph(para)
    return "".join(body), toc


def render_toc(toc):
    out = []
    pad = {1: 0, 2: 16, 3: 32, 4: 48}
    for level, anchor, title in toc:
        out.append(
            '<a class="toc-h%d" href="#%s" style="padding-left:%dpx">%s</a>\n'
            % (level, anchor, pad.get(level, 48), title)
        )
    return "".join(out)


def main():
    with open(SRC, "r", encoding="utf-8") as fh:
        md_lines = fh.readlines()
    with open(DST, "r", encoding="utf-8") as fh:
        current = fh.read()

    idx = current.find(SHELL_END)
    if idx < 0:
        sys.exit("cannot locate sidebar shell in %s" % DST)
    shell = current[: idx + len(SHELL_END)]

    body, toc = render(md_lines)
    html = shell + render_toc(toc) + "</nav>\n<main class=\"main\">\n" + body + "\n</main>\n</body>\n</html>\n"

    with open(DST, "w", encoding="utf-8", newline="\n") as fh:
        fh.write(html)

    print("rendered %s  (%d headings, %d bytes)" % (DST, len(toc), len(html)))


if __name__ == "__main__":
    main()
