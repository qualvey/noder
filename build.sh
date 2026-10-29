#!/usr/bin/env bash
# ===================================================================
# Noder 自动化构建与重启脚本 (Linux / macOS / Bash)
# 支持前端 Vue 打包、Go 后端编译、跨平台交叉编译与服务平滑重启
# ===================================================================

set -e

GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
PLAIN='\033[0m'

log_info() { echo -e "${CYAN}[INFO]${PLAIN} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${PLAIN} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${PLAIN} $1"; }
log_error() { echo -e "${RED}[ERROR]${PLAIN} $1"; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${SCRIPT_DIR}"

FRONTEND_ONLY=false
BACKEND_ONLY=false
RESTART=false
CLEAN=false
CROSS=false

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case "$1" in
        -fe|--frontend|--frontend-only)
            FRONTEND_ONLY=true
            shift
            ;;
        -be|--backend|--backend-only)
            BACKEND_ONLY=true
            shift
            ;;
        -r|--restart)
            RESTART=true
            shift
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        --cross)
            CROSS=true
            shift
            ;;
        -h|--help)
            echo "用法: ./build.sh [选项]"
            echo ""
            echo "选项:"
            echo "  -fe, --frontend     仅构建前端资源 (pnpm build)"
            echo "  -be, --backend      仅编译 Go 后端二进制 (go build)"
            echo "  -r,  --restart      构建后自动平滑重启 noder 服务 (systemd 或本地后台进程)"
            echo "  -c,  --clean        清理旧的编译产物与静态缓存"
            echo "       --cross        交叉编译多平台发布包 (Linux amd64/arm64, Windows amd64)"
            echo "  -h,  --help         查看此帮助信息"
            exit 0
            ;;
        *)
            log_error "未知参数: $1"
            echo "运行 ./build.sh -h 查看帮助"
            exit 1
            ;;
    esac
done

# 1. 清理
if [[ "$CLEAN" == true ]]; then
    log_info "正在清理旧的构建缓存与二进制产物..."
    rm -rf static/assets/*
    rm -f noder noder.exe dist/*
    log_success "清理完成"
    if [[ "$FRONTEND_ONLY" == false && "$BACKEND_ONLY" == false && "$CROSS" == false ]]; then
        exit 0
    fi
fi

# 2. 前端构建
if [[ "$BACKEND_ONLY" == false ]]; then
    log_info "===> [1/2] 开始构建前端 (web/)..."
    if ! command -v pnpm &>/dev/null; then
        if command -v npm &>/dev/null; then
            log_warn "未检测到 pnpm，尝试使用 npm run build..."
            (cd web && npm run build)
        else
            log_error "未找到 Node.js / pnpm 工具，无法构建前端！"
            exit 1
        fi
    else
        (cd web && pnpm build)
    fi
    log_success "前端构建完成 -> static/"
fi

# 3. 跨平台交叉编译模式
if [[ "$CROSS" == true ]]; then
    log_info "===> 开始交叉编译发布包..."
    mkdir -p dist
    
    PLATFORMS=("linux/amd64" "linux/arm64" "darwin/amd64" "darwin/arm64" "windows/amd64")
    for PLATFORM in "${PLATFORMS[@]}"; do
        OS="${PLATFORM%/*}"
        ARCH="${PLATFORM#*/}"
        OUTPUT_NAME="noder-${OS}-${ARCH}"
        if [[ "$OS" == "windows" ]]; then
            OUTPUT_NAME="${OUTPUT_NAME}.exe"
        fi
        
        log_info "正在编译: ${PLATFORM} -> dist/${OUTPUT_NAME} ..."
        GOOS="${OS}" GOARCH="${ARCH}" CGO_ENABLED=0 go build -ldflags "-s -w" -o "dist/${OUTPUT_NAME}" ./cmd/server
    done
    log_success "交叉编译全部完成！产物已存入 dist/ 目录"
    ls -lh dist/
    exit 0
fi

# 4. 本地后端编译
if [[ "$FRONTEND_ONLY" == false ]]; then
    log_info "===> [2/2] 开始编译 Go 后端..."
    
    OUTPUT_BIN="noder"
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
        OUTPUT_BIN="noder.exe"
    fi

    # 解除可能存在的运行中进程占用
    PID=$(pgrep -f "${OUTPUT_BIN}" 2>/dev/null || true)
    if [[ -n "$PID" && "$RESTART" == true ]]; then
        log_warn "停止正在运行的旧进程 (PID: ${PID})..."
        kill -9 "${PID}" 2>/dev/null || true
        sleep 0.5
    fi

    CGO_ENABLED=0 go build -ldflags "-s -w" -o "${OUTPUT_BIN}" ./cmd/server
    log_success "Go 后端编译完成: ./${OUTPUT_BIN}"

    # 5. 重启服务逻辑
    if [[ "$RESTART" == true ]]; then
        log_info "正在拉起新版服务..."
        if command -v systemctl &>/dev/null && systemctl is-active --quiet noder 2>/dev/null; then
            systemctl restart noder
            log_success "Systemd 服务已重启: systemctl status noder"
        else
            nohup ./"${OUTPUT_BIN}" > noder.log 2>&1 &
            NEW_PID=$!
            sleep 1
            if kill -0 "${NEW_PID}" 2>/dev/null; then
                echo ""
                echo -e "${GREEN}====================================================${PLAIN}"
                echo -e "${GREEN} 🎉 Noder 后端服务编译并成功拉起！${PLAIN}"
                echo -e " 🌐 访问地址 : http://localhost:8000"
                echo -e " ⚡ 进程 PID  : ${NEW_PID} (日志: ./noder.log)"
                echo -e "${GREEN}====================================================${PLAIN}"
            else
                log_error "服务启动异常，请查看 ./noder.log 日志。"
            fi
        fi
    else
        log_info "提示: 如需构建完成后自动拉起服务，可添加 -r 参数: ./build.sh -r"
    fi
fi
