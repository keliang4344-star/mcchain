package com.mcchain.miner.ui.wallet

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.content.Intent
import android.view.WindowManager
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.TxRecord
import com.mcchain.miner.data.model.TxStatus
import com.mcchain.miner.data.model.TxType
import com.mcchain.miner.data.model.WalletAccount
import com.mcchain.miner.network.Delegation
import com.mcchain.miner.ui.theme.*
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import java.math.BigDecimal
import java.text.DecimalFormat
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

private fun formatMcDisplay(umc: Long): String =
    DecimalFormat("#,##0.######").format(BigDecimal.valueOf(umc, 6)) + " MC"

private fun formatClock(timestamp: Long): String =
    SimpleDateFormat("HH:mm:ss", Locale.getDefault()).format(Date(timestamp))

private fun formatDateTime(timestamp: Long): String =
    SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.getDefault()).format(Date(timestamp))

@Composable
fun WalletScreen(viewModel: WalletViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
    val accounts by viewModel.accounts.collectAsState()
    val transactions by viewModel.transactions.collectAsState()
    val delegations by viewModel.delegations.collectAsState()
    val validators by viewModel.validators.collectAsState()
    val validatorsLoading by viewModel.validatorsLoading.collectAsState()
    val validatorError by viewModel.validatorError.collectAsState()
    val backupMnemonic by viewModel.backupMnemonic.collectAsState()
    val revealedMnemonic by viewModel.revealedMnemonic.collectAsState()
    val busy by viewModel.busy.collectAsState()
    val refreshing by viewModel.refreshing.collectAsState()
    val refreshError by viewModel.refreshError.collectAsState()
    val chainHint by viewModel.chainHint.collectAsState()
    val lastUpdated by viewModel.lastUpdated.collectAsState()
    val message by viewModel.message.collectAsState()
    val outcome by viewModel.outcome.collectAsState()
    val txDetail by viewModel.txDetail.collectAsState()

    val defaultAccount = accounts.firstOrNull { it.isDefault }
    val context = LocalContext.current
    val clipboardManager = LocalClipboardManager.current

    // 助记词展示期间禁止截屏/录屏，关闭弹窗后恢复。
    SecureWindow(enabled = backupMnemonic != null || revealedMnemonic != null)

    var showSendDialog by remember { mutableStateOf(false) }
    var showDelegateDialog by remember { mutableStateOf(false) }
    var undelegateTarget by remember { mutableStateOf<Delegation?>(null) }
    var redelegateTarget by remember { mutableStateOf<Delegation?>(null) }
    var showImportDialog by remember { mutableStateOf(false) }
    var showMnemonicConfirm by remember { mutableStateOf(false) }
    var addressCopied by remember { mutableStateOf(false) }

    LaunchedEffect(addressCopied) {
        if (addressCopied) {
            delay(1_800)
            addressCopied = false
        }
    }

    backupMnemonic?.let { words ->
        // 随机抽 3 个位置做抄写校验，避免「直接点我已备份」绕过。
        val quizIndices = remember(words) { (0 until words.size).shuffled().take(3).sorted() }
        val answers = remember(words) { mutableStateListOf("", "", "") }
        var quizError by remember(words) { mutableStateOf<String?>(null) }
        AlertDialog(
            onDismissRequest = { viewModel.discardBackup() },
            title = { Text("备份助记词") },
            text = {
                Column(
                    modifier = Modifier
                        .heightIn(max = 420.dp)
                        .verticalScroll(rememberScrollState()),
                ) {
                    Text(
                        "请抄写并离线保存以下助记词。任何人获得它都能控制钱包资产；App 不会再次展示。",
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Spacer(Modifier.height(12.dp))
                    Text(
                        words.joinToString("  "),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                    )
                    Spacer(Modifier.height(12.dp))
                    Text(
                        "请按顺序填写第 ${quizIndices.joinToString("、") { (it + 1).toString() }} 个词，确认已正确抄写：",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.7f),
                    )
                    Spacer(Modifier.height(8.dp))
                    quizIndices.forEachIndexed { index, wordIndex ->
                        OutlinedTextField(
                            value = answers[index],
                            onValueChange = {
                                answers[index] = it
                                quizError = null
                            },
                            label = { Text("第 ${wordIndex + 1} 个词") },
                            singleLine = true,
                        )
                        Spacer(Modifier.height(6.dp))
                    }
                    quizError?.let { error ->
                        Text(
                            error,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        val correct = quizIndices.indices.all { i ->
                            answers[i].trim().equals(words[quizIndices[i]], ignoreCase = true)
                        }
                        if (correct) {
                            viewModel.confirmCreateWallet()
                        } else {
                            quizError = "助记词校验失败：请按顺序核对第 " +
                                quizIndices.joinToString("、") { (it + 1).toString() } + " 个词"
                        }
                    }
                ) { Text("确认已备份") }
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

    if (showMnemonicConfirm) {
        AlertDialog(
            onDismissRequest = { showMnemonicConfirm = false },
            title = { Text("查看助记词") },
            text = {
                Text(
                    "助记词可完全控制钱包资产。请确认周围无人窥视；不要截图或通过网络发送。",
                    style = MaterialTheme.typography.bodySmall,
                )
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        showMnemonicConfirm = false
                        viewModel.revealMnemonic()
                    }
                ) { Text("继续查看") }
            },
            dismissButton = {
                TextButton(onClick = { showMnemonicConfirm = false }) { Text("取消") }
            },
        )
    }

    revealedMnemonic?.let { words ->
        var mnemonicCopied by remember(words) { mutableStateOf(false) }
        val scope = rememberCoroutineScope()
        AlertDialog(
            onDismissRequest = { viewModel.hideMnemonic() },
            title = { Text("助记词") },
            text = {
                Column {
                    Text(
                        "请逐词核对并离线保存，任何拿到助记词的人都能转走资产。",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                    Spacer(Modifier.height(12.dp))
                    Text(
                        words.joinToString("  "),
                        style = MaterialTheme.typography.bodyMedium,
                        fontWeight = FontWeight.Medium,
                    )
                    if (mnemonicCopied) {
                        Spacer(Modifier.height(8.dp))
                        Text(
                            "已复制到剪贴板（60 秒后自动清空）",
                            style = MaterialTheme.typography.labelSmall,
                            color = StatusActive,
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        val text = words.joinToString(" ")
                        clipboardManager.setText(AnnotatedString(text))
                        mnemonicCopied = true
                        // 60 秒后仅在剪贴板仍是该助记词时清空，避免误删用户后复制的内容。
                        scope.launch {
                            delay(60_000)
                            if (clipboardManager.getText()?.text == text) {
                                clipboardManager.setText(AnnotatedString(""))
                            }
                        }
                    }
                ) { Text("复制助记词") }
            },
            dismissButton = {
                TextButton(onClick = { viewModel.hideMnemonic() }) { Text("完成") }
            },
        )
    }

    if (showSendDialog) {
        var toAddress by remember { mutableStateOf("") }
        var amountMc by remember { mutableStateOf("") }
        var sendError by remember { mutableStateOf<String?>(null) }
        AlertDialog(
            onDismissRequest = { if (!busy) showSendDialog = false },
            title = { Text("转账 MC") },
            text = {
                Column {
                    OutlinedTextField(
                        value = toAddress,
                        onValueChange = { toAddress = it; sendError = null },
                        label = { Text("收款地址（mc1...）") },
                        singleLine = true,
                        enabled = !busy,
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = amountMc,
                        onValueChange = { amountMc = it; sendError = null },
                        label = { Text("金额（MC，最多 6 位小数）") },
                        singleLine = true,
                        enabled = !busy,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "网络手续费按节点最低 gas price 动态计算，从可用余额中扣除",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    Text(
                        "可用余额：${formatMcDisplay(defaultAccount?.balance ?: 0L)}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    sendError?.let { error ->
                        Spacer(Modifier.height(8.dp))
                        Text(
                            error,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !busy,
                    onClick = {
                        val error = viewModel.validateTransfer(toAddress, amountMc)
                        if (error != null) {
                            sendError = error
                        } else {
                            showSendDialog = false
                            viewModel.send(toAddress, amountMc)
                        }
                    }
                ) { Text(if (busy) "提交中..." else "确认发送") }
            },
            dismissButton = {
                TextButton(onClick = { showSendDialog = false }, enabled = !busy) { Text("取消") }
            },
        )
    }

    if (showDelegateDialog) {
        var validatorAddress by remember { mutableStateOf("") }
        var amountMc by remember { mutableStateOf("") }
        var delegateError by remember { mutableStateOf<String?>(null) }
        var showValidatorPicker by remember { mutableStateOf(false) }
        AlertDialog(
            onDismissRequest = { if (!busy) showDelegateDialog = false },
            title = { Text("质押委托") },
            text = {
                Column {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            "选择链上验证人或手动填写地址；手机端不运行验证节点。",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                            modifier = Modifier.weight(1f),
                        )
                        TextButton(
                            enabled = !busy,
                            onClick = {
                                viewModel.loadValidators()
                                showValidatorPicker = true
                            },
                        ) { Text("从链上选择") }
                    }
                    Spacer(Modifier.height(4.dp))
                    OutlinedTextField(
                        value = validatorAddress,
                        onValueChange = { validatorAddress = it; delegateError = null },
                        label = { Text("验证人地址（mcvaloper1...）") },
                        singleLine = true,
                        enabled = !busy,
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = amountMc,
                        onValueChange = { amountMc = it; delegateError = null },
                        label = { Text("委托金额（MC，最多 6 位小数）") },
                        singleLine = true,
                        enabled = !busy,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "网络手续费按节点最低 gas price 动态计算，从可用余额中扣除",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    Text(
                        "可用余额：${formatMcDisplay(defaultAccount?.balance ?: 0L)}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    delegateError?.let { error ->
                        Spacer(Modifier.height(8.dp))
                        Text(
                            error,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !busy,
                    onClick = {
                        val error = viewModel.validateDelegation(validatorAddress, amountMc)
                        if (error != null) {
                            delegateError = error
                        } else {
                            showDelegateDialog = false
                            viewModel.delegate(validatorAddress, amountMc)
                        }
                    }
                ) { Text(if (busy) "提交中..." else "确认委托") }
            },
            dismissButton = {
                TextButton(onClick = { showDelegateDialog = false }, enabled = !busy) { Text("取消") }
            },
        )

        if (showValidatorPicker) {
            AlertDialog(
                onDismissRequest = { showValidatorPicker = false },
                title = { Text("选择验证人") },
                text = {
                    when {
                        validatorsLoading -> Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(120.dp),
                            contentAlignment = Alignment.Center,
                        ) { CircularProgressIndicator() }

                        validators.isEmpty() -> Text(
                            validatorError ?: "暂无可用验证人，请检查 RPC 节点后重试",
                            style = MaterialTheme.typography.bodySmall,
                        )

                        else -> LazyColumn(modifier = Modifier.heightIn(max = 360.dp)) {
                            items(validators) { validator ->
                                Column(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clickable {
                                            validatorAddress = validator.operatorAddress
                                            delegateError = null
                                            showValidatorPicker = false
                                        }
                                        .padding(vertical = 8.dp),
                                ) {
                                    Text(
                                        validator.moniker.ifBlank {
                                            shortAddress(validator.operatorAddress)
                                        },
                                        style = MaterialTheme.typography.bodyMedium,
                                        fontWeight = FontWeight.Medium,
                                    )
                                    Text(
                                        shortAddress(validator.operatorAddress) + " · 佣金 " +
                                            String.format(
                                                Locale.getDefault(),
                                                "%.1f%%",
                                                validator.commissionRate * 100,
                                            ),
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                                    )
                                }
                                Divider()
                            }
                        }
                    }
                },
                confirmButton = {
                    TextButton(onClick = { showValidatorPicker = false }) { Text("关闭") }
                },
            )
        }
    }

    undelegateTarget?.let { delegation ->
        var amountMc by remember(delegation.validatorAddress) { mutableStateOf("") }
        var undelegateError by remember(delegation.validatorAddress) { mutableStateOf<String?>(null) }
        AlertDialog(
            onDismissRequest = { if (!busy) undelegateTarget = null },
            title = { Text("解除委托") },
            text = {
                Column {
                    Text(
                        "验证人：${delegation.validatorAddress}",
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Spacer(Modifier.height(4.dp))
                    Text(
                        "可解除数量：${formatMcDisplay(delegation.amount)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = amountMc,
                        onValueChange = { amountMc = it; undelegateError = null },
                        label = { Text("解除金额（MC，最多 6 位小数）") },
                        singleLine = true,
                        enabled = !busy,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "解除后进入 ${MCParams.UNBONDING_DAYS} 天解绑期，期间不再产生奖励，" +
                            "仍受罚没规则约束。手续费从可用余额扣除。",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    undelegateError?.let { error ->
                        Spacer(Modifier.height(8.dp))
                        Text(
                            error,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !busy,
                    onClick = {
                        val error = viewModel.validateUndelegate(delegation.validatorAddress, amountMc)
                        if (error != null) {
                            undelegateError = error
                        } else {
                            undelegateTarget = null
                            viewModel.undelegate(delegation.validatorAddress, amountMc)
                        }
                    }
                ) { Text(if (busy) "提交中..." else "确认解除") }
            },
            dismissButton = {
                TextButton(onClick = { undelegateTarget = null }, enabled = !busy) { Text("取消") }
            },
        )
    }

    redelegateTarget?.let { delegation ->
        var sourceAddress by remember(delegation.validatorAddress) {
            mutableStateOf(delegation.validatorAddress)
        }
        var destinationAddress by remember(delegation.validatorAddress) { mutableStateOf("") }
        var amountMc by remember(delegation.validatorAddress) { mutableStateOf("") }
        var redelegateError by remember(delegation.validatorAddress) { mutableStateOf<String?>(null) }
        AlertDialog(
            onDismissRequest = { if (!busy) redelegateTarget = null },
            title = { Text("转委托") },
            text = {
                Column {
                    Text(
                        "源验证人可转数量：${formatMcDisplay(delegation.amount)}",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = sourceAddress,
                        onValueChange = { sourceAddress = it; redelegateError = null },
                        label = { Text("源验证人（mcvaloper1...）") },
                        singleLine = true,
                        enabled = !busy,
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = destinationAddress,
                        onValueChange = { destinationAddress = it; redelegateError = null },
                        label = { Text("目标验证人（mcvaloper1...）") },
                        singleLine = true,
                        enabled = !busy,
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = amountMc,
                        onValueChange = { amountMc = it; redelegateError = null },
                        label = { Text("转委托金额（MC，最多 6 位小数）") },
                        singleLine = true,
                        enabled = !busy,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Decimal),
                    )
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "转委托立即生效，无 21 天解绑期；但 21 天内源、目标验证人的罚没仍可能影响该笔委托。" +
                            "手续费从可用余额扣除。",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    redelegateError?.let { error ->
                        Spacer(Modifier.height(8.dp))
                        Text(
                            error,
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.error,
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(
                    enabled = !busy,
                    onClick = {
                        val error = viewModel.validateRedelegate(
                            sourceAddress,
                            destinationAddress,
                            amountMc,
                        )
                        if (error != null) {
                            redelegateError = error
                        } else {
                            redelegateTarget = null
                            viewModel.redelegate(sourceAddress, destinationAddress, amountMc)
                        }
                    }
                ) { Text(if (busy) "提交中..." else "确认转委托") }
            },
            dismissButton = {
                TextButton(onClick = { redelegateTarget = null }, enabled = !busy) { Text("取消") }
            },
        )
    }

    outcome?.let { result ->
        TxOutcomeDialog(outcome = result, onDismiss = { viewModel.clearOutcome() })
    }

    txDetail?.let { detail ->
        TxDetailDialog(state = detail, onDismiss = { viewModel.closeTxDetail() })
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        message?.let { text ->
            item {
                WalletMessageCard(text = text, onClose = { viewModel.clearMessage() })
            }
        }

        if (busy) {
            item { BusyCard() }
        }

        if (defaultAccount == null) {
            item {
                EmptyWalletCard(
                    busy = busy,
                    onCreateWallet = { viewModel.createWallet() },
                    onImportWallet = { showImportDialog = true },
                )
            }
        } else {
            item {
                AssetOverviewCard(
                    account = defaultAccount,
                    refreshing = refreshing,
                    lastUpdated = lastUpdated,
                    hint = chainHint,
                    onRefresh = { viewModel.refresh() },
                )
            }

            refreshError?.let { error ->
                item {
                    RefreshErrorCard(error = error, onRetry = { viewModel.refresh() })
                }
            }

            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    FilledTonalButton(
                        onClick = { showSendDialog = true },
                        enabled = !busy && !refreshing,
                        modifier = Modifier.weight(1f),
                    ) {
                        Icon(Icons.Filled.Send, null, Modifier.size(18.dp))
                        Spacer(Modifier.width(4.dp))
                        Text("转账")
                    }
                    OutlinedButton(
                        onClick = {
                            clipboardManager.setText(AnnotatedString(defaultAccount.address))
                            addressCopied = true
                        },
                        enabled = !busy,
                        modifier = Modifier.weight(1f),
                    ) {
                        Icon(Icons.Filled.ContentCopy, null, Modifier.size(18.dp))
                        Spacer(Modifier.width(4.dp))
                        Text("收款地址")
                    }
                }
            }

            item {
                ReceiveCard(
                    address = defaultAccount.address,
                    copied = addressCopied,
                    onCopy = {
                        clipboardManager.setText(AnnotatedString(defaultAccount.address))
                        addressCopied = true
                    },
                    onShare = {
                        val intent = Intent(Intent.ACTION_SEND).apply {
                            type = "text/plain"
                            putExtra(Intent.EXTRA_TEXT, "我的 MC 收款地址：${defaultAccount.address}")
                        }
                        context.startActivity(Intent.createChooser(intent, "分享收款地址"))
                    },
                )
            }

            item {
                StakingCard(
                    delegations = delegations,
                    rewardUmc = defaultAccount.rewardBalance,
                    busy = busy,
                    onDelegateClick = { showDelegateDialog = true },
                    onUndelegateClick = { undelegateTarget = it },
                    onRedelegateClick = { redelegateTarget = it },
                    onClaim = { viewModel.claimRewards() },
                )
            }

            item {
                WalletSecurityCard(onViewMnemonic = { showMnemonicConfirm = true })
            }

            item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Text(
                        "交易记录",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.weight(1f),
                    )
                    if (transactions.isNotEmpty()) {
                        Text(
                            "${transactions.size} 笔",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                        )
                    }
                }
            }

            if (transactions.isEmpty()) {
                item { EmptyTransactionsCard() }
            } else {
                items(transactions, key = { it.hash }) { tx ->
                    TransactionItem(tx = tx, onClick = { viewModel.openTxDetail(tx) })
                }
            }
        }
    }
}

@Composable
private fun WalletMessageCard(text: String, onClose: () -> Unit) {
    Card(
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text,
                modifier = Modifier.weight(1f),
                style = MaterialTheme.typography.bodySmall,
            )
            TextButton(onClick = onClose) { Text("关闭") }
        }
    }
}

@Composable
private fun BusyCard() {
    Card(
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            CircularProgressIndicator(modifier = Modifier.size(16.dp), strokeWidth = 2.dp)
            Spacer(Modifier.width(8.dp))
            Text("处理中，请稍候…", style = MaterialTheme.typography.bodySmall)
        }
    }
}

@Composable
private fun EmptyWalletCard(
    busy: Boolean,
    onCreateWallet: () -> Unit,
    onImportWallet: () -> Unit,
) {
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
                    onClick = onCreateWallet,
                    enabled = !busy,
                    modifier = Modifier.weight(1f),
                ) { Text("创建钱包") }
                OutlinedButton(
                    onClick = onImportWallet,
                    enabled = !busy,
                    modifier = Modifier.weight(1f),
                ) { Text("导入钱包") }
            }
        }
    }
}

@Composable
private fun AssetOverviewCard(
    account: WalletAccount?,
    refreshing: Boolean,
    lastUpdated: Long?,
    hint: String?,
    onRefresh: () -> Unit,
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(20.dp),
        colors = CardDefaults.cardColors(containerColor = McPrimary)
    ) {
        Column(modifier = Modifier.padding(20.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "资产总览",
                    color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.8f),
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.weight(1f),
                )
                if (refreshing) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(18.dp),
                        color = MaterialTheme.colorScheme.onPrimary,
                        strokeWidth = 2.dp,
                    )
                } else {
                    IconButton(onClick = onRefresh) {
                        Icon(
                            Icons.Filled.Refresh,
                            contentDescription = "刷新链上数据",
                            tint = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.8f),
                        )
                    }
                }
            }
            Spacer(Modifier.height(8.dp))
            Text(
                "可用余额",
                color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                style = MaterialTheme.typography.labelMedium,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                formatMcDisplay(account?.balance ?: 0L),
                style = MaterialTheme.typography.headlineMedium,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.onPrimary,
            )
            Spacer(Modifier.height(16.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
            ) {
                AssetStat(
                    label = "质押金额",
                    value = formatMcDisplay(account?.stakeBalance ?: 0L),
                    modifier = Modifier.weight(1f),
                )
                AssetStat(
                    label = "待领委托奖励",
                    value = formatMcDisplay(account?.rewardBalance ?: 0L),
                    valueColor = McGold,
                    modifier = Modifier.weight(1f),
                )
            }
            lastUpdated?.let {
                Spacer(Modifier.height(10.dp))
                Text(
                    "更新于 ${formatClock(it)}（链上实时查询）",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.55f),
                )
            }
            hint?.let {
                Spacer(Modifier.height(8.dp))
                Text(
                    it,
                    style = MaterialTheme.typography.labelSmall,
                    color = McGold,
                )
            }
        }
    }
}

