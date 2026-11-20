package com.mcchain.miner.ui.node

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.NodeStatus
import com.mcchain.miner.data.model.NodeStatusSnapshot
import com.mcchain.miner.data.model.PeerNode
import com.mcchain.miner.data.model.PhoneNodeRecord
import com.mcchain.miner.domain.node.HardwareCheck
import com.mcchain.miner.network.ChainAttestation
import com.mcchain.miner.network.ChainSlash
import com.mcchain.miner.ui.theme.*
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 节点与设备认证页。
 *
 * 只展示真实数据：节点状态来自链上回执写入的 Room 记录，认证/心跳时间来自安全存储，
 * 区块高度来自 RPC 实时查询（失败时标注「本地缓存」）。手机端不运行全节点、不参与 PoW。
 *
 * @param onOpenSettings 由宿主（E0）接入：跳转到「设置 → RPC 端点」。
 * @param onOpenWallet   由宿主（E0）接入：跳转到「钱包」页创建/导入钱包。
 */
@Composable
fun NodeScreen(
    vm: NodeViewModel = viewModel(),
    onOpenSettings: () -> Unit = {},
    onOpenWallet: () -> Unit = {},
) {
    val nodeStatus by vm.nodeStatus.collectAsState()
    val phoneNode by vm.phoneNode.collectAsState()
    val peers by vm.peers.collectAsState()
    val hwCheck by vm.hardwareCheck.collectAsState()
    val capability by vm.capability.collectAsState()
    val localInfo by vm.localInfo.collectAsState()
    val steps by vm.steps.collectAsState()
    val busyStep by vm.busyStep.collectAsState()
    val message by vm.message.collectAsState()
    val messageIsError by vm.messageIsError.collectAsState()
    val isRefreshing by vm.isRefreshing.collectAsState()
    val refreshError by vm.refreshError.collectAsState()
    val isLive by vm.isLive.collectAsState()
    val chainAttestation by vm.chainAttestation.collectAsState()
    val chainSlashes by vm.chainSlashes.collectAsState()
    val chainStateError by vm.chainStateError.collectAsState()

    val now = System.currentTimeMillis()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        item {
            StatusHeader(
                record = phoneNode,
                snapshot = nodeStatus,
                steps = steps,
                busyStep = busyStep,
                isRefreshing = isRefreshing,
                isLive = isLive,
                onRefresh = { vm.refresh() },
                onContinue = { step ->
                    if (step.id == AuthStepId.WALLET) onOpenWallet() else vm.runStep(step.id)
                },
            )
        }

        refreshError?.let { error ->
            item { RpcGuideCard(error, onOpenSettings) }
        }

        item {
            AttestationDetailCard(
                capability = capability,
                record = phoneNode,
                localInfo = localInfo,
                now = now,
            )
        }

        item {
            ChainAttestationCard(
                attestation = chainAttestation,
                slashes = chainSlashes,
                error = chainStateError,
            )
        }

        item {
            PipelineCard(
                steps = steps,
                busyStep = busyStep,
                message = message,
                messageIsError = messageIsError,
                onRun = { step ->
                    if (step.id == AuthStepId.WALLET && step.state != StepState.DONE) {
                        vm.runStep(step.id)
                        onOpenWallet()
                    } else {
                        vm.runStep(step.id)
                    }
                },
            )
        }

        item { HardwareCard(hwCheck) }

        item { ChainRulesCard(phoneNode) }

        item {
            Text(
                text = "对等节点（${peers.size}）",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold
            )
        }

        items(peers.size.coerceAtMost(15)) { index ->
            PeerItem(peers[index])
        }
    }
}

// ----------------------------------------------------------------------
// 认证状态机（六态）
// ----------------------------------------------------------------------

private data class StatusMeta(
    val title: String,
    val color: Color,
    val description: String,
    val suggestion: String,
)

