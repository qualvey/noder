#!/usr/bin/env bash
# 修改 Noder SQLite 数据库中的管理员 Token。

set -euo pipefail

DB_FILE="/var/lib/noder/data.db"
SERVICE_NAME="noder.service"
NEW_TOKEN=""
NO_RESTART=false

usage() {
    cat <<'EOF'
用法：sudo ./scripts/change-admin-password.sh [选项]

选项：
  --token <值>       使用指定 Token；省略时交互式输入
  --db-file <路径>   SQLite 数据库路径（默认：/var/lib/noder/data.db）
  --no-restart       修改后不重启 noder.service
  -h, --help         显示帮助
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --token)
            [[ $# -ge 2 ]] || { echo "错误：--token 缺少值" >&2; exit 1; }
            NEW_TOKEN="$2"
            shift 2
            ;;
        --db-file)
            [[ $# -ge 2 ]] || { echo "错误：--db-file 缺少路径" >&2; exit 1; }
            DB_FILE="$2"
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

command -v sqlite3 >/dev/null 2>&1 || {
    echo "错误：未找到 sqlite3，请先安装 sqlite3。" >&2
    exit 1
}

if [[ -z "${NEW_TOKEN}" ]]; then
    read -r -s -p "请输入新的管理员 Token: " NEW_TOKEN
    printf '\n'
    read -r -s -p "请再次输入新的管理员 Token: " CONFIRM_TOKEN
    printf '\n'
    [[ "${NEW_TOKEN}" == "${CONFIRM_TOKEN}" ]] || {
        echo "错误：两次输入的管理员 Token 不一致。" >&2
        exit 1
    }
fi

[[ -n "${NEW_TOKEN}" ]] || { echo "错误：Token 不能为空。" >&2; exit 1; }
[[ "${NEW_TOKEN}" =~ ^[A-Za-z0-9._~+/=-]+$ ]] || {
    echo "错误：Token 只能包含字母、数字及 . _ ~ + / = -" >&2
    exit 1
}
[[ -f "${DB_FILE}" ]] || { echo "错误：数据库不存在：${DB_FILE}" >&2; exit 1; }

SQL="INSERT INTO appsetting (key, value) VALUES ('admin_secret_token', '$(printf '%s' "${NEW_TOKEN}" | sed "s/'/''/g")') ON CONFLICT(key) DO UPDATE SET value = excluded.value;"
sqlite3 "${DB_FILE}" "${SQL}"

if [[ "${NO_RESTART}" == false ]] && command -v systemctl >/dev/null 2>&1; then
    systemctl restart "${SERVICE_NAME}"
fi

echo "管理员 Token 已写入数据库：${DB_FILE}"
if [[ "${NO_RESTART}" == false ]]; then
    echo "noder.service 已重启。"
fi
