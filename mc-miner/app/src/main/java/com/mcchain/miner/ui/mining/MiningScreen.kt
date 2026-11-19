package com.mcchain.miner.ui.mining

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.mcchain.miner.data.model.AiTaskStatus
import com.mcchain.miner.data.model.ContribStatus
import com.mcchain.miner.data.model.Contribution
import com.mcchain.miner.data.model.EdgeAiTask
import com.mcchain.miner.data.model.NodeStatus
import com.mcchain.miner.data.model.TaskType
import com.mcchain.miner.ui.theme.*
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 贡献 / EdgeAI 页面。
 *
 * 数据边界：所有数字来自 Room 本机计量记录与链上查询；
 * 贡献状态只反映本机计量与广播回执，最终结算以链上为准。
 *
 * [onOpenWallet] / [onOpenNode] 为跨页跳转入口，由导航层注入；
 * 未注入时只展示文字引导，不做静默跳转。
 */
@Composable
fun MiningScreen(
    viewModel: MiningViewModel = androidx.lifecycle.viewmodel.compose.viewModel(),
    onOpenWallet: (() -> Unit)? = null,
    onOpenNode: (() -> Unit)? = null,
) {
    val contributions by viewModel.contributions.collectAsState()
    val summary by viewModel.summary.collectAsState()
    val edgeTasks by viewModel.edgeTasks.collectAsState()
    val walletBalance by viewModel.walletBalance.collectAsState()
    val serviceRunning by viewModel.serviceRunning.collectAsState()
    val lastHeartbeat by viewModel.lastHeartbeat.collectAsState()
    val startBlockReason by viewModel.startBlockReason.collectAsState()
    val nodeStatus by viewModel.nodeStatus.collectAsState()
    val prerequisites by viewModel.prerequisites.collectAsState()
    val checkingPrerequisites by viewModel.checkingPrerequisites.collectAsState()
    val powerPolicyText by viewModel.powerPolicyText.collectAsState()
    val checkingTasks by viewModel.checkingTasks.collectAsState()
    val edgeTaskMessage by viewModel.edgeTaskMessage.collectAsState()
    val edgeTaskMessageIsError by viewModel.edgeTaskMessageIsError.collectAsState()
    val lastTaskCheckAt by viewModel.lastTaskCheckAt.collectAsState()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // 顶部链上同步边界说明
        item {
            BoundaryNotice()
        }

        // 服务控制区
        item {
            ServiceControlCard(
                serviceRunning = serviceRunning,
                nodeStatus = nodeStatus,
                walletBalance = walletBalance,
                lastHeartbeat = lastHeartbeat,
                startBlockReason = startBlockReason,
                prerequisites = prerequisites,
                checkingPrerequisites = checkingPrerequisites,
                powerPolicyText = powerPolicyText,
                onStart = { viewModel.startMining() },
                onStop = { viewModel.stopMining() },
                onCheckPrerequisites = { viewModel.checkPrerequisites() },
                onOpenWallet = onOpenWallet,
                onOpenNode = onOpenNode,
            )
        }

        // 贡献总览
        item {
            ContributionOverviewCard(
                summary = summary,
                serviceRunning = serviceRunning,
                onStart = { viewModel.startMining() }
            )
        }

        // EdgeAI 区
        item {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        "EdgeAI 任务",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                    Text(
                        "任务列表来自本机缓存，可由链上同步刷新",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
                    )
                }
                TextButton(
                    onClick = { viewModel.checkEdgeAiTasks() },
                    enabled = !checkingTasks
                ) {
                    if (checkingTasks) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(16.dp),
                            strokeWidth = 2.dp
                        )
                        Spacer(modifier = Modifier.width(6.dp))
                    } else {
                        Icon(
                            Icons.Filled.Refresh,
                            contentDescription = null,
                            modifier = Modifier.size(18.dp)
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                    }
                    Text(if (checkingTasks) "检查中…" else "立即检查任务")
                }
            }
        }

        if (edgeTaskMessage != null) {
            item {
                Text(
                    text = edgeTaskMessage.orEmpty(),
                    style = MaterialTheme.typography.labelSmall,
                    color = if (edgeTaskMessageIsError) MaterialTheme.colorScheme.error
                    else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                )
            }
        }

        if (lastTaskCheckAt > 0L) {
            item {
                Text(
                    text = "最近检查时间：${formatDateTime(lastTaskCheckAt)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.45f)
                )
            }
        }

        // 执行记录说明：本机不持久化 EdgeAI 执行日志，执行结果以任务状态与链上回执为准。
        item {
            Text(
                text = "暂无本机执行记录：本机不保存独立执行日志，执行结果以任务状态与链上回执为准；" +
                    "执行失败时任务状态会回退为「待领取」，不会伪造成功。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.45f)
            )
        }

        if (edgeTasks.isEmpty()) {
            item {
                EmptyGuideCard(
                    title = "暂无 EdgeAI 任务",
                    description = "本机还没有任务缓存。贡献服务运行时会自动同步链上开放任务；" +
                        "也可以点击下方按钮主动拉取。",
                    actionText = if (checkingTasks) null else "立即检查任务",
                    onAction = if (checkingTasks) null else {
                        { viewModel.checkEdgeAiTasks() }
                    }
                )
            }
        } else {
            items(items = edgeTasks, key = { it.taskId }) { task ->
                EdgeTaskCard(task)
            }
        }

        // 贡献记录
        item {
            Column {
                Text(
                    "贡献记录",
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold
                )
                Text(
                    "按上报时间倒序，点击查看记录详情",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
                )
            }
        }

        if (contributions.isEmpty()) {
            item {
                EmptyGuideCard(
                    title = "暂无贡献记录",
                    description = if (!serviceRunning) {
                        "贡献服务未启动。启动后本机会按功耗策略计量带宽 / 存储贡献并写入本机记录。"
                    } else {
                        "服务已启动，但尚未产生计量记录。当前可能受功耗策略或网络条件限制，" +
                            "可在「设置」中调整功耗策略。"
                    },
                    actionText = if (!serviceRunning) "开始贡献" else null,
                    onAction = if (!serviceRunning) {
                        { viewModel.startMining() }
                    } else {
                        null
                    }
                )
            }
        } else {
            items(items = contributions, key = { it.id }) { contribution ->
                ContributionRecordCard(contribution)
            }
        }

        // 底部链上同步边界说明
        item {
            Text(
                "贡献记录以本机计量与广播回执为准，最终结算以链上为准；" +
                    "链上暂未提供贡献记录查询接口，本页不会轮询或推断「已验证」状态。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.45f)
            )
        }
    }
}