@Composable
private fun statusMeta(status: NodeStatus?): StatusMeta = when (status) {
    null, NodeStatus.UNREGISTERED -> StatusMeta(
        title = "未注册",
        color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.6f),
        description = "本机尚未完成钱包、设备与节点注册，也没有链上认证记录。",
        suggestion = "按下方「认证流程」从第 ① 步开始。"
    )
    NodeStatus.PENDING -> StatusMeta(
        title = "认证中",
        color = StatusPending,
        description = "认证交易已广播，正在等待链上确认。",
        suggestion = "保持网络畅通，稍后点击「刷新」查看链上结果。"
    )
    NodeStatus.ACTIVE -> StatusMeta(
        title = "在线",
        color = StatusActive,
        description = "设备已通过硬件认证并持续发送心跳，可正常接单与计奖。",
        suggestion = "保持前台服务运行；心跳超时会先进入宽限期。"
    )
    NodeStatus.GRACE -> StatusMeta(
        title = "宽限期",
        color = StatusGrace,
        description = "心跳已超时，节点进入宽限期，仍未恢复将触发离线罚没。",
        suggestion = "立即恢复网络与前台服务，并完成一次认证续期。"
    )
    NodeStatus.JAILED -> StatusMeta(
        title = "已关押",
        color = StatusJailed,
        description = "节点已被链上关押，暂停接单与计奖资格。",
        suggestion = "检查离线或违规原因，等待关押期结束后重新认证。"
    )
    NodeStatus.SLASHED -> StatusMeta(
        title = "已罚没",
        color = StatusSlashed,
        description = "节点已被罚没，质押按规则扣除，认证状态失效。",
        suggestion = "查看罚没次数；如需继续参与，需重新质押并重新完成认证。"
    )
}

@Composable
private fun StatusHeader(
    record: PhoneNodeRecord?,
    snapshot: NodeStatusSnapshot?,
    steps: List<AuthStep>,
    busyStep: AuthStepId?,
    isRefreshing: Boolean,
    isLive: Boolean,
    onRefresh: () -> Unit,
    onContinue: (AuthStep) -> Unit,
) {
    val meta = statusMeta(record?.status)
    val currentStep = steps.firstOrNull { it.state != StepState.DONE }

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        colors = CardDefaults.cardColors(containerColor = McPrimary)
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            Text(
                text = "节点认证状态",
                color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.6f),
                style = MaterialTheme.typography.labelMedium
            )
            Spacer(modifier = Modifier.height(8.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(12.dp)
                        .clip(CircleShape)
                        .background(meta.color)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = meta.title,
                    color = Color.White,
                    style = MaterialTheme.typography.titleLarge,
                    fontWeight = FontWeight.Bold
                )
            }
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = meta.description,
                color = Color.White.copy(alpha = 0.8f),
                style = MaterialTheme.typography.bodySmall
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "建议：${meta.suggestion}",
                color = Color.White.copy(alpha = 0.68f),
                style = MaterialTheme.typography.bodySmall
            )

            if (snapshot != null) {
                Spacer(modifier = Modifier.height(14.dp))
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceEvenly
                ) {
                    HeaderStat("区块高度", "#${snapshot.currentHeight}")
                    HeaderStat("同步节点", "${snapshot.syncedPeers}")
                    HeaderStat("链 ID", snapshot.chainId.ifBlank { MCParams.CHAIN_ID })
                }
            }

            Spacer(modifier = Modifier.height(10.dp))
            Text(
                text = when {
                    isLive && (snapshot?.lastUpdated ?: 0L) > 0 ->
                        "实时链上数据 · ${formatTime(snapshot?.lastUpdated ?: 0L)}"
                    (snapshot?.lastUpdated ?: 0L) > 0 ->
                        "本地缓存，上次更新 ${formatTime(snapshot?.lastUpdated ?: 0L)}"
                    else -> "尚未获取链上数据，请点击刷新"
                },
                color = Color.White.copy(alpha = 0.55f),
                style = MaterialTheme.typography.labelSmall
            )

            Spacer(modifier = Modifier.height(12.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                if (currentStep != null) {
                    Button(
                        onClick = { onContinue(currentStep) },
                        enabled = busyStep == null,
                        colors = ButtonDefaults.buttonColors(
                            containerColor = McGold,
                            contentColor = McPrimary
                        )
                    ) {
                        if (busyStep != null) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(14.dp),
                                color = McPrimary,
                                strokeWidth = 2.dp
                            )
                            Spacer(modifier = Modifier.width(8.dp))
                        }
                        Text("继续第 ${currentStep.index} 步")
                    }
                }
                Spacer(modifier = Modifier.weight(1f))
                TextButton(onClick = onRefresh, enabled = !isRefreshing) {
                    if (isRefreshing) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(14.dp),
                            color = Color.White,
                            strokeWidth = 2.dp
                        )
                        Spacer(modifier = Modifier.width(6.dp))
                    }
                    Text(
                        text = if (isRefreshing) "刷新中…" else "刷新链上状态",
                        color = Color.White
                    )
                }
            }
        }
    }
}

