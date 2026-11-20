package com.mcchain.miner.util

import android.content.Context
import android.content.Intent
import android.os.Build
import androidx.core.content.FileProvider
import androidx.core.content.pm.PackageInfoCompat
import java.io.File
import java.io.PrintWriter
import java.io.StringWriter
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 本地崩溃日志：只落盘到应用缓存目录，用户主动分享时才离开设备。
 * 不引入第三方采集 SDK，也不自动上传。
 */
object CrashReporter {

    private const val DIR_NAME = "crashes"
    private const val MAX_FILES = 5

    /** 安装全局未捕获异常处理器，保留系统默认行为（崩溃后进程仍会退出）。 */
    fun install(context: Context) {
        val appContext = context.applicationContext
        val previous = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, throwable ->
            runCatching { write(appContext, thread, throwable) }
            if (previous != null) {
                previous.uncaughtException(thread, throwable)
            } else {
                android.os.Process.killProcess(android.os.Process.myPid())
                kotlin.system.exitProcess(10)
            }
        }
    }

    /** 最近一次崩溃日志；没有则返回 null。 */
    fun latest(context: Context): File? =
        crashDir(context).listFiles()
            ?.filter { it.isFile && it.extension == "txt" }
            ?.maxByOrNull { it.lastModified() }

    /**
     * 通过系统分享面板导出最近一次崩溃日志。
     * @return false 表示没有日志或系统无法处理分享意图。
     */
    fun shareLatest(context: Context): Boolean {
        val file = latest(context) ?: return false
        return runCatching {
            val uri = FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                file,
            )
            val intent = Intent(Intent.ACTION_SEND).apply {
                type = "text/plain"
                putExtra(Intent.EXTRA_STREAM, uri)
                putExtra(Intent.EXTRA_SUBJECT, "MC 矿工崩溃日志")
                addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
            context.startActivity(Intent.createChooser(intent, "分享崩溃日志"))
            true
        }.getOrDefault(false)
    }

    private fun write(context: Context, thread: Thread, throwable: Throwable) {
        val dir = crashDir(context)
        if (!dir.exists()) dir.mkdirs()

        val timestamp = SimpleDateFormat("yyyyMMdd-HHmmss", Locale.US).format(Date())
        val file = File(dir, "crash-$timestamp.txt")
        val stack = StringWriter().also { writer ->
            PrintWriter(writer).use { throwable.printStackTrace(it) }
        }.toString()

        val version = runCatching {
            val info = context.packageManager.getPackageInfo(context.packageName, 0)
            "${info.versionName} (${PackageInfoCompat.getLongVersionCode(info)})"
        }.getOrDefault("unknown")

        file.writeText(
            buildString {
                appendLine("时间: ${Date()}")
                appendLine("版本: $version")
                appendLine("设备: ${Build.MANUFACTURER} ${Build.MODEL}")
                appendLine("系统: Android ${Build.VERSION.RELEASE} (API ${Build.VERSION.SDK_INT})")
                appendLine("线程: ${thread.name}")
                appendLine()
                append(stack)
            }
        )

        // 只保留最近 MAX_FILES 份，避免长期占用缓存。
        dir.listFiles()?.filter { it.isFile }
            ?.sortedByDescending { it.lastModified() }
            ?.drop(MAX_FILES)
            ?.forEach { it.delete() }
    }

    private fun crashDir(context: Context): File = File(context.cacheDir, DIR_NAME)
}