@Composable
private fun AssetStat(
    label: String,
    value: String,
    modifier: Modifier = Modifier,
    valueColor: Color? = null,
) {
    Column(modifier = modifier) {
        Text(
            label,
            style = MaterialTheme.typography.labelMedium,
            color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
        )
        Spacer(Modifier.height(4.dp))
        Text(
            value,
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold,
            color = valueColor ?: MaterialTheme.colorScheme.onPrimary,
        )
    }
}

@Composable
private fun RefreshErrorCard(error: String, onRetry: () -> Unit) {
    Card(
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                Icons.Filled.ErrorOutline,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onErrorContainer,
            )
            Spacer(Modifier.width(8.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    "链上数据刷新失败",
                    style = MaterialTheme.typography.labelLarge,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onErrorContainer,
                )
                Text(
                    error,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onErrorContainer,
                )
            }
            TextButton(onClick = onRetry) { Text("重试") }
        }
    }
}

@Composable
private fun ReceiveCard(
    address: String,
    copied: Boolean,
    onCopy: () -> Unit,
    onShare: () -> Unit,
) {
    Card(shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "收款地址",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.weight(1f),
                )
                if (copied) {
                    Text(
                        "已复制",
                        style = MaterialTheme.typography.labelSmall,
                        color = StatusActive,
                    )
                }
            }
            Spacer(Modifier.height(8.dp))
            Text(
                address,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.75f),
            )
            Spacer(Modifier.height(12.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                FilledTonalButton(onClick = onCopy, modifier = Modifier.weight(1f)) {
                    Icon(Icons.Filled.ContentCopy, null, Modifier.size(18.dp))
                    Spacer(Modifier.width(4.dp))
                    Text("复制地址")
                }
                OutlinedButton(onClick = onShare, modifier = Modifier.weight(1f)) {
                    Icon(Icons.Filled.Share, null, Modifier.size(18.dp))
                    Spacer(Modifier.width(4.dp))
                    Text("分享")
                }
            }
        }
    }
}