@Composable
private fun HeaderStat(label: String, value: String) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = value,
            color = Color.White,
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold
        )
        Text(
            text = label,
            color = Color.White.copy(alpha = 0.7f),
            style = MaterialTheme.typography.labelSmall
        )
    }
}

// ----------------------------------------------------------------------
// 认证详情
// ----------------------------------------------------------------------

@Composable
private fun AttestationDetailCard(
    capability: AttestationCapability?,
    record: PhoneNodeRecord?,
    localInfo: NodeLocalInfo,
    now: Long,
) {
    val expiry = if (localInfo.attestationExpiry > 0) {
        localInfo.attestationExpiry
    } else {
        record?.attestationExpiry ?: 0L
    }
    val heartbeat = if (localInfo.lastHeartbeat > 0) {
        localInfo.lastHeartbeat
    } else {
        record?.lastHeartbeat ?: 0L
    }
    val nodeAddress = record?.nodeAddress ?: localInfo.registeredNodeAddress

    Card(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            SectionTitle("认证详情")

            DetailLine(
                label = "认证方式",
                value = capability?.summary ?: "正在检测设备硬件证明能力…",
                valueColor = when (capability?.level) {
                    "STRONGBOX", "TEE" -> StatusActive
                    "SOFTWARE" -> StatusGrace
                    else -> MaterialTheme.colorScheme.onSurface
                }
            )
            if (capability != null && capability.strongBoxSupported && capability.level != "STRONGBOX") {
                Text(
                    text = "设备声明支持 StrongBox，但当前证书安全等级为 ${levelText(capability.level)}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
                )
            }

            DetailLine(
                label = "认证时间",
                value = if ((record?.attestationTime ?: 0L) > 0) {
                    formatDateTime(record?.attestationTime ?: 0L)
                } else {
                    "暂无认证记录"
                }
            )
            DetailLine(
                label = "凭证有效期",
                value = expiryText(expiry, now),
                valueColor = when {
                    expiry <= 0 -> MaterialTheme.colorScheme.onSurface
                    expiry <= now -> MaterialTheme.colorScheme.error
                    expiry - now < 3L * 24 * 60 * 60 * 1000 -> StatusGrace
                    else -> StatusActive
                }
            )
            DetailLine(
                label = "最近心跳",
                value = relativeTime(heartbeat, now) +
                    "（每 ${MCParams.PHONENODE_HEARTBEAT_INTERVAL_SECONDS} 秒一次）"
            )

            if (record?.status == NodeStatus.ACTIVE && heartbeat > 0 &&
                now - heartbeat > MCParams.PHONENODE_OFFLINE_GRACE_SECONDS * 1000L
            ) {
                Text(
                    text = "心跳已超过 ${MCParams.PHONENODE_OFFLINE_GRACE_SECONDS} 秒宽限阈值，链上可能已判定离线",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.error
                )
            }

            Spacer(modifier = Modifier.height(8.dp))
            CopyableLine("设备 ID", localInfo.deviceId)
            CopyableLine(
                label = "节点地址",
                value = nodeAddress.orEmpty(),
                placeholder = "尚未注册节点",
            )
        }
    }
}

@Composable
private fun CopyableLine(label: String, value: String, placeholder: String = "暂无数据") {
    val clipboard = LocalClipboardManager.current
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
            )
            Text(
                text = if (value.isBlank()) placeholder else shortenMiddle(value),
                style = MaterialTheme.typography.bodySmall,
                fontFamily = if (value.isBlank()) null else FontFamily.Monospace,
                color = MaterialTheme.colorScheme.onSurface
            )
        }
        if (value.isNotBlank()) {
            IconButton(onClick = { clipboard.setText(AnnotatedString(value)) }) {
                Icon(
                    imageVector = Icons.Filled.ContentCopy,
                    contentDescription = "复制$label",
                    modifier = Modifier.size(16.dp),
                    tint = MaterialTheme.colorScheme.primary
                )
            }
        }
    }
}

