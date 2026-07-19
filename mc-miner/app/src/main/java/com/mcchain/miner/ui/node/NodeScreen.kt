package com.mcchain.miner.ui.node

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
import com.mcchain.miner.data.model.*
import com.mcchain.miner.ui.theme.*
@Composable
fun NodeScreen(viewModel: NodeViewModel = androidx.lifecycle.viewmodel.compose.viewModel()) {
    val nodeStatus by viewModel.nodeStatus.collectAsState()
    val phoneNode by viewModel.phoneNode.collectAsState()
    val peers by viewModel.peers.collectAsState()
    val hardwareCheck by viewModel.hardwareCheck.collectAsState()
    val isAttesting by viewModel.isAttesting.collectAsState()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
        // 节点状态
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
                    val isActive = phoneNode?.status == NodeStatus.ACTIVE
                    Text(
                        text = if (isActive) "全节点运行中" else "节点未激活",
                        color = MaterialTheme.colorScheme.onPrimary,
                        style = MaterialTheme.typography.titleMedium
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = if (isActive) "设备已认证，参与网络共识" else "请完成设备认证以激活节点",
                        color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                        style = MaterialTheme.typography.bodySmall
                    )

                    Spacer(modifier = Modifier.height(16.dp))

                    if (phoneNode?.status != NodeStatus.ACTIVE) {
                        Button(
                            onClick = { viewModel.performAttestation() },
                            enabled = !isAttesting,
                            colors = ButtonDefaults.buttonColors(
                                containerColor = McGold,
                                contentColor = McPrimary
                            )
                        ) {
                            if (isAttesting) {
                                CircularProgressIndicator(
                                    modifier = Modifier.size(16.dp),
                                    color = McPrimary,
                                    strokeWidth = 2.dp
                                )
                                Spacer(modifier = Modifier.width(8.dp))
                            }
                            Text(if (isAttesting) "认证中..." else "认证设备")
                        }
                    } else {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceEvenly
                        ) {
                            NodeStatItem("区块高度", "#${nodeStatus?.currentHeight ?: 0}",
                                MaterialTheme.colorScheme.onPrimary)
                            NodeStatItem("连接节点", "${nodeStatus?.syncedPeers ?: 0}",
                                MaterialTheme.colorScheme.onPrimary)
                            NodeStatItem("认证状态", "有效",
                                StatusActive)
                        }
                    }
                }
            }
        }

        // 硬件检查
        item {
            hardwareCheck?.let { hw ->
                Card(shape = RoundedCornerShape(16.dp)) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text("硬件要求检查", fontWeight = FontWeight.Bold,
                            style = MaterialTheme.typography.titleSmall)
                        Spacer(modifier = Modifier.height(8.dp))

                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween
                        ) {
                            HwItem("RAM", "${hw.ramGb} GB", if (hw.ramGb >= 4) StatusActive else StatusSlashed)
                            HwItem("存储", "${hw.storageGb} GB", if (hw.storageGb >= 8) StatusActive else StatusSlashed)
                            HwItem("CPU", "${hw.cpuCores} 核", if (hw.cpuCores >= 4) StatusActive else StatusSlashed)
                        }

                        if (hw.issues.isNotEmpty()) {
                            Spacer(modifier = Modifier.height(8.dp))
                            hw.issues.forEach { issue ->
                                Text(issue, style = MaterialTheme.typography.labelSmall,
                                    color = StatusSlashed)
                            }
                        }
                    }
                }
            }
        }

        // PhoneNode 参数
        item {
            Card(shape = RoundedCornerShape(16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("PhoneNode 参数", fontWeight = FontWeight.Bold,
                        style = MaterialTheme.typography.titleSmall)
                    Spacer(modifier = Modifier.height(12.dp))
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        PhoneParamRow("离线宽限", "${MCParams.PHONENODE_OFFLINE_GRACE_SECONDS}s (${MCParams.PHONENODE_OFFLINE_GRACE_BLOCKS}块)")
                        PhoneParamRow("心跳间隔", "${MCParams.PHONENODE_HEARTBEAT_INTERVAL_SECONDS}s")
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        PhoneParamRow("认证有效期", "${MCParams.PHONENODE_ATTESTATION_VALIDITY_DAYS}天")
                        PhoneParamRow("防女巫", if (MCParams.PHONENODE_SYBIL_DEVICE_BINDING) "启用" else "未启用")
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("罚没规则", style = MaterialTheme.typography.labelMedium,
                        fontWeight = FontWeight.Medium)
                    Spacer(modifier = Modifier.height(4.dp))
                    SlashInfoRow("离线罚没", "${MCParams.PHONENODE_OFFLINE_SLASH_PERCENT}%")
                    SlashInfoRow("贡献作恶", "${MCParams.PHONENODE_CONTRIB_SLASH_PERCENT}%")
                    SlashInfoRow("认证造假", "${MCParams.PHONENODE_ATTEST_SLASH_PERCENT}%")
                }
            }
        }

        // 连接节点
        item {
            Text("对等节点 (${peers.size})", fontWeight = FontWeight.Bold,
                style = MaterialTheme.typography.titleMedium)
        }

        items(peers.size.coerceAtMost(15)) { index ->
            PeerItem(peers[index])
        }
    }
}

@Composable
fun NodeStatItem(label: String, value: String, color: androidx.compose.ui.graphics.Color) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(value, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.Bold, color = color)
        Text(label, style = MaterialTheme.typography.labelSmall, color = color.copy(alpha = 0.7f))
    }
}

@Composable
fun HwItem(label: String, value: String, color: androidx.compose.ui.graphics.Color) {
    Column {
        Text(value, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Bold, color = color)
        Text(label, style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
    }
}

@Composable
fun PhoneParamRow(label: String, value: String) {
    Column {
        Text(label, style = MaterialTheme.typography.labelSmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
        Text(value, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
    }
}

@Composable
fun SlashInfoRow(reason: String, percent: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 2.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(reason, style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
        Text(percent, style = MaterialTheme.typography.bodySmall,
            fontWeight = FontWeight.Bold,
            color = if (percent.startsWith("5") || percent.startsWith("1")) StatusGrace else StatusSlashed)
    }
}

@Composable
fun PeerItem(peer: PeerNode) {
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
            Column(modifier = Modifier.weight(1f)) {
                Text(peer.moniker.ifBlank { peer.nodeId.take(12) + "..." },
                    fontWeight = FontWeight.Medium,
                    style = MaterialTheme.typography.bodyMedium)
                Text(peer.address,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
            }
            Column(horizontalAlignment = Alignment.End) {
                Text("${peer.latency}ms",
                    style = MaterialTheme.typography.labelSmall,
                    color = if (peer.latency < 100) StatusActive else StatusGrace)
                Text(
                    java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault())
                        .format(java.util.Date(peer.lastSeen)),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.3f)
                )
            }
        }
    }
}
