# 白皮书归档

本目录保存 MobileChain 白皮书的历史版本，供回溯查阅。

**现行版本不在本目录内。** 对外发布与审计以仓库根目录的正典为准，`docs/` 下的派生件
（HTML、PDF）由正典渲染生成。

## 一、现行版本

| 文件 | 说明 |
|---|---|
| `../../WHITEPAPER_CN.md` | 中文正典。中文内容的唯一来源，也是英文版的翻译基准 |
| `../../WHITEPAPER.md` | 英文正典，与中文正典逐章逐节对应 |
| `../../docs/whitepaper.html` | 中文在线版，由中文正典渲染 |
| `../../docs/whitepaper_en.html` | 英文在线版，由英文正典渲染 |
| `../../docs/MobileChain白皮书_完整典藏版.pdf` | 中文印刷版，A4，含目录书签 |
| `../../docs/MobileChain-Whitepaper-Collector-Edition.pdf` | 英文印刷版，A4，含目录书签 |

渲染流水线见 `../../scripts/render_whitepaper_pdf.sh`。修订只在正典上进行，派生件一律由正典重新生成。

## 二、归档内容

| 文件 | 原位置 | 说明 |
|---|---|---|
| `v3.0/WHITEPAPER_CN_v3.0.md` | `WHITEPAPER_CN.md` | 中文旧版。五卷三十四章体系，含附录 A 参数表与附录 B 验证指引。主体论证与现行版重合，差异集中在章节归属与行文整理 |
| `v3.0/WHITEPAPER_CN_v3.0_mc-miner.md` | `mc-miner/WHITEPAPER_v3.md` | 中文旧版的另一支。与上者主体相同，差异集中在第二十三章（验证节点）的表述 |
| `v3.0/WHITEPAPER_en_v3.0_22sections.md` | `WHITEPAPER.md` | 英文旧版。二十二节技术体结构，与中文版章节体系不一致 |
| `v3.0/WHITEPAPER_en_v3.0_duplicate.md` | `docs/WHITEPAPER.md` | 与上者内容完全相同的副本，原为 `docs/` 下的镜像 |
| `v3.0/MC-WHITEPAPER_v3.0.pdf` | `docs/MC-WHITEPAPER.pdf` | 早期 PDF 印刷件。Letter 纸型、浅色主题，无目录书签 |

## 三、归档原因

1. 早期英文版以技术说明为体裁独立撰写，未与中文版保持逐章对应，形成两套叙述体系。
2. 仓库内曾同时存在多份中文白皮书副本与两处英文镜像，读者难以判断何者为权威。
3. 部分早期印刷件存在章节缺失。

## 四、回溯方式

```bash
less archive/whitepaper/v3.0/WHITEPAPER_CN_v3.0.md        # 中文旧版
less archive/whitepaper/v3.0/WHITEPAPER_en_v3.0_22sections.md   # 英文旧版
```

本目录内容仅作存档，不再更新。
