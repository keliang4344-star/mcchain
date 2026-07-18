package com.mcchain.miner;

import android.content.Intent;
import android.os.Build;
import android.os.Bundle;
import android.view.View;
import android.os.Handler;
import android.os.Looper;
import android.webkit.JavascriptInterface;
import android.webkit.RenderProcessGoneDetail;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.ScrollView;
import android.widget.TextView;

import androidx.appcompat.app.AppCompatActivity;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.PrintWriter;
import java.io.StringWriter;
import java.util.Arrays;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Consumer;

public class MainActivity extends AppCompatActivity {

    private WebView webView;
    private TextView tvStatus, tvAddress, tvBalance, tvLog, tvBuild, tvMnemonic;
    private EditText etNodeUrl;
    private Button btnStart, btnRefresh, btnConnect, btnBackup, btnReset;
    private FrameLayout webHost;

    private android.content.SharedPreferences prefs;
    private static final String PREFS_NAME = "mc_wallet";
    private static final String KEY_MNEMONIC = "mnemonic";
    private static final String KEY_ADDRESS = "address";

    private String walletMnemonic = "";
    private String walletAddress = "";
    private String nodeHost = "localhost";
    private String nodePort = "26657";
    private int taskCounter = 0;
    private boolean mining = false;
    private boolean walletAsked = false;

