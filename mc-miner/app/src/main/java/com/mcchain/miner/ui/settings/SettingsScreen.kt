package com.mcchain.miner.ui.settings

import android.content.Intent
import android.provider.Settings
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import com.mcchain.miner.BuildConfig
import com.mcchain.miner.MCParams
import com.mcchain.miner.ui.theme.*

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(viewModel: SettingsViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
    val nodeSyncEnabled by viewModel.nodeSyncEnabled.collectAsState()
    val miningEnabled by viewModel.miningEnabled.collectAsState()
    val wifiOnly by viewModel.wifiOnly.collectAsState()
    val batteryConfig by viewModel.batteryConfig.collectAsState()
    val notificationEnabled by viewModel.notificationEnabled.collectAsState()
    val autoStartEnabled by viewModel.autoStartEnabled.collectAsState()
    val rpcEndpoint by viewModel.rpcEndpoint.collectAsState()
    val oracleEndpoint by viewModel.oracleEndpoint.collectAsState()
    val oracleToken by viewModel.oracleToken.collectAsState()
    val rpcTestResult by viewModel.rpcTestResult.collectAsState()
    val clearingData by viewModel.clearingData.collectAsState()
    val clearResult by viewModel.clearResult.collectAsState()
    val context = LocalContext.current

    var showEndpointDialog by remember { mutableStateOf(false) }
    var showOracleDialog by remember { mutableStateOf(false) }
    var showBatteryDialog by remember { mutableStateOf(false) }
    var showClearDialog by remember { mutableStateOf(false) }

    val batterySummary = when (batteryConfig) {
        "low" -> "节能：仅 Wi-Fi + 充电时计量贡献（心跳保持在线）"
        "high" -> "高性能：任何网络下都持续计量贡献（最耗电）"
        else -> "均衡：仅 Wi-Fi 时计量贡献（心跳保持在线）"
    }
    val wifiSummary = when {
        batteryConfig == "low" -> "节能模式固定要求 Wi-Fi + 充电，此项不生效"
        wifiOnly -> "当前仅 Wi-Fi/以太网下计量贡献，移动数据不计量"
        else -> "当前允许移动数据计量；开启后仅在 Wi-Fi/以太网下计量"
    }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        if (!viewModel.storageAvailable) {
            item {
                Card(
                    shape = RoundedCornerShape(12.dp),
                    colors = CardDefaults.cardColors(
                        containerColor = MaterialTheme.colorScheme.errorContainer
                    )
                ) {
                    Text(
                        text = "设备安全存储不可用：设置无法保存，钱包相关功能不可用。请检查系统 Keystore 后重启 App。",
                        modifier = Modifier.padding(12.dp),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onErrorContainer
                    )
                }
            }
        }

        // === 运行开关 ===
        item { SettingsSection("运行开关") }
        item {
            SwitchSettingItem(
                icon = Icons.Filled.Hub,
                title = "节点同步",
                description = "启动节点同步服务，同步区块头并维护 P2P 连接；关闭将立即停止节点同步服务",
                checked = nodeSyncEnabled,
                onCheckedChange = { viewModel.toggleNodeSync(it) }
            )
        }
        item {
            SwitchSettingItem(
                icon = Icons.Filled.Memory,
                title = "贡献挖矿",
                description = "计量带宽/存储贡献并执行 EdgeAI 任务；关闭将停止贡献服务，设备心跳一并停止，链上可能判离线",
                checked = miningEnabled,
                onCheckedChange = { viewModel.toggleMining(it) }
            )
        }
        item {
            SwitchSettingItem(
                icon = Icons.Filled.Wifi,
                title = "仅 Wi-Fi 计量",
                description = wifiSummary,
                checked = wifiOnly,
                onCheckedChange = { viewModel.toggleWifiOnly(it) },
                enabled = batteryConfig != "low"
            )
        }
        item {
            ClickableSettingItem(
                icon = Icons.Filled.BatteryChargingFull,
                title = "功耗模式",
                description = batterySummary,
                onClick = { showBatteryDialog = true }
            )
        }
        item {
            NoteSettingItem(
                "功耗模式只决定是否执行贡献计量，不改变前台服务的唤醒锁与固定 30 秒轮询周期；" +
                    "心跳在任何模式下都保持在线。"
            )
        }
        item {
            ClickableSettingItem(
                icon = Icons.Filled.BatterySaver,
                title = "电池优化",
                description = "前往系统设置关闭对 MC Miner 的电池优化，避免后台服务被休眠",
                onClick = {
                    runCatching {
                        context.startActivity(Intent(Settings.ACTION_IGNORE_BATTERY_OPTIMIZATION_SETTINGS))
                    }
                }
            )
        }

        // === 通知与自启动 ===
        item { SettingsSection("通知与自启动") }
        item {
            SwitchSettingItem(
                icon = Icons.Filled.Notifications,
                title = "常驻通知",
                description = "显示前台服务的运行状态与暂停原因；关闭后服务仍运行，但不再更新状态通知（系统仍保留一条必需的最小常驻通知）",
                checked = notificationEnabled,
                onCheckedChange = { viewModel.toggleNotification(it) }
            )
        }
        item {
            SwitchSettingItem(
                icon = Icons.Filled.RestartAlt,
                title = "开机自启",
                description = "设备重启后自动恢复已开启的节点同步与贡献挖矿，适合长期挂机；默认关闭",
                checked = autoStartEnabled,
                onCheckedChange = { viewModel.toggleAutoStart(it) }
            )
        }

        // === 网络配置 ===
        item { SettingsSection("网络配置") }
        item {
            ClickableSettingItem(
                icon = Icons.Filled.Dns,
                title = "RPC 端点",
                description = rpcEndpoint.ifBlank { "未设置，使用默认 http://127.0.0.1:${MCParams.DEFAULT_RPC_PORT}" },
                onClick = {
                    viewModel.resetRpcTestResult()
                    showEndpointDialog = true
                }
            )
        }
        item {
            ClickableSettingItem(
                icon = Icons.Filled.VerifiedUser,
                title = "认证预言机",
                description = oracleEndpoint.ifBlank { "未配置（设备认证不可用）" },
                onClick = { showOracleDialog = true }
            )
        }

        // === 本地数据 ===
        item { SettingsSection("本地数据") }
        item {
            ClickableSettingItem(
                icon = Icons.Filled.DeleteForever,
                title = "清空本地数据",
                description = "删除本地钱包与助记词、数据库及全部设置（不可撤销）",
                danger = true,
                onClick = { showClearDialog = true }
            )
        }
        if (clearingData) {
            item {
                Card(shape = RoundedCornerShape(12.dp)) {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(12.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(18.dp),
                            strokeWidth = 2.dp
                        )
                        Spacer(modifier = Modifier.width(12.dp))
                        Text("正在清空本地数据…", style = MaterialTheme.typography.bodySmall)
                    }
                }
            }
        }

        // === 网络参数 ===
        item { SettingsSection("网络参数") }
        item { InfoSettingItem("区块时间", "${MCParams.BLOCK_TIME_SECONDS} 秒") }
        item { InfoSettingItem("地址前缀", MCParams.BECH32_PREFIX) }
        item { InfoSettingItem("RPC 端口", MCParams.DEFAULT_RPC_PORT.toString()) }
        item { InfoSettingItem("P2P 端口", MCParams.DEFAULT_P2P_PORT.toString()) }

        // === 验证人参数 ===
        item { SettingsSection("验证人参数") }
        item { InfoSettingItem("最大验证人数", MCParams.MAX_VALIDATORS.toString()) }
        item {
            InfoSettingItem(
                "最小自抵押",
                String.format("%,.0f MC", MCParams.MIN_SELF_DELEGATE_UMC / 1_000_000.0)
            )
        }

        // === 罚没规则 ===
        item { SettingsSection("罚没规则") }
        item { InfoSettingItem("双签罚没", "${MCParams.DOUBLE_SIGN_SLASH_PERCENT}%") }
        item { InfoSettingItem("缺块罚没", "${MCParams.DOWNTIME_SLASH_PERCENT}%") }
        item { InfoSettingItem("缺块统计窗口", "${MCParams.DOWNTIME_SLASH_WINDOW_BLOCKS} 块") }
        item { InfoSettingItem("设备离线罚没", "${MCParams.PHONENODE_OFFLINE_SLASH_PERCENT}%") }
        item {
            InfoSettingItem(
                "设备离线宽限",
                "${MCParams.PHONENODE_OFFLINE_GRACE_BLOCKS} 块 / ${MCParams.PHONENODE_OFFLINE_GRACE_SECONDS} 秒"
            )
        }

        // === Tokenomics ===
        item { SettingsSection("代币经济") }
        item { InfoSettingItem("总供应量", "10 亿 MC") }
        item { InfoSettingItem("初始通胀", "${MCParams.INFLATION_RATE_PERCENT}%") }
        item { InfoSettingItem("目标质押率", "${MCParams.TARGET_STAKE_RATIO_PERCENT}%") }
        item { InfoSettingItem("社区池", "${MCParams.COMMUNITY_TAX_PERCENT}%") }

        // === DePIN 奖励 ===
        item { SettingsSection("DePIN 奖励池") }
        item {
            val poolMc = MCParams.DEPIN_INITIAL_POOL_UMC / 1_000_000.0
            InfoSettingItem("初始奖池", String.format("%,.0f MC", poolMc))
        }
        item {
            InfoSettingItem(
                "贡献赏金",
                "${MCParams.CONTRIBUTOR_SHARE}% 归贡献节点（链上不抽成、不销毁）"
            )
        }

        // === 关于 ===
        item { SettingsSection("关于") }
        item {
            InfoSettingItem(
                "应用版本",
                "${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE})"
            )
        }
        item { InfoSettingItem("链 ID", MCParams.CHAIN_ID) }
        item {
            InfoSettingItem(
                "计价单位",
                "${MCParams.DENOM}（${MCParams.DISPLAY_DENOM}，${MCParams.DECIMALS} 位小数）"
            )
        }
        item { InfoSettingItem("Cosmos SDK", "v0.47") }
        item { InfoSettingItem("CometBFT", "v0.37") }
        item { InfoSettingItem("IBC", "v7.3.0") }
        item {
            NoteSettingItem(
                "隐私说明：钱包助记词与私钥仅保存在本机 Keystore 加密存储中，不会上传；" +
                    "App 仅向链与认证预言机上传贡献证明、设备认证材料与已签名的交易。" +
                    "清空本地数据会同时删除助记词，请务必提前备份。"
            )
        }
        item {
            NoteSettingItem(
                "开源与链信息：MC Chain 基于 Cosmos SDK / CometBFT 构建，链上参数以 genesis 为准；" +
                    "本 App 为开源客户端，源代码与链配置随项目仓库发布。"
            )
        }

        item { Spacer(modifier = Modifier.height(32.dp)) }
    }

    // === RPC 端点对话框 ===
    if (showEndpointDialog) {
        var endpoint by remember { mutableStateOf(rpcEndpoint) }
        AlertDialog(
            onDismissRequest = { showEndpointDialog = false },
            title = { Text("RPC 端点") },
            text = {
                Column {
                    OutlinedTextField(
                        value = endpoint,
                        onValueChange = { endpoint = it },
                        label = { Text("端点地址") },
                        placeholder = { Text("http://<节点IP>:26657") },
                        supportingText = {
                            Text(
                                "留空将使用默认 http://127.0.0.1:${MCParams.DEFAULT_RPC_PORT}",
                                style = MaterialTheme.typography.labelSmall
                            )
                        },
                        singleLine = true
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        TextButton(onClick = { viewModel.testRpcConnection(endpoint) }) {
                            Text("测试连接")
                        }
                        val result = rpcTestResult
                        Text(
                            text = result ?: "",
                            style = MaterialTheme.typography.labelSmall,
                            color = when {
                                result == null -> MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                                result?.startsWith("连接成功") == true -> StatusActive
                                result?.startsWith("连接中") == true -> MaterialTheme.colorScheme.primary
                                else -> MaterialTheme.colorScheme.error
                            }
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(onClick = {
                    viewModel.updateRpcEndpoint(endpoint)
                    showEndpointDialog = false
                }) { Text("保存") }
            },
            dismissButton = {
                TextButton(onClick = { showEndpointDialog = false }) { Text("取消") }
            }
        )
    }

    // === 认证预言机对话框 ===
    if (showOracleDialog) {
        var oracleUrl by remember { mutableStateOf(oracleEndpoint) }
        var oracleTokenInput by remember { mutableStateOf(oracleToken) }
        var tokenVisible by remember { mutableStateOf(false) }
        AlertDialog(
            onDismissRequest = { showOracleDialog = false },
            title = { Text("认证预言机") },
            text = {
                Column {
                    Text(
                        "用于设备认证：App 把硬件证明发给预言机换取认证材料。Token 用于 Bearer 鉴权，留空表示服务端未启用鉴权。",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    OutlinedTextField(
                        value = oracleUrl,
                        onValueChange = { oracleUrl = it },
                        label = { Text("预言机地址（/sign 根地址）") },
                        placeholder = { Text("http://192.168.1.10:8080") },
                        singleLine = true
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = oracleTokenInput,
                        onValueChange = { oracleTokenInput = it },
                        label = { Text("Bearer Token（可留空）") },
                        singleLine = true,
                        visualTransformation = if (tokenVisible) {
                            VisualTransformation.None
                        } else {
                            PasswordVisualTransformation()
                        },
                        trailingIcon = {
                            IconButton(onClick = { tokenVisible = !tokenVisible }) {
                                Icon(
                                    imageVector = if (tokenVisible) Icons.Filled.VisibilityOff else Icons.Filled.Visibility,
                                    contentDescription = if (tokenVisible) "隐藏令牌" else "显示令牌"
                                )
                            }
                        }
                    )
                }
            },
            confirmButton = {
                TextButton(onClick = {
                    viewModel.updateOracle(oracleUrl, oracleTokenInput)
                    showOracleDialog = false
                }) { Text("保存") }
            },
            dismissButton = {
                TextButton(onClick = { showOracleDialog = false }) { Text("取消") }
            }
        )
    }

    // === 功耗模式对话框 ===
    if (showBatteryDialog) {
        AlertDialog(
            onDismissRequest = { showBatteryDialog = false },
            title = { Text("功耗模式") },
            text = {
                Column {
                    BatteryOption(
                        key = "low",
                        title = "节能模式",
                        desc = "仅 Wi-Fi 且充电时计量；其余时间不计量。心跳保持在线，最省电。",
                        selected = batteryConfig == "low"
                    ) {
                        viewModel.setBatteryConfig("low")
                        showBatteryDialog = false
                    }
                    BatteryOption(
                        key = "medium",
                        title = "均衡模式",
                        desc = "Wi-Fi 下计量（受「仅 Wi-Fi 计量」开关控制）；心跳保持在线，耗电适中。",
                        selected = batteryConfig == "medium"
                    ) {
                        viewModel.setBatteryConfig("medium")
                        showBatteryDialog = false
                    }
                    BatteryOption(
                        key = "high",
                        title = "高性能模式",
                        desc = "任何网络与充电状态下都计量贡献，耗电最高，长期使用可能发热。",
                        selected = batteryConfig == "high"
                    ) {
                        viewModel.setBatteryConfig("high")
                        showBatteryDialog = false
                    }
                }
            },
            confirmButton = {},
            dismissButton = {
                TextButton(onClick = { showBatteryDialog = false }) { Text("取消") }
            }
        )
    }

    // === 清空本地数据二次确认 ===
    if (showClearDialog) {
        AlertDialog(
            onDismissRequest = { showClearDialog = false },
            icon = { Icon(Icons.Filled.DeleteForever, null, tint = MaterialTheme.colorScheme.error) },
            title = { Text("清空本地数据？") },
            text = {
                Column {
                    Text("此操作不可撤销，将执行：")
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("· 删除本地钱包与助记词，务必已备份")
                    Text("· 停止节点同步与贡献挖矿服务")
                    Text("· 清空本地数据库与全部设置")
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        "完成后必须重启 App 才能恢复使用。",
                        fontWeight = FontWeight.Medium
                    )
                }
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        showClearDialog = false
                        viewModel.clearLocalData()
                    },
                    colors = ButtonDefaults.textButtonColors(
                        contentColor = MaterialTheme.colorScheme.error
                    )
                ) { Text("确认清空") }
            },
            dismissButton = {
                TextButton(onClick = { showClearDialog = false }) { Text("取消") }
            }
        )
    }

    // === 清空结果提示 ===
    clearResult?.let { result ->
        AlertDialog(
            onDismissRequest = { viewModel.dismissClearResult() },
            title = { Text(if (result.success) "清理完成" else "清理未完全成功") },
            text = { Text(result.message) },
            confirmButton = {
                TextButton(onClick = { viewModel.dismissClearResult() }) { Text("知道了") }
            }
        )
    }
}

