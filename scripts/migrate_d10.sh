#!/usr/bin/env bash
# ===================================================================
# migrate_d10.sh — Noder 权益粒度迁移（D10）
#   access_grants 旧唯一索引 (user_id, endpoint_id) 废弃
#   → 新增 external_ref 列 + 唯一索引（幂等 / 防重复交付）
#
# 生产是 SQLite + systemd（/opt/noder/data.db）。本脚本：
#   ① .backup 热备（自动含 WAL 数据，安全于直接 cp）
#   ② 预检：grant 数 / 共享 credential 统计（informational，不阻断）
#   ③ 迁移：DROP 旧唯一索引（保留非唯一 access_grant_credential_idx）
#   ④ 新增 external_ref 列 + 唯一索引
#   ⑤ 可选：从 Mall 导出的 CSV 回填 external_ref
#   ⑥ 校验：external_ref 唯一性、integrity_check
#
# 用法：
#   scripts/migrate_d10.sh /opt/noder/data.db [mapping.csv]
#
#   mapping.csv 格式（含表头）：
#     order_item_id,grant_id
#     oi_xxxxxxxxx,<grant uuid>
#
# 回滚（仅限“迁移后无新订单交付”窗口）：
#   sudo systemctl stop noder
#   cp ${DB}.bak-<时间戳> ${DB}          # 覆盖 data.db
#   rm -f ${DB}-wal ${DB}-shm            # 旧 WAL 不再有效
#   sudo systemctl start noder
# 迁移后出现新订单交付 → 只能前进修复，不再回滚。
#
# 注意：legacy 投影里多个 grant 共享同一 credential（同 UUID 渲染到多个
# 节点）是合法状态。D10 不对 credential_id 强制唯一；external_ref 唯一
# 才是权益粒度的约束。新权益由 activate 语义保证每次新建凭证。
# ===================================================================

set -euo pipefail

DB="${1:-}"
CSV="${2:-}"

if [ -z "$DB" ]; then
  echo "用法: $0 <data.db 路径> [mapping.csv]" >&2
  exit 2
fi
if [ ! -f "$DB" ]; then
  echo "找不到数据库文件: $DB" >&2
  exit 2
fi
if ! command -v sqlite3 >/dev/null; then
  echo "缺少 sqlite3 CLI，请先安装 (apt install sqlite3)" >&2
  exit 2
fi

TS="$(date +%Y%m%d-%H%M%S)"
BACKUP="${DB}.bak-${TS}"

echo "==> [1/6] 热备: $BACKUP"
sqlite3 "$DB" ".backup '$BACKUP'"
INTEGRITY="$(sqlite3 "$BACKUP" "PRAGMA integrity_check;")"
echo "    integrity_check=$INTEGRITY"
[ "$INTEGRITY" = "ok" ] || { echo "备份完整性校验失败，终止" >&2; exit 1; }

echo "==> [2/6] 预检（仅展示，不阻断）："
sqlite3 "$DB" "SELECT 'grants=' || COUNT(*) FROM access_grants;"
sqlite3 "$DB" "SELECT 'credentials_with_multi_grants=' || COUNT(*) FROM (SELECT credential_id FROM access_grants GROUP BY credential_id HAVING COUNT(*) > 1);"
sqlite3 "$DB" "SELECT 'user_endpoint_pairs_with_multi_grants=' || COUNT(*) FROM (SELECT user_id, endpoint_id FROM access_grants GROUP BY user_id, endpoint_id HAVING COUNT(*) > 1);"

echo "==> [3/6] 迁移: DROP 旧唯一索引"
sqlite3 "$DB" <<'SQL'
DROP INDEX IF EXISTS access_grant_user_endpoint_idx;
SQL
echo "    旧唯一索引已 drop（保留存量非唯一 access_grant_credential_idx 供查询）"

echo "==> [4/6] 外部权益引用 + 唯一索引"
HAS_COL="$(sqlite3 "$DB" "SELECT COUNT(*) FROM pragma_table_info('access_grants') WHERE name='external_ref';")"
if [ "$HAS_COL" = "0" ]; then
  sqlite3 "$DB" "ALTER TABLE access_grants ADD COLUMN external_ref TEXT;"
  echo "    已新增列 external_ref"
else
  echo "    列 external_ref 已存在，跳过"
fi

if [ -n "$CSV" ]; then
  echo "==> [5/6] Mall 映射回填: $CSV"
  if [ ! -f "$CSV" ]; then
    echo "找不到 CSV: $CSV" >&2
    exit 2
  fi
  FIRST_LINE="$(head -1 "$CSV")"
  if [ "$FIRST_LINE" != "order_item_id,grant_id" ]; then
    echo "CSV 首行必须是: order_item_id,grant_id（实际: $FIRST_LINE）" >&2
    exit 2
  fi
  sqlite3 "$DB" <<SQL
CREATE TEMP TABLE IF NOT EXISTS _d10_map(order_item_id TEXT PRIMARY KEY, grant_id TEXT NOT NULL);
.mode csv
.import --skip 1 '$CSV' _d10_map
UPDATE access_grants SET external_ref = (SELECT order_item_id FROM _d10_map WHERE _d10_map.grant_id = access_grants.id)
 WHERE external_ref IS NULL;
DROP TABLE _d10_map;
SQL
  echo "    回填完成"
else
  echo "==> [5/6] 未提供 mapping.csv，跳过回填（external_ref 保持待回填）"
fi

sqlite3 "$DB" <<'SQL'
DROP INDEX IF EXISTS access_grant_external_ref_idx;
CREATE UNIQUE INDEX IF NOT EXISTS access_grant_external_ref_idx ON access_grants(external_ref);
SQL
echo "    唯一索引 access_grant_external_ref_idx 已建"

echo "==> [6/6] 校验"
sqlite3 "$DB" "SELECT 'missing_external_ref=' || COUNT(*) FROM access_grants WHERE external_ref IS NULL;"
sqlite3 "$DB" "SELECT 'external_ref_duplicates=' || COUNT(*) FROM (SELECT external_ref FROM access_grants WHERE external_ref IS NOT NULL GROUP BY external_ref HAVING COUNT(*) > 1);"
sqlite3 "$DB" "SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name='access_grants';"
INTEGRITY="$(sqlite3 "$DB" "PRAGMA integrity_check;")"
echo "    integrity_check=$INTEGRITY"
[ "$INTEGRITY" = "ok" ] || { echo "integrity_check 失败!" >&2; exit 1; }

echo "==> 完成。备份: $BACKUP"
echo "接下来：部署包含 db.go D10 改动的新版二进制 → 启动 → 抽测 /sub 渲染与下单流程。"
