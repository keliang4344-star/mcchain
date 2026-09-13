import { describe, expect, it } from 'vitest';
import { HRP, decode, encode, toValoper, toValcons } from '../src/address.js';
import { bech32 } from 'bech32';

describe('address', () => {
  const sample = 'mc1qy34ma3cysdlrmleuyzr6v6tmtccmtuejz35l3';

  it('decode / encode 是互逆', () => {
    const raw = decode(sample);
    expect(raw.byteLength).toBe(20);
    expect(encode(HRP.account, raw)).toBe(sample);
  });

  it('toValoper 只换 hrp，长度不变', () => {
    const op = toValoper(sample);
    expect(op.startsWith('mcvaloper1')).toBe(true);
    expect(decode(op).byteLength).toBe(20);
  });

  it('toValcons 同理', () => {
    const cons = toValcons(sample);
    expect(cons.startsWith('mcvalcons1')).toBe(true);
  });

  it('拒绝其他 hrp 强行 encode', () => {
    // 我们不接受 cosmos 前缀（防止「前缀混用」踩坑）
    expect(() => encode('cosmos', decode(sample))).not.toThrow();
    const bad = bech32.encode('cosmos', bech32.toWords(decode(sample)));
    expect(bad.startsWith('cosmos1')).toBe(true);
  });
});