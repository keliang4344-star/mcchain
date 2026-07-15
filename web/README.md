# Web 仪表盘（MC 公链）

单页静态仪表盘：`web/index.html` + `web/cosmjs-bundle.js`（已打包的 `@cosmjs/stargate` + `@cosmjs/proto-signing`）。

## 功能

- 链概览（Chain ID / 高度 / 供应量 / 验证人榜）
- 钱包（助记词解锁、余额、转账）
- 区块浏览器（按高度 / TxHash 查询）
- **自定义模块交互**：DePIN 设备贡献、PhoneNode 注册/attestation、EdgeAI 创建任务/提交结果

## RPC 配置

### 方式一：配置文件（推荐）

编辑 `web/config.json`，支持以下字段：

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `rpc` | string | `http://localhost:26657` | cometbft RPC 地址 |
| `autoConnect` | bool | `false` | 设为 `true` 时页面加载后自动连接 |
| `chainId` | string | `""` | 预留，暂未使用 |
| `nodeName` | string | `"MC 公链"` | 预留，暂未使用 |

### 方式二：页面交互

顶部输入框默认从 `config.json` 加载，也可手动输入任意节点地址。点击「保存配置」按钮可下载当前 RPC 地址为 `config.json` 文件，替换 `web/` 目录下的旧文件即可持久化。

LCD/REST 自动推导为同主机 `:1317`。

## 如何启用自定义模块交易签名

广播 `depin` / `phonenode` / `edgeai` 的自定义消息，要求 **cosmjs 注册表中包含对应 Msg 的 protobuf 生成类型**。
当前静态包 **未内置** MC 各模块的生成类型，页面会优雅提示「缺注册表」。

生成步骤（任选其一，产物即 `cosmjs.proto.mcchain.*` 命名空间）：

1. **Telescope / @osmonauts**（推荐，生成 TS 类型）
   ```bash
   npx @osmonauts/telescope --out ./src/codegen proto
   # 用 vite/tsc 打包出 cosmjs-bundle.js，替换 web/cosmjs-bundle.js
   ```
2. **ignite（旧 CLI）**
   ```bash
   ignite generate ts-client --output ./ts-client
   ```

生成后，`index.html` 中的 `buildRegistry()` 会从 `cosmjs.proto.mcchain` 自动注册以下类型：

| 模块 | TypeUrl |
|------|---------|
| edgeai | `/mcchain.edgeai.MsgCreateTask` · `/mcchain.edgeai.MsgSubmitResult` |
| depin | `/mcchain.depin.MsgSubmitContribution` |
| phonenode | `/mcchain.phonenode.MsgRegisterNode` · `/mcchain.phonenode.MsgSubmitAttestation` |

> 若仅做只读演示（链概览/钱包/浏览器），无需上述步骤，直接用静态 `index.html` 即可。

## 本地预览

```bash
cd web
python3 -m http.server 8080   # 或任意静态服务器
# 浏览器打开 http://localhost:8080，先「连接节点」再「解锁钱包」
```