@Composable
private fun StakingCard(
    delegations: List<Delegation>,
    rewardUmc: Long,
    busy: Boolean,
    onDelegateClick: () -> Unit,
    onUndelegateClick: (Delegation) -> Unit,
    onRedelegateClick: (Delegation) -> Unit,
    onClaim: () -> Unit,
) {
    Card(shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    "质押委托",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.weight(1f),
                )
                FilledTonalButton(onClick = onDelegateClick, enabled = !busy) {
                    Icon(Icons.Filled.AddCircle, null, Modifier.size(18.dp))
                    Spacer(Modifier.width(4.dp))
                    Text("委托")
                }
            }
            Spacer(Modifier.height(8.dp))
            Text(
                "待领委托奖励：${formatMcDisplay(rewardUmc)}",
                style = MaterialTheme.typography.bodySmall,
                color = McGold,
            )
            Spacer(Modifier.height(8.dp))
            if (delegations.isEmpty()) {
                Text(
                    "暂无链上委托记录。手机端不运行验证节点，收益以链上实际发放为准。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
            } else {
                delegations.forEach { delegation ->
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 6.dp),
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(modifier = Modifier.weight(1f)) {
                                Text(
                                    "验证人",
                                    style = MaterialTheme.typography.labelSmall,
                                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                                )
                                Text(
                                    shortAddress(delegation.validatorAddress),
                                    style = MaterialTheme.typography.bodySmall,
                                )
                            }
                            Text(
                                formatMcDisplay(delegation.amount),
                                style = MaterialTheme.typography.bodySmall,
                                fontWeight = FontWeight.Medium,
                            )
                        }
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.End,
                        ) {
                            TextButton(
                                onClick = { onRedelegateClick(delegation) },
                                enabled = !busy,
                            ) { Text("转委托") }
                            TextButton(
                                onClick = { onUndelegateClick(delegation) },
                                enabled = !busy,
                            ) { Text("解除") }
                        }
                    }
                }
            }
            Spacer(Modifier.height(8.dp))
            OutlinedButton(
                onClick = onClaim,
                enabled = !busy && rewardUmc > 0,
                modifier = Modifier.fillMaxWidth(),
            ) {
                Text(if (busy) "处理中..." else "领取全部委托奖励")
            }
        }
    }
}

