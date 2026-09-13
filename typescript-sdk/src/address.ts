// MC 公链地址常量与转换。
//
// bech32 体系：
//   mc            用户账户
//   mcvaloper     验证人操作地址
//   mcvalcons     验证人共识公钥
//
// 见 docs/PROTOCOL_PARAMS.md「地址体系」一节。

import { bech32 } from 'bech32';

export const HRP = {
  account: 'mc',
  valoper: 'mcvaloper',
  valcons: 'mcvalcons',
} as const;

/**
 * bech32 ↔ 原始 20-byte 地址。
 * 校验 HRP 与长度：不检查 hrp 与字节数时，
 * 任意 bech32 串（含 cosmos1xxx）都能被换前缀成 mc 地址静默发上链，
 * 手续费白烧、交易失败。
 */
export function decode(addr: string): Uint8Array {
  const { prefix, words } = bech32.decode(addr);
  if (prefix !== HRP.account) {
    throw new Error(`invalid address prefix: expected "${HRP.account}", got "${prefix}"`);
  }
  const raw = new Uint8Array(bech32.fromWords(words));
  if (raw.length !== 20) {
    throw new Error(`invalid address length: expected 20 bytes, got ${raw.length}`);
  }
  return raw;
}

/** 地址合法性检查（发送前调用，避免无效地址上链烧手续费）。 */
export function isValidAddress(addr: string): boolean {
  try {
    decode(addr);
    return true;
  } catch {
    return false;
  }
}

export function encode(hrp: string, raw: Uint8Array): string {
  return bech32.encode(hrp, bech32.toWords(raw));
}

/** mc 操作地址 ↔ mcvaloper 操作地址：只换 hrp，长度不变。 */
export function toValoper(addr: string): string {
  return bech32.encode(HRP.valoper, bech32.toWords(decode(addr)));
}

export function toValcons(addr: string): string {
  return bech32.encode(HRP.valcons, bech32.toWords(decode(addr)));
}

// 旧名（保留以免业务方依赖）
export const addressFrom = decode;
export const addressToValidator = toValoper;
export const addressToConsensus = toValcons;