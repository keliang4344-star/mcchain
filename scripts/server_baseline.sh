#!/usr/bin/env bash
#
# MC 公链 · 服务器安全基线加固（上线前必跑）
#
# 目标：把 docs/VALIDATOR_RUNBOOK / NODE_CONFIG 里要求的「服务器基线」
# 一次执行到位：SSH 仅密钥、fail2ban、防火墙、NTP、自动安全更新。
#
# 用法（目标服务器上以 root 执行）：
#   sudo ./scripts/server_baseline.sh [--apply]
#   默认 dry-run 只打印将执行的变更；--apply 才真正修改。
#
# 原则：显式幂等；每一步先检测再变更；不使用任何通配删除。
#
set -uo pipefail

APPLY=0
[[ "${1:-}" == "--apply" ]] && APPLY=1

log()  { printf '\033[1;34m[BASELINE]\033[0m %s\n' "$*"; }
act()  { if (( APPLY )); then eval "$1"; else printf '  (dry-run) %s\n' "$1"; fi; }

[[ $EUID -eq 0 ]] || { echo "[FATAL] 请以 root 执行" >&2; exit 2; }

# ---------------------------------------------------------------------------
log "1/7 SSH 仅密钥登录"
# ---------------------------------------------------------------------------
# 防锁死守卫必须在任何 sshd_config 修改**之前**，且拒绝时直接退出：
# 先改文件再检查、失败也不退出，会让配置文件与运行态不一致 —— 下次
# sshd 重启或机器重启时按新配置生效，守卫形同虚设。
if (( APPLY )) && ! who | grep -q .; then
  echo "[FATAL] 未检测到活跃登录会话，拒绝修改 sshd（防止锁死）——未做任何修改" >&2
  exit 2
fi
if grep -qE '^\s*PasswordAuthentication\s+yes' /etc/ssh/sshd_config 2>/dev/null; then
  act "sed -i.bak.'$(date +%s)' -E 's/^#?\s*PasswordAuthentication\s+yes/PasswordAuthentication no/' /etc/ssh/sshd_config"
  log "  PasswordAuthentication yes → no（已备份原文件）"
else
  log "  已是密钥登录，跳过"
fi
# 允许 root 密钥登录但不允许密码（root 密钥是运维生命线，不直接禁 root）
if ! grep -qE '^\s*PermitRootLogin\s+prohibit-password' /etc/ssh/sshd_config 2>/dev/null; then
  act "sed -i.bak.'$(date +%s)' -E 's/^#?\s*PermitRootLogin\s+.*/PermitRootLogin prohibit-password/' /etc/ssh/sshd_config"
fi
if (( APPLY )); then
  systemctl reload sshd 2>/dev/null || systemctl reload ssh 2>/dev/null || true
  log "  sshd 已 reload"
fi

# ---------------------------------------------------------------------------
log "2/7 fail2ban（SSH 爆破防护）"
# ---------------------------------------------------------------------------
if command -v fail2ban-client >/dev/null 2>&1; then
  log "  fail2ban 已安装"
else
  act "(apt-get install -y fail2ban || yum install -y fail2ban) >/dev/null"
  log "  已安装 fail2ban"
fi
if (( APPLY )) && command -v fail2ban-client >/dev/null 2>&1; then
  cat > /etc/fail2ban/jail.d/mcchain.conf <<'EOF'
[sshd]
enabled  = true
maxretry = 5
findtime = 10m
bantime  = 1h
EOF
  systemctl enable --now fail2ban >/dev/null 2>&1 || true
  log "  fail2ban sshd jail 已启用（5 次失败封 1h）"
fi

# ---------------------------------------------------------------------------
log "3/7 防火墙（白名单：SSH/RPC 26657/REST 1317/gRPC 9090/P2P 26656/prom 26660-26661）"
# ---------------------------------------------------------------------------
if command -v ufw >/dev/null 2>&1; then
  FW="ufw"
elif command -v firewall-cmd >/dev/null 2>&1; then
  FW="firewalld"
