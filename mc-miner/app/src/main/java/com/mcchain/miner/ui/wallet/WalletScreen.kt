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
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.ui.theme.*

@Composable
fun WalletScreen(viewModel: WalletViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
    val accounts by viewModel.accounts.collectAsState()
    val transactions by viewModel.transactions.collectAsState()
    val backupMnemonic by viewModel.backupMnemonic.collectAsState()
    val busy by viewModel.busy.collectAsState()
    val message by viewModel.message.collectAsState()

    val defaultAccount = accounts.firstOrNull { it.isDefault }
    val clipboardManager = LocalClipboardManager.current

    var showSendDialog by remember { mutableStateOf(false) }
    var showImportDialog by remember { mutableStateOf(false) }

    if (backupMnemonic != null) {
        AlertDialog(
            onDismissRequest = { viewModel.discardBackup() },
            title = { Text("备份助记词") },
            text = {
                Column {
                    Text(
                        "请抄写并离线保存以下助记词。任何人获得它都能控制钱包资产；App 不会再次展示。",
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Spacer(Modifier.height(12.dp))
                    Text(
                        backupMnemonic!!.joinToString("  "),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                    )
                }
            },
            confirmButton = {
                TextButton(onClick = { viewModel.confirmCreateWallet() }) {
                    Text("我已安全备份")
                }
            },
            dismissButton = {
                TextButton(onClick = { viewModel.discardBackup() }) { Text("取消") }
            },
        )
    }

    if (showImportDialog) {
        var importText by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { showImportDialog = false },
            title = { Text("导入钱包") },
            text = {
                OutlinedTextField(
                    value = importText,
                    onValueChange = { importText = it },
                    label = { Text("助记词（空格分隔）") },
                    minLines = 3,
                )
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        viewModel.importWallet(importText)
                        showImportDialog = false
                    }
                ) { Text("导入") }
            },
            dismissButton = {
                TextButton(onClick = { showImportDialog = false }) { Text("取消") }
            },
        )
    }

    if (showSendDialog) {
        var toAddress by remember { mutableStateOf("") }
        var amountMc by remember { mutableStateOf("") }
        AlertDialog(
            onDismissRequest = { showSendDialog = false },
            title = { Text("发送 MC") },
            text = {
                Column {
                    OutlinedTextField(
                        value = toAddress,
                        onValueChange = { toAddress = it },
                        label = { Text("收款地址（mc1...）") },
                        singleLine = true,
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = amountMc,
                        onValueChange = { amountMc = it },
                        label = { Text("金额（MC，最多 6 位小数）") },
                        singleLine = true,
                    )
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !busy,
                    onClick = {
                        viewModel.send(toAddress, amountMc)
                        showSendDialog = false
                    }
                ) { Text("确认发送") }
            },
            dismissButton = {
                TextButton(onClick = { showSendDialog = false }) { Text("取消") }
            },
        )
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        message?.let { text ->
            item {
                Card(
                    shape = RoundedCornerShape(12.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.surfaceVariant
                    )
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(12.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text,
                            modifier = Modifier.weight(1f),
                            style = MaterialTheme.typography.bodySmall,
                        )
                        TextButton(onClick = { viewModel.clearMessage() }) { Text("关闭") }
                    }
                }
            }
        }

        if (defaultAccount == null) {
            item {
                Card(shape = RoundedCornerShape(16.dp)) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text(
                            "还没有钱包",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                        )
                        Spacer(Modifier.height(8.dp))
                        Text(
                            "私钥只保存在本机加密存储中。请创建新钱包或导入已有助记词。",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                        )
                        Spacer(Modifier.height(12.dp))
                        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            FilledTonalButton(
                                onClick = { viewModel.createWallet() },
                                enabled = !busy,
                                modifier = Modifier.weight(1f),
                            ) { Text("创建钱包") }
                            OutlinedButton(
                                onClick = { showImportDialog = true },
                                enabled = !busy,
                                modifier = Modifier.weight(1f),
                            ) { Text("导入钱包") }
                        }
                    }
                }
            }
        }

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

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp)
                    ) {
                        Text(
                            text = defaultAccount?.address?.let { it.take(16) + "..." + it.takeLast(8) } ?: "无地址",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Spacer(modifier = Modifier.weight(1f))
                        IconButton(
                            onClick = {
                                defaultAccount?.address?.let {
                                    clipboardManager.setText(AnnotatedString(it))
                                }
                            }
                        ) {
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
                    onClick = { if (defaultAccount != null) showSendDialog = true },
                    enabled = !busy && defaultAccount != null,
                    modifier = Modifier.weight(1f)
                ) {
                    Icon(Icons.Filled.Send, null, Modifier.size(18.dp))
                    Spacer(modifier = Modifier.width(4.dp))
                    Text("发送")
                }
                FilledTonalButton(
                    onClick = {
                        defaultAccount?.address?.let {
                            clipboardManager.setText(AnnotatedString(it))
                            viewModel.clearMessage()
                        }
                    },
                    enabled = defaultAccount != null,
                    modifier = Modifier.weight(1f)
                ) {
                    Icon(Icons.Filled.Download, null, Modifier.size(18.dp))
                    Spacer(modifier = Modifier.width(4.dp))
                    Text("收款地址")
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
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Column {
                        Text("质押余额", style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
                        Text(String.format("%.2f MC", (defaultAccount?.stakeBalance ?: 0L) / 1_000_000.0),
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold)
                    }
                    Column(horizontalAlignment = Alignment.End) {
                        Text("待领奖励", style = MaterialTheme.typography.labelMedium,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
                        Text(String.format("%.6f MC", (defaultAccount?.rewardBalance ?: 0L) / 1_000_000.0),
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold,
                            color = McGold)
                        TextButton(
                            onClick = { viewModel.claimRewards() },
                            enabled = !busy && (defaultAccount?.rewardBalance ?: 0L) > 0,
                        ) {
                            Text(if (busy) "处理中..." else "领取")
                        }
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
