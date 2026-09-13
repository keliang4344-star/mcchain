# MC 公链 TypeScript SDK（最小骨架）

> 这是一个**最小可用骨架**，目标是「5 分钟接入、转账 30 行」。
> 业务模块（x/depin / x/phonenode / x/edgeai）的 API 按需增量添加。

## 安装

```bash
pnpm add mcchain-sdk
# 或
npm install mcchain-sdk
```

## 30 行转账

```typescript
import { McClient } from 'mcchain-sdk';
import { DirectSecp256k1HdWallet } from '@cosmjs/proto-signing';

async function main() {
  // 1) 准备签名者（实际项目里用 Keplr / Ledger 替代）
  const signer = await DirectSecp256k1HdWallet.fromMnemonic(
    'your mnemonic here',
    { prefix: 'mc' },
  );

  // 2) 连链
  const client = await McClient.connect(
    'https://rpc.mcchain.io:443',
    signer,
  );

  // 3) 转账
  const sender = (await signer.getAccounts())[0].address;
  const { txHash } = await client.bank.send(
    sender,
    'mc1qy34ma3cysdlrmleuyzr6v6tmtccmtuejz35l3',
    [{ denom: 'umc', amount: '1000000' }], // 1 MC = 1,000,000 umc
  );
  console.log('tx:', txHash);

  // 4) 查询
  console.log('height:', await client.query.getHeight());
  console.log('balance:', await client.query.getBalance(sender));
}

main().catch(console.error);
```

## API 速查

| 入口 | 方法 | 说明 |
|---|---|---|
| `client.bank` | `send(from, to, amount, opts?)` | 转账，返回 tx hash |
| `client.query` | `getHeight()` | 当前区块高度 |
| `client.query` | `getBalance(addr, denom?)` | 余额（denom 默认 `umc`） |
| `address.toValoper(mcAddr)` | — | 用户地址 → 验证人操作地址 |
| `address.toValcons(mcAddr)` | — | 用户地址 → 共识公钥地址 |

## 开发

```bash
pnpm install
pnpm test        # 单元测试（地址转换）
pnpm test:e2e    # E2E：起 devnet，发一笔真实转账
pnpm build       # 输出 dist/
```

## 已知限制

- 仅 `x/bank` 模块的 `MsgSend` 与基础查询。其它模块见 `src/` 下逐步加入。
- `fee = "auto"` 时，`@cosmjs/stargate` 默认 gas adjustment = 1.5，与 `docs/DEVELOPER_GUIDE.md` 一致。
- **不在浏览器里直接放助记词**：上生产请走 Keplr / Ledger 注入的 signer。

## 配套文档

- `docs/DEVELOPER_GUIDE.md` — 工程速查
- `docs/sdk_event_contract.md` — 事件订阅契约
- `docs/BACKEND_API.md` — REST + gRPC 接口