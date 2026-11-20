package com.mcchain.miner.ui.ecosystem

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.network.DexPool
import com.mcchain.miner.network.DexPrice
import java.math.BigDecimal
import java.math.RoundingMode

/**
 * DEX 只读行情。
 *
 * 只展示池状态、价格与链上试算结果；不提供兑换、加池、撤池等任何签名入口。
 */
@Composable
fun DexScreen(viewModel: EcosystemViewModel, onBack: () -> Unit) {
    val pools by viewModel.dexPools.collectAsState()
    val prices by viewModel.dexPrices.collectAsState()
    val loading by viewModel.dexLoading.collectAsState()
    val error by viewModel.dexError.collectAsState()
    val quote by viewModel.dexQuote.collectAsState()
    val quoteBusy by viewModel.dexQuoteBusy.collectAsState()
    val quoteError by viewModel.dexQuoteError.collectAsState()

    LaunchedEffect(Unit) { viewModel.loadDex() }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            EcosystemHeader(
                title = "DEX 行情",
                subtitle = "只读池状态、价格与兑换试算",
                onBack = onBack,
                onRefresh = { viewModel.loadDex() },
                refreshing = loading,
            )
        }

        item {
            EcosystemInfoCard(
                title = "只读说明",
                lines = listOf(
                    "本页只查询链上池状态、价格与 estimate_swap 试算结果。",
                    "不提供兑换、添加/移除流动性等任何签名入口；相关交易需单独风控与审计评审。",
                    "试算结果与最终成交可能因区块状态变化而不同，不作为成交承诺。",
                ),
            )
        }

        if (error != null) {
            item { EcosystemInfoCard("读取失败", listOf(error.orEmpty())) }
        }
        if (quoteError != null) {
            item { EcosystemInfoCard("试算失败", listOf(quoteError.orEmpty())) }
        }

        if (loading && pools.isEmpty()) {
            item {
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                    CircularProgressIndicator(modifier = Modifier.size(22.dp), strokeWidth = 2.dp)
                }
            }
        }

        if (!loading && error == null && pools.isEmpty()) {
            item { EcosystemInfoCard("暂无交易池", listOf("链上还没有创建任何 DEX 池。")) }
        }

        items(pools, key = { it.id }) { pool ->
            DexPoolCard(
                pool = pool,
                price = prices[pool.id],
                quote = quote?.takeIf { it.poolId == pool.id }?.quote?.amountOutUmc,
                priceImpactBps = quote?.takeIf { it.poolId == pool.id }?.quote?.priceImpactBps,
                quoteBusy = quoteBusy,
                onQuote = { amount -> viewModel.estimateDexSwap(pool, amount) },
            )
        }

        item {
            Text(
                "数据来源：/mcchain/dex/v1/pools、/price、/estimate_swap；价格按 1e6 定点展示。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
            )
        }
    }
}

@Composable
private fun DexPoolCard(
    pool: DexPool,
    price: DexPrice?,
    quote: Long?,
    priceImpactBps: Int?,
    quoteBusy: Boolean,
    onQuote: (String) -> Unit,
) {
    var amount by remember(pool.id) { mutableStateOf("") }
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(14.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "池 #${pool.id}",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    "手续费 ${"%.2f".format(pool.feeRateBps / 100.0)}%",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                )
            }
            Text(
                "${denomLabel(pool.denomA)} / ${denomLabel(pool.denomB)}",
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
            )
            Text(
                "储备 ${formatAmount(pool.reserveA, pool.denomA)} · ${formatAmount(pool.reserveB, pool.denomB)}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
            Text(
                "LP 总量 ${formatPlain(pool.totalLp.toString())}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
            )
            price?.let {
                Text(
                    "价格 ${formatPrice(it.price)} ${denomLabel(it.priceDenom)} / ${denomLabel(pool.denomA)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f),
                )
            }
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                OutlinedTextField(
                    value = amount,
                    onValueChange = { amount = it },
                    label = { Text("输入 ${denomLabel(pool.denomA)} 数量") },
                    singleLine = true,
                    modifier = Modifier.weight(1f),
                )
                Button(
                    onClick = { onQuote(amount) },
                    enabled = !quoteBusy && amount.isNotBlank(),
                ) { Text(if (quoteBusy) "试算中…" else "试算") }
            }
            if (quote != null) {
                Text(
                    "预计得到 ${formatAmount(quote, pool.denomB)}" +
                        (priceImpactBps?.let { " · 价格影响 ${"%.2f".format(it / 100.0)}%" } ?: ""),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
            if (pool.owner.isNotBlank()) {
                Text(
                    "创建者 ${shortenAddress(pool.owner)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.45f),
                )
            }
        }
    }
}

private fun denomLabel(denom: String): String = when (denom) {
    "umc" -> "MC"
    "" -> "—"
    else -> denom
}

/** 最小单位金额转显示值（umc 按 1e6）。 */
private fun formatAmount(raw: Long, denom: String): String {
    val value = if (denom == "umc") formatMcAmount(raw) else formatPlain(raw.toString())
    return "$value ${denomLabel(denom)}"
}

private fun formatPlain(raw: String): String =
    raw.toBigIntegerOrNull()?.toString() ?: raw

/** 价格是乘 1e6 的定点整数，去掉多余 0。 */
private fun formatPrice(raw: String): String {
    val value = raw.toBigIntegerOrNull() ?: return raw
    return BigDecimal(value)
        .divide(BigDecimal(1_000_000), 6, RoundingMode.DOWN)
        .stripTrailingZeros()
        .toPlainString()
        .ifBlank { "0" }
}
