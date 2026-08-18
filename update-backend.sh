#!/usr/bin/env bash
# ===================================================================
# Sing-Box Subscription Middleman - 单独热更新后端服务脚本
# 增量拉取最新后端 Python 代码与依赖，安全保留 data.db 数据库并重启 Systemd
# ===================================================================

set -e

GREEN='\033[0;32m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
PLAIN='\033[0m'

log_info() { echo -e "${GREEN}[INFO] ${PLAIN} $1"; }
log_warn() { echo -e "${YELLOW}[WARN] ${PLAIN} $1"; }
log_error() { echo -e "${RED}[ERROR] ${PLAIN} $1"; }

if [[ $EUID -ne 0 ]]; then
    log_error "请使用 root 权限运行此更新脚本！(例如: sudo ./update-backend.sh)"
    exit 1
fi

INSTALL_DIR="/opt/sub-server"
SERVICE_NAME="sub-server"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEMP_DIR="/tmp/noder-backend-update"

log_info "开始更新后端 Python 代码与依赖..."

mkdir -p "${INSTALL_DIR}"

if [[ -f "${SCRIPT_DIR}/main.py" ]]; then
    log_info "使用本地现有目录文件进行后端更新..."
    if [[ "${SCRIPT_DIR}" != "${INSTALL_DIR}" ]]; then
        # 复制除了 data.db 之外的所有后端源码
        rsync -av --exclude='data.db' --exclude='.venv' "${SCRIPT_DIR}/" "${INSTALL_DIR}/" 2>/dev/null || cp -rf "${SCRIPT_DIR}/main.py" "${SCRIPT_DIR}/pyproject.toml" "${INSTALL_DIR}/"
    fi
else
    log_info "从 GitHub 远程仓库同步最新后端代码..."
    if [[ -d "${INSTALL_DIR}/.git" ]]; then
        log_info "使用 git pull 增量拉取最新代码 (保全本地 data.db)..."
        git -C "${INSTALL_DIR}" fetch --all
        git -C "${INSTALL_DIR}" reset --hard origin/main
    else
        log_info "通过临时解压方式同步更新代码 (保全本地 data.db)..."
        rm -rf "${TEMP_DIR}"
        mkdir -p "${TEMP_DIR}"
        curl -sSL https://github.com/qualvey/noder/archive/refs/heads/main.tar.gz | tar -xz -C "${TEMP_DIR}"
        
        # 排除 data.db 覆盖到安装目录
        rsync -av --exclude='data.db' "${TEMP_DIR}/noder-main/" "${INSTALL_DIR}/" 2>/dev/null || cp -rf "${TEMP_DIR}/noder-main/main.py" "${TEMP_DIR}/noder-main/pyproject.toml" "${INSTALL_DIR}/"
        rm -rf "${TEMP_DIR}"
    fi
fi

cd "${INSTALL_DIR}"

# 寻找全局 uv 命令
UV_BIN="$(command -v uv || echo "/usr/local/bin/uv")"
if [[ ! -x "$UV_BIN" && -x "$HOME/.local/bin/uv" ]]; then
    UV_BIN="$HOME/.local/bin/uv"
fi

if [[ -x "$UV_BIN" ]]; then
    log_info "使用 uv 增量安装/升级 Python 运行依赖..."
    "$UV_BIN" venv --allow-existing .venv 2>/dev/null || true
    "$UV_BIN" sync --frozen --no-dev 2>/dev/null || \
        "$UV_BIN" pip install fastapi uvicorn sqlmodel pydantic python-multipart pyyaml 2>/dev/null || true
fi

# 重启后端 Systemd 服务
log_info "正在安全重启 ${SERVICE_NAME} 服务..."
systemctl daemon-reload
systemctl restart "${SERVICE_NAME}"

sleep 2
if systemctl is-active --quiet "${SERVICE_NAME}"; then
    echo -e ""
    echo -e "${GREEN}====================================================${PLAIN}"
    echo -e "${GREEN} 🎉 Sing-Box 后端服务与代码热更新成功！${PLAIN}"
    echo -e " 📍 部署路径   : ${INSTALL_DIR}"
    echo -e " 💾 数据库状态 : data.db 已安全保留"
    echo -e " ⚡ 服务状态   : $(systemctl is-active ${SERVICE_NAME})"
    echo -e "${GREEN}====================================================${PLAIN}"
else
    log_error "后端服务重启失败，请运行 'journalctl -u ${SERVICE_NAME} -n 50' 查看错误日志。"
    exit 1
fi