@Composable
fun SettingsSection(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleSmall,
        fontWeight = FontWeight.Bold,
        color = MaterialTheme.colorScheme.primary,
        modifier = Modifier.padding(top = 8.dp)
    )
}

@Composable
fun SwitchSettingItem(
    icon: ImageVector,
    title: String,
    description: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    enabled: Boolean = true
) {
    val contentAlpha = if (enabled) 1f else 0.4f
    Card(shape = RoundedCornerShape(12.dp)) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.primary.copy(alpha = contentAlpha),
                modifier = Modifier.size(24.dp)
            )
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    title,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = contentAlpha)
                )
                Text(
                    description,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f * contentAlpha)
                )
            }
            Switch(checked = checked, onCheckedChange = onCheckedChange, enabled = enabled)
        }
    }
}

@Composable
fun ClickableSettingItem(
    icon: ImageVector,
    title: String,
    description: String,
    danger: Boolean = false,
    onClick: () -> Unit
) {
    val accent = if (danger) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary
    Card(
        modifier = Modifier.clickable { onClick() },
        shape = RoundedCornerShape(12.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(icon, null, tint = accent, modifier = Modifier.size(24.dp))
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    title,
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    color = if (danger) accent else MaterialTheme.colorScheme.onSurface
                )
                Text(
                    description,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
                )
            }
            Icon(
                Icons.Filled.ChevronRight,
                null,
                tint = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.3f)
            )
        }
    }
}

@Composable
fun InfoSettingItem(label: String, value: String) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(label, style = MaterialTheme.typography.bodyMedium)
            Text(
                value,
                style = MaterialTheme.typography.bodyMedium,
                fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.primary
            )
        }
    }
}

@Composable
fun NoteSettingItem(text: String) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Text(
            text = text,
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f)
        )
    }
}

@Composable
fun BatteryOption(
    key: String,
    title: String,
    desc: String,
    selected: Boolean,
    onClick: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { onClick() }
            .padding(12.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        RadioButton(selected = selected, onClick = onClick)
        Spacer(modifier = Modifier.width(12.dp))
        Column {
            Text(title, fontWeight = FontWeight.Medium)
            Text(
                desc,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f)
            )
        }
    }
}
