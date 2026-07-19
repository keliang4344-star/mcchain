package com.mcchain.miner.ui.wallet

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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.ui.theme.*
@Composable
fun WalletScreen(viewModel: WalletViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
    val accounts by viewModel.accounts.collectAsState()
    val transactions by viewModel.transactions.collectAsState()

    val defaultAccount = accounts.firstOrNull { it.isDefault }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // 余额头图
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
                    Text("总余额", color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f))
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        text = String.format("%.6f MC", (defaultAccount?.balance ?: 0L) / 1_000_000.0),
                        style = MaterialTheme.typography.headlineLarge,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                    Spacer(modifier = Modifier.height(16.dp))

                    // 地址
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp)
                    ) {
                        Text(
                            text = defaultAccount?.address?.take(16) + "..." + defaultAccount?.address?.takeLast(8) ?: "无地址",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Spacer(modifier = Modifier.weight(1f))
                        IconButton(onClick = { /* 复制 */ }) {
                            Icon(Icons.Filled.ContentCopy, "复制地址",
                                tint = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f))
                        }
                    }
                }
            }
        }

        // 操作按钮
        item {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                FilledTonalButton(
                    onClick = { },
                    modifier = Modifier.weight(1f)
                ) {
                    Icon(Icons.Filled.Send, null, Modifier.size(18.dp))
                    Spacer(modifier = Modifier.width(4.dp))
                    Text("发送")
                }
                FilledTonalButton(
                    onClick = { },
                    modifier = Modifier.weight(1f)
                ) {
                    Icon(Icons.Filled.Download, null, Modifier.size(18.dp))
                    Spacer(modifier = Modifier.width(4.dp))
                    Text("接收")
                }
            }
        }

        // 质押信息
        item {
            Card(shape = RoundedCornerShape(16.dp)) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    horizontalArrangement = Arrangement.SpaceBetween
                ) {
                    Column {
                        Text("质押余额", style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
                        Text(String.format("%.2f MC", (defaultAccount?.stakeBalance ?: 0L) / 1_000_000.0),
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold)
                    }
                    Column {
                        Text("待领奖励", style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
                        Text(String.format("%.6f MC", (defaultAccount?.rewardBalance ?: 0L) / 1_000_000.0),
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                            color = McGold)
                    }
                }
            }
        }

        // 交易历史
        item {
            Text("交易历史", style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold)
        }

        items(transactions.size) { index ->
            TransactionItem(transactions[index])
        }
    }
}

@Composable
fun TransactionItem(tx: TxRecord) {
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
            Icon(
                imageVector = when (tx.type) {
                    com.mcchain.miner.data.model.TxType.SEND -> Icons.Filled.ArrowUpward
                    com.mcchain.miner.data.model.TxType.DELEGATE -> Icons.Filled.AddCircle
                    com.mcchain.miner.data.model.TxType.CLAIM_REWARDS -> Icons.Filled.CardGiftcard
                    else -> Icons.Filled.SwapHoriz
                },
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(32.dp)
            )
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = when (tx.type) {
                        com.mcchain.miner.data.model.TxType.SEND -> "转账"
                        com.mcchain.miner.data.model.TxType.DELEGATE -> "质押"
                        com.mcchain.miner.data.model.TxType.CLAIM_REWARDS -> "领取收益"
                        com.mcchain.miner.data.model.TxType.ATTEST_NODE -> "节点认证"
                        com.mcchain.miner.data.model.TxType.REPORT_CONTRIB -> "贡献上报"
                        com.mcchain.miner.data.model.TxType.SUBMIT_TASK -> "提交任务"
                        else -> "其他"
                    },
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = tx.hash.take(12) + "...",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(
                    text = String.format("%.2f MC", tx.amount / 1_000_000.0),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = if (tx.type == com.mcchain.miner.data.model.TxType.SEND) McHighlight else StatusActive
                )
                Text(
                    text = when (tx.status) {
                        com.mcchain.miner.data.model.TxStatus.SUCCESS -> "成功"
                        com.mcchain.miner.data.model.TxStatus.PENDING -> "确认中"
                        com.mcchain.miner.data.model.TxStatus.FAILED -> "失败"
                    },
                    style = MaterialTheme.typography.labelSmall,
                    color = when (tx.status) {
                        com.mcchain.miner.data.model.TxStatus.SUCCESS -> StatusActive
                        com.mcchain.miner.data.model.TxStatus.PENDING -> StatusPending
                        com.mcchain.miner.data.model.TxStatus.FAILED -> StatusSlashed
                    }
                )
            }
        }
    }
}
