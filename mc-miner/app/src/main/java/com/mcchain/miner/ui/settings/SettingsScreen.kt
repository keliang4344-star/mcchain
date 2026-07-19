package com.mcchain.miner.ui.settings

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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.MCParams
import com.mcchain.miner.service.MinerService
import com.mcchain.miner.service.NodeService
import com.mcchain.miner.ui.theme.*
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(viewModel: SettingsViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
    val nodeSyncEnabled by viewModel.nodeSyncEnabled.collectAsState()
    val miningEnabled by viewModel.miningEnabled.collectAsState()
    val batteryConfig by viewModel.batteryConfig.collectAsState()
    val rpcEndpoint by viewModel.rpcEndpoint.collectAsState()

    var showEndpointDialog by remember { mutableStateOf(false) }
    var showBatteryDialog by remember { mutableStateOf(false) }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // 节点同步
        item {
            SettingsSection("节点同步")
        }
        item {
            SwitchSettingItem(
                icon = Icons.Filled.Hub,
                title = "节点同步",
                description = "同步 MCChain 区块头，维护 P2P 网络连接",
                checked = nodeSyncEnabled,
                onCheckedChange = { viewModel.toggleNodeSync(it) }
            )
        }
        item {
            ClickableSettingItem(
                icon = Icons.Filled.Dns,
                title = "RPC 端点",
                description = rpcEndpoint,
                onClick = { showEndpointDialog = true }
            )
        }

        // 挖矿设置
        item {
            SettingsSection("贡献即挖矿")
        }
        item {
            SwitchSettingItem(
                icon = Icons.Filled.Memory,
                title = "贡献挖矿",
                description = "贡献带宽、存储和 AI 推理资源获取 MC 奖励",
                checked = miningEnabled,
                onCheckedChange = { viewModel.toggleMining(it) }
            )
        }
        item {
            ClickableSettingItem(
                icon = Icons.Filled.BatteryChargingFull,
                title = "功耗模式",
                description = when(batteryConfig) {
                    "low" -> "节能模式 - 仅 Wi-Fi 时贡献"
                    "high" -> "高性能模式 - 始终全速贡献"
                    else -> "均衡模式 - 智能调节资源占用"
                },
                onClick = { showBatteryDialog = true }
            )
        }

        // 网络参数
        item {
            SettingsSection("网络参数")
        }
        item {
            InfoSettingItem("链 ID", MCParams.CHAIN_ID)
        }
        item {
            InfoSettingItem("区块时间", "${MCParams.BLOCK_TIME_SECONDS} 秒")
        }
        item {
            InfoSettingItem("地址前缀", MCParams.BECH32_PREFIX)
        }
        item {
            InfoSettingItem("RPC 端口", MCParams.DEFAULT_RPC_PORT.toString())
        }
        item {
            InfoSettingItem("P2P 端口", MCParams.DEFAULT_P2P_PORT.toString())
        }

        // 验证人参数
        item {
            SettingsSection("验证人参数")
        }
        item {
            InfoSettingItem("最大验证人数", MCParams.MAX_VALIDATORS.toString())
        }
        item {
            InfoSettingItem("最小自抵押", String.format("%.0f MC", MCParams.MIN_SELF_DELEGATE_UMC / 1_000_000.0))
        }

        // 惩罚规则
        item {
            SettingsSection("罚没规则")
        }
        item {
            InfoSettingItem("双签罚没", "${MCParams.DOUBLE_SIGN_SLASH_PERCENT}%")
        }
        item {
            InfoSettingItem("离线罚没", "${MCParams.DOWNTIME_SLASH_PERCENT}%")
        }
        item {
            InfoSettingItem("离线宽限", "${MCParams.DOWNTIME_SLASH_WINDOW_BLOCKS} 块")
        }

        // Tokenomics
        item {
            SettingsSection("代币经济")
        }
        item {
            InfoSettingItem("总供应量", "10 亿 MC")
        }
        item {
            InfoSettingItem("初始通胀", "${MCParams.INFLATION_RATE_PERCENT}%")
        }
        item {
            InfoSettingItem("目标质押率", "${MCParams.TARGET_STAKE_RATIO_PERCENT}%")
        }
        item {
            InfoSettingItem("社区池", "${MCParams.COMMUNITY_TAX_PERCENT}%")
        }

        // DePIN 奖励
        item {
            SettingsSection("DePIN 奖励池")
        }
        item {
            val poolMc = MCParams.DEPIN_INITIAL_POOL_UMC / 1_000_000.0
            InfoSettingItem("初始奖池", String.format("%.0f MC", poolMc))
        }
        item {
            InfoSettingItem("奖励分配", "${MCParams.CONTRIBUTOR_SHARE}% (贡献者) / ${MCParams.SECURITY_POOL_SHARE}% (安全池) / ${MCParams.BURN_SHARE}% (销毁)")
        }

        // 关于
        item {
            SettingsSection("关于")
        }
        item {
            InfoSettingItem("应用版本", "3.0.0")
        }
        item {
            InfoSettingItem("Cosmos SDK", "v0.47")
        }
        item {
            InfoSettingItem("CometBFT", "v0.37")
        }
        item {
            InfoSettingItem("IBC", "v7.1.0")
        }

        item {
            Spacer(modifier = Modifier.height(32.dp))
        }
    }

    // RPC 端点对话框
    if (showEndpointDialog) {
        var endpoint by remember { mutableStateOf(rpcEndpoint) }
        AlertDialog(
            onDismissRequest = { showEndpointDialog = false },
            title = { Text("RPC 端点") },
            text = {
                OutlinedTextField(
                    value = endpoint,
                    onValueChange = { endpoint = it },
                    label = { Text("端点地址") },
                    singleLine = true
                )
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

    // 功耗模式对话框
    if (showBatteryDialog) {
        AlertDialog(
            onDismissRequest = { showBatteryDialog = false },
            title = { Text("功耗模式") },
            text = {
                Column {
                    BatteryOption("low", "节能模式", "仅 Wi-Fi 充电时贡献资源", batteryConfig == "low") {
                        viewModel.setBatteryConfig("low"); showBatteryDialog = false
                    }
                    BatteryOption("medium", "均衡模式", "智能调节，平衡性能与功耗", batteryConfig == "medium") {
                        viewModel.setBatteryConfig("medium"); showBatteryDialog = false
                    }
                    BatteryOption("high", "高性能模式", "始终全速贡献（可能影响续航）", batteryConfig == "high") {
                        viewModel.setBatteryConfig("high"); showBatteryDialog = false
                    }
                }
            },
            confirmButton = {},
            dismissButton = {
                TextButton(onClick = { showBatteryDialog = false }) { Text("取消") }
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
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    description: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit
) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Icon(icon, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(24.dp))
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(title, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
                Text(description, style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
            }
            Switch(checked = checked, onCheckedChange = onCheckedChange)
        }
    }
}

@Composable
fun ClickableSettingItem(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    description: String,
    onClick: () -> Unit
) {
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
            Icon(icon, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(24.dp))
            Spacer(modifier = Modifier.width(12.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(title, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
                Text(description, style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
            }
            Icon(Icons.Filled.ChevronRight, null, tint = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.3f))
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
            Text(value, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium,
                color = MaterialTheme.colorScheme.primary)
        }
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
            Text(desc, style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
        }
    }
}