// ----------------------------------------------------------------------
// 认证流程分步引导
// ----------------------------------------------------------------------

@Composable
private fun PipelineCard(
    steps: List<AuthStep>,
    busyStep: AuthStepId?,
    message: String?,
    messageIsError: Boolean,
    onRun: (AuthStep) -> Unit,
) {
    Card(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            SectionTitle("认证流程")
            Text(
                text = "点击任一步骤可执行对应的真实链上操作；前置步骤未完成时会提示先完成哪一步。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
            )
            Spacer(modifier = Modifier.height(8.dp))

            steps.forEach { step ->
                StepRow(
                    step = step,
                    busy = busyStep == step.id,
                    enabled = busyStep == null,
                    onClick = { onRun(step) }
                )
            }

            if (message != null) {
                Spacer(modifier = Modifier.height(8.dp))
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(10.dp),
                    color = if (messageIsError) {
                        MaterialTheme.colorScheme.errorContainer
                    } else {
                        MaterialTheme.colorScheme.surfaceVariant
                    }
                ) {
                    Text(
                        text = message,
                        modifier = Modifier.padding(horizontal = 10.dp, vertical = 8.dp),
                        style = MaterialTheme.typography.bodySmall,
                        color = if (messageIsError) {
                            MaterialTheme.colorScheme.onErrorContainer
                        } else {
                            MaterialTheme.colorScheme.onSurface
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun StepRow(
    step: AuthStep,
    busy: Boolean,
    enabled: Boolean,
    onClick: () -> Unit,
) {
    val color = when (step.state) {
        StepState.DONE -> StatusActive
        StepState.CURRENT -> StatusGrace
        StepState.TODO -> MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f)
    }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .clickable(enabled = enabled, onClick = onClick)
            .padding(vertical = 8.dp, horizontal = 4.dp),
        verticalAlignment = Alignment.Top
    ) {
        Box(
            modifier = Modifier
                .size(26.dp)
                .clip(CircleShape)
                .background(color.copy(alpha = 0.16f)),
            contentAlignment = Alignment.Center
        ) {
            Text(
                text = "${step.index}",
                color = color,
                style = MaterialTheme.typography.labelMedium,
                fontWeight = FontWeight.Bold
            )
        }
        Spacer(modifier = Modifier.width(12.dp))
        Column(modifier = Modifier.weight(1f)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = step.title,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = when (step.state) {
                        StepState.DONE -> "已完成"
                        StepState.CURRENT -> "进行中"
                        StepState.TODO -> "未开始"
                    },
                    style = MaterialTheme.typography.labelSmall,
                    color = color
                )
            }
            Text(
                text = step.description,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
            )
            Text(
                text = step.detail,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.75f)
            )
        }
        Spacer(modifier = Modifier.width(8.dp))
        if (busy) {
            CircularProgressIndicator(
                modifier = Modifier.size(18.dp),
                strokeWidth = 2.dp,
                color = MaterialTheme.colorScheme.primary
            )
        } else if (step.state == StepState.DONE) {
            Icon(
                imageVector = Icons.Filled.CheckCircle,
                contentDescription = "已完成",
                modifier = Modifier.size(20.dp),
                tint = StatusActive
            )
        } else {
            Icon(
                imageVector = Icons.Filled.ChevronRight,
                contentDescription = "执行该步骤",
                modifier = Modifier.size(20.dp),
                tint = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.3f)
            )
        }
    }
}

// ----------------------------------------------------------------------
// 硬件自检
// ----------------------------------------------------------------------

