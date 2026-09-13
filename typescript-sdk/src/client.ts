// McClient 是面向 dApp 的「一切从这开始」入口。
//
//   const client = await McClient.connect('https://rpc.mcchain.io:443', signer);
//   const { txHash } = await client.bank.send(
//     'mc1...', 'mc1...', [{ denom: 'umc', amount: '1000000' }],
//   );
//
// 设计取舍：
//   - 继承自 StargateClient（@cosmjs/stargate）以复用 broadcast / query 工具；
//   - 业务方法集中在 bank / staking / gov 子模块下，避免大 flat API；
//   - 不试图直接生成 proto 类型，而是把 proto 细节藏在 client.bank 内部。

import { StargateClient, SigningStargateClient } from '@cosmjs/stargate';

export class McClient {
  readonly endpoint: string;
  private readonly stargate: SigningStargateClient;
  private readonly query: StargateClient;

  private constructor(
    endpoint: string,
    stargate: SigningStargateClient,
    query: StargateClient,
  ) {
    this.endpoint = endpoint;
    this.stargate = stargate;
    this.query = query;
  }

  static async connect(endpoint: string, signer: any): Promise<McClient> {
    const stargate = await SigningStargateClient.connectWithSigner(endpoint, signer);
    const query = await StargateClient.connect(endpoint);
    return new McClient(endpoint, stargate, query);
  }

  get bank(): BankAPI {
    return new BankAPI(this.stargate);
  }
  get query(): QueryAPI {
    return new QueryAPI(this.query);
  }
}

// ---- 业务子模块 ----

import type { Coin, MsgSend, MsgSendResponse } from './types.js';

class BankAPI {
  constructor(private readonly c: SigningStargateClient) {}

  async send(
    from: string,
    to: string,
    amount: Coin[],
    opts: { memo?: string; gas?: string; fee?: string } = {},
  ): Promise<MsgSendResponse> {
    const msg = {
      typeUrl: '/cosmos.bank.v1beta1.MsgSend',
      value: { fromAddress: from, toAddress: to, amount },
    };
    const fee = opts.fee
      ? { amount: [{ denom: 'umc', amount: opts.fee }], gas: opts.gas ?? '200000' }
      : 'auto';
    const tx = await this.c.sendTokens(from, to, amount, fee, opts.memo ?? '');
    return { txHash: tx.transactionHash, height: tx.height, code: tx.code, rawLog: tx.rawLog };
  }
}

class QueryAPI {
  constructor(private readonly c: StargateClient) {}

  async getHeight(): Promise<number> {
    return this.c.getHeight();
  }

  async getBalance(addr: string, denom = 'umc'): Promise<Coin | null> {
    const coins = await this.c.getAllBalances(addr);
    return coins.find((c) => c.denom === denom) ?? null;
  }
}

// 类型导出（client.ts 也用）
export type { MsgSend, MsgSendResponse };