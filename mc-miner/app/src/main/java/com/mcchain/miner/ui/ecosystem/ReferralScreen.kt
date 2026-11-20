package com.mcchain.miner.ui.ecosystem

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.mcchain.miner.network.ReferralRecord

/**
 * 邀请返佣页。
 *
 * 链端语义：由邀请人（当前钱包）为已注册节点的被邀请人建立关系；
 * 返佣金额与状态全部来自链上查询，App 不预估收益。
 */
@Composable
fun ReferralScreen(viewModel: EcosystemViewModel, onBack: () -> Unit) {
    val loading by viewModel.referralLoading.collectAsState()
    val referrals by viewModel.referrals.collectAsState()
    val pending by viewModel.referralPendingUmc.collectAsState()
    val error by viewModel.referralError.collectAsState()
    val message by viewModel.referralMessage.collectAsState()
    val busy by viewModel.referralBusy.collectAsState()
    val account by viewModel.accountAddress.collectAsState()

    var invitee by rememberSaveable { mutableStateOf("") }
    var inviteCode by rememberSaveable { mutableStateOf("") }

    LaunchedEffect(Unit) { viewModel.loadReferral() }

    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            EcosystemHeader(
                title = "邀请返佣",
                subtitle = account?.let { "邀请人 ${shortenAddress(it)}" } ?: "未连接钱包",
                onBack = onBack,
                onRefresh = { viewModel.loadReferral() },
                refreshing = loading,
            )
        }

        item { EcosystemMessageCard(message) { viewModel.clearReferralMessage() } }

        item {
            EcosystemInfoCard(
                title = "链上规则",
                lines = listOf(
                    "你是邀请人：填写对方的账户地址完成绑定，对方必须已完成节点注册。",
                    "被邀请人不能已被他人邀请，也不能邀请自己；同一邀请人连续邀请受冷却区块限制。",
                    "返佣按链上多级比例累计，资金来自生态池，不从被邀请人余额扣除。",
                    "App 只展示链上待领金额与邀请记录，不提供任何收益预估。",
                ),
            )
        }

        item {
            Card(shape = RoundedCornerShape(12.dp)) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text("待领返佣", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            text = if (pending == null) "—" else "${formatMcAmount(pending!!)} MC",
                            style = MaterialTheme.typography.headlineSmall,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.weight(1f),
                        )
                        Button(
                            onClick = { viewModel.claimReferralRewards() },
                            enabled = !busy && (pending ?: 0L) > 0L,
                        ) {
                            if (busy) {
                                CircularProgressIndicator(
                                    modifier = Modifier.size(14.dp),
                                    strokeWidth = 2.dp,
                                    color = MaterialTheme.colorScheme.onPrimary,
                                )
                                Spacer(Modifier.width(6.dp))
                            }
                            Text("领取")
                        }
                    }
                    if (pending == 0L) {
                        Text(
                            "当前没有可领取的返佣",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                        )
                    }
                }
            }
        }

        item {
            Card(shape = RoundedCornerShape(12.dp)) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    Text("绑定被邀请人", style = MaterialTheme.typography.titleSmall, fontWeight = FontWeight.SemiBold)
                    OutlinedTextField(
                        value = invitee,
                        onValueChange = { invitee = it.trim() },
                        modifier = Modifier.fillMaxWidth(),
                        label = { Text("被邀请人地址（mc1…）") },
                        singleLine = true,
                    )
                    OutlinedTextField(
                        value = inviteCode,
                        onValueChange = { if (it.length <= 32) inviteCode = it },
                        modifier = Modifier.fillMaxWidth(),
                        label = { Text("渠道码（可选，最多 32 字符）") },
                        singleLine = true,
                    )
                    Button(
                        onClick = { viewModel.createReferral(invitee, inviteCode) },
                        enabled = !busy && invitee.isNotBlank(),
                        modifier = Modifier.fillMaxWidth(),
                    ) {
                        Text(if (busy) "提交中…" else "提交绑定")
                    }
                    Text(
                        "绑定成功后立即写入链上；失败原因（未注册节点、已被邀请、冷却期等）会原样展示。",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.6f),
                    )
                }
            }
        }

        item { SectionLabel("我的邀请（${referrals.size}）") }

        when {
            error != null -> item { EcosystemInfoCard("暂时无法读取", listOf(error.orEmpty())) }
            loading && referrals.isEmpty() -> item {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.Center,
                ) {
                    CircularProgressIndicator(modifier = Modifier.size(22.dp), strokeWidth = 2.dp)
                }
            }
            referrals.isEmpty() -> item {
                EcosystemInfoCard("暂无邀请记录", listOf("完成一次绑定后，记录会显示在这里。"))
            }
            else -> items(referrals, key = { it.referralId }) { record ->
                ReferralItem(record)
            }
        }
    }
}

@Composable
private fun ReferralItem(record: ReferralRecord) {
    Card(shape = RoundedCornerShape(12.dp)) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = shortenAddress(record.invitee),
                    style = MaterialTheme.typography.bodyMedium,
                    fontWeight = FontWeight.Medium,
                    modifier = Modifier.weight(1f),
                )
                Text(
                    text = referralStatusLabel(record.status),
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.primary,
                )
            }
            Text(
                "编号 #${record.referralId} · 创建高度 ${record.createdAtHeight}",
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
            )
            if (record.inviteCode.isNotBlank()) {
                Text(
                    "渠道码 ${record.inviteCode}",
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.55f),
                )
            }
        }
    }
}

private fun referralStatusLabel(status: String): String = when (status) {
    "active" -> "生效中"
    "claimed" -> "已结算"
    "" -> "状态未知"
    else -> status
}
