#!/usr/bin/env bash
# ===================================================================
# Sing-Box Subscription Middleman - 单独热更新前端资源脚本
# 仅更新 /opt/sub-server/static/ 目录，无需重启后端服务
# ===================================================================

set -e

GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
PLAIN='\033[0m'

log_info() { echo -e "${GREEN}[INFO] ${PLAIN} $1"; }
log_error() { echo -e "${RED}[ERROR] ${PLAIN} $1"; }

if [[ $EUID -ne 0 ]]; then
    log_error "请使用 root 权限运行此更新脚本！(例如: sudo ./update-frontend.sh)"
    exit 1
fi

INSTALL_DIR="/opt/sub-server"
STATIC_DIR="${INSTALL_DIR}/static"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMP_DIR="/tmp/noder-static-update"

log_info "开始单独更新前端静态页面资源..."

mkdir -p "${STATIC_DIR}"

if [[ -f "${SCRIPT_DIR}/static/index.html" ]]; then
    log_info "使用本地目录中的 static 文件进行更新..."
    cp -rf "${SCRIPT_DIR}/static/"* "${STATIC_DIR}/"
else
    log_info "从 GitHub 远程拉取最新 static 页面代码..."
    rm -rf "${TEMP_DIR}"
    mkdir -p "${TEMP_DIR}"
    
    if command -v git &>/dev/null; then
        git clone --depth 1 https://github.com/qualvey/noder.git "${TEMP_DIR}"
        cp -rf "${TEMP_DIR}/static/"* "${STATIC_DIR}/"
        rm -rf "${TEMP_DIR}"
    else
        log_info "正在通过 curl/tar 提取最新 static 文件..."
        curl -sSL https://github.com/qualvey/noder/archive/refs/heads/main.tar.gz | tar -xz -C "${TEMP_DIR}"
        cp -rf "${TEMP_DIR}/noder-main/static/"* "${STATIC_DIR}/"
        rm -rf "${TEMP_DIR}"
    fi
fi

# 设置标准读取权限
chmod -R 644 "${STATIC_DIR}"/* 2>/dev/null || true

echo -e ""
echo -e "${GREEN}====================================================${PLAIN}"
echo -e "${GREEN} 🎉 前端静态资源单独更新成功！${PLAIN}"
echo -e " 📍 前端资源目录 : ${STATIC_DIR}"
echo -e " ⚡ FastAPI 已自动热加载，无需重启后端 Systemd 服务。"
echo -e "${GREEN}====================================================${PLAIN}"