// === 顶部边界说明 ===

@Composable
private fun BoundaryNotice() {
    Card(
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant
        )
    ) {
        Row(modifier = Modifier.padding(16.dp)) {
            Icon(
                Icons.Filled.Info,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(20.dp)
            )
            Spacer(modifier = Modifier.width(12.dp))
            Text(
                "链上同步边界：贡献状态以广播回执与链上后续结算为准，本页不轮询伪造「已验证」。",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f)
            )
        }
    }
}

// === 服务控制区 ===

@Composable
private fun ServiceControlCard(
    serviceRunning: Boolean,
    nodeStatus: NodeStatus?,
    walletBalance: Long,
    lastHeartbeat: Long,
    startBlockReason: String?,
    prerequisites: List<PrerequisiteItem>,
    checkingPrerequisites: Boolean,
    powerPolicyText: String,
    onStart: () -> Unit,
    onStop: () -> Unit,
    onCheckPrerequisites: () -> Unit,
    onOpenWallet: (() -> Unit)?,
    onOpenNode: (() -> Unit)?,
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        colors = CardDefaults.cardColors(containerColor = McPrimary)
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        "贡献服务",
                        color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                        style = MaterialTheme.typography.labelMedium
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = when {
                            !serviceRunning -> "已停止"
                            nodeStatus != NodeStatus.ACTIVE -> "运行中 · 设备未在线，已暂停计量"
                            else -> "运行中 · 正在计量与周期提交"
                        },
                        color = if (serviceRunning) StatusActive
                        else MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.75f),
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )
                }
                StatusChip(
                    text = if (serviceRunning) "运行中" else "已停止",
                    color = if (serviceRunning) StatusActive else MaterialTheme.colorScheme.onPrimary
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            Row(modifier = Modifier.fillMaxWidth()) {
                StatBlock(
                    label = "钱包余额",
                    value = formatMc(walletBalance),
                    valueColor = McGold
                )
                Spacer(modifier = Modifier.weight(1f))
                StatBlock(
                    label = "最近链上心跳",
                    value = if (lastHeartbeat > 0) formatDateTime(lastHeartbeat) else "尚未发送",
                    valueColor = MaterialTheme.colorScheme.onPrimary
                )
            }

            Spacer(modifier = Modifier.height(16.dp))

            Button(
                onClick = { if (serviceRunning) onStop() else onStart() },
                modifier = Modifier.fillMaxWidth(),
                colors = ButtonDefaults.buttonColors(
                    containerColor = if (serviceRunning) {
                        MaterialTheme.colorScheme.surface
                    } else {
                        McGold
                    },
                    contentColor = McPrimary
                )
            ) {
                Icon(
                    imageVector = if (serviceRunning) Icons.Filled.Stop else Icons.Filled.PlayArrow,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(if (serviceRunning) "停止贡献" else "开始贡献")
            }

            if (startBlockReason != null) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = startBlockReason.orEmpty(),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.error
                )
            }

            Spacer(modifier = Modifier.height(16.dp))
            HorizontalLine(color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.15f))
            Spacer(modifier = Modifier.height(12.dp))

            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "前置条件",
                    style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                    modifier = Modifier.weight(1f)
                )
                TextButton(
                    onClick = onCheckPrerequisites,
                    enabled = !checkingPrerequisites
                ) {
                    if (checkingPrerequisites) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(14.dp),
                            strokeWidth = 2.dp,
                            color = MaterialTheme.colorScheme.onPrimary
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                    }
                    Text(
                        if (checkingPrerequisites) "检测中…" else "检测前置条件",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                }
            }

            prerequisites.forEach { item ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 3.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Icon(
                        imageVector = if (item.ok) Icons.Filled.CheckCircle
                        else Icons.Filled.Warning,
                        contentDescription = null,
                        tint = if (item.ok) StatusActive else McGold,
                        modifier = Modifier.size(16.dp)
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    Text(
                        item.label,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onPrimary,
                        modifier = Modifier.width(72.dp)
                    )
                    Text(
                        item.detail,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                        modifier = Modifier.weight(1f)
                    )
                }
            }

            when {
                prerequisites.any { it.label == "钱包账户" && !it.ok } && onOpenWallet != null -> {
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedButton(
                        onClick = { onOpenWallet?.invoke() },
                        modifier = Modifier.fillMaxWidth(),
                        colors = ButtonDefaults.outlinedButtonColors(
                            contentColor = MaterialTheme.colorScheme.onPrimary
                        )
                    ) {
                        Text("去创建钱包")
                    }
                }
                prerequisites.any { it.label == "设备认证" && !it.ok } && onOpenNode != null -> {
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedButton(
                        onClick = { onOpenNode?.invoke() },
                        modifier = Modifier.fillMaxWidth(),
                        colors = ButtonDefaults.outlinedButtonColors(
                            contentColor = MaterialTheme.colorScheme.onPrimary
                        )
                    ) {
                        Text("去完成设备认证")
                    }
                }
            }

            Spacer(modifier = Modifier.height(12.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    Icons.Filled.Bolt,
                    contentDescription = null,
                    tint = McGold,
                    modifier = Modifier.size(16.dp)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = "功耗策略：${powerPolicyText.ifBlank { "读取中…" }}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f)
                )
            }
        }
    }
}