@Composable
private fun HardwareCard(hw: HardwareCheck?) {
    Card(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = Icons.Filled.Memory,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp),
                    tint = MaterialTheme.colorScheme.primary
                )
                Spacer(modifier = Modifier.width(6.dp))
                Text(
                    text = "硬件自检",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold
                )
            }
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "由系统接口实时采集：内存总量取自系统内存信息服务，可用存储取自文件系统统计，处理器核心数取自运行时环境。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
            )
            Spacer(modifier = Modifier.height(10.dp))

            if (hw == null) {
                Text(
                    text = "正在读取硬件信息…",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                )
            } else {
                HardwareMetric(
                    label = "内存（总）",
                    value = "${hw.ramGb} GB",
                    threshold = "要求 ≥ 4 GB",
                    passed = hw.ramGb >= 4
                )
                HardwareMetric(
                    label = "可用存储",
                    value = "${hw.storageGb} GB",
                    threshold = "要求 ≥ 8 GB",
                    passed = hw.storageGb >= 8
                )
                HardwareMetric(
                    label = "处理器核心",
                    value = "${hw.cpuCores} 核",
                    threshold = "要求 ≥ 4 核",
                    passed = hw.cpuCores >= 4
                )

                val unmet = buildList {
                    if (hw.ramGb < 4) add("运行内存不足 4 GB（当前 ${hw.ramGb} GB）")
                    if (hw.storageGb < 8) add("可用存储不足 8 GB（当前 ${hw.storageGb} GB）")
                    if (hw.cpuCores < 4) add("处理器核心不足 4 核（当前 ${hw.cpuCores} 核）")
                }

                Spacer(modifier = Modifier.height(6.dp))
                Text(
                    text = if (unmet.isEmpty()) "硬件自检通过" else "存在未达标项（${unmet.size} 项）",
                    style = MaterialTheme.typography.bodySmall,
                    fontWeight = FontWeight.Medium,
                    color = if (unmet.isEmpty()) StatusActive else StatusSlashed
                )
                unmet.forEach { issue ->
                    Text(
                        text = "· $issue",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.error
                    )
                }
            }
        }
    }
}

@Composable
private fun HardwareMetric(label: String, value: String, threshold: String, passed: Boolean) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Column(modifier = Modifier.weight(1f)) {
            Text(text = label, style = MaterialTheme.typography.bodySmall)
            Text(
                text = threshold,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
            )
        }
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Bold,
            color = if (passed) StatusActive else StatusSlashed
        )
        Spacer(modifier = Modifier.width(6.dp))
        Text(
            text = if (passed) "达标" else "不足",
            style = MaterialTheme.typography.labelSmall,
            color = if (passed) StatusActive else StatusSlashed
        )
    }
}

// ----------------------------------------------------------------------
// 链上参数、惩罚规则与质押/关押状态
// ----------------------------------------------------------------------

@Composable
private fun ChainRulesCard(record: PhoneNodeRecord?) {
    Card(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            SectionTitle("链上参数与惩罚规则")

            RuleRow("认证有效期", "${MCParams.PHONENODE_ATTESTATION_VALIDITY_DAYS} 天")
            RuleRow(
                "本机心跳",
                "${MCParams.PHONENODE_HEARTBEAT_INTERVAL_SECONDS} 秒"
            )
            RuleRow(
                "离线宽限",
                "${MCParams.PHONENODE_OFFLINE_GRACE_SECONDS} 秒（超时后进入宽限期）"
            )
            RuleRow(
                "节点最小质押",
                String.format(Locale.getDefault(), "%.0f MC", MCParams.PHONENODE_MIN_STAKE_UMC / 1_000_000.0)
            )

            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "惩罚规则",
                style = MaterialTheme.typography.labelMedium,
                fontWeight = FontWeight.Medium
            )
            SlashRuleRow(
                "离线超过宽限期未心跳",
                "${MCParams.PHONENODE_OFFLINE_SLASH_PERCENT}%",
                StatusGrace
            )
            SlashRuleRow(
                "贡献数据作弊",
                "${MCParams.PHONENODE_CONTRIB_SLASH_PERCENT}%",
                StatusJailed
            )
            SlashRuleRow(
                "认证材料造假",
                "${MCParams.PHONENODE_ATTEST_SLASH_PERCENT}%",
                StatusSlashed
            )

            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "当前节点数据",
                style = MaterialTheme.typography.labelMedium,
                fontWeight = FontWeight.Medium
            )
            if (record == null) {
                Text(
                    text = "尚未注册节点，暂无质押、关押与信誉数据。",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                )
            } else {
                RuleRow(
                    "质押数量",
                    if (record.stakedAmount > 0) {
                        String.format(Locale.getDefault(), "%.2f MC", record.stakedAmount / 1_000_000.0)
                    } else {
                        "链上未返回质押数据"
                    }
                )
                RuleRow("罚没次数", "${record.slashCount} 次")
                RuleRow("信誉分", "${record.reputation}")
                RuleRow("节点当前状态", statusTitle(record.status))
            }

            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = "说明：手机端不运行全节点、不参与工作量证明（PoW）出块，仅做轻节点状态查询与贡献计量。认证过期或离线超时将失去接单与计奖资格。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
            )
        }
    }
}