@Composable
private fun WalletSecurityCard(onViewMnemonic: () -> Unit) {
    Card(shape = RoundedCornerShape(16.dp)) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    "钱包安全",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.Bold,
                )
                Text(
                    "助记词仅保存在本机加密存储，查看前需要二次确认。",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                )
            }
            TextButton(onClick = onViewMnemonic) { Text("查看助记词") }
        }
    }
}

@Composable
private fun EmptyTransactionsCard() {
    Card(shape = RoundedCornerShape(12.dp)) {
        Text(
            "暂无交易记录。链上转账、委托与领取奖励后会显示在这里。",
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
        )
    }
}

@Composable
fun TransactionItem(tx: TxRecord, onClick: () -> Unit = {}) {
    val timeText = remember(tx.timestamp) {
        SimpleDateFormat("MM-dd HH:mm", Locale.getDefault()).format(Date(tx.timestamp))
    }
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() },
        shape = RoundedCornerShape(12.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(
                imageVector = txTypeIcon(tx.type),
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(32.dp)
            )
            Spacer(Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = txTypeLabel(tx.type),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium
                )
                Text(
                    text = timeText + " · " + shortHash(tx.hash),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Column(horizontalAlignment = Alignment.End) {
                Text(
                    text = formatSignedAmount(tx),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = txAmountColor(tx.type),
                )
                Text(
                    text = txStatusLabel(tx.status),
                    style = MaterialTheme.typography.labelSmall,
                    color = txStatusColor(tx.status),
                )
            }
        }
    }
}

