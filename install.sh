#!/usr/bin/env bash
# ===================================================================
# Sing-Box Subscription Middleman - Systemd 一键安装部署脚本
# 适用于 Ubuntu / Debian / CentOS / AlmaLinux / Rocky Linux
# ===================================================================

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PLAIN='\033[0m'

log_info() { echo -e "${GREEN}[INFO] ${PLAIN} $1"; }
log_warn() { echo -e "${YELLOW}[WARN] ${PLAIN} $1"; }
log_error() { echo -e "${RED}[ERROR] ${PLAIN} $1"; }

# 1. 权限检查
if [[ $EUID -ne 0 ]]; then
    log_error "请使用 root 权限运行此安装脚本！(例如: sudo ./install.sh)"
    exit 1
fi

# 2. 目录设置
INSTALL_DIR="/opt/sub-server"
SERVICE_NAME="sub-server"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 支持单独更新前端指令: ./install.sh --frontend 或 ./install.sh static
if [[ "$1" == "static" || "$1" == "--frontend" || "$1" == "frontend" || "$1" == "--static" ]]; then
    log_info "检测到单独更新前端选项，正在单独更新静态资源..."
    if [[ -f "${SCRIPT_DIR}/update-frontend.sh" ]]; then
        bash "${SCRIPT_DIR}/update-frontend.sh"
    else
        curl -fsSL https://raw.githubusercontent.com/qualvey/noder/main/update-frontend.sh | bash
    fi
    exit 0
fi

# 支持单独更新后端指令: ./install.sh --backend 或 ./install.sh backend
if [[ "$1" == "backend" || "$1" == "--backend" ]]; then
    log_info "检测到单独更新后端选项，正在单独更新 Python 代码与依赖..."
    if [[ -f "${SCRIPT_DIR}/update-backend.sh" ]]; then
        bash "${SCRIPT_DIR}/update-backend.sh"
    else
        curl -fsSL https://raw.githubusercontent.com/qualvey/noder/main/update-backend.sh | bash
    fi
    exit 0
fi

log_info "开始安装/更新 Sing-Box Subscription Middleman 全套服务..."

# 2.5 检测是否已安装过服务（已安装 -> 更新模式，跳过系统依赖检查）
IS_INSTALLED="false"
if [[ -f "$SERVICE_FILE" ]]; then
    IS_INSTALLED="true"
    log_info "检测到已安装 ${SERVICE_NAME} 服务（更新模式），将跳过系统依赖检查..."
fi

# 3. 安装依赖工具 (curl, git, python3) —— 已安装过服务则跳过
if [[ "${IS_INSTALLED}" == "true" ]]; then
    log_info "已安装服务，跳过系统依赖安装 (curl/git/python3/openssl)..."
else
    log_info "检查并安装必要依赖工具..."
    if command -v apt-get &>/dev/null; then
        apt-get update -y
        apt-get install -y curl git python3 python3-pip openssl
    elif command -v dnf &>/dev/null; then
        dnf install -y curl git python3 python3-pip openssl
    elif command -v yum &>/dev/null; then
        yum install -y curl git python3 python3-pip openssl
    fi
fi

# 4. 检查/安装 uv (Astral uv - 安装并软链接到 /usr/local/bin 实现全局系统可用)
if command -v uv &>/dev/null || [[ -x "$HOME/.local/bin/uv" ]] || [[ -x "/usr/local/bin/uv" ]]; then
    log_info "检测到 uv 已存在，跳过 uv 安装..."
else
    if [[ "${IS_INSTALLED}" == "true" ]]; then
        log_warn "服务已安装但未检测到 uv（环境异常），将重新安装 uv..."
    fi
    log_info "未检测到 uv，正在自动安装 Astral uv 并设置为全局命令..."
    curl -LsSf https://astral.sh/uv/install.sh | UV_INSTALL_DIR="/usr/local/bin" sh 2>/dev/null || \
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="/usr/local/bin:$HOME/.local/bin:$HOME/.cargo/bin:$PATH"
fi

