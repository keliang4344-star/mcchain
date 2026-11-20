package com.mcchain.miner.service

import android.Manifest
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.hilt.work.HiltWorker
import androidx.work.CoroutineWorker
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import com.mcchain.miner.McMinerApp
import com.mcchain.miner.R
import com.mcchain.miner.data.pref.SecurePrefs
import com.mcchain.miner.ui.MainActivity
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import java.util.concurrent.TimeUnit

/**
 * 认证到期提醒。
 *
 * 数据来源只有 [SecurePrefs.attestationExpiry]（认证成功后写入的真实到期时间）；
 * 未完成认证时不做任何提醒，也不估算有效期。
 */
@HiltWorker
class AttestationReminderWorker @AssistedInject constructor(
    @Assisted context: Context,
    @Assisted params: WorkerParameters,
    private val securePrefs: SecurePrefs,
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val expiry = runCatching { securePrefs.attestationExpiry }.getOrDefault(0L)
        if (expiry <= 0L) return Result.success()

        val remaining = expiry - System.currentTimeMillis()
        if (remaining > REMIND_BEFORE_MS) return Result.success()

        showNotification(remaining)
        return Result.success()
    }

    private fun showNotification(remainingMs: Long) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ContextCompat.checkSelfPermission(
                applicationContext,
                Manifest.permission.POST_NOTIFICATIONS,
            ) != PackageManager.PERMISSION_GRANTED
        ) {
            return
        }
        if (!NotificationManagerCompat.from(applicationContext).areNotificationsEnabled()) return

        val content = if (remainingMs <= 0L) {
            "本机设备认证已过期，将失去接单与计奖资格，请立即打开 App 重新认证"
        } else {
            val days = (remainingMs / DAY_MS).coerceAtLeast(1L)
            "本机设备认证将在 $days 天内到期，请打开 App 完成一键续期"
        }

        val intent = Intent(applicationContext, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            applicationContext,
            NOTIFICATION_ID,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val notification = NotificationCompat.Builder(applicationContext, McMinerApp.CHANNEL_ALERT)
            .setSmallIcon(R.mipmap.ic_launcher)
            .setContentTitle("设备认证即将到期")
            .setContentText(content)
            .setStyle(NotificationCompat.BigTextStyle().bigText(content))
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()

        runCatching {
            NotificationManagerCompat.from(applicationContext).notify(NOTIFICATION_ID, notification)
        }
    }

    companion object {
        const val WORK_NAME = "mcchain_attestation_reminder"

        private const val DAY_MS = 24L * 60 * 60 * 1000

        /** 到期前 3 天开始提醒，与节点页的「即将到期」阈值保持一致。 */
        const val REMIND_BEFORE_MS = 3L * DAY_MS

        private const val NOTIFICATION_ID = 2001

        /** 注册每日检查（幂等：已存在则保留，不重置周期）。 */
        fun schedule(context: Context) {
            val request = PeriodicWorkRequestBuilder<AttestationReminderWorker>(1, TimeUnit.DAYS)
                .build()
            runCatching {
                WorkManager.getInstance(context).enqueueUniquePeriodicWork(
                    WORK_NAME,
                    ExistingPeriodicWorkPolicy.KEEP,
                    request,
                )
            }
        }
    }
}
