// MC 公链 SDK —— 最小可用导出。
//
// 设计目标：
//   - **5 分钟接入**：构造 McClient → 调 send → 完成；
//   - **与 @cosmjs 兼容**：底层用 @cosmjs/stargate 做签名 / 编码；
//   - **避免过度抽象**：只暴露业务真正关心的接口，proto 细节下沉到子模块。
//
// 当前仅实现 Bank 模块转账 + 区块/交易查询；x/depin / x/phonenode / x/edgeai
// 模块 API 在后续迭代中按 docs/sdk_event_contract.md 逐步加入。
//
// 详细用法：docs/DEVELOPER_GUIDE.md
export { McClient } from './client.js';
export { Bech32 } from './bech32.js';
export { addressFrom, addressToValidator, addressToConsensus } from './address.js';
export type { Denom, Coin, MsgSend, MsgSendResponse } from './types.js';

import { McClient } from './client.js';
/** 工厂：从 RPC endpoint 与 signer 构造客户端。 */
export async function connect(endpoint: string, signer: any): Promise<McClient> {
  return McClient.connect(endpoint, signer);
}