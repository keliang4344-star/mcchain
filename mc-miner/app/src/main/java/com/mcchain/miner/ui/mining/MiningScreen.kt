package com.mcchain.miner.ui.mining

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.*
import com.mcchain.miner.ui.theme.*
@Composable
fun MiningScreen(viewModel: MiningViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
    val contributions by viewModel.contributions.collectAsState()
    val availableTasks by viewModel.availableTasks.collectAsState()
    val totalReward by viewModel.totalReward.collectAsState()
    val contributionCount by viewModel.contributionCount.collectAsState()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // 挖矿状态头部
        item {
            Card(
                modifier = Modifier.fillMaxWidth(),
                shape = RoundedCornerShape(20.dp),
                colors = CardDefaults.cardColors(containerColor = McPrimary)
            ) {
                Column(
                    modifier = Modifier.padding(24.dp),
                    horizontalAlignment = Alignment.CenterHorizontally
                ) {
                    Text("贡献挖矿",
                        color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f))
                    Spacer(modifier = Modifier.height(4.dp))
                    Text("运行中",
                        color = StatusActive,
                        style = MaterialTheme.typography.titleMedium)

                    Spacer(modifier = Modifier.height(16.dp))

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceEvenly
                    ) {
                        MiningStatItem("累计收益", String.format("%.2f MC", totalReward / 1_000_000.0),
                            McGold, MaterialTheme.colorScheme.onPrimary)
                        MiningStatItem("总贡献", "$contributionCount 次",
                            MaterialTheme.colorScheme.onPrimary, MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f))
                    }
                }
            }
        }

        // 贡献类型卡片
        item {
            Text("贡献方式", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        }

        item {
            Card(shape = RoundedCornerShape(16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    ContributionTypeRow(Icons.Filled.Cloud, "带宽贡献",
                        "共享网络带宽，每 GB 获得奖励", StatusActive)
                    Spacer(modifier = Modifier.height(12.dp))
                    ContributionTypeRow(Icons.Filled.Storage, "存储贡献",
                        "共享存储空间，每 GB 获得奖励", StatusPending)
                    Spacer(modifier = Modifier.height(12.dp))
                    ContributionTypeRow(Icons.Filled.Psychology, "AI 推理",
                        "执行 AI 推理任务，按任务奖励", McGold)
                }
            }
        }

        // 佣金费率透明化卡片
        item {
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("贡献即挖矿 · 收益透明",
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.Bold)
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("所有贡献收益均上链可查，无需信任第三方。DePIN 奖励池初始 ${String.format("%.0f 亿 MC", MCParams.DEPIN_INITIAL_POOL_UMC / 100_000_000.0)}，按贡献分配。",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
                }
            }
        }

        // 最近任务
        item {
            Text("EdgeAI 任务", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        }

        items(availableTasks.size.coerceAtMost(10)) { index ->
            AiTaskCard(availableTasks[index])
        }

        // 贡献历史
        item {
            Text("贡献记录", style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold)
        }

        items(contributions.size.coerceAtMost(20)) { index ->
            ContributionItem(contributions[index])
        }
    }
}

@Composable
fun MiningStatItem(label: String, value: String, valueColor: androidx.compose.ui.graphics.Color, labelColor: androidx.compose.ui.graphics.Color) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(value, style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold, color = valueColor)
        Text(label, style = MaterialTheme.typography.labelSmall, color = labelColor)
    }
}

@Composable
fun ContributionTypeRow(icon: androidx.compose.ui.graphics.vector.ImageVector, title: String, desc: String, accent: androidx.compose.ui.graphics.Color) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Box(
            modifier = Modifier
                .size(40.dp)
                .padding(8.dp)
        ) {
            Icon(icon, null, tint = accent, modifier = Modifier.fillMaxSize())
        }
        Spacer(modifier = Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Text(title, fontWeight = FontWeight.Medium)
            Text(desc, style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
        }
    }
}

@Composable
fun AiTaskCard(task: EdgeAiTask) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text("${task.taskType.uppercase()} 任务",
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Bold)
                Text("模型: ${task.modelHash.take(12)}...",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(String.format("%.2f MC", task.reward / 1_000_000.0),
                    style = MaterialTheme.typography.titleSmall,
                    color = McGold)
                Text("${task.submissionCount}/${task.maxSubmissions} 提交",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
            }
        }
    }
}

@Composable
fun ContributionItem(contrib: Contribution) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = when (contrib.taskType) {
                        TaskType.AI_INFERENCE -> "AI 推理"
                        TaskType.DATA_STORAGE -> "数据存储"
                        TaskType.BANDWIDTH -> "带宽贡献"
                        TaskType.COMPUTATION -> "通用计算"
                    },
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = java.text.SimpleDateFormat("MM-dd HH:mm", java.util.Locale.getDefault())
                        .format(java.util.Date(contrib.reportedAt)),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(
                    text = String.format("%.4f MC", contrib.amount / 1_000_000.0),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Bold,
                    color = if (contrib.status == ContribStatus.PAID) StatusActive else McGold
                )
                Text(
                    text = when (contrib.status) {
                        ContribStatus.SUBMITTED -> "已提交"
                        ContribStatus.VERIFYING -> "验证中"
                        ContribStatus.VERIFIED -> "已验证"
                        ContribStatus.DISPUTED -> "争议中"
                        ContribStatus.REJECTED -> "已拒绝"
                        ContribStatus.PAID -> "已支付"
                    },
                    style = MaterialTheme.typography.labelSmall,
                    color = when (contrib.status) {
                        ContribStatus.VERIFIED, ContribStatus.PAID -> StatusActive
                        ContribStatus.REJECTED -> StatusSlashed
                        else -> StatusPending
                    }
                )
            }
        }
    }
}