// === 贡献总览 ===

@Composable
private fun ContributionOverviewCard(
    summary: ContributionSummary,
    serviceRunning: Boolean,
    onStart: () -> Unit,
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "贡献总览",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.weight(1f)
                )
                Text(
                    "累计 ${summary.totalCount} 次",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
                )
            }

            if (summary.truncated) {
                Spacer(modifier = Modifier.height(4.dp))
                Text(
                    "记录数超过单次加载上限，今日分项与上链统计仅覆盖最近 1000 条本机记录",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.45f)
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            Row(modifier = Modifier.fillMaxWidth()) {
                SummaryMetric(
                    label = "今日贡献次数",
                    value = "${summary.todayCount} 次",
                    modifier = Modifier.weight(1f)
                )
                SummaryMetric(
                    label = "今日累计上报量",
                    value = formatMc(summary.todayAmount),
                    modifier = Modifier.weight(1f)
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            Row(modifier = Modifier.fillMaxWidth()) {
                SummaryMetric(
                    label = "已上链次数",
                    value = "${summary.onChainCount} 次",
                    valueColor = StatusActive,
                    modifier = Modifier.weight(1f)
                )
                SummaryMetric(
                    label = "待上链（仅本机）",
                    value = "${summary.pendingCount} 次",
                    valueColor = StatusPending,
                    modifier = Modifier.weight(1f)
                )
            }

            Spacer(modifier = Modifier.height(16.dp))
            HorizontalLine()
            Spacer(modifier = Modifier.height(12.dp))

            Text(
                "今日分项",
                style = MaterialTheme.typography.labelMedium,
                fontWeight = FontWeight.Bold
            )

            if (summary.typeStats.isEmpty()) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = if (serviceRunning) {
                        "今日暂无贡献记录：服务运行时按功耗策略计量带宽 / 存储贡献。"
                    } else {
                        "今日暂无贡献记录：启动贡献服务后开始计量。"
                    },
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                )
                if (!serviceRunning) {
                    Spacer(modifier = Modifier.height(8.dp))
                    TextButton(onClick = onStart) {
                        Text("开始贡献")
                    }
                }
            } else {
                summary.typeStats.forEach { stat ->
                    TypeBreakdownRow(stat = stat, todayTotal = summary.todayCount)
                }
            }

            Spacer(modifier = Modifier.height(12.dp))
            Text(
                "上报量为本机计量 amount 字段累计（umc → MC），不代表链上最终奖励。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.45f)
            )
        }
    }
}

