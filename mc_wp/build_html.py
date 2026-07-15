# -*- coding: utf-8 -*-
import re, markdown

SRC = r"$HOME/mcchain/docs/WHITEPAPER.md"
OUT = r"$HOME/mcchain/docs/WHITEPAPER.html"

with open(SRC, "r", encoding="utf-8") as f:
    md_text = f.read()

# Split off the leading centered title block (between first <div align="center"> ... </div>)
# We'll render the whole thing but build a cover + TOC ourselves.

md = markdown.Markdown(extensions=["tables", "fenced_code", "toc"], output_format="html5")
body_html = md.convert(md_text)

# Build TOC from h1 headings only for a clean chapter list
h1s = re.findall(r'<h1[^>]*>(.*?)</h1>', body_html, flags=re.S)
def strip_tags(s):
    return re.sub(r'<[^>]+>', '', s).strip()

# add ids to h1 for anchor linking (markdown toc adds ids already via 'toc' ext)
# Extract existing ids
h1_pairs = re.findall(r'<h1 id="([^"]+)">(.*?)</h1>', body_html, flags=re.S)
toc_items = "".join(
    f'<li><a href="#{hid}">{strip_tags(txt)}</a></li>' for hid, txt in h1_pairs
)

cover = """
<section class="cover">
  <div class="cover-badge">PUBLIC CHAIN WHITEPAPER</div>
  <h1 class="cover-title">MobileChain</h1>
  <div class="cover-sub">MC 白皮书 · 一条把全节点装进每一部手机的公链</div>
  <div class="cover-line"></div>
  <div class="cover-meta">
    <div><b>链标识</b> mcchain-mainnet-1</div>
    <div><b>主币</b> MC（最小单位 umc · 精度 6）</div>
    <div><b>总量</b> 固定 10 亿 · 零通胀</div>
    <div><b>共识</b> CometBFT v0.37.6 · Cosmos SDK v0.47.14</div>
    <div><b>版本</b> v3.0（叙事体）</div>
  </div>
  <div class="cover-foot">开源可审计 · 参数写代码 · 链上求真 · 共识共生</div>
</section>
<section class="toc-page">
  <h1 class="toc-h">目录</h1>
  <ol class="toc">__TOC__</ol>
</section>
""".replace("__TOC__", toc_items)

html = """<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>MobileChain（MC）白皮书 v3.0</title>
<style>
:root{
  --bg:#ffffff; --ink:#1f2328; --muted:#5a6169; --line:#e3e6ea;
  --brand:#0b5cff; --brand2:#0a3d91; --accent:#e8f0ff; --code:#f6f8fa;
}
*{box-sizing:border-box}
html,body{margin:0;padding:0;background:#f2f4f7;color:var(--ink);
  font-family:"Segoe UI","PingFang SC","Microsoft YaHei",system-ui,sans-serif;
  font-size:15px;line-height:1.9;-webkit-font-smoothing:antialiased}
.page{max-width:820px;margin:24px auto;background:var(--bg);
  padding:64px 72px;box-shadow:0 4px 24px rgba(0,0,0,.08);border-radius:6px}
/* Cover */
.cover{min-height:940px;display:flex;flex-direction:column;justify-content:center;
  text-align:center;padding:60px 40px;
  background:linear-gradient(160deg,#0a3d91 0%,#0b5cff 55%,#3b82f6 100%);
  color:#fff;border-radius:6px;margin:24px auto;max-width:820px}
.cover-badge{letter-spacing:4px;font-size:12px;opacity:.85;margin-bottom:28px}
.cover-title{font-size:64px;margin:0;font-weight:800;letter-spacing:1px}
.cover-sub{font-size:19px;margin-top:16px;opacity:.95;font-weight:500}
.cover-line{width:80px;height:4px;background:#fff;opacity:.9;margin:36px auto;border-radius:2px}
.cover-meta{display:inline-block;text-align:left;font-size:15px;line-height:2.1;opacity:.96}
.cover-meta b{display:inline-block;width:64px;opacity:.85;font-weight:600}
.cover-foot{margin-top:44px;font-size:14px;letter-spacing:2px;opacity:.9}
/* TOC */
.toc-page{max-width:820px;margin:24px auto;background:#fff;padding:56px 72px;
  box-shadow:0 4px 24px rgba(0,0,0,.08);border-radius:6px}
.toc-h{font-size:30px;color:var(--brand2);border-bottom:3px solid var(--brand);
  padding-bottom:12px;margin:0 0 24px}
ol.toc{list-style:none;counter-reset:t;padding:0;margin:0;
  columns:2;column-gap:48px}
ol.toc li{counter-increment:t;padding:7px 0;border-bottom:1px dashed var(--line);
  font-size:14.5px;break-inside:avoid}
ol.toc li::before{content:counter(t)".";color:var(--brand);font-weight:700;margin-right:8px}
ol.toc a{color:var(--ink);text-decoration:none}
ol.toc a:hover{color:var(--brand)}
/* Content */
.content h1{font-size:27px;color:var(--brand2);margin:0 0 22px;padding:18px 0 12px;
  border-bottom:3px solid var(--brand);page-break-before:always;font-weight:800}
.content h1:first-of-type{page-break-before:avoid}
.content h2{font-size:20px;color:#0d2a5c;margin:34px 0 12px;
  padding-left:12px;border-left:5px solid var(--brand);font-weight:700}
.content h3{font-size:16.5px;color:#173a6b;margin:24px 0 8px;font-weight:700}
.content p{margin:12px 0;text-align:justify}
.content blockquote{margin:16px 0;padding:14px 18px;background:var(--accent);
  border-left:4px solid var(--brand);border-radius:0 6px 6px 0;color:#0d2a5c}
.content blockquote p{margin:6px 0}
.content ul,.content ol{padding-left:24px;margin:12px 0}
.content li{margin:6px 0}
.content table{border-collapse:collapse;width:100%;margin:18px 0;font-size:13.5px;
  break-inside:avoid}
.content th{background:var(--brand2);color:#fff;padding:9px 12px;text-align:left;font-weight:600}
.content td{border:1px solid var(--line);padding:8px 12px;vertical-align:top}
.content tr:nth-child(even) td{background:#f7f9fc}
.content code{background:var(--code);padding:2px 6px;border-radius:4px;
  font-family:"Consolas","Courier New",monospace;font-size:13px;color:#b3204a}
.content pre{background:#0d1b2a;color:#e6edf3;padding:16px 20px;border-radius:8px;
  overflow-x:auto;margin:16px 0;line-height:1.6}
.content pre code{background:none;color:#e6edf3;padding:0}
.content hr{border:none;border-top:1px solid var(--line);margin:28px 0}
.content strong{color:#0d2a5c}
.content div[align="center"]{text-align:center}
@media print{
  html,body{background:#fff}
  .page,.toc-page{box-shadow:none;margin:0 auto;border-radius:0;max-width:none;padding:40px 54px}
  .cover{margin:0 auto;border-radius:0;max-width:none;page-break-after:always}
  .toc-page{page-break-after:always}
}
</style>
</head>
<body>
__COVER__
<div class="page content">
__BODY__
</div>
</body>
</html>
""".replace("__COVER__", cover).replace("__BODY__", body_html)

with open(OUT, "w", encoding="utf-8") as f:
    f.write(html)

print("OK ->", OUT)
print("h1 count:", len(h1_pairs))
print("html bytes:", len(html))
