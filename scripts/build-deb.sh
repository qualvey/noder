#!/usr/bin/env bash
# ===================================================================
# Noder Debian Package (.deb) 自动化打包脚本
# 支持 amd64 与 arm64 交叉打包，集成前端构建与 FHS 目录结构组装
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
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

VERSION="1.0.0"
TARGET_ARCH="amd64"
OUTPUT_DIR="${ROOT_DIR}/dist"
SKIP_FRONTEND=false

# 从 Git Tag 或传参读取版本
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
if [[ -n "${GIT_TAG}" ]]; then
    VERSION="${GIT_TAG#v}"
fi

while [[ $# -gt 0 ]]; do
    case "$1" in
        --arch)
            TARGET_ARCH="$2"
            shift 2
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --skip-frontend)
            SKIP_FRONTEND=true
            shift
            ;;
        -h|--help)
            echo "用法: ./scripts/build-deb.sh [选项]"
            echo ""
            echo "选项:"
            echo "  --arch <amd64|arm64|all>   目标 CPU 架构 (默认: amd64)"
            echo "  --version <版本号>         软件包版本 (默认: ${VERSION})"
            echo "  --output <目录>            deb 产物输出目录 (默认: dist/)"
            echo "  --skip-frontend            跳过前端 Vue 构建"
            echo "  -h, --help                 查看帮助信息"
            exit 0
            ;;
        *)
            log_error "未知选项: $1"
            exit 1
            ;;
    esac
done

mkdir -p "${OUTPUT_DIR}"

# 1. 前端构建 (静态资源将内嵌进 Go 二进制)
if [[ "${SKIP_FRONTEND}" == false ]]; then
    log_info "===> [1/3] 构建前端静态资源 (web/)..."
    if command -v pnpm &>/dev/null; then
        (cd "${ROOT_DIR}/web" && pnpm build)
    elif command -v npm &>/dev/null; then
        (cd "${ROOT_DIR}/web" && npm run build)
    else
        log_warn "未找到 pnpm/npm，跳过前端构建，使用现有 static/ 资源。"
    fi
else
    log_info "===> [1/3] 跳过前端构建，使用现有 static/ 产物"
fi

# 封装单架构打包函数
build_single_deb() {
    local ARCH="$1"
    local PKG_NAME="noder_${VERSION}_${ARCH}"
    local STAGING_DIR="${ROOT_DIR}/dist/.staging/${PKG_NAME}"
    local DEB_FILE="${OUTPUT_DIR}/${PKG_NAME}.deb"

    log_info "===> [2/3] 编译 Linux/${ARCH} Go 二进制..."
    rm -rf "${STAGING_DIR}"
    mkdir -p "${STAGING_DIR}"

    local BIN_OUT="${STAGING_DIR}/usr/bin/noder"
    mkdir -p "$(dirname "${BIN_OUT}")"

    GOOS=linux GOARCH="${ARCH}" CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=${VERSION}" -o "${BIN_OUT}" ./cmd/server
    chmod 755 "${BIN_OUT}"

    log_info "===> [3/3] 组装 FHS 目录结构并打包 .deb (${ARCH})..."

    # 1. 拷贝 Systemd Service
    local SYSTEMD_DIR="${STAGING_DIR}/lib/systemd/system"
    mkdir -p "${SYSTEMD_DIR}"
    cp "${ROOT_DIR}/package/systemd/noder.service" "${SYSTEMD_DIR}/noder.service"
    chmod 644 "${SYSTEMD_DIR}/noder.service"

    # 2. 拷贝环境配置模板
    local CONF_DIR="${STAGING_DIR}/etc/noder"
    mkdir -p "${CONF_DIR}"
    cp "${ROOT_DIR}/package/etc/noder/noder.env" "${CONF_DIR}/noder.env"
    chmod 600 "${CONF_DIR}/noder.env"

    # 3. 创建持久化数据目录结构
    mkdir -p "${STAGING_DIR}/var/lib/noder/data/files"
    chmod 755 "${STAGING_DIR}/var/lib/noder"

    # 4. 组装 DEBIAN 元数据与脚本
    local DEBIAN_DIR="${STAGING_DIR}/DEBIAN"
    mkdir -p "${DEBIAN_DIR}"

    sed -e "s/Architecture: ARCH_PLACEHOLDER/Architecture: ${ARCH}/g" \
        -e "s/Version: .*/Version: ${VERSION}/g" \
        "${ROOT_DIR}/package/debian/control" > "${DEBIAN_DIR}/control"

    cp "${ROOT_DIR}/package/debian/conffiles" "${DEBIAN_DIR}/conffiles"
    cp "${ROOT_DIR}/package/debian/postinst" "${DEBIAN_DIR}/postinst"
    cp "${ROOT_DIR}/package/debian/prerm" "${DEBIAN_DIR}/prerm"
    cp "${ROOT_DIR}/package/debian/postrm" "${DEBIAN_DIR}/postrm"

    chmod 755 "${DEBIAN_DIR}/postinst" "${DEBIAN_DIR}/prerm" "${DEBIAN_DIR}/postrm"
    chmod 644 "${DEBIAN_DIR}/control" "${DEBIAN_DIR}/conffiles"

    # 5. 执行打包生成 .deb
    if command -v dpkg-deb &>/dev/null; then
        dpkg-deb -Zxz --build "${STAGING_DIR}" "${DEB_FILE}"
    else
        # 兼容无 dpkg-deb 工具的环境 (如在 Windows/macOS 通过内置纯 Go debpack 打包)
        log_info "宿主环境无 dpkg-deb，调用内置纯 Go 打包工具生成符合规范的 .deb..."
        go run "${ROOT_DIR}/cmd/debpack" -staging "${STAGING_DIR}" -output "${DEB_FILE}"
    fi

    # 清理临时 staging 目录
    rm -rf "${STAGING_DIR}"

    if [[ -f "${DEB_FILE}" ]]; then
        local FILE_SIZE=$(ls -lh "${DEB_FILE}" | awk '{print $5}')
        log_success "Debian 安装包生成成功: ${DEB_FILE} (${FILE_SIZE})"
    else
        log_error "打包失败: 未生成 ${DEB_FILE}"
        exit 1
    fi
}

if [[ "${TARGET_ARCH}" == "all" ]]; then
    build_single_deb "amd64"
    build_single_deb "arm64"
else
    build_single_deb "${TARGET_ARCH}"
fi

echo ""
echo -e "${GREEN}====================================================${PLAIN}"
echo -e "${GREEN} 🎉 全部 Debian 软件包打包完成！${PLAIN}"
echo -e " 📍 产物目录 : ${OUTPUT_DIR}"
echo -e " 📦 安装命令 : sudo apt install ./dist/noder_${VERSION}_<arch>.deb"
echo -e " 🔍 校验包体 : dpkg-deb -c ./dist/noder_${VERSION}_<arch>.deb"
echo -e "${GREEN}====================================================${PLAIN}"
