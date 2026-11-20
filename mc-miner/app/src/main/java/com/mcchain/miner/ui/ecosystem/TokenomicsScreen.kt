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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.network.TeamRelease
import com.mcchain.miner.network.TokenomicsPool
import com.mcchain.miner.network.TokenomicsSnapshot
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter

/**
 * 代币经济页：只读展示链上 supply / allocations / release。
 * 所有数值来自节点查询，静态白皮书数值不在此重复展示。
 */
@Composable
fun TokenomicsScreen(viewModel: EcosystemViewModel, onBack: () -> Unit) {
    val snapshot by viewModel.tokenomics.collectAsState()
    val loading by viewModel.tokenomicsLoading.collectAsState()
    val error by viewModel.tokenomicsError.collectAsState()

    LaunchedEffect(Unit) { viewModel.loadTokenomics() }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            EcosystemHeader(
                title = "代币经济",
                subtitle = "链上实时供应、分配与团队释放",
                onBack = onBack,
                onRefresh = { viewModel.loadTokenomics() },
                refreshing = loading,
            )
        }

        if (loading && snapshot == null) {
            item {
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                    CircularProgressIndicator(modifier = Modifier.size(22.dp), strokeWidth = 2.dp)
                }
            }
        }

        if (error != null) {
            item { EcosystemInfoCard("暂时无法读取", listOf(error.orEmpty())) }
        }

        snapshot?.let { data ->
            item { SupplyCard(data) }
            item { SectionLabel("分配池（${data.pools.size}）") }
            if (data.pools.isEmpty()) {
                item { EcosystemInfoCard("暂无分配数据", listOf("节点未返回 allocations，可能尚未初始化创世分配。")) }
            } else {
                items(data.pools.size) { index -> PoolItem(data.pools[index]) }
            }
            item { SectionLabel("团队释放") }
            data.team?.let { item { TeamReleaseCard(it) } }
                ?: item { EcosystemInfoCard("暂无释放数据", listOf("节点未返回 release 信息。")) }
        }

        if (snapshot == null && !loading && error == null) {
            item { EcosystemInfoCard("暂无数据", listOf("点击右上角刷新读取链上代币信息。")) }
        }

        item {
            Text(
                "数据来源：/mcchain/tokenomics/supply、/allocations、/release；释放进度按区块时间实时计算。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
            )
        }
    }
}

@Composable
private fun SupplyCard(data: TokenomicsSnapshot) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(14.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text("供应", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Text(
                "${formatMcAmount(data.mintedUmc)} MC",
                style = MaterialTheme.typography.headlineSmall,
                fontWeight = FontWeight.Bold,
            )
            Text(
                "已发行 · 总量上限 ${formatMcAmount(data.totalCapUmc)} MC（${data.denom.ifBlank { "umc" }}）",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
            val ratio = if (data.totalCapUmc > 0) {
                (data.mintedUmc.toDouble() / data.totalCapUmc).coerceIn(0.0, 1.0).toFloat()
            } else {
                0f
            }
            LinearProgressIndicator(
                progress = { ratio },
                modifier = Modifier.fillMaxWidth(),
            )
            Text(
                "已发行占比 ${"%.2f".format(ratio * 100)}%",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
            )
        }
    }
}

@Composable
private fun PoolItem(pool: TokenomicsPool) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    pool.name.ifBlank { "未命名池" },
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    "${"%.2f".format(pool.percentBps / 100.0)}%",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
            LinearProgressIndicator(
                progress = { (pool.percentBps / 10000f).coerceIn(0f, 1f) },
                modifier = Modifier.fillMaxWidth(),
            )
            Text(
                "分配 ${formatMcAmount(pool.allocatedUmc)} MC · 余额 ${formatMcAmount(pool.balanceUmc)} MC",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
            if (pool.address.isNotBlank()) {
                Text(
                    "地址 ${shortenAddress(pool.address)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                )
            }
        }
    }
}

@Composable
private fun TeamReleaseCard(team: TeamRelease) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(14.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text("团队池", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
            Text(
                "${formatMcAmount(team.vestedUmc)} / ${formatMcAmount(team.totalLockedUmc)} MC",
                style = MaterialTheme.typography.bodyLarge,
                fontWeight = FontWeight.Medium,
            )
            LinearProgressIndicator(
                progress = { (team.progressBps / 10000f).coerceIn(0f, 1f) },
                modifier = Modifier.fillMaxWidth(),
            )
            Text(
                "已释放 ${"%.2f".format(team.progressBps / 100.0)}% · 剩余 ${formatMcAmount(team.remainingUmc)} MC",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.65f),
            )
            if (team.startTimeSeconds > 0) {
                Text(
                    "开始 ${formatTime(team.startTimeSeconds)} · 结束 ${formatTime(team.endTimeSeconds)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                )
            }
            if (team.cliffTimeSeconds > 0) {
                Text(
                    "锁定期至 ${formatTime(team.cliffTimeSeconds)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                )
            }
        }
    }
}

private fun formatTime(seconds: Long): String = runCatching {
    DateTimeFormatter.ofPattern("yyyy-MM-dd")
        .withZone(ZoneId.systemDefault())
        .format(Instant.ofEpochSecond(seconds))
}.getOrDefault("—")