@Composable
private fun ChainAttestationCard(
    attestation: ChainAttestation?,
    slashes: List<ChainSlash>,
    error: String?,
) {
    Card(shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    Icons.Filled.VerifiedUser,
                    contentDescription = null,
                    tint = McHighlight,
                    modifier = Modifier.size(20.dp),
                )
                Spacer(Modifier.width(8.dp))
                Text(
                    "链上认证与罚没",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                )
            }
            Spacer(Modifier.height(8.dp))
            if (attestation == null) {
                Text(
                    error ?: "链上暂无认证记录",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
            } else {
                RuleRow("链上状态", chainAttestationStatusLabel(attestation.status))
                RuleRow("到期时间", formatChainTime(attestation.expirySeconds))
                RuleRow("设备指纹", shortDeviceHash(attestation.deviceIdHash))
            }
            Spacer(Modifier.height(12.dp))
            Text(
                "罚没记录（${slashes.size}）",
                style = MaterialTheme.typography.labelMedium,
                fontWeight = FontWeight.Medium,
            )
            if (slashes.isEmpty()) {
                Spacer(Modifier.height(4.dp))
                Text(
                    "链上暂无罚没记录",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
            } else {
                slashes.take(5).forEach { slash ->
                    Spacer(Modifier.height(4.dp))
                    Text(
                        formatChainTime(slash.timeSeconds) + " · " +
                            slashReasonLabel(slash.reason) + " · " +
                            String.format(Locale.getDefault(), "%.1f%%", slash.penaltyBps / 100.0),
                        style = MaterialTheme.typography.bodySmall,
                        color = StatusSlashed,
                    )
                }
            }
        }
    }
}

private fun formatChainTime(epochSeconds: Long): String =
    if (epochSeconds <= 0) {
        "—"
    } else {
        SimpleDateFormat("yyyy-MM-dd HH:mm", Locale.getDefault()).format(Date(epochSeconds * 1000))
    }

private fun shortDeviceHash(hash: String): String {
    if (hash.isBlank()) return "—"
    return if (hash.length <= 20) hash else hash.take(10) + "..." + hash.takeLast(6)
}

private fun chainAttestationStatusLabel(status: String): String = when (status.lowercase()) {
    "active", "attested" -> "已认证"
    "pending" -> "认证中"
    "revoked", "slashed" -> "已吊销"
    else -> status.ifBlank { "未知" }
}

private fun slashReasonLabel(reason: String): String = when {
    reason.contains("offline", ignoreCase = true) -> "离线"
    reason.contains("contrib", ignoreCase = true) -> "贡献作弊"
    reason.contains("attest", ignoreCase = true) -> "认证失败"
    reason.contains("double", ignoreCase = true) -> "双签"
    reason.isBlank() -> "未知原因"
    else -> reason
}

@Composable
private fun RuleRow(label: String, value: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 3.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f)
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodySmall,
            fontWeight = FontWeight.Medium
        )
    }
}

@Composable
private fun SlashRuleRow(reason: String, percent: String, color: Color) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 2.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = reason,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f)
        )
        Text(
            text = percent,
            style = MaterialTheme.typography.bodySmall,
            fontWeight = FontWeight.Bold,
            color = color
        )
    }
}

// ----------------------------------------------------------------------
// RPC 引导 / 对等节点
// ----------------------------------------------------------------------

@Composable
private fun RpcGuideCard(error: String, onOpenSettings: () -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = Icons.Filled.Warning,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp),
                    tint = MaterialTheme.colorScheme.onErrorContainer
                )
                Spacer(modifier = Modifier.width(6.dp))
                Text(
                    text = "链上数据不可用",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onErrorContainer
                )
            }
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = error,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onErrorContainer
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = "配置路径：设置 → RPC 端点 → 填写节点地址并点击「测试连接」。",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onErrorContainer.copy(alpha = 0.8f)
            )
            Spacer(modifier = Modifier.height(10.dp))
            Button(
                onClick = onOpenSettings,
                colors = ButtonDefaults.buttonColors(
                    containerColor = MaterialTheme.colorScheme.onErrorContainer,
                    contentColor = MaterialTheme.colorScheme.errorContainer
                )
            ) {
                Icon(
                    imageVector = Icons.Filled.Settings,
                    contentDescription = null,
                    modifier = Modifier.size(16.dp)
                )
                Spacer(modifier = Modifier.width(6.dp))
                Text("去设置页配置 RPC")
            }
        }
    }
}

@Composable
private fun PeerItem(peer: PeerNode) {
    Card(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp)) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = peer.moniker.ifBlank { shortenMiddle(peer.nodeId) },
                    fontWeight = FontWeight.Medium,
                    style = MaterialTheme.typography.bodyMedium
                )
                Text(
                    text = shortenMiddle(peer.address),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(
                    text = "${peer.latency} 毫秒",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (peer.latency < 100) StatusActive else StatusGrace
                )
                Text(
                    text = SimpleDateFormat("HH:mm", Locale.getDefault()).format(Date(peer.lastSeen)),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.3f)
                )
            }
        }
    }
}

// ----------------------------------------------------------------------
// 通用小组件与格式化
// ----------------------------------------------------------------------

@Composable
private fun SectionTitle(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleSmall,
        fontWeight = FontWeight.Bold
    )
    Spacer(modifier = Modifier.height(8.dp))
}

@Composable
private fun DetailLine(label: String, value: String, valueColor: Color = MaterialTheme.colorScheme.onSurface) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.Top
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f)
        )
        Spacer(modifier = Modifier.width(12.dp))
        Text(
            text = value,
            style = MaterialTheme.typography.bodySmall,
            color = valueColor,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.weight(1f, fill = false)
        )
    }
}

private fun statusTitle(status: NodeStatus?): String = when (status) {
    NodeStatus.ACTIVE -> "在线"
    NodeStatus.PENDING -> "认证中"
    NodeStatus.GRACE -> "宽限期"
    NodeStatus.JAILED -> "已关押"
    NodeStatus.SLASHED -> "已罚没"
    NodeStatus.UNREGISTERED, null -> "未注册"
}

private fun levelText(level: String): String = when (level.uppercase()) {
    "STRONGBOX" -> "StrongBox（独立安全芯片）"
    "TEE" -> "TEE（可信执行环境）"
    "SOFTWARE" -> "软件密钥"
    else -> "未知等级"
}

private fun formatTime(millis: Long): String =
    SimpleDateFormat("MM-dd HH:mm", Locale.getDefault()).format(Date(millis))

private fun formatDateTime(millis: Long): String =
    SimpleDateFormat("yyyy-MM-dd HH:mm", Locale.getDefault()).format(Date(millis))

/** 相对时间：刚刚 / X 分钟前 / X 小时 X 分钟前 / X 天前。 */
private fun relativeTime(timestamp: Long, now: Long): String {
    if (timestamp <= 0L) return "暂无记录"
    val diff = (now - timestamp).coerceAtLeast(0L)
    val minutes = diff / 60_000L
    return when {
        diff < 60_000L -> "刚刚"
        minutes < 60L -> "$minutes 分钟前"
        minutes < 1440L -> "${minutes / 60} 小时 ${minutes % 60} 分钟前"
        else -> "${minutes / 1440} 天前"
    }
}

/** 凭证有效期：剩余 X 天 X 小时 / 已过期。 */
private fun expiryText(expiry: Long, now: Long): String {
    if (expiry <= 0L) return "暂无有效期记录"
    val diff = expiry - now
    if (diff <= 0L) return "已过期"
    val days = diff / (24L * 60 * 60 * 1000)
    val hours = (diff % (24L * 60 * 60 * 1000)) / (60L * 60 * 1000)
    return "剩余 $days 天 $hours 小时"
}

/** 地址/ID 中间截断，便于在窄屏上同时看清首尾。 */
private fun shortenMiddle(value: String): String {
    if (value.length <= 18) return value
    return value.take(10) + "…" + value.takeLast(8)
}
