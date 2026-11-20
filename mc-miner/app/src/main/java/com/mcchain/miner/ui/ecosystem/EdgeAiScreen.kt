package com.mcchain.miner.ui.ecosystem

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.network.EdgeAiDispute
import com.mcchain.miner.network.EdgeAiResult
import com.mcchain.miner.network.EdgeAiTaskDetail
import com.mcchain.miner.shared.util.McAmount

/**
 * EdgeAI 任务页：发布任务（奖励托管）、查看结果与争议、发起争议。
 *
 * 链上结算由 BeginBlock 完成，App 不提供「立即结算」入口，也不展示未实现的收益。
 */
@Composable
fun EdgeAiScreen(viewModel: EcosystemViewModel, onBack: () -> Unit) {
    val loading by viewModel.edgeAiLoading.collectAsState()
    val tasks by viewModel.edgeAiTasks.collectAsState()
    val results by viewModel.edgeAiResults.collectAsState()
    val disputes by viewModel.edgeAiDisputes.collectAsState()
    val error by viewModel.edgeAiError.collectAsState()
    val message by viewModel.edgeAiMessage.collectAsState()
    val busy by viewModel.edgeAiBusy.collectAsState()
    val account by viewModel.accountAddress.collectAsState()

    var description by rememberSaveable { mutableStateOf("") }
    var rewardInput by rememberSaveable { mutableStateOf("") }
    var inputError by remember { mutableStateOf<String?>(null) }
    var disputeTarget by remember { mutableStateOf<EdgeAiTaskDetail?>(null) }

    LaunchedEffect(Unit) { viewModel.loadEdgeAi() }

    val myTasks = tasks.filter { it.creator == account }
    val resultTaskIds = results.map { it.taskId }.toSet()
    val openDisputeTaskIds = disputes.filter { it.status == "open" }.map { it.taskId }.toSet()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            EcosystemHeader(
                title = "AI 任务",
                subtitle = "任务奖励由发布者托管，结算由链上完成",
                onBack = onBack,
                onRefresh = { viewModel.loadEdgeAi() },
                refreshing = loading,
            )
        }

        item { EcosystemMessageCard(message) { viewModel.clearEdgeAiMessage() } }

        item {
            EcosystemInfoCard(
                title = "结算规则",
                lines = listOf(
                    "发布任务时奖励从你的可用余额托管到模块账户，任务零奖励也可发布。",
                    "节点提交结果指纹，争议窗口结束后由链上自动结算：85% 给提交者，15% 进入验证者预留池。",
                    "任务过期未结算时，托管奖励退回发布者；争议由链上仲裁者裁定。",
                    "本页只读取链上任务/结果/争议，不代替节点执行推理。",
                ),
            )
        }

        item {
            Card(shape = RoundedCornerShape(12.dp)) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text("发布任务", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                    OutlinedTextField(
                        value = description,
                        onValueChange = {
                            if (it.length <= 500) description = it
                            inputError = null
                        },
                        modifier = Modifier.fillMaxWidth(),
                        label = { Text("任务描述（最多 500 字符）") },
                        minLines = 3,
                        maxLines = 5,
                    )
                    OutlinedTextField(
                        value = rewardInput,
                        onValueChange = {
                            rewardInput = it
                            inputError = null
                        },
                        modifier = Modifier.fillMaxWidth(),
                        label = { Text("奖励（MC，可为 0）") },
                        singleLine = true,
                    )
                    Button(
                        onClick = {
                            val rewardUmc = McAmount.parseToUmc(rewardInput)
                            when {
                                description.isBlank() -> inputError = "请填写任务描述"
                                rewardUmc == null -> inputError = "奖励格式无效（最多 6 位小数，不能为负）"
                                else -> {
                                    viewModel.createTask(description, rewardUmc)
                                    rewardInput = ""
                                }
                            }
                        },
                        enabled = !busy,
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(if (busy) "提交中…" else "发布并托管奖励")
                    }
                    inputError?.let {
                        Text(it, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.error)
                    }
                }
            }
        }

        item { SectionLabel("我的任务（${myTasks.size}）") }
        if (myTasks.isEmpty()) {
            item { EcosystemInfoCard("暂无发布记录", listOf("发布任务后，可在这里查看状态并发起争议。")) }
        } else {
            items(myTasks, key = { "mine-${it.taskId}" }) { task ->
                EdgeAiTaskItem(
                    task = task,
                    canDispute = task.taskId in resultTaskIds && task.taskId !in openDisputeTaskIds,
                    onDispute = { disputeTarget = task },
                )
            }
        }

        item { SectionLabel("开放任务（${tasks.count { it.status == "open" }}）") }
        if (loading && tasks.isEmpty()) {
            item {
                Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.Center) {
                    CircularProgressIndicator(modifier = Modifier.size(22.dp), strokeWidth = 2.dp)
                }
            }
        } else if (tasks.filter { it.status == "open" }.isEmpty()) {
            item { EcosystemInfoCard("暂无开放任务", listOf("链上当前没有 status=open 的任务。")) }
        } else {
            items(tasks.filter { it.status == "open" }, key = { "open-${it.taskId}" }) { task ->
                EdgeAiTaskItem(
                    task = task,
                    canDispute = task.taskId in resultTaskIds && task.taskId !in openDisputeTaskIds,
                    onDispute = { disputeTarget = task },
                )
            }
        }

        if (error != null) {
            item { EcosystemInfoCard("暂时无法读取", listOf(error.orEmpty())) }
        }

        item { SectionLabel("最近结果（${results.size}）") }
        if (results.isEmpty()) {
            item { EcosystemInfoCard("暂无结果", listOf("节点提交结果后会显示在这里。")) }
        } else {
            items(results.takeLast(10).reversed(), key = { "result-${it.taskId}-${it.submitter}" }) { result ->
                EdgeAiResultItem(result)
            }
        }

        item { SectionLabel("争议记录（${disputes.size}）") }
        if (disputes.isEmpty()) {
            item { EcosystemInfoCard("暂无争议", listOf("发起争议后，状态与裁定结果会显示在这里。")) }
        } else {
            items(disputes.takeLast(10).reversed(), key = { "dispute-${it.taskId}" }) { dispute ->
                EdgeAiDisputeItem(dispute)
            }
        }

        item {
            Text(
                "数据来源：链上 edgeai 查询接口；结果与争议均为链上原始状态的中文展示。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                modifier = Modifier.padding(top = 4.dp),
            )
        }
    }

    disputeTarget?.let { task ->
        var reason by remember(task.taskId) { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { disputeTarget = null },
            title = { Text("发起争议") },
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    Text(
                        "任务 #${task.taskId}：${task.description.take(60)}",
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Text(
                        "争议由链上仲裁者裁定；无链上重算证据时默认维持乐观结算。",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.65f),
                    )
                    OutlinedTextField(
                        value = reason,
                        onValueChange = { if (it.length <= 200) reason = it },
                        label = { Text("争议理由（最多 200 字）") },
                        modifier = Modifier.fillMaxWidth(),
                        minLines = 2,
                    )
                }
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        viewModel.openDispute(task.taskId, reason)
                        disputeTarget = null
                    },
                    enabled = reason.isNotBlank() && !busy,
                ) { Text("提交争议") }
            },
            dismissButton = {
                TextButton(onClick = { disputeTarget = null }) { Text("取消") }
            },
        )
    }
}

