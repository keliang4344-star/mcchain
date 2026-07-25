---
AIGC:
    Label: "1"
    ContentProducer: 001191440300708461136T1XGW3
    ProduceID: c66a8379f631ade5ceee5cebcd48deca_ad5a3fa8827a11f180b3525400bff409
    ReservedCode1: E5uy8dzWFRpqjvaMi68wZlYu7keGTbZZQoHQBZ9b96OglpCejrxalOkQHUeHMDhfYzaYNrRylVp1wPFd6ZW/JfbDnKA/eDjQd79HG7eX7VHHt21ezjgb65ZUrhkazVcoNA1f0PKPYxiUhTo6admkM8p5LW0UNdQDdkASNFZ4jy87HiiNl0O87gYd8tg=
    ContentPropagator: 001191440300708461136T1XGW3
    PropagateID: c66a8379f631ade5ceee5cebcd48deca_ad5a3fa8827a11f180b3525400bff409
    ReservedCode2: E5uy8dzWFRpqjvaMi68wZlYu7keGTbZZQoHQBZ9b96OglpCejrxalOkQHUeHMDhfYzaYNrRylVp1wPFd6ZW/JfbDnKA/eDjQd79HG7eX7VHHt21ezjgb65ZUrhkazVcoNA1f0PKPYxiUhTo6admkM8p5LW0UNdQDdkASNFZ4jy87HiiNl0O87gYd8tg=
---

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
*（内容由AI生成，仅供参考）*