@Composable
private fun TxOutcomeDialog(outcome: TxOutcome, onDismiss: () -> Unit) {
    val clipboardManager = LocalClipboardManager.current
    var copied by remember(outcome.txHash) { mutableStateOf(false) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(outcome.title) },
        text = {
            Column {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        imageVector = if (outcome.success) Icons.Filled.CheckCircle else Icons.Filled.ErrorOutline,
                        contentDescription = null,
                        tint = if (outcome.success) StatusActive else MaterialTheme.colorScheme.error,
                    )
                    Spacer(Modifier.width(8.dp))
                    Text(outcome.message, style = MaterialTheme.typography.bodySmall)
                }
                if (outcome.pending) {
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "交易已广播，正在等待链上确认。",
                        style = MaterialTheme.typography.bodySmall,
                        color = StatusPending,
                    )
                }
                if (outcome.txHash.isNotBlank()) {
                    Spacer(Modifier.height(12.dp))
                    Text(
                        "交易哈希",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                    Text(
                        outcome.txHash,
                        style = MaterialTheme.typography.bodySmall,
                        fontFamily = FontFamily.Monospace,
                    )
                    TextButton(
                        contentPadding = PaddingValues(0.dp),
                        onClick = {
                            clipboardManager.setText(AnnotatedString(outcome.txHash))
                            copied = true
                        }
                    ) { Text(if (copied) "已复制" else "复制哈希") }
                }
            }
        },
        confirmButton = {
            TextButton(onClick = onDismiss) { Text("关闭") }
        },
    )
}