@Composable
private fun TypeBreakdownRow(stat: ContributionTypeStat, todayTotal: Int) {
    val fraction = if (todayTotal > 0) stat.count.toFloat() / todayTotal else 0f
    val accent = when (stat.type) {
        TaskType.BANDWIDTH -> StatusActive
        TaskType.DATA_STORAGE -> StatusPending
        TaskType.AI_INFERENCE -> McGold
        TaskType.COMPUTATION -> StatusGrace
    }
    Column(modifier = Modifier.padding(vertical = 6.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                taskTypeName(stat.type),
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.weight(1f)
            )
            Text(
                "${stat.count} 次 · ${percentText(fraction)}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f)
            )
        }
        Spacer(modifier = Modifier.height(4.dp))
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(6.dp)
                .background(
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.08f),
                    shape = RoundedCornerShape(3.dp)
                )
        ) {
            if (fraction > 0f) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth(fraction.coerceIn(0f, 1f))
                        .height(6.dp)
                        .background(color = accent, shape = RoundedCornerShape(3.dp))
                )
            }
        }
    }
}

@Composable
private fun SummaryMetric(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
    valueColor: Color = MaterialTheme.colorScheme.onSurface,
) {
    Column(modifier = modifier) {
        Text(
            value,
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.Bold,
            color = valueColor
        )
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
        )
    }
}

// === EdgeAI 任务卡 ===

@Composable
private fun EdgeTaskCard(task: EdgeAiTask) {
    var expanded by rememberSaveable(task.taskId) { mutableStateOf(false) }
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { expanded = !expanded },
        shape = RoundedCornerShape(12.dp)
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = edgeTaskIcon(task),
                    contentDescription = null,
                    tint = MaterialTheme.colorScheme.primary,
                    modifier = Modifier.size(22.dp)
                )
                Spacer(modifier = Modifier.width(10.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        edgeTaskTypeName(task.taskType),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Text(
                        "任务 ${task.taskId.take(12)}…",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
                    )
                }
                Column(horizontalAlignment = Alignment.End) {
                    Text(
                        formatMc(task.reward),
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.Bold,
                        color = McGold
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    StatusChip(
                        text = aiTaskStatusText(task.status),
                        color = aiTaskStatusColor(task.status)
                    )
                }
            }

            Spacer(modifier = Modifier.height(10.dp))

            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = "提交 ${task.submissionCount}/${task.maxSubmissions}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                )
                Spacer(modifier = Modifier.width(12.dp))
                Text(
                    text = if (task.deadlineHeight > 0) {
                        "截止高度 #${task.deadlineHeight}"
                    } else {
                        "截止高度未提供"
                    },
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                )
                Spacer(modifier = Modifier.weight(1f))
                Icon(
                    imageVector = if (expanded) Icons.Filled.ExpandLess
                    else Icons.Filled.ExpandMore,
                    contentDescription = if (expanded) "收起" else "展开",
                    tint = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                    modifier = Modifier.size(18.dp)
                )
            }

            if (expanded) {
                Spacer(modifier = Modifier.height(8.dp))
                HorizontalLine()
                Spacer(modifier = Modifier.height(8.dp))
                DetailRow("任务 ID", task.taskId)
                DetailRow(
                    "模型 ID",
                    task.modelId.ifBlank { "未提供" }
                )
                DetailRow(
                    "截止高度",
                    if (task.deadlineHeight > 0) {
                        "#${task.deadlineHeight}（由 RPC 按任务过期块数推算）"
                    } else {
                        "未提供"
                    }
                )
                DetailRow("截止时间", "未提供（以链上区块高度为准）")
                DetailRow(
                    "创建时间",
                    if (task.createdAt > 0) formatDateTime(task.createdAt) else "未提供"
                )
                DetailRow(
                    "输入大小",
                    if (task.inputSizeBytes > 0) "${task.inputSizeBytes} 字节" else "未提供"
                )
                DetailRow("创建者", task.creator.ifBlank { "未提供" })
                DetailRow(
                    "输入哈希",
                    task.inputHash.ifBlank { "未提供" }
                )
            }
        }
    }
}

