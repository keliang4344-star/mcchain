# 双机协作同步指南（MC 公链）

> 适用：开发团队机器（本机 $HOME/mcchain）与另一协作者机器，**同时**修改同一仓库。
> 目的：防止两台机器各改各的、互相覆盖、丢失工作。

---

## 一、为什么必须有共享 remote

2026-07-15 扫描发现：git 历史里只有 1 个 Ignite 脚手架提交（57 文件，仅含 `x/mcchain`）。
4 个业务模块（`depin`/`edgeai`/`phonenode`/`tokenomics`）+ oracle + monitoring + deploy + docs + web **全部在磁盘上但从未被 git 跟踪**。
两台机器同时改，随时可能互相覆盖或丢失。

已建立本地基线（**务必尽快 push 到共享 remote**）：
- `ea011fa` — 全量纳入 MC 源码与交付物（双机协作基线，294 文件）
- `c36a732` — 统一主网链 ID（mcchain-1 → mcchain-mainnet-1）+ 修正创世自抵押金额 + oracle/治理状态更新

---

## 二、建立共享 remote（需你执行，提供仓库 URL）

```bash
# 在 GitHub / GitLab / 腾讯工蜂 建一个空仓库，然后本机：
cd $HOME/mcchain
git remote add origin <你的仓库URL>
git branch -M master
git push -u origin master
```

另一台机器：
```bash
git clone <你的仓库URL>
cd mcchain
# 安装 Go 1.21+，按 DEVELOPMENT.md 配置工具链后：
go build ./...
```

---

## 三、日常协作纪律（两台机器都遵守）

1. **开工前先 `git pull --rebase`**，结束前 `git push`。
2. **小步提交**：每完成一个独立模块/文档就 commit，不要攒一大坨。
3. **改文件前先 `git status`**，确认自己没在改别人正在改的文件。
4. **冲突当场解决**，不要 `--ours` 强行覆盖（会丢对方工作）。

---

## 四、当前分工边界（避免两人改同一处）

| 范围 | 负责方 | 备注 |
|------|--------|------|
| `web/`（含 RPC 可配置化、钱包/浏览器前端） | 另一协作者 | 本轮 P1：web RPC 可配置化 + go.mod 依赖升级 |
| `go.mod` / `go.sum`（依赖升级） | 另一协作者 | 升级后需重新 `go build`/`go test` 验证 |
| `docs/keplr-chain-registry.json`、双机同步类文档 | 本机 | 不碰 web/ 与 go.mod |
| 仿真（simulation）、`cmd/event-subscriber`、历史模块修复 | 本机 | 与 web/go.mod 无交集 |

> 注意：除上述明确边界外，任何文件都可能被任一方触碰。**沟通优先**——决定改某个共享文件前，先在对话里说一声。

---

## 五、踩坑记录（已解决，供参考）

- **链 ID 不一致**：本机早期 runbook 误用 `mcchain-1`，真实跑起来的链是 `mcchain-mainnet-1`。
  已在 `docs/MAINNET_DEPLOY_RUNBOOK.md`、`docs/mobile_sdk_integration.md`、`scripts/mainnet-genesis-config.json` 统一为 `mcchain-mainnet-1`。
- **创世自抵押金额**：ante 装饰器强制最低自抵押 100k MC（1e11 umc）；早期 gentx 写 1000 MC 会被拒。
  已修正 runbook 的 `add-genesis-account`（200k MC）与 `gentx`（100k MC）。
- **go.mod 升级后**：cometbft / cosmos-sdk 若跨大版本（如 0.47→0.50），API 会变，仿真与 ante 可能需适配。升级方负责验证 `go build ./...` 与 `go test ./...` 全绿后再 push。
