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
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.network.IndexerClient

/**
 * 贡献历史页：从本地索引器读取 depin 贡献奖励与防御拒绝事件。
 *
 * 索引器是独立后端，非链上实时；未配置或不可达时明确提示，不展示推测数据。
 */
@Composable
fun ContributionHistoryScreen(
    viewModel: EcosystemViewModel,
    onBack: () -> Unit,
    onOpenSettings: () -> Unit,
) {
    val events by viewModel.contributions.collectAsState()
    val total by viewModel.contributionTotal.collectAsState()
    val loading by viewModel.contributionLoading.collectAsState()
    val error by viewModel.contributionError.collectAsState()
    val notConfigured by viewModel.contributionNotConfigured.collectAsState()

    LaunchedEffect(Unit) { viewModel.loadContributions() }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            EcosystemHeader(
                title = "贡献历史",
                subtitle = "来自本地索引器的 depin 事件",
                onBack = onBack,
                onRefresh = { viewModel.loadContributions() },
                refreshing = loading,
            )
        }

        item {
            EcosystemInfoCard(
                title = "数据来源",
                lines = listOf(
                    "本页读取 mcchain-indexer serve 的只读 HTTP 接口，不是链上实时查询。",
                    "仅展示 depin.RewardPaid（贡献奖励到账）与 depin.DefenseBlocked（防御拒绝）。",
                    "索引器未运行或未配置时，本页不会展示任何本地推测记录。",
                ),
            )
        }

        if (notConfigured) {
            item {
                Card(shape = RoundedCornerShape(12.dp)) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(14.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        Text(
                            "尚未配置索引器地址",
                            style = MaterialTheme.typography.titleSmall,
                            fontWeight = FontWeight.SemiBold,
                        )
                        Text(
                            "在「设置 → 网络配置 → 索引器地址」填写 mcchain-indexer serve 的地址" +
                                "（例如 http://192.168.1.10:8090），保存后回到本页刷新。",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.75f),
                        )
                        Button(onClick = onOpenSettings) { Text("去设置") }
                    }
                }
            }
        }

        if (error != null) {
            item { EcosystemInfoCard("读取失败", listOf(error.orEmpty())) }
        }

        if (loading && events.isEmpty()) {
            item {
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                    CircularProgressIndicator(modifier = Modifier.size(22.dp), strokeWidth = 2.dp)
                }
            }
        }

        if (!loading && !notConfigured && error == null && events.isEmpty()) {
            item {
                EcosystemInfoCard(
                    "暂无贡献记录",
                    listOf("索引器中还没有该地址的 depin 奖励或拒绝事件。"),
                )
            }
        }

        if (events.isNotEmpty()) {
            item { SectionLabel("事件记录（${events.size}/$total）") }
            items(events, key = { "${it.height}-${it.type}-${it.attrs["task_id"]}" }) { event ->
                ContributionEventItem(event)
            }
            if (events.size < total) {
                item {
                    Button(
                        onClick = { viewModel.loadContributions(loadMore = true) },
                        enabled = !loading,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(if (loading) "加载中…" else "加载更多")
                    }
                }
            }
        }
    }
}

@Composable
private fun ContributionEventItem(event: IndexerClient.IndexerEvent) {
    val isReward = event.type == "depin.RewardPaid"
    val taskId = event.attrs["task_id"].orEmpty()
    val taskType = event.attrs["task_type"].orEmpty()
    val score = event.attrs["score"].orEmpty()
    val payout = event.attrs["payout"]?.toLongOrNull()
    val layer = event.attrs["failed_layer"].orEmpty()
    val reason = event.attrs["reason"].orEmpty()

    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = if (isReward) "贡献奖励" else "防御拒绝",
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f),
                )
                if (isReward && payout != null) {
                    Text(
                        "+${formatMcAmount(payout)} MC",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.primary,
                    )
                }
            }
            if (taskId.isNotBlank()) {
                Text(
                    "任务 $taskId" + if (taskType.isNotBlank()) " · $taskType" else "",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.65f),
                )
            }
            if (isReward && score.isNotBlank()) {
                Text(
                    "质量评分 $score",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                )
            }
            if (!isReward) {
                Text(
                    if (layer.isNotBlank()) "第 $layer 层拒绝：${reason.ifBlank { "—" }}" else reason.ifBlank { "—" },
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.error,
                )
            }
            Text(
                "区块高度 ${event.height}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.45f),
            )
        }
    }
}