// === 贡献记录卡 ===

@Composable
private fun ContributionRecordCard(contribution: Contribution) {
    var expanded by rememberSaveable(contribution.id) { mutableStateOf(false) }
    val displayAmount = if (contribution.reward > 0) contribution.reward else contribution.amount

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { expanded = !expanded },
        shape = RoundedCornerShape(12.dp)
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = taskTypeIcon(contribution.taskType),
                    contentDescription = null,
                    tint = taskTypeColor(contribution.taskType),
                    modifier = Modifier.size(22.dp)
                )
                Spacer(modifier = Modifier.width(10.dp))
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        taskTypeName(contribution.taskType),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium
                    )
                    Text(
                        formatDateTime(contribution.reportedAt),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
                    )
                }
                Column(horizontalAlignment = Alignment.End) {
                    Text(
                        formatMc(displayAmount),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Bold,
                        color = if (contribution.status == ContribStatus.PAID) StatusActive else McGold
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    StatusChip(
                        text = contribStatusText(contribution.status),
                        color = contribStatusColor(contribution.status)
                    )
                }
            }

            Spacer(modifier = Modifier.height(8.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "点击查看详情",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f),
                    modifier = Modifier.weight(1f)
                )
                Icon(
                    imageVector = if (expanded) Icons.Filled.ExpandLess
                    else Icons.Filled.ExpandMore,
                    contentDescription = if (expanded) "收起" else "展开",
                    tint = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                    modifier = Modifier.size(18.dp)
                )
            }

            if (expanded) {
                Spacer(modifier = Modifier.height(8.dp))
                HorizontalLine()
                Spacer(modifier = Modifier.height(8.dp))
                DetailRow("记录 ID", contribution.id)
                DetailRow(
                    "任务 ID",
                    contribution.taskId.ifBlank { "无（本机计量未关联链上任务）" }
                )
                DetailRow("证明哈希", proofHashText(contribution.proofHash))
                DetailRow("上报时间", formatDateTimeSeconds(contribution.reportedAt))
                DetailRow(
                    "验证时间",
                    contribution.verifiedAt?.let { formatDateTimeSeconds(it) } ?: "未验证（以链上为准）"
                )
                DetailRow("上报量", formatMc(contribution.amount))
                DetailRow("奖励", formatMc(contribution.reward))
            }
        }
    }
}

// === 通用组件 ===

@Composable
private fun EmptyGuideCard(
    title: String,
    description: String,
    actionText: String? = null,
    onAction: (() -> Unit)? = null,
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant
        )
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Icon(
                Icons.Filled.Info,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(24.dp)
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                title,
                style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.Bold
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                description,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.65f)
            )
            if (actionText != null && onAction != null) {
                Spacer(modifier = Modifier.height(8.dp))
                TextButton(onClick = { onAction?.invoke() }) {
                    Text(actionText)
                }
            }
        }
    }
}

@Composable
private fun DetailRow(label: String, value: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 2.dp),
        verticalAlignment = Alignment.Top
    ) {
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
            modifier = Modifier.width(80.dp)
        )
        Text(
            value,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.8f),
            modifier = Modifier.weight(1f)
        )
    }
}

@Composable
private fun StatusChip(text: String, color: Color) {
    Surface(
        shape = RoundedCornerShape(50),
        color = color.copy(alpha = 0.15f)
    ) {
        Text(
            text,
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 2.dp),
            style = MaterialTheme.typography.labelSmall,
            color = color
        )
    }
}

@Composable
private fun StatBlock(label: String, value: String, valueColor: Color) {
    Column {
        Text(
            value,
            style = MaterialTheme.typography.titleSmall,
            fontWeight = FontWeight.Bold,
            color = valueColor
        )
        Text(
            label,
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.6f)
        )
    }
}

