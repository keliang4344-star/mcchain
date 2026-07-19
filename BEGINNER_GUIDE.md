# MC 公链 · 新手上链指南（云服务器）

> 目标：把一台云服务器变成 MC 公链的**第一个创世节点**（出块）。
> 前置：准备一台**云服务器**（Linux/Ubuntu 22.04，建议 ≥2 vCPU / ≥4GB / ≥60GB SSD）。

---

## 一、准备（上传文件到服务器）

链的 Linux 二进制和一键启动脚本已随仓库提供，需要把以下 2 个文件传到服务器：

1. `build/mcchaind` —— 链程序（Linux 版）
2. `server_setup.sh` —— 一键启动脚本

传文件的两种方式（任选其一）：

### 方式 A：云服务商控制台网页上传（最简单，不用配 SSH）
1. 打开云服务商控制台 → 找到你的服务器 → 点 **「登录」**（浏览器内会打开一个终端窗口）。
2. 在文件管理器里，把 `build/mcchaind` 和 `server_setup.sh` 上传到服务器（建议传到 `/root/` 目录）。

### 方式 B：用 scp 命令上传（需要先设置服务器密码/SSH）
在终端里执行：
```bash
# 把 <服务器IP> 换成你的公网 IP，<用户名> 通常是 root
scp build/mcchaind root@<服务器IP>:/root/
scp server_setup.sh root@<服务器IP>:/root/
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

## 四、新手必踩的 3 个坑（脚本已自动规避，知道即可）

1. **币种必须是 umc**：默认初始化出来币种叫 `stake`，MC 实际叫 `umc`。脚本第 2 步已自动改好，不改的话链起不来。
2. **自抵押最低 30k MC**：gentx 必须带 `--min-self-delegation 30000000`，否则链在创世时就 panic。脚本已写死。
3. **重启前先杀干净旧进程**：如果节点没正常退出，旧进程会锁住数据目录，再启动会报
   `failed to initialize database: ... used by another process`。
   解决：`pkill -9 -f mcchaind` 后再启动。

---

## 五、重要提醒（认真读）

- 这是**单验证人 solo 网络**：单个节点出块、占 100% 质押。服务器宕机链就停，不算去中心化主网。
- 密钥是 **test 后端（免密）** 存在服务器上，仅适合测试/演示。**真主网前**请改用 `file` 后端 + 独立签名机，并备份 `validator_key.json` 里的助记词（谁有助记词谁就控制这笔质押）。
- 想从浏览器仪表盘（`web/index.html`）连这个节点，需要把服务器**安全组**放行 `26657`(RPC) 端口，并让 RPC 监听 `0.0.0.0`（默认只监听 127.0.0.1）。这一步涉及安全，建议先跑通出块、再单独处理。
- 真去中心化主网还需要：≥4 个独立验证人、代币透明分配、TMKMS/签名机安全架构、第三方安全审计。详见项目部署文档。

---

## 六、如果出错了

把服务器终端**完整的报错文字**提交到 Issue，社区会协助定位。最常见就上面那 3 个坑，都有现成解法。
