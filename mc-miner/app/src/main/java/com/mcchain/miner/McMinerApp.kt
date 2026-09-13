package com.mcchain.miner

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.os.Build
import androidx.hilt.work.HiltWorkerFactory
import androidx.work.Configuration
import dagger.hilt.android.HiltAndroidApp
import timber.log.Timber
import java.io.File
import javax.inject.Inject

@HiltAndroidApp
class McMinerApp : Application(), Configuration.Provider {

    @Inject
    lateinit var workerFactory: HiltWorkerFactory

    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder()
            .setWorkerFactory(workerFactory)
            .setMinimumLoggingLevel(
                if (BuildConfig.DEBUG) android.util.Log.DEBUG
                else android.util.Log.INFO
            )
            .build()

    override fun onCreate() {
        super.onCreate()
        instance = this

        if (BuildConfig.DEBUG) {
            Timber.plant(Timber.DebugTree())
        }

        createNotificationChannels()
    }

    private fun createNotificationChannels() {
        val manager = getSystemService(NotificationManager::class.java)

        // 节点同步通知通道
        val nodeChannel = NotificationChannel(
            CHANNEL_NODE,
            "节点同步",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "显示区块同步和节点状态"
            setShowBadge(false)
        }
        manager.createNotificationChannel(nodeChannel)

        // 挖矿贡献通知通道
        val miningChannel = NotificationChannel(
            CHANNEL_MINING,
            "贡献挖矿",
            NotificationManager.IMPORTANCE_LOW
        ).apply {
            description = "显示挖矿贡献状态和收益"
            setShowBadge(false)
        }
        manager.createNotificationChannel(miningChannel)

        // 重要提醒通道
        val alertChannel = NotificationChannel(
            CHANNEL_ALERT,
            "重要提醒",
            NotificationManager.IMPORTANCE_HIGH
        ).apply {
            description = "罚没警告、下线通知等重要提醒"
        }
        manager.createNotificationChannel(alertChannel)
    }

    companion object {
        const val CHANNEL_NODE = "mcchain_node"
        const val CHANNEL_MINING = "mcchain_mining"
        const val CHANNEL_ALERT = "mcchain_alert"

        lateinit var instance: McMinerApp
            private set
    }
}
