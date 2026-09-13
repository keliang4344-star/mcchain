// Re-export for convenience.
export { bech32 } from 'bech32';
export const Bech32 = {
  encode: (hrp: string, words: number[]) => bech32.encode(hrp, words),
  decode: (addr: string) => bech32.decode(addr),
};