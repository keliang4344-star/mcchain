
# MC Chain CosmJS 类型与工具

## 文件说明
- `mc-types.d.ts` - TypeScript 类型定义（编辑器智能提示）
- `mc-tx-builder.js` - 交易构建辅助函数

## 使用方式

### 在 HTML 中
```html
<script src="cosmjs-bundle.js"></script>
<script src="mc-tx-builder.js"></script>
<script>
// 创建 swap 消息
const msg = MC.dex.swapExactIn(address, 1, 'umc', '1000000', 'usdc', '950000');
// 签名广播
const fee = { amount: [], gas: '200000' };
const result = await client.signAndBroadcast(address, [msg], fee, 'MC Swap');
</script>
```

### 与 Miner / Dashboard 配合
mc-tx-builder.js 已被 miner.html 和 web/index.html 加载使用，
可以直接通过 `window.MC` 调用。
