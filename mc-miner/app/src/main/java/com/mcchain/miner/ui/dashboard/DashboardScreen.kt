package com.mcchain.miner.ui.dashboard

import androidx.compose.animation.*
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material.icons.outlined.RadioButtonUnchecked
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.BlockInfo
import com.mcchain.miner.data.model.NodeStatus
import com.mcchain.miner.data.model.NodeStatusSnapshot
import com.mcchain.miner.data.model.PhoneNodeRecord
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.ui.theme.*
@Composable
fun DashboardScreen(
    onOpenWallet: () -> Unit = {},
    onOpenNode: () -> Unit = {},
    onOpenMining: () -> Unit = {},
    viewModel: DashboardViewModel = androidx.lifecycle.viewmodel.compose.viewModel()
) {
    val nodeStatus by viewModel.nodeStatus.collectAsState()
    val phoneNode by viewModel.phoneNode.collectAsState()
    val account by viewModel.account.collectAsState()
    val totalReward by viewModel.totalReward.collectAsState()
    val recentBlocks by viewModel.recentBlocks.collectAsState()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // 顶部状态栏
        item {
            NodeStatusCard(nodeStatus, phoneNode)
        }

        // 首启接入进度
        item {
            OnboardingCard(
                account = account,
                phoneNode = phoneNode,
                onOpenWallet = onOpenWallet,
                onOpenNode = onOpenNode,
                onOpenMining = onOpenMining
            )
        }

        // 余额卡片
        item {
            BalanceCard(account, totalReward)
        }

        // 快捷操作
        item {
            QuickActions(onOpenWallet)
        }

        // 链参数速览
        item {
            ChainParamsCard()
        }

        // 最近区块
        item {
            Text("最近区块", style = MaterialTheme.typography.titleMedium)
        }
        items(recentBlocks.size) { index ->
            BlockItem(recentBlocks[index])
        }
    }
}

@Composable
fun NodeStatusCard(status: NodeStatusSnapshot?, phoneNode: PhoneNodeRecord?) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = McPrimary),
        shape = RoundedCornerShape(16.dp)
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(12.dp)
                        .clip(CircleShape)
                        .background(
                            when (phoneNode?.status) {
                                NodeStatus.ACTIVE -> StatusActive
                                NodeStatus.GRACE -> StatusGrace
                                NodeStatus.JAILED -> StatusJailed
                                else -> StatusPending
                            }
                        )
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = when (phoneNode?.status) {
                        NodeStatus.ACTIVE -> "在线参与中"
                        NodeStatus.GRACE -> "宽限期"
                        NodeStatus.JAILED -> "已关押"
                        NodeStatus.PENDING -> "认证中"
                        else -> "未注册"
                    },
                    color = Color.White,
                    style = MaterialTheme.typography.titleMedium
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceEvenly
            ) {
                StatItem("区块高度", "#${status?.currentHeight ?: 0}", Color.White)
                StatItem("同步节点", "${status?.syncedPeers ?: 0}", Color.White)
                StatItem("链ID", MCParams.CHAIN_ID, Color.White)
            }

            if (status == null || status.currentHeight <= 0) {
                Spacer(modifier = Modifier.height(12.dp))
                Text(
                    "未连接链上节点：请到「设置 → RPC 端点」填写可达节点并测试连接",
                    style = MaterialTheme.typography.labelSmall,
                    color = McGold
                )
            }
        }
    }
}

@Composable
fun BalanceCard(account: WalletAccount?, totalReward: Long) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            Text("总资产", style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))

            Spacer(modifier = Modifier.height(4.dp))

            Row(verticalAlignment = Alignment.Bottom) {
                Text(
                    text = String.format("%.2f", (account?.balance ?: 0L) / 1_000_000.0),
                    style = MaterialTheme.typography.headlineLarge,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Spacer(modifier = Modifier.width(4.dp))
                Text("MC", style = MaterialTheme.typography.titleMedium,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    modifier = Modifier.padding(bottom = 6.dp))
            }

            Spacer(modifier = Modifier.height(12.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                Column {
                    Text("质押", style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
                    Text(String.format("%.2f MC", (account?.stakeBalance ?: 0L) / 1_000_000.0),
                        style = MaterialTheme.typography.bodyMedium)
                }
                Column {
                    Text("待领委托奖励", style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
                    Text(String.format("%.2f MC", totalReward / 1_000_000.0),
                        style = MaterialTheme.typography.bodyMedium,
                        color = StatusActive)
                }
            }
        }
    }
}

@Composable
fun OnboardingCard(
    account: WalletAccount?,
    phoneNode: PhoneNodeRecord?,
    onOpenWallet: () -> Unit,
    onOpenNode: () -> Unit,
    onOpenMining: () -> Unit
) {
    val walletDone = account != null
    val nodeDone = phoneNode?.status == NodeStatus.ACTIVE
    // 未完成钱包与认证时不把「服务在跑」算作已开始贡献，避免误导。
    val miningDone = walletDone && nodeDone && com.mcchain.miner.service.MinerService.isRunning
    val doneCount = listOf(walletDone, nodeDone, miningDone).count { it }

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text("接入进度", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.Bold)
                Text("$doneCount / 3", style = MaterialTheme.typography.labelMedium,
                    color = MaterialTheme.colorScheme.primary)
            }
            Spacer(modifier = Modifier.height(8.dp))
            LinearProgressIndicator(
                progress = doneCount / 3f,
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(modifier = Modifier.height(12.dp))
            OnboardingStep(
                index = 1,
                title = "创建或导入钱包",
                done = walletDone,
                actionText = if (walletDone) "已完成" else "去创建",
                enabled = !walletDone,
                onClick = onOpenWallet
            )
            OnboardingStep(
                index = 2,
                title = "完成设备认证",
                done = nodeDone,
                actionText = if (nodeDone) "已在线" else if (walletDone) "去认证" else "需先创建钱包",
                enabled = walletDone && !nodeDone,
                onClick = onOpenNode
            )
            OnboardingStep(
                index = 3,
                title = "开始贡献资源",
                done = miningDone,
                actionText = if (miningDone) "运行中" else if (nodeDone) "去开启" else "需先完成认证",
                enabled = nodeDone && !miningDone,
                onClick = onOpenMining
            )
        }
    }
}