@Composable
private fun TxDetailDialog(state: TxDetailState, onDismiss: () -> Unit) {
    val clipboardManager = LocalClipboardManager.current
    var rawExpanded by remember(state.record.hash) { mutableStateOf(false) }
    val receipt = state.receipt
    val failureReason = when {
        receipt != null && receipt.code != 0 -> {
            val codespace = receipt.codespace.ifBlank { "unknown" }
            receipt.log.ifBlank { "链上返回错误码 ${receipt.code}（$codespace）" }
        }
        state.record.status == TxStatus.FAILED ->
            "链上未返回失败日志（节点可能已裁剪该交易回执）"
        else -> null
    }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("交易详情") },
        text = {
            Column(modifier = Modifier.verticalScroll(rememberScrollState())) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        imageVector = txTypeIcon(state.record.type),
                        contentDescription = null,
                        tint = txStatusColor(state.record.status),
                    )
                    Spacer(Modifier.width(8.dp))
                    Text(
                        txTypeLabel(state.record.type),
                        style = MaterialTheme.typography.titleSmall,
                        fontWeight = FontWeight.Bold,
                        modifier = Modifier.weight(1f),
                    )
                    Text(
                        txStatusLabel(state.record.status),
                        color = txStatusColor(state.record.status),
                        style = MaterialTheme.typography.labelLarge,
                    )
                }
                Spacer(Modifier.height(8.dp))
                DetailRow("金额", formatSignedAmount(state.record))
                DetailRow(
                    label = "交易哈希",
                    value = state.record.hash,
                    onCopy = { clipboardManager.setText(AnnotatedString(state.record.hash)) },
                )
                DetailRow(
                    "区块高度",
                    if (state.record.height > 0) state.record.height.toString() else "待确认",
                )
                DetailRow("时间", formatDateTime(state.record.timestamp))
                DetailRow("手续费", formatMcDisplay(state.record.fee))
                if (state.record.toAddress.isNotBlank()) {
                    DetailRow(
                        label = "对方地址",
                        value = state.record.toAddress,
                        onCopy = { clipboardManager.setText(AnnotatedString(state.record.toAddress)) },
                    )
                }
                if (state.record.memo.isNotBlank()) {
                    DetailRow("备注", state.record.memo)
                }

                if (state.loading) {
                    Spacer(Modifier.height(8.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        CircularProgressIndicator(modifier = Modifier.size(16.dp), strokeWidth = 2.dp)
                        Spacer(Modifier.width(8.dp))
                        Text("正在查询链上回执…", style = MaterialTheme.typography.bodySmall)
                    }
                } else if (receipt != null) {
                    DetailRow("链上返回码", receipt.code.toString())
                    if (receipt.gasUsed > 0) DetailRow("Gas 消耗", receipt.gasUsed.toString())
                } else {
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "链上暂未查询到该交易回执（可能仍在打包或节点未保留历史）",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                }

                failureReason?.let { reason ->
                    Spacer(Modifier.height(8.dp))
                    Text(
                        "失败原因",
                        style = MaterialTheme.typography.labelMedium,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.error,
                    )
                    Text(
                        reason,
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.error,
                    )
                }

                Spacer(Modifier.height(8.dp))
                Text("原始信息", style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Bold)
                TextButton(
                    contentPadding = PaddingValues(0.dp),
                    onClick = { rawExpanded = !rawExpanded },
                ) { Text(if (rawExpanded) "收起原始数据" else "展开原始数据") }
                Text(
                    state.record.rawJson.ifBlank { "无" },
                    style = MaterialTheme.typography.bodySmall,
                    fontFamily = FontFamily.Monospace,
                    maxLines = if (rawExpanded) Int.MAX_VALUE else 3,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        },
        confirmButton = {
            TextButton(onClick = onDismiss) { Text("关闭") }
        },
    )
}

