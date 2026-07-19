package com.mcchain.miner.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.mcchain.miner.data.pref.SecurePrefs
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject

/**
 * 开机自启广播接收器
 */
@AndroidEntryPoint
class BootReceiver : BroadcastReceiver() {

    @Inject lateinit var securePrefs: SecurePrefs

    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Intent.ACTION_BOOT_COMPLETED) return

        if (securePrefs.nodeSyncEnabled) {
            NodeService.start(context)
        }
        if (securePrefs.miningEnabled) {
            MinerService.start(context)
        }
    }
}