# 建立全局软链接，确保所有 Shell（如 zsh/bash）及所有用户均全局可用 uv / uvx（仅全新安装时执行）
if [[ "${IS_INSTALLED}" != "true" ]]; then
    if [[ -f "$HOME/.local/bin/uv" && ! -f "/usr/local/bin/uv" ]]; then
        cp -f "$HOME/.local/bin/uv" /usr/local/bin/uv 2>/dev/null || ln -sf "$HOME/.local/bin/uv" /usr/local/bin/uv 2>/dev/null || true
    fi
    if [[ -f "$HOME/.local/bin/uvx" && ! -f "/usr/local/bin/uvx" ]]; then
        cp -f "$HOME/.local/bin/uvx" /usr/local/bin/uvx 2>/dev/null || ln -sf "$HOME/.local/bin/uvx" /usr/local/bin/uvx 2>/dev/null || true
    fi
    chmod +x /usr/local/bin/uv /usr/local/bin/uvx 2>/dev/null || true
fi

# 确认 uv 可用
UV_BIN="$(command -v uv || echo "/usr/local/bin/uv")"
if [[ ! -x "$UV_BIN" && ! -x "$HOME/.local/bin/uv" ]]; then
    log_error "uv 安装失败，请检查网络连接。"
    exit 1
fi
log_info "使用 uv 路径: $UV_BIN (全局环境对所有 Shell 可用)"

# 5. 克隆/同步项目文件到 /opt/sub-server
log_info "准备项目代码文件至 ${INSTALL_DIR}..."
if [[ -f "${SCRIPT_DIR}/main.py" ]]; then
    log_info "使用本地现有脚本目录同步文件..."
    mkdir -p "${INSTALL_DIR}"
    if [[ "${SCRIPT_DIR}" != "${INSTALL_DIR}" ]]; then
        # 排除 data.db / data/ 上传目录 / .venv，避免覆盖线上数据与虚拟环境
        rsync -av --exclude='data.db' --exclude='data/' --exclude='.venv' "${SCRIPT_DIR}/" "${INSTALL_DIR}/" 2>/dev/null || \
        cp -rf "${SCRIPT_DIR}/main.py" "${SCRIPT_DIR}/pyproject.toml" "${SCRIPT_DIR}/uv.lock" "${SCRIPT_DIR}/template.json" "${SCRIPT_DIR}/static" "${SCRIPT_DIR}/doc" "${INSTALL_DIR}/"
    fi
else
    log_info "准备从 GitHub 远程仓库克隆最新代码..."
    if [[ -d "${INSTALL_DIR}/.git" ]]; then
        log_info "通过 git pull 更新已有项目代码..."
        git -C "${INSTALL_DIR}" fetch --all
        git -C "${INSTALL_DIR}" reset --hard origin/main
    else
        log_info "清理并重新克隆项目代码到 ${INSTALL_DIR}..."
        rm -rf "${INSTALL_DIR}"
        git clone https://github.com/qualvey/noder.git "${INSTALL_DIR}"
    fi
fi

cd "${INSTALL_DIR}"

# 6. 配置密钥与端口 (兼容管道 curl | bash 非交互模式)
RANDOM_SECRET=$(openssl rand -hex 16 2>/dev/null || echo "secret-$(date +%s)")

# 检测已有服务：默认只更新代码，保留现有配置，不重新交互配置参数
UPDATE_ONLY="N"
if [[ "${IS_INSTALLED}" == "true" ]]; then
    if [ -t 0 ] || [ -c /dev/tty ]; then
        read -p "检测到已有 ${SERVICE_NAME} 服务。只更新代码并保留现有配置？[Y/n] " UPDATE_ONLY < /dev/tty || true
        UPDATE_ONLY=${UPDATE_ONLY:-Y}
    else
        UPDATE_ONLY="Y"
    fi
fi
if [[ "${UPDATE_ONLY}" =~ ^[Yy]$ ]]; then
    log_info "只更新代码，保留现有 Admin Token / 端口 / 绑定 IP 配置..."
    EXISTING_ADMIN_TOKEN=$(sed -n 's/.*ADMIN_SECRET_TOKEN="\([^"]*\)".*/\1/p' "$SERVICE_FILE" | head -1)
    EXISTING_PORT=$(sed -n 's/.*--port \([0-9]*\).*/\1/p' "$SERVICE_FILE" | head -1)
    EXISTING_HOST=$(sed -n 's/.*--host \([^ ]*\).*/\1/p' "$SERVICE_FILE" | head -1)
    ADMIN_TOKEN="${EXISTING_ADMIN_TOKEN:-$RANDOM_SECRET}"
    LISTEN_PORT="${EXISTING_PORT:-8000}"
    LISTEN_HOST="${EXISTING_HOST:-0.0.0.0}"
    SKIP_SERVICE_REGENERATE=1
fi