@Composable
private fun DetailRow(label: String, value: String, onCopy: (() -> Unit)? = null) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp)
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                label,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                modifier = Modifier.weight(1f),
            )
            if (onCopy != null) {
                IconButton(onClick = onCopy, modifier = Modifier.size(24.dp)) {
                    Icon(
                        Icons.Filled.ContentCopy,
                        contentDescription = "复制$label",
                        modifier = Modifier.size(14.dp),
                    )
                }
            }
        }
        Text(value, style = MaterialTheme.typography.bodySmall)
    }
}

private fun txTypeLabel(type: TxType): String = when (type) {
    TxType.SEND -> "转账"
    TxType.DELEGATE -> "质押委托"
    TxType.UNDELEGATE -> "解除委托"
    TxType.REDELEGATE -> "转委托"
    TxType.CLAIM_REWARDS -> "领取奖励"
    TxType.REGISTER_DEVICE -> "设备注册"
    TxType.ATTEST_NODE -> "节点认证"
    TxType.REPORT_CONTRIB -> "贡献上报"
    TxType.SUBMIT_TASK -> "提交任务"
    TxType.UNKNOWN -> "其他"
}

private fun txTypeIcon(type: TxType): ImageVector = when (type) {
    TxType.SEND -> Icons.Filled.ArrowUpward
    TxType.DELEGATE -> Icons.Filled.AddCircle
    TxType.UNDELEGATE, TxType.REDELEGATE -> Icons.Filled.SwapHoriz
    TxType.CLAIM_REWARDS -> Icons.Filled.CardGiftcard
    TxType.REGISTER_DEVICE -> Icons.Filled.PhoneAndroid
    TxType.ATTEST_NODE -> Icons.Filled.CheckCircle
    TxType.REPORT_CONTRIB -> Icons.Filled.Upload
    TxType.SUBMIT_TASK -> Icons.Filled.Memory
    TxType.UNKNOWN -> Icons.Filled.SwapHoriz
}