@Composable
private fun EdgeAiTaskItem(task: EdgeAiTaskDetail, canDispute: Boolean, onDispute: () -> Unit) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "任务 #${task.taskId}",
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    edgeAiStatusLabel(task.status),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
            Text(
                task.description.ifBlank { "（无描述）" },
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.8f),
            )
            Text(
                "奖励 ${formatMcAmount(task.rewardUmc)} MC · 创建者 ${shortenAddress(task.creator)}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
            )
            if (task.assignee.isNotBlank()) {
                Text(
                    "执行节点 ${shortenAddress(task.assignee)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                )
            }
            OutlinedButton(
                onClick = onDispute,
                enabled = canDispute,
                modifier = Modifier.padding(top = 4.dp),
            ) {
                Text(if (canDispute) "发起争议" else "暂无可争议结果")
            }
        }
    }
}

@Composable
private fun EdgeAiResultItem(result: EdgeAiResult) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(3.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "任务 #${result.taskId}",
                    style = MaterialTheme.typography.bodySmall,
                    fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    resultStatusLabel(result.status),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
            Text(
                "提交者 ${shortenAddress(result.submitter)}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
            )
            Text(
                "指纹 ${result.resultHash.take(24)}${if (result.resultHash.length > 24) "…" else ""}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
            )
        }
    }
}

@Composable
private fun EdgeAiDisputeItem(dispute: EdgeAiDispute) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(3.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "任务 #${dispute.taskId}",
                    style = MaterialTheme.typography.bodySmall,
                    fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    if (dispute.status == "open") "待裁定" else "已结案",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
            Text(
                "理由：${dispute.reason.ifBlank { "—" }}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f),
            )
            if (dispute.resolution.isNotBlank() && dispute.resolution != "none") {
                Text(
                    "裁定：${if (dispute.resolution == "cheat") "作弊" else "诚实"}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
            }
        }
    }
}

private fun edgeAiStatusLabel(status: String): String = when (status) {
    "open" -> "开放"
    "assigned" -> "已分配"
    "done" -> "已完成"
    "disputed" -> "争议中"
    "cheated" -> "已判作弊"
    "expired" -> "已过期"
    "" -> "状态未知"
    else -> status
}

private fun resultStatusLabel(status: String): String = when (status) {
    "pending" -> "待结算"
    "valid" -> "已通过"
    "rejected" -> "已拒绝"
    "" -> "状态未知"
    else -> status
}
