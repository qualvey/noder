#!/usr/bin/env bash
# 重置 Noder 管理员 API Token
# 默认修改 /etc/noder/noder.env，并重启 noder.service。

set -euo pipefail

ENV_FILE="/etc/noder/noder.env"
NEW_TOKEN=""
NO_RESTART=false

usage() {
    cat <<'EOF'
用法：sudo ./scripts/reset-admin-token.sh [选项]

选项：
  --token <值>       使用指定 Token；省略时自动生成随机 Token
  --env-file <路径>  环境配置文件（默认：/etc/noder/noder.env）
  --no-restart       只修改配置，不重启 noder.service
  -h, --help         显示帮助
EOF
}

generate_token() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 24
    else
        od -An -N24 -tx1 /dev/urandom | tr -d ' \n'
    fi
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --token)
            [[ $# -ge 2 ]] || { echo "错误：--token 缺少值" >&2; exit 1; }
            NEW_TOKEN="$2"
            shift 2
            ;;
        --env-file)
            [[ $# -ge 2 ]] || { echo "错误：--env-file 缺少路径" >&2; exit 1; }
            ENV_FILE="$2"
            shift 2
            ;;
        --no-restart)
            NO_RESTART=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "错误：未知选项 $1" >&2
            usage >&2
            exit 1
            ;;
    esac
done

if [[ -z "${NEW_TOKEN}" ]]; then
    NEW_TOKEN="$(generate_token)"
fi

# 避免把换行或 shell 特殊内容写入 EnvironmentFile。
if [[ ! "${NEW_TOKEN}" =~ ^[A-Za-z0-9._~+/=-]+$ ]]; then
    echo "错误：Token 只能包含字母、数字及 . _ ~ + / = -" >&2
    exit 1
fi

ENV_DIR="$(dirname "${ENV_FILE}")"
if [[ ! -f "${ENV_FILE}" ]]; then
    echo "错误：配置文件不存在：${ENV_FILE}" >&2
    echo "可先复制 package/etc/noder/noder.env 到该路径。" >&2
    exit 1
fi

umask 077
TEMP_FILE="$(mktemp "${ENV_DIR}/.noder.env.XXXXXX")"
trap 'rm -f "${TEMP_FILE}"' EXIT

awk -v token="${NEW_TOKEN}" '
    BEGIN { replaced = 0 }
    /^([[:space:]]*)ADMIN_SECRET_TOKEN=/ {
        print "ADMIN_SECRET_TOKEN=" token
        replaced = 1
        next
    }
    { print }
    END {
        if (!replaced) print "ADMIN_SECRET_TOKEN=" token
    }
' "${ENV_FILE}" > "${TEMP_FILE}"

chmod 600 "${TEMP_FILE}"
mv -f "${TEMP_FILE}" "${ENV_FILE}"
trap - EXIT

if [[ "${NO_RESTART}" == false ]] && command -v systemctl >/dev/null 2>&1 \
    && systemctl list-unit-files noder.service >/dev/null 2>&1; then
    systemctl restart noder.service
    echo "管理员 Token 已重置，noder.service 已重启。"
else
    echo "管理员 Token 已写入：${ENV_FILE}"
    echo "请重启 Noder 服务使配置生效。"
fi

echo "新的管理员 Token：${NEW_TOKEN}"
