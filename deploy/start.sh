#!/usr/bin/env bash
# MobileChain 主网启动脚本
set -euo pipefail
HOME_DIR="${HOME_DIR:-$HOME/.mcchain}"
mcchaind start --home "$HOME_DIR"