private fun txStatusLabel(status: TxStatus): String = when (status) {
    TxStatus.SUCCESS -> "成功"
    TxStatus.PENDING -> "确认中"
    TxStatus.FAILED -> "失败"
}

private fun txStatusColor(status: TxStatus): Color = when (status) {
    TxStatus.SUCCESS -> StatusActive
    TxStatus.PENDING -> StatusPending
    TxStatus.FAILED -> StatusSlashed
}

private fun txAmountColor(type: TxType): Color = when (type) {
    TxType.SEND, TxType.DELEGATE -> McHighlight
    TxType.CLAIM_REWARDS -> StatusActive
    else -> Color.Unspecified
}

private fun formatSignedAmount(tx: TxRecord): String = when (tx.type) {
    TxType.SEND, TxType.DELEGATE -> "-" + formatMcDisplay(tx.amount)
    TxType.CLAIM_REWARDS -> "+" + formatMcDisplay(tx.amount)
    TxType.UNDELEGATE -> "解除 " + formatMcDisplay(tx.amount)
    TxType.REDELEGATE -> "转委托 " + formatMcDisplay(tx.amount)
    else -> formatMcDisplay(tx.amount)
}

private fun shortHash(hash: String): String =
    if (hash.length <= 16) hash else hash.take(8) + "..." + hash.takeLast(6)

private fun shortAddress(address: String): String =
    if (address.length <= 24) address else address.take(14) + "..." + address.takeLast(8)

/** 助记词展示期间给 Activity 窗口加 FLAG_SECURE，阻止截屏/录屏。 */
@Composable
private fun SecureWindow(enabled: Boolean) {
    val context = LocalContext.current
    DisposableEffect(enabled) {
        val window = context.findActivity()?.window
        if (enabled) {
            window?.addFlags(WindowManager.LayoutParams.FLAG_SECURE)
        } else {
            window?.clearFlags(WindowManager.LayoutParams.FLAG_SECURE)
        }
        onDispose { window?.clearFlags(WindowManager.LayoutParams.FLAG_SECURE) }
    }
}

private fun Context.findActivity(): Activity? {
    var current = this
    while (current is ContextWrapper) {
        if (current is Activity) return current
        current = current.baseContext
    }
    return null
}