else
  FW="none"; log "  未检测到防火墙工具，跳过（请手动配置安全组）"
fi
if [[ "$FW" == "ufw" ]]; then
  act "ufw allow 22/tcp >/dev/null"
  for p in 26656/tcp 26657/tcp 1317/tcp 9090/tcp 26660/tcp 26661/tcp; do
    act "ufw allow $p >/dev/null"
  done
  if (( APPLY )); then
    ufw --force enable >/dev/null 2>&1 || true
    log "  ufw 已启用（22/26656/26657/1317/9090/26660/26661）"
  fi
elif [[ "$FW" == "firewalld" ]] && (( APPLY )); then
  for p in 26656 26657 1317 9090 26660 26661; do
    firewall-cmd --permanent --add-port=${p}/tcp >/dev/null 2>&1
  done
  firewall-cmd --reload >/dev/null 2>&1 || true
  log "  firewalld 端口已放行"
fi

# ---------------------------------------------------------------------------
log "4/7 NTP 时间同步（验证人签名正确性的前提）"
# ---------------------------------------------------------------------------
if command -v chronyc >/dev/null 2>&1; then
  drift=$(chronyc tracking 2>/dev/null | awk '/System time/ {print $5}')
  log "  chrony 在跑，系统偏差 ≈ ${drift:-unknown}s（>0.1s 需处理，见 RUNBOOK §1.1）"
elif command -v timedatectl >/dev/null 2>&1 && timedatectl show 2>/dev/null | grep -q "NTPSynchronized=yes"; then
  log "  systemd-timesyncd 已同步"
else
  act "(apt-get install -y chrony || yum install -y chrony) >/dev/null"
  act "systemctl enable --now chronyd 2>/dev/null || systemctl enable --now chrony"
  log "  已安装并启用 chrony"
fi

# ---------------------------------------------------------------------------
log "5/7 自动安全更新（仅安全补丁，不做发行版升级）"
# ---------------------------------------------------------------------------
if command -v apt-get >/dev/null 2>&1; then
  if dpkg -l unattended-upgrades 2>/dev/null | grep -q ^ii; then
    log "  unattended-upgrades 已装"
  else
    act "apt-get install -y unattended-upgrades >/dev/null"
    act "dpkg-reconfigure -f noninteractive unattended-upgrades >/dev/null"
    log "  已启用 Ubuntu 自动安全更新"
  fi
else
  log "  非 apt 系统，请用 yum-cron / dnf-automatic 自行配置"
fi

# ---------------------------------------------------------------------------
log "6/7 基础内核参数（连接数与文件句柄）"
# ---------------------------------------------------------------------------
SYSCTL_CONF=/etc/sysctl.d/90-mcchain.conf
if (( APPLY )); then
  cat > "$SYSCTL_CONF" <<'EOF'
# MC 公链节点基线（上线前 server_baseline.sh 写入）
net.core.somaxconn = 4096
net.ipv4.tcp_max_syn_backlog = 8192
fs.file-max = 1048576
vm.swappiness = 10
EOF
  sysctl --system >/dev/null 2>&1
  log "  已写入 $SYSCTL_CONF 并生效"
else
  log "  (dry-run) 将写入 $SYSCTL_CONF"
fi

# ---------------------------------------------------------------------------
log "7/7 汇总检查清单"
# ---------------------------------------------------------------------------
ok=1
grep -qE '^\s*PasswordAuthentication\s+no' /etc/ssh/sshd_config 2>/dev/null && log "  [OK] SSH 密码登录已关闭" || { log "  [--] SSH 密码登录状态未确认"; ok=0; }
(( APPLY )) && command -v fail2ban-client >/dev/null 2>&1 && fail2ban-client status sshd >/dev/null 2>&1 && log "  [OK] fail2ban sshd jail 运行中"
[[ "$FW" != "none" ]] && log "  [OK] 防火墙工具：$FW"
command -v chronyc >/dev/null 2>&1 && log "  [OK] chrony 在位"
log "完成。dry-run 结果仅代表将要执行的动作；--apply 才真正生效。"
exit 0
