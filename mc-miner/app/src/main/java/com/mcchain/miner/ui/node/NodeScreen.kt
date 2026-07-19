package com.mcchain.miner.ui.node

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.mcchain.miner.MCParams
import com.mcchain.miner.data.model.NodeStatus
import com.mcchain.miner.data.model.PeerNode
import com.mcchain.miner.ui.theme.*

@Composable
fun NodeScreen(vm: NodeViewModel = viewModel()) {
    val nodeStatus by vm.nodeStatus.collectAsState()
    val phoneNode by vm.phoneNode.collectAsState()
    val peers by vm.peers.collectAsState()
    val hwCheck by vm.hardwareCheck.collectAsState()
    val isAttesting by vm.isAttesting.collectAsState()

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp)
    ) {
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
                        text = if (isActive) "Full Node Running" else "Node Inactive",
                        color = MaterialTheme.colorScheme.onPrimary,
                        style = MaterialTheme.typography.titleMedium
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = if (isActive) "Device verified, participating in consensus" else "Verify device to activate node",
                        color = MaterialTheme.colorScheme.onPrimary.copy(alpha = 0.7f),
                        style = MaterialTheme.typography.bodySmall
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    if (phoneNode?.status != NodeStatus.ACTIVE) {
                        Button(
                            onClick = { vm.performAttestation() },
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
                            Text(if (isAttesting) "Verifying..." else "Verify Device")
                        }
                    } else {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceEvenly
                        ) {
                            NodeStatItem("Block", "#${nodeStatus?.currentHeight ?: 0}", MaterialTheme.colorScheme.onPrimary)
                            NodeStatItem("Peers", "${nodeStatus?.syncedPeers ?: 0}", MaterialTheme.colorScheme.onPrimary)
                            NodeStatItem("Status", "Valid", StatusActive)
                        }
                    }
                }
            }
        }

        val hardware = hwCheck
        if (hardware != null) {
            item {
                HardwareSection(hardware)
            }
        }

        item {
            Card(shape = RoundedCornerShape(16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("PhoneNode Parameters", fontWeight = FontWeight.Bold, style = MaterialTheme.typography.titleSmall)
                    Spacer(modifier = Modifier.height(12.dp))
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        PhoneParamRow("Offline Grace", "${MCParams.PHONENODE_OFFLINE_GRACE_SECONDS}s (${MCParams.PHONENODE_OFFLINE_GRACE_BLOCKS} blocks)")
                        PhoneParamRow("Heartbeat", "${MCParams.PHONENODE_HEARTBEAT_INTERVAL_SECONDS}s")
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
                        PhoneParamRow("Cert Validity", "${MCParams.PHONENODE_ATTESTATION_VALIDITY_DAYS}d")
                        PhoneParamRow("Anti-Sybil", if (MCParams.PHONENODE_SYBIL_DEVICE_BINDING) "ON" else "OFF")
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                    Text("Slashing Rules", style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Medium)
                    SlashInfoRow("Offline", "${MCParams.PHONENODE_OFFLINE_SLASH_PERCENT}%")
                    SlashInfoRow("Contribution Malice", "${MCParams.PHONENODE_CONTRIB_SLASH_PERCENT}%")
                    SlashInfoRow("Cert Fraud", "${MCParams.PHONENODE_ATTEST_SLASH_PERCENT}%")
                }
            }
        }

        item {
            Text("Peers (${peers.size})", fontWeight = FontWeight.Bold, style = MaterialTheme.typography.titleMedium)
        }

        items(peers.size.coerceAtMost(15)) { index ->
            PeerItem(peers[index])
        }
    }
}

@Composable
private fun HardwareSection(hw: com.mcchain.miner.domain.node.HardwareCheck) {
    Card(shape = RoundedCornerShape(16.dp)) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text("Hardware Check", fontWeight = FontWeight.Bold, style = MaterialTheme.typography.titleSmall)
            Spacer(modifier = Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween
            ) {
                HwItem("RAM", "${hw.ramGb} GB", if (hw.ramGb >= 4) StatusActive else StatusSlashed)
                HwItem("Storage", "${hw.storageGb} GB", if (hw.storageGb >= 8) StatusActive else StatusSlashed)
                HwItem("CPU", "${hw.cpuCores} cores", if (hw.cpuCores >= 4) StatusActive else StatusSlashed)
            }

            if (hw.issues.isNotEmpty()) {
                Spacer(modifier = Modifier.height(8.dp))
                hw.issues.forEach { issue ->
                    Text(issue, style = MaterialTheme.typography.labelSmall, color = StatusSlashed)
                }
            }
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
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
    }
}

@Composable
fun PhoneParamRow(label: String, value: String) {
    Column {
        Text(label, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
        Text(value, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.Medium)
    }
}

@Composable
fun SlashInfoRow(reason: String, percent: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(reason, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f))
        Text(percent, style = MaterialTheme.typography.bodySmall, fontWeight = FontWeight.Bold, color = if (percent.startsWith("5") || percent.startsWith("1")) StatusGrace else StatusSlashed)
    }
}

@Composable
fun PeerItem(peer: PeerNode) {
    Card(modifier = Modifier.fillMaxWidth(), shape = RoundedCornerShape(12.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(12.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(peer.moniker.ifBlank { peer.nodeId.take(12) + "..." }, fontWeight = FontWeight.Medium, style = MaterialTheme.typography.bodyMedium)
                Text(peer.address, style = MaterialTheme.typography.labelSmall, color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.5f))
            }
            Column(horizontalAlignment = Alignment.End) {
                Text("${peer.latency}ms", style = MaterialTheme.typography.labelSmall, color = if (peer.latency < 100) StatusActive else StatusGrace)
                Text(
                    java.text.SimpleDateFormat("HH:mm", java.util.Locale.getDefault()).format(java.util.Date(peer.lastSeen)),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.3f)
                )
            }
        }
    }
}
