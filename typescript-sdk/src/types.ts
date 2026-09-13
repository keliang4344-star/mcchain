// MC 公链 SDK 公共类型。

export interface Denom {
  denom: string;
}

/** 1 MC = 1e6 umc；umc 是最小单位。 */
export interface Coin {
  denom: string;
  amount: string; // SDK 用 string 表示大整数；前端用 BigInt 处理
}

export interface MsgSend {
  fromAddress: string;
  toAddress: string;
  amount: Coin[];
}

export interface MsgSendResponse {
  txHash: string;
  height?: number;
  code?: number;
  rawLog?: string;
}