@Composable
private fun OnboardingStep(
    index: Int,
    title: String,
    done: Boolean,
    actionText: String,
    enabled: Boolean,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(enabled = enabled, onClick = onClick)
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = if (done) Icons.Filled.CheckCircle else Icons.Outlined.RadioButtonUnchecked,
            contentDescription = null,
            tint = if (done) StatusActive else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.35f)
        )
        Spacer(modifier = Modifier.width(10.dp))
        Text(
            text = "$index. $title",
            style = MaterialTheme.typography.bodyMedium,
            modifier = Modifier.weight(1f),
            color = if (done) MaterialTheme.colorScheme.onSurface
            else MaterialTheme.colorScheme.onSurface.copy(alpha = 0.8f)
        )
        Text(
            text = actionText,
            style = MaterialTheme.typography.labelSmall,
            color = when {
                done -> StatusActive
                enabled -> MaterialTheme.colorScheme.primary
                else -> MaterialTheme.colorScheme.onSurface.copy(alpha = 0.4f)
            }
        )
    }
}

@Composable
fun QuickActions(onOpenWallet: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        ActionButton(Icons.Filled.Send, "转账", Modifier.weight(1f), onOpenWallet)
        ActionButton(Icons.Filled.Download, "领取收益", Modifier.weight(1f), onOpenWallet)
        ActionButton(Icons.Filled.AccountBalanceWallet, "钱包", Modifier.weight(1f), onOpenWallet)
    }
}

@Composable
fun ActionButton(icon: ImageVector, label: String, modifier: Modifier = Modifier, onClick: () -> Unit = {}) {
    Card(
        modifier = modifier.clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface)
    ) {
        Column(
            modifier = Modifier.padding(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Icon(icon, contentDescription = label, tint = MaterialTheme.colorScheme.primary)
            Spacer(modifier = Modifier.height(4.dp))
            Text(label, style = MaterialTheme.typography.labelSmall)
        }
    }
}

@Composable
fun ChainParamsCard() {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text("链参数", style = MaterialTheme.typography.titleSmall,
                fontWeight = FontWeight.Bold)
            Spacer(modifier = Modifier.height(12.dp))
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                ParamRow("总供应量", "10亿 MC")
                ParamRow("区块时间", "${MCParams.BLOCK_TIME_SECONDS}s")
            }
            Spacer(modifier = Modifier.height(8.dp))
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                ParamRow("验证人数", MCParams.MAX_VALIDATORS.toString())
                ParamRow("解绑期", "${MCParams.UNBONDING_DAYS}天")
            }
            Spacer(modifier = Modifier.height(8.dp))
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                ParamRow("离线宽限", "${MCParams.PHONENODE_OFFLINE_GRACE_SECONDS}s")
                ParamRow("本机心跳", "${MCParams.PHONENODE_HEARTBEAT_INTERVAL_SECONDS}s")
            }
        }
    }
}

@Composable
fun ParamRow(label: String, value: String) {
    Column {
        Text(label, style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
        Text(value, style = MaterialTheme.typography.bodyMedium,
            fontWeight = FontWeight.Medium)
    }
}

@Composable
fun StatItem(label: String, value: String, textColor: Color) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(value, style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold, color = textColor)
        Text(label, style = MaterialTheme.typography.labelSmall,
            color = textColor.copy(alpha = 0.7f))
    }
}

@Composable
fun BlockItem(block: BlockInfo) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column {
                Text("#${block.height}", fontWeight = FontWeight.Bold)
                Text(
                    "${block.numTxs} 笔交易",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
                )
            }
            Text(
                java.text.SimpleDateFormat("HH:mm:ss", java.util.Locale.getDefault())
                    .format(java.util.Date(block.time * 1000)),
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
            )
        }
    }
}