if [[ -z "$SKIP_SERVICE_REGENERATE" ]]; then
if [ -t 0 ] || [ -c /dev/tty ]; then
    # 有可用的交互终端
    read -p "请输入 Admin Secret Token (留空使用随机生成的密钥: ${RANDOM_SECRET}): " ADMIN_TOKEN < /dev/tty || true
    ADMIN_TOKEN=${ADMIN_TOKEN:-$RANDOM_SECRET}

    read -p "请输入服务监听端口 [默认 8000]: " LISTEN_PORT < /dev/tty || true
    LISTEN_PORT=${LISTEN_PORT:-8000}

    read -p "请输入绑定 IP [默认 0.0.0.0]: " LISTEN_HOST < /dev/tty || true
    LISTEN_HOST=${LISTEN_HOST:-0.0.0.0}
else
    # 纯非交互式管道，使用默认值与随机密钥
    ADMIN_TOKEN=${ADMIN_TOKEN:-$RANDOM_SECRET}
    LISTEN_PORT=${LISTEN_PORT:-8000}
    LISTEN_HOST=${LISTEN_HOST:-0.0.0.0}
fi
fi

# 7. 构建虚拟环境并安装 Python 依赖
log_info "正在使用 uv 安装依赖与初始化 Python 虚拟环境..."
"$UV_BIN" venv --allow-existing .venv
"$UV_BIN" sync --frozen --no-dev || {
    log_warn "uv sync 失败，回退安装基础依赖..."
    "$UV_BIN" pip install fastapi uvicorn sqlmodel pydantic python-multipart pyyaml
}

UVICORN_BIN="${INSTALL_DIR}/.venv/bin/uvicorn"
if [[ ! -x "$UVICORN_BIN" ]]; then
    # 兜底寻找 uvicorn 路径
    UVICORN_BIN="$(which uvicorn || echo "${INSTALL_DIR}/.venv/bin/uvicorn")"
fi

# 8. 创建 Systemd 服务文件 (仅全新安装/重新配置时生成；更新模式保留现有配置)
if [[ -z "$SKIP_SERVICE_REGENERATE" ]]; then
log_info "生成 Systemd 服务文件 ${SERVICE_FILE}..."
cat <<EOF > "${SERVICE_FILE}"
[Unit]
Description=Sing-Box Subscription Middleman Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
Environment="ADMIN_SECRET_TOKEN=${ADMIN_TOKEN}"
ExecStart=${UVICORN_BIN} main:app --host ${LISTEN_HOST} --port ${LISTEN_PORT}
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
fi

# 9. 启动 Systemd 服务
log_info "重新加载 Systemd 并启动服务..."
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"

# 10. 输出安装成功提示
sleep 2
if systemctl is-active --quiet "${SERVICE_NAME}"; then
    echo -e ""
    echo -e "${GREEN}====================================================${PLAIN}"
    echo -e "${GREEN} 🎉 Sing-Box Subscription Middleman 安装成功！${PLAIN}"
    echo -e "${GREEN}====================================================${PLAIN}"
    echo -e " 📍 安装路径   : ${BLUE}${INSTALL_DIR}${PLAIN}"
    echo -e " 🔑 Admin Token: ${YELLOW}${ADMIN_TOKEN}${PLAIN}"
    echo -e " 🌐 面板地址   : ${BLUE}http://${LISTEN_HOST}:${LISTEN_PORT}${PLAIN}"
    echo -e " 📡 订阅 API   : ${BLUE}http://${LISTEN_HOST}:${LISTEN_PORT}/sub?token={USER_TOKEN}${PLAIN}"
    echo -e " 📡 节点 API   : ${BLUE}http://${LISTEN_HOST}:${LISTEN_PORT}/node?token={USER_TOKEN}${PLAIN}"
    echo -e "${GREEN}====================================================${PLAIN}"
    echo -e " 🛠️ 服务管理常用命令:"
    echo -e "   查看运行状态: ${YELLOW}systemctl status ${SERVICE_NAME}${PLAIN}"
    echo -e "   实时查看日志: ${YELLOW}journalctl -u ${SERVICE_NAME} -f${PLAIN}"
    echo -e "   重启服务    : ${YELLOW}systemctl restart ${SERVICE_NAME}${PLAIN}"
    echo -e "   停止服务    : ${YELLOW}systemctl stop ${SERVICE_NAME}${PLAIN}"
    echo -e "${GREEN}====================================================${PLAIN}"
else
    log_error "服务启动失败，请运行 'journalctl -u ${SERVICE_NAME} -n 50' 查看错误日志。"
    exit 1
fi
