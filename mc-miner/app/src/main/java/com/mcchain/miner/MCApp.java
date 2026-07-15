package com.mcchain.miner;

import android.app.Application;
import android.content.Context;
import android.os.Build;
import android.webkit.WebView;

import java.io.File;
import java.io.FileWriter;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.text.SimpleDateFormat;
import java.util.Date;
import java.util.Locale;

/**
 * 全局 Application：
 * 1) 安装进程级未捕获异常处理器，把任何崩溃（含 native 之外的 Java 异常）
 *    连同设备/WebView 版本 + BUILD_ID 写入外部存储 crash.txt。
 * 2) 这样即便用户无法连 USB，也能用任意文件管理器打开
 *    Android/data/com.mcchain.miner/files/crash.txt 把内容发给我们。
 */
public class MCApp extends Application {

    // 每次出包改这个值 —— 让用户在界面上即可确认装的是哪个版本
    public static final String BUILD_ID = "2026-07-13-16";
    public static final String VERSION_NAME = "1.0";

    private static Context appContext;

    @Override
    public void onCreate() {
        super.onCreate();
        appContext = getApplicationContext();
        installGlobalCrashHandler();
    }

    private void installGlobalCrashHandler() {
        final Thread.UncaughtExceptionHandler previous =
                Thread.getDefaultUncaughtExceptionHandler();
        Thread.setDefaultUncaughtExceptionHandler((thread, throwable) -> {
            try {
                writeCrashLog(throwable, "UncaughtException@" + thread.getName());
            } catch (Throwable ignore) {
                // 写日志本身失败也不能影响后续流程
            }
            if (previous != null) {
                previous.uncaughtException(thread, throwable);
            }
        });
    }

    /** 把崩溃信息写入外部存储 crash.txt（best-effort，绝不抛异常） */
    public static void writeCrashLog(Throwable t, String where) {
        try {
            if (t == null) return;
            StringWriter sw = new StringWriter();
            t.printStackTrace(new PrintWriter(sw));

            StringBuilder sb = new StringBuilder();
            sb.append("MC Miner 崩溃日志\n");
            sb.append("BUILD_ID=").append(BUILD_ID).append("\n");
            sb.append("WHERE=").append(where == null ? "unknown" : where).append("\n");
            sb.append("TIME=")
              .append(new SimpleDateFormat("yyyy-MM-dd HH:mm:ss", Locale.US).format(new Date()))
              .append("\n");
            sb.append("DEVICE=").append(Build.MANUFACTURER).append(" ")
              .append(Build.MODEL).append(" SDK=").append(Build.VERSION.SDK_INT).append("\n");
            try {
                sb.append("WEBVIEW=").append(WebView.getCurrentWebViewPackage().versionName).append("\n");
            } catch (Throwable e) {
                sb.append("WEBVIEW=unknown\n");
            }
            sb.append("\n").append(t.getClass().getName()).append(": ")
              .append(t.getMessage()).append("\n\n");
            sb.append("==== 堆栈 ====\n").append(sw.toString());

            File dir = (appContext != null) ? appContext.getExternalFilesDir(null) : null;
            if (dir != null) {
                File f = new File(dir, "crash.txt");
                try (FileWriter w = new FileWriter(f)) {
                    w.write(sb.toString());
                }
            }
        } catch (Throwable ignore) {
            // 最后兜底：什么都不能做
        }
    }
}
