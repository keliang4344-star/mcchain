# MC矿工 - 手机挖矿 APP

在手机上生成 MC 公链钱包，连接主网/测试网节点，自动执行挖矿流程（注册节点 → 注册设备 → 提交贡献）。

## 已为你编译好的 APK

> **`app/build/outputs/apk/debug/app-debug.apk`**（已生成，可直接装手机测试）
> 包名：`com.mcchain.miner`，最低 Android 8.0（API 26），目标 API 34。

安装方式（任选其一）：
1. **adb 安装**（电脑连手机，开 USB 调试）：
   ```
   adb install app/build/outputs/apk/debug/app-debug.apk
   ```
2. **直接拷到手机**：把 `app-debug.apk` 传到手机，用文件管理器点击安装（需允许"未知来源"）。

---

## 用法（手机端）

1. 打开 APP → 自动生成钱包地址（助记词存手机本地，`data` 分区，删 APP 即丢失）。
2. 底部"节点地址:端口"填你的节点，例如 `192.168.1.100:26657`（默认已填 RPC 端口 26657）。
3. 点"连接" → 显示链 ID 和高度即成功。
4. 点"开始挖矿" → 自动循环：注册节点 → 注册设备 → 提交贡献（每轮间隔约 40 秒）。
5. 点"刷新余额"查看 `umc` 到账。

### 节点侧要求
- RPC 端口 `26657` 必须对外可访问。启动节点时加：
  ```
  mcchaind start --rpc.laddr tcp://0.0.0.0:26657
  ```
- 手机与节点需在同一网络，或节点有公网 IP。
- 当前挖矿手续费为 0，但钱包地址需先有 `umc` 余额才能广播交易（主网需先转入）。

---

## 技术原理

APP 用 **WebView + CosmJS**（Cosmos 官方 JS 库）直接通过 CometBFT RPC 与链交互：
- 钱包：`BIP39` 助记词 + `secp256k1` HD 钱包（纯本地，离线可生成）。
- 交易：本地签名 → 直接广播到节点 RPC，不经过任何第三方服务器。
- **CosmJS 已打包进 APK**（`assets/cosmjs-bundle.js`，由 esbuild 把 5 个 `@cosmjs/*@0.32.4` 包打成一个 UMD bundle），**无需联网即可运行**；浏览器 `crypto.subtle` 不可用时自动回退到纯 JS 的 `@noble/hashes` 实现。

---

## 本机工具链（已安装，供重新编译用）

| 组件 | 路径 |
|---|---|
| JDK 17 | `$HOME/jdk17` |
| Android SDK | `$HOME/android-sdk`（含 platform-tools、platforms;android-34、build-tools;34.0.0） |
| Gradle 8.4 | `$HOME/gradle-8.4` |

> 注意：项目**没有 gradlew 包装脚本**，直接用 `gradle-8.4/bin/gradle.bat` 构建（见下）。

### 重新编译（命令行）
```bat
set JAVA_HOME=$HOME/jdk17
set ANDROID_HOME=$HOME/android-sdk
cd $HOME/mc-miner
$HOME/gradle-8.4/bin/gradle.bat assembleDebug
:: 产出 app/build/outputs/apk/debug/app-debug.apk
```
或在 Android Studio 里 `Open` 本工程后点 Run。

### 若要正式发布包（release）
debug 包已可测试。正式发布需自签名：
```bat
:: 1) 生成密钥（仅一次）
keytool -genkey -v -keystore mc-release.keystore -alias mc -keyalg RSA -keysize 2048 -validity 10000
:: 2) 用 Android Studio Build > Generate Signed Bundle / APK，或用命令行 zipalign + apksigner
```

---

## 文件结构
```
mc-miner/
├── build.gradle.kts              # 项目配置（AGP 8.2）
├── settings.gradle.kts           # 模块 + 仓库（dependencyResolutionManagement）
├── local.properties              # sdk.dir 指向本地 Android SDK
├── gradle.properties
├── app/
│   ├── build.gradle.kts          # APP 编译配置
│   └── src/main/
│       ├── AndroidManifest.xml   # 权限: INTERNET / ACCESS_NETWORK_STATE
│       ├── assets/
│       │   ├── miner.html        # 挖矿逻辑（引用本地 cosmjs-bundle.js）
│       │   └── cosmjs-bundle.js  # 本地化的 CosmJS（已打入 APK）
│       ├── java/com/mcchain/miner/MainActivity.java
│       └── res/ (layout/values/themes)
```

## 已知边界
- 当前为 debug 自签名包，仅供测试；安装到手机需在"设置"允许未知来源。
- 主网未上线前，请连测试网节点验证挖矿链路。
- 助记词仅存于手机内存（运行期），不做持久化备份，重装 APP 钱包丢失——后续如需持久化再加本地加密存储。
