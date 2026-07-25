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
fun DashboardScreen(viewModel: DashboardViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
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

        // 余额卡片
        item {
            BalanceCard(account, totalReward)
        }

        // 快捷操作
        item {
            QuickActions()
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
                        NodeStatus.ACTIVE -> "节点运行中"
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
                    Text("挖矿收益", style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
                    Text(String.format("%.2f MC", totalReward / 1_000_000.0),
                        style = MaterialTheme.typography.bodyMedium,
                        color = McGold)
                }
                Column {
                    Text("待领奖励", style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
                    Text(String.format("%.2f MC", (account?.rewardBalance ?: 0L) / 1_000_000.0),
                        style = MaterialTheme.typography.bodyMedium,
                        color = StatusActive)
                }
            }
        }
    }
}

@Composable
fun QuickActions() {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(12.dp)
    ) {
        ActionButton(Icons.Filled.Send, "转账", Modifier.weight(1f))
        ActionButton(Icons.Filled.AddCircle, "质押", Modifier.weight(1f))
        ActionButton(Icons.Filled.Download, "领取收益", Modifier.weight(1f))
        ActionButton(Icons.Filled.SwapHoriz, "兑换", Modifier.weight(1f))
    }
}

@Composable
fun ActionButton(icon: ImageVector, label: String, modifier: Modifier = Modifier) {
    Card(
        modifier = modifier.clickable { },
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
                ParamRow("心跳间隔", "${MCParams.PHONENODE_HEARTBEAT_INTERVAL_SECONDS}s")
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