    private final Handler handler = new Handler(Looper.getMainLooper());
    private final ConcurrentHashMap<String, Consumer<String>> pending = new ConcurrentHashMap<>();
    private final AtomicInteger idCounter = new AtomicInteger(0);

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        try {
            super.onCreate(savedInstanceState);
            setContentView(R.layout.activity_main);

            tvStatus = findViewById(R.id.tvStatus);
            tvAddress = findViewById(R.id.tvAddress);
            tvBalance = findViewById(R.id.tvBalance);
            tvLog = findViewById(R.id.tvLog);
            tvBuild = findViewById(R.id.tvBuild);
            etNodeUrl = findViewById(R.id.etNodeUrl);
            btnStart = findViewById(R.id.btnStart);
            btnRefresh = findViewById(R.id.btnRefresh);
            btnConnect = findViewById(R.id.btnConnect);
            webHost = findViewById(R.id.webHost);
            tvMnemonic = findViewById(R.id.tvMnemonic);
            btnBackup = findViewById(R.id.btnBackup);
            btnReset = findViewById(R.id.btnReset);

            prefs = getSharedPreferences(PREFS_NAME, MODE_PRIVATE);

            // 显示构建号，便于确认装的是哪个版本
            if (tvBuild != null) tvBuild.setText("构建: " + MCApp.BUILD_ID);

            // 初始化 WebView（部分国产 ROM 系统 WebView 被禁用，这里单独兜底）
            try {
                webView = new WebView(this);
            } catch (Throwable t) {
                showFatalError(t, "创建 WebView 失败：系统可能未安装/启用了 Android System WebView。\n" +
                        "请到「设置 → 应用 → Android System WebView」启用，或安装最新版 Chrome。");
                return;
            }

            webView.getSettings().setJavaScriptEnabled(true);
            webView.getSettings().setDomStorageEnabled(true);
            webView.getSettings().setAllowFileAccess(true);
            // 允许 file:// 页面加载同目录下的本地 JS（cosmjs-bundle.js）
            webView.getSettings().setAllowFileAccessFromFileURLs(true);
            webView.getSettings().setAllowUniversalAccessFromFileURLs(true);
            // 部分 ROM/GPU 上 WebView 硬件加速会触发 native 渲染崩溃，降级为软件渲染兜底
            try { webView.setLayerType(android.view.View.LAYER_TYPE_SOFTWARE, null); } catch (Throwable ignore) {}
            webView.addJavascriptInterface(new JsBridge(), "Android");
            webView.setWebViewClient(new WebViewClient() {
                @Override
                public void onPageFinished(WebView view, String url) {
                    super.onPageFinished(view, url);
                    if (!walletAsked) {
                        walletAsked = true;
                        String saved = prefs.getString(KEY_MNEMONIC, "");
                        if (saved != null && !saved.isEmpty()) {
                            restoreWallet(saved);
                        } else {
                            generateWallet();
                        }
                    }
                }

                // 关键兜底：WebView 渲染进程（native）崩溃时，默认行为会让整个 App 死亡
                // （这就是"崩溃自显示版仍然闪退"的最可能根因 —— Java try/catch 抓不到 native 崩溃）。
                // 重写它并返回 true，把崩溃拦截成可显示的界面，而不是杀进程。
                @Override
                public boolean onRenderProcessGone(WebView view, RenderProcessGoneDetail detail) {
                    String reason = (detail != null)
                            ? ("rendererCrashed=" + detail.didCrash())
                            : "unknown";
                    RuntimeException crash = new RuntimeException(
                            "WebView 渲染进程崩溃 (" + reason + ")。\n" +
                            "常见于：系统 WebView 版本过旧/损坏、2.6MB cosmjs 包在低内存机型 OOM、" +
                            "或 ROM 自带 WebView 缺陷。\n" +
                            "建议：到「设置→应用→Android System WebView」更新到最新版，或安装最新 Chrome 并设为默认。");
                    MCApp.writeCrashLog(crash, "onRenderProcessGone");
                    showFatalError(crash, "WebView 渲染进程崩溃，已拦截，App 未退出。");
                    return true; // 已处理，不触发默认崩溃
                }
            });
            webHost.addView(webView);
            webView.loadUrl("file:///android_asset/miner.html");

            btnConnect.setOnClickListener(v -> {
                String url = etNodeUrl.getText().toString().trim();
                if (url.contains(":")) {
                    nodeHost = url.split(":")[0];
                    nodePort = url.split(":")[1];
                } else {
                    nodeHost = url;
                }
                connectToNode();
            });

            btnStart.setOnClickListener(v -> {
                if (walletAddress.isEmpty()) {
                    generateWallet();
                    return;
                }
                if (mining) {
                    log("挖矿已停止");
                    mining = false;
                    btnStart.setText("开始挖矿");
                    return;
                }
                startMining();
            });

            btnRefresh.setOnClickListener(v -> refreshBalance());

            btnBackup.setOnClickListener(v -> {
                if (walletMnemonic.isEmpty()) {
                    toast("钱包尚未生成");
                    return;
                }
                copyToClipboard(walletMnemonic);
                toast("助记词已复制，请立即离线备份");
            });

            btnReset.setOnClickListener(v -> {
                new android.app.AlertDialog.Builder(this)
                        .setTitle("重新生成钱包？")
                        .setMessage("将清空当前助记词并生成新钱包。\n请确认已备份旧助记词，否则旧地址资产永久无法找回！")
                        .setPositiveButton("我已备份，重新生成", (d, w) -> {
                            prefs.edit().remove(KEY_MNEMONIC).remove(KEY_ADDRESS).apply();
                            walletMnemonic = "";
                            walletAddress = "";
                            generateWallet();
                        })
                        .setNegativeButton("取消", null)
                        .show();
            });

        } catch (Throwable t) {
            showFatalError(t, "MainActivity.onCreate 初始化失败");
        }
    }

    // 异步调用 JS（回调在主线程）
    private void callJs(String name, Consumer<String> onResult, Object... args) {
        String id = "a" + idCounter.incrementAndGet();
        pending.put(id, onResult);
        String jsArgs = new JSONArray(Arrays.asList(args)).toString();
        final String expr = "MC.dispatch('" + id + "','" + name + "'," + jsArgs + ")";
        handler.post(() -> webView.evaluateJavascript(expr, null));
    }

    // 同步调用 JS（后台线程用，最多等 30s）
    private String callJsSync(String name, Object... args) throws Exception {
        final String[] out = {null};
        final CountDownLatch latch = new CountDownLatch(1);
        String id = "s" + idCounter.incrementAndGet();
        pending.put(id, r -> { out[0] = r; latch.countDown(); });
        String jsArgs = new JSONArray(Arrays.asList(args)).toString();
        final String expr = "MC.dispatch('" + id + "','" + name + "'," + jsArgs + ")";
        handler.post(() -> webView.evaluateJavascript(expr, null));
        latch.await(30, TimeUnit.SECONDS);
        return out[0];
    }

    private void generateWallet() {
        handler.post(() -> tvStatus.setText("正在生成钱包..."));
        callJs("genWallet", result -> {
            try {
                JSONObject outer = new JSONObject(result);
                if (!outer.getBoolean("ok")) {
                    tvStatus.setText("钱包生成失败: " + outer.getString("error"));
                    return;
                }
                JSONObject w = outer.getJSONObject("data");
                walletMnemonic = w.getString("mnemonic");
                walletAddress = w.getString("address");
                tvAddress.setText("地址: " + walletAddress);
                showMnemonic(walletMnemonic);
                persistWallet();
                log("钱包已生成: " + walletAddress);
                tvStatus.setText("钱包就绪，请连接节点");
            } catch (Exception e) {
                tvStatus.setText("钱包生成失败: " + e.getMessage());
            }
        }, "mc");
    }

    private void connectToNode() {
        handler.post(() -> tvStatus.setText("连接节点 " + nodeHost + ":" + nodePort + "..."));
        callJs("getChainId", result -> {
            try {
                JSONObject outer = new JSONObject(result);
                if (!outer.getBoolean("ok")) {
                    tvStatus.setText("连接失败: " + outer.getString("error"));
                    log("连接失败: " + outer.getString("error"));
                    return;
                }
                JSONObject info = outer.getJSONObject("data");
                String chainId = info.getString("chainId");
                long height = info.getLong("height");
                tvStatus.setText("已连接: " + chainId + " (高度 " + height + ")");
                log("节点连接成功: " + nodeHost + ":" + nodePort);
                refreshBalance();
            } catch (Exception e) {
                tvStatus.setText("连接失败: " + e.getMessage());
                log("连接失败: " + e.getMessage());
            }
        }, nodeHost, nodePort);
    }

    private void refreshBalance() {
        if (walletAddress.isEmpty()) return;
        callJs("getBalance", result -> {
            try {
                JSONObject outer = new JSONObject(result);
                if (!outer.getBoolean("ok")) {
                    log("查询余额失败: " + outer.getString("error"));
                    return;
                }
                JSONArray balances = outer.getJSONArray("data");
                long total = 0;
                for (int i = 0; i < balances.length(); i++) {
                    JSONObject coin = balances.getJSONObject(i);
                    if (coin.getString("denom").equals("umc")) {
                        total = coin.getLong("amount");
                    }
                }
                double mc = total / 1000000.0;
                tvBalance.setText(String.format("余额: %d umc (%.4f MC)", total, mc));
            } catch (Exception e) {
                log("查询余额失败: " + e.getMessage());
            }
        }, walletAddress, nodeHost, nodePort);
    }

    private void startMining() {
        mining = true;
        btnStart.setText("停止挖矿");
        log("挖矿开始...");

        new Thread(() -> {
            while (mining) {
                try {
                    final int taskId = ++taskCounter;
                    handler.post(() -> log("第" + taskId + "轮挖矿开始..."));

                    handler.post(() -> tvStatus.setText("注册节点中..."));
                    final String r1 = callJsSync("registerNode", walletMnemonic, nodeHost, nodePort);
                    handler.post(() -> log("注册节点: " + summarize(r1)));
                    Thread.sleep(5000);

                    handler.post(() -> tvStatus.setText("注册设备中..."));
                    final String r2 = callJsSync("registerDevice", walletMnemonic, nodeHost, nodePort);
                    handler.post(() -> log("注册设备: " + summarize(r2)));
                    Thread.sleep(5000);

                    String type = (taskId % 3 == 0) ? "inference" : (taskId % 3 == 1) ? "data_label" : "bandwidth";
                    handler.post(() -> tvStatus.setText("提交贡献 " + type + "..."));
                    final String r3 = callJsSync("submitContribution", walletMnemonic, nodeHost, nodePort,
                            "task-mobile-" + taskId, type, "85");
                    handler.post(() -> log("贡献结果: " + summarize(r3)));

                    handler.postDelayed(this::refreshBalance, 2000);

                    for (int i = 0; i < 30 && mining; i++) Thread.sleep(1000);

                } catch (Exception e) {
                    handler.post(() -> log("挖矿错误: " + e.getMessage()));
                }
            }
        }).start();
    }

    private String summarize(String json) {
        if (json == null) return "null";
        try {
            JSONObject outer = new JSONObject(json);
            if (!outer.getBoolean("ok")) return "ERR:" + outer.getString("error");
            JSONObject d = outer.getJSONObject("data");
            if (d.has("code")) return "code=" + d.getInt("code") + " tx=" + d.optString("txHash", "");
            return json.length() > 120 ? json.substring(0, 120) + "..." : json;
        } catch (Exception e) {
            return json.length() > 120 ? json.substring(0, 120) + "..." : json;
        }
    }

    private void log(String msg) {
        String current = tvLog.getText().toString();
        tvLog.setText(current + "\n" + msg);
    }

    // 把助记词显示到界面，提醒用户备份
    private void showMnemonic(String m) {
        if (tvMnemonic != null) {
            tvMnemonic.setText("助记词（请抄写备份）:\n" + m);
            tvMnemonic.setTextIsSelectable(true);
        }
    }

    // 持久化到 SharedPreferences，重启（同包未卸载）后免重新生成
    private void persistWallet() {
        try {
            prefs.edit().putString(KEY_MNEMONIC, walletMnemonic)
                    .putString(KEY_ADDRESS, walletAddress).apply();
        } catch (Throwable ignore) {}
    }

    // 从已保存的助记词恢复钱包（不重新生成）
    private void restoreWallet(String mnemonic) {
        handler.post(() -> tvStatus.setText("正在恢复钱包..."));
        callJs("restoreWallet", result -> {
            try {
                JSONObject outer = new JSONObject(result);
                if (!outer.getBoolean("ok")) {
                    log("恢复失败，重新生成: " + outer.getString("error"));
                    generateWallet();
                    return;
                }
                JSONObject w = outer.getJSONObject("data");
                walletMnemonic = w.getString("mnemonic");
                walletAddress = w.getString("address");
                tvAddress.setText("地址: " + walletAddress);
                showMnemonic(walletMnemonic);
                persistWallet();
                log("钱包已恢复: " + walletAddress);
                tvStatus.setText("钱包就绪，请连接节点");
            } catch (Exception e) {
                tvStatus.setText("钱包恢复失败: " + e.getMessage());
                generateWallet();
            }
        }, mnemonic, "mc");
    }

    private void copyToClipboard(String text) {
        try {
            android.content.ClipboardManager cm =
                    (android.content.ClipboardManager) getSystemService(CLIPBOARD_SERVICE);
            if (cm != null) cm.setPrimaryClip(android.content.ClipData.newPlainText("mc_mnemonic", text));
        } catch (Throwable ignore) {}
    }

    private void toast(String msg) {
        handler.post(() -> android.widget.Toast.makeText(this, msg, android.widget.Toast.LENGTH_SHORT).show());
    }

    public class JsBridge {
        @JavascriptInterface
        public void log(String msg) {
            handler.post(() -> MainActivity.this.log("[JS] " + msg));
        }

        @JavascriptInterface
        public void bridge(String id, String json) {
            Consumer<String> cb = pending.remove(id);
            if (cb != null) {
                final String r = json;
                handler.post(() -> cb.accept(r));
            }
        }
    }

    // 把崩溃原因显示到屏幕（而不是闪退），同时写入外部存储便于后续取回
    private void showFatalError(Throwable t, String hint) {
        try {
            StringWriter sw = new StringWriter();
            t.printStackTrace(new PrintWriter(sw));

            StringBuilder sb = new StringBuilder();
            sb.append("MC Miner 启动失败\n");
            sb.append("构建: ").append(MCApp.BUILD_ID).append("\n");
            sb.append("机型: ").append(Build.MANUFACTURER).append(" ").append(Build.MODEL)
              .append(" SDK=").append(Build.VERSION.SDK_INT).append("\n");
            try {
                sb.append("WebView: ").append(WebView.getCurrentWebViewPackage().versionName).append("\n");
            } catch (Throwable e) {
                sb.append("WebView: 未知\n");
            }
            sb.append("\n");
            sb.append(hint != null ? hint + "\n\n" : "");
            sb.append(t.getClass().getName()).append(": ").append(t.getMessage()).append("\n\n");
            sb.append("==== 堆栈 ====\n");
            sb.append(sw.toString());

            // 写入外部存储（也可被任意文件管理器直接打开，无需 USB）
            try {
                java.io.File dir = getExternalFilesDir(null);
                if (dir != null) {
                    java.io.File f = new java.io.File(dir, "crash.txt");
                    try (java.io.FileWriter w = new java.io.FileWriter(f)) {
                        w.write(sb.toString());
                    }
                }
            } catch (Throwable ignore) {}

            ScrollView sv = new ScrollView(this);
            android.widget.LinearLayout box = new android.widget.LinearLayout(this);
            box.setOrientation(android.widget.LinearLayout.VERTICAL);
            box.setPadding(24, 24, 24, 24);

            TextView tv = new TextView(this);
            tv.setText(sb.toString());
            tv.setTextSize(11);
            box.addView(tv);

            Button shareBtn = new Button(this);
            shareBtn.setText("分享/复制错误日志");
            shareBtn.setOnClickListener(v -> {
                Intent intent = new Intent(Intent.ACTION_SEND);
                intent.setType("text/plain");
                intent.putExtra(Intent.EXTRA_TEXT, sb.toString());
                startActivity(Intent.createChooser(intent, "把崩溃日志发给开发者"));
            });
            box.addView(shareBtn);

            sv.addView(box);
            setContentView(sv);
        } catch (Throwable ignore) {
            // 连错误界面都出不来就放弃（进程仍将结束，但这是最后兜底）
        }
    }

    @Override
    protected void onDestroy() {
        try {
            if (webView != null) webView.destroy();
        } catch (Throwable ignore) {}
        super.onDestroy();
    }
}