@Composable
private fun HorizontalLine(color: Color = MaterialTheme.colorScheme.outline.copy(alpha = 0.3f)) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(1.dp)
            .background(color)
    )
}

// === 文案与格式 ===

private fun taskTypeName(type: TaskType): String = when (type) {
    TaskType.AI_INFERENCE -> "AI 推理"
    TaskType.DATA_STORAGE -> "数据存储"
    TaskType.BANDWIDTH -> "带宽贡献"
    TaskType.COMPUTATION -> "通用计算"
}

private fun taskTypeIcon(type: TaskType): ImageVector = when (type) {
    TaskType.AI_INFERENCE -> Icons.Filled.Psychology
    TaskType.DATA_STORAGE -> Icons.Filled.Storage
    TaskType.BANDWIDTH -> Icons.Filled.Cloud
    TaskType.COMPUTATION -> Icons.Filled.Memory
}

@Composable
private fun taskTypeColor(type: TaskType): Color = when (type) {
    TaskType.AI_INFERENCE -> McGold
    TaskType.DATA_STORAGE -> StatusPending
    TaskType.BANDWIDTH -> StatusActive
    TaskType.COMPUTATION -> StatusGrace
}

private fun contribStatusText(status: ContribStatus): String = when (status) {
    ContribStatus.LOCAL_ONLY -> "仅本机"
    ContribStatus.SUBMITTED -> "已提交"
    ContribStatus.VERIFYING -> "验证中"
    ContribStatus.VERIFIED -> "已验证"
    ContribStatus.DISPUTED -> "争议中"
    ContribStatus.REJECTED -> "已拒绝"
    ContribStatus.PAID -> "已支付"
}

@Composable
private fun contribStatusColor(status: ContribStatus): Color = when (status) {
    ContribStatus.VERIFIED, ContribStatus.PAID -> StatusActive
    ContribStatus.VERIFYING -> StatusGrace
    ContribStatus.DISPUTED, ContribStatus.REJECTED -> StatusSlashed
    ContribStatus.SUBMITTED -> StatusPending
    ContribStatus.LOCAL_ONLY -> MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
}

private fun aiTaskStatusText(status: AiTaskStatus): String = when (status) {
    AiTaskStatus.OPEN -> "待领取"
    AiTaskStatus.IN_PROGRESS -> "进行中"
    AiTaskStatus.AWAITING_VERIFICATION -> "待验证"
    AiTaskStatus.COMPLETED -> "已完成"
    AiTaskStatus.DISPUTED -> "争议中"
    AiTaskStatus.CANCELLED -> "已取消"
}

@Composable
private fun aiTaskStatusColor(status: AiTaskStatus): Color = when (status) {
    AiTaskStatus.OPEN -> StatusPending
    AiTaskStatus.IN_PROGRESS -> StatusPending
    AiTaskStatus.AWAITING_VERIFICATION -> StatusGrace
    AiTaskStatus.COMPLETED -> StatusActive
    AiTaskStatus.DISPUTED -> StatusSlashed
    AiTaskStatus.CANCELLED -> MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
}

private fun edgeTaskTypeName(raw: String): String = when {
    raw.contains("hash_chain") -> "哈希链校验任务"
    raw.contains("fine_tuning") -> "模型微调"
    raw.contains("inference") -> "AI 推理"
    raw.contains("training") -> "模型训练"
    raw.isBlank() -> "未分类任务"
    else -> raw.take(24)
}

private fun edgeTaskIcon(task: EdgeAiTask): ImageVector = when {
    task.taskType.contains("inference") -> Icons.Filled.Psychology
    task.taskType.contains("training") || task.taskType.contains("fine_tuning") ->
        Icons.Filled.Memory
    else -> Icons.Filled.Bolt
}

private fun formatMc(amountUmc: Long): String =
    String.format(Locale.getDefault(), "%.4f MC", amountUmc / 1_000_000.0)

private fun formatDateTime(millis: Long): String =
    SimpleDateFormat("MM-dd HH:mm", Locale.getDefault()).format(Date(millis))

private fun formatDateTimeSeconds(millis: Long): String =
    SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault()).format(Date(millis))

private fun percentText(fraction: Float): String =
    String.format(Locale.getDefault(), "%.0f%%", fraction * 100f)

private fun proofHashText(hash: String): String = when {
    hash.isBlank() -> "无（本机计量未生成证明）"
    hash.length <= 20 -> hash
    else -> "${hash.take(12)}…${hash.takeLast(6)}"
}
