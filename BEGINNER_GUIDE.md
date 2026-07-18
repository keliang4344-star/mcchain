# MC 公链 · 新手上链指南（云服务商新加坡轻量服务器）

> 目标：把你买的云服务商新加坡轻量服务器，变成 MC 公链的**第一个创世节点**（出块）。
> 前置：你已经有一台云服务商**轻量应用服务器**（新加坡地域，Linux/Ubuntu 22.04，建议 ≥2 vCPU / ≥4GB / ≥60GB SSD）。

---

## 一、准备（在你自己这台 Windows 上做）

我（AI）已经帮你编译好了 Linux 二进制，并写好了"一键脚本"。你需要把这 **2 个文件** 传到服务器：

1. `build/mcchaind` —— 链程序（Linux 版）
2. `server_setup.sh` —— 一键启动脚本

传文件的两种方式（任选其一）：

### 方式 A：云服务商控制台网页上传（最简单，不用配 SSH）
1. 打开云服务商轻量服务器控制台 → 找到你的服务器 → 点 **「登录」**（浏览器内会打开一个终端窗口）。
2. 在左侧文件管理器里，把本机 `$HOME/mcchain\build\mcchaind` 和 `$HOME/mcchain\server_setup.sh` 拖进去（或点上传按钮）。
   - 建议传到服务器的 `/root/` 目录。

### 方式 B：本机用 scp 命令上传（需要你先设置服务器密码/SSH）
在你这台 Windows 的终端（或 AI 这边的 Bash）里：
```bash
# 把下面 <服务器IP> 换成你的公网 IP，<用户名> 通常是 root
scp $HOME/mcchain\build\mcchaind root@<服务器IP>:/root/
scp $HOME/mcchain\server_setup.sh root@<服务器IP>:/root/
```

---

## 二、在服务器上运行（在服务器终端里输入）

```bash
cd /root
chmod +x server_setup.sh
sudo ./server_setup.sh
```

脚本会自动完成：
1. 初始化节点
2. 把创世规范成 MC 标准（币种 umc、通胀清零、治理参数）
3. 创建验证人密钥（**助记词会保存到 `validator_key.json`，务必备份！**）
4. 给验证人拨款 200k MC
5. 生成 gentx（自抵押 30k MC）
6. 收集并校验创世
7. **后台启动节点**

---

## 三、确认"出块"了（链活着的标志）

等脚本跑完，执行：
```bash
tail -f chain.log
```
如果看到类似下面的行、数字一直在涨，就成功了：
```
committed state ... height=1
committed state ... height=2
committed state ... height=3
...
```
按 `Ctrl+C` 退出查看（节点仍在后台跑）。

> 想随时看节点是否在跑：`ps aux | grep mcchaind`
> 想停掉节点：`pkill -9 -f mcchaind`

---

## 四、新手必踩的 3 个坑（我已经写进脚本规避了，知道即可）

1. **币种必须是 umc**：默认初始化出来币种叫 `stake`，MC 实际叫 `umc`。脚本第 2 步已自动改好，不改的话链起不来。
2. **自抵押最低 30k MC**：gentx 必须带 `--min-self-delegation 30000000`，否则链在创世时就 panic。脚本已写死。
3. **重启前先杀干净旧进程**：如果节点没正常退出，旧进程会锁住数据目录，再启动会报
   `failed to initialize database: ... used by another process`。
   解决：`pkill -9 -f mcchaind` 后再启动。

---

## 五、重要提醒（认真读）

- 这是**单验证人 solo 网络**：你一个人出块、占 100% 质押。服务器宕机链就停，不算去中心化主网。
- 密钥是 **test 后端（免密）** 存在服务器上，仅适合测试/演示。**真主网前**请改用 `file` 后端 + 独立签名机，并备份 `validator_key.json` 里的助记词（谁有助记词谁就控制这笔质押）。
- 想从你笔记本的浏览器仪表盘（`web/index.html`）连这个节点，需要把服务器**安全组**放行 `26657`(RPC) 端口，并让 RPC 监听 `0.0.0.0`（默认只监听本机）。这一步涉及安全，建议先跑通出块、再单独处理。
- 真去中心化主网还需要：≥4 个独立验证人、代币透明分配、TMKMS/签名机安全架构、第三方安全审计。详见 `docs/MAINNET_DEPLOY_RUNBOOK.md`。

---

## 六、如果出错了

把服务器终端**完整的报错文字**发给我（或另一名协作者），我来定位。最常见就上面那 3 个坑，都有现成解法。
