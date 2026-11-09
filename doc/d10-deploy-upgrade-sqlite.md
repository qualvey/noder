# Noder Mall 交付（D10）升级部署指南 — SQLite + systemd（生产）

> 适用：当前生产为 **SQLite + systemd**（`noder.service`，二进制 `/usr/bin/noder`，数据目录 `/var/lib/noder/`）。  
> 后续计划迁移到 Docker Compose（见文末）；本指南**仅覆盖当前形态**。  
> **任何生产写操作/停服必须由你授权后执行；本文不代执行。**

## 1. 现状与变更影响

- 数据目录：`/var/lib/noder/`（`data.db`、`data.db-shm`、`data.db-wal`、`templates/`）
- 服务：`systemd` `noder.service`（`/usr/lib/systemd/system/noder.service`），`ExecStart=/usr/bin/noder`
- 本次变更（D10 Mall 订阅交付，A1–B5 + release 语义）：
  - `access_grants` 新增字段（`format/exit_route_tag/delivered_at/traffic_quota_bytes/traffic_used_bytes/traffic_updated_at` 等）与操作幂等表 `subscription_operations`；**新二进制启动时会自动 `CREATE TABLE IF NOT EXISTS` / `ALTER TABLE` 补列**（`internal/db/db.go`）。
  - D10 索引约束：**弃用**旧唯一索引 `access_grant_user_endpoint_idx`（同用户同 endpoint 允许多 grant）；**需要** `access_grant_external_ref_idx` `UNIQUE(external_ref)`（订单项幂等）。`scripts/migrate_d10.sh` 覆盖此意图，但**默认目标是 `/opt/noder/data.db`**，生产实际路径为 `/var/lib/noder/data.db`，见 §3。

## 2. 升级前准备

1. 选维护窗口（需停服，订阅/服务中断）。
2. 准备新二进制（已验证 `go build -o noder ./cmd/server` 可构建；用与生产一致的 OS/arch）。保留回滚用旧版。
3. 确认 `noder.service` 的 `WorkingDirectory`/环境变量/`DB` 路径仍指向 `/var/lib/noder`（避免新旧进程写错库）。
4. 准备 Mall 侧 `X-Service-Token` 用于升级后功能校验。

## 3. 升级步骤（停服 → 一致性备份 → D10 索引对齐 → 换包 → 起服 → 校验）

```bash
# 0) 确认现状
sudo systemctl status noder --no-pager
ls -lh /var/lib/noder/

# 1) 停服
sudo systemctl stop noder

# 2) 一致性备份（优先 sqlite3 .backup；再做目录级冷备）
TS=$(date +%Y%m%d%H%M%S)
sudo sqlite3 /var/lib/noder/data.db ".backup '/var/lib/noder/data.db.bak-${TS}'"
sudo cp -a /var/lib/noder "/var/lib/noder.bak-${TS}"

# 3) D10 索引/约束对齐（针对 /var/lib/noder/data.db 的等价步骤）
#    说明：scripts/migrate_d10.sh 默认 DB=/opt/noder/data.db。若要用脚本，请先
#    修改其 DB 变量指向 /var/lib/noder/data.db（并通读脚本）再执行；否则直接执行：
sudo sqlite3 /var/lib/noder/data.db <<'SQL'
DROP INDEX IF EXISTS access_grant_user_endpoint_idx;
CREATE UNIQUE INDEX IF NOT EXISTS access_grant_external_ref_idx ON access_grants(external_ref);
CREATE INDEX IF NOT EXISTS access_grant_credential_idx ON access_grants(credential_id);
-- 校验
SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name='access_grants' ORDER BY name;
SELECT COUNT(*) AS grants FROM access_grants;
SQL
# 期望：存在 access_grant_external_ref_idx(UNIQUE)、access_grant_credential_idx；不存在 access_grant_user_endpoint_idx。

# 4) 替换二进制（保留旧版）
sudo cp -a /usr/bin/noder "/usr/bin/noder.bak-${TS}"
sudo install -m 0755 /path/to/new/noder /usr/bin/noder

# 5) 起服
sudo systemctl start noder
sudo systemctl status noder --no-pager
sudo journalctl -u noder -n 120 --no-pager   # 确认无 migration/启动报错

# 6) 索引/表复核（可选，服务已启动）
sudo sqlite3 /var/lib/noder/data.db "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='access_grants' ORDER BY name;"
sudo sqlite3 /var/lib/noder/data.db "SELECT name FROM sqlite_master WHERE type='table' AND name='subscription_operations';"
```

## 4. 功能冒烟（生产，需 `X-Service-Token`；**会写库**，建议用测试订单项并在验证后 revoke）

```bash
BASE="https://<prod-host>"
TOK="<X-Service-Token>"
# activate（注意：当前语义见 §5，可能为 pending_activation 需 release）
curl -sS -H "X-Service-Token: $TOK" -H 'Content-Type: application/json' \
  -d '{"external_id":"smoke_u","endpoint_id":<EP_ID>,"format":"landing","exit_route_tag":"<TAG>","duration_seconds":3600,"traffic_quota_bytes":0,"order_item_id":"oi_smoke_1"}' \
  "$BASE/api/service/subscriptions/activate"
# GET / extend / quota / revoke 按返回的 subscription_id 验证；smoke 数据 revoke 清理。
```

## 5. 交付语义（Agent 未就绪期）与探测标准（§10.4）

- **activate 不先放行**：订单创建后为 `pending_activation`（客户端不可用，`/sub`/数据面不生效），需管理员/受权接口**手动放行（release/下发）**后客户端才可使用。`activate` 的 `expires_at/delivered_at` 由 Noder 时钟在 activate 时计算（计费起点按 activate 确定）；放行负责使其生效。
- **release（下发）**：`POST /api/service/subscriptions/{id}/release`（`ServiceAuth`，`X-Service-Token`），将 `pending_activation -> active` 并落 desired_state；幂等（已 active 返回 200）。Mall 可直接调用。
- **pending 的 revoke/extend/quota**：
  - `revoke`：允许，`pending_activation -> revoked`，返回 200（退款/取消前置，重试安全）。
  - `extend`：允许，在同一 pending grant 上顺延 `expires_at`（`max(now,expires_at)+duration`，quota 可累加），保持 pending。
  - `quota`：允许，累加 `traffic_quota_bytes`，保持 pending。
- **探测标准（暂定）**：出口可用性以 `http://www.gstatic.com/generate_204` 返回 204 作为基准信号；**保留对 IP 判定（`exit IP == 落地机 IP`）的未来扩展**。Agent 就绪后再接入端到端探测与阻塞/自动放行策略（需负责人确认完整判定与 URL）。

## 6. 回滚

```bash
sudo systemctl stop noder
sudo install -m 0755 /usr/bin/noder.bak-<TS> /usr/bin/noder
# 如需回滚数据（会丢失升级后新写入，谨慎）：
# sudo cp -a /var/lib/noder.bak-<TS>/data.db* /var/lib/noder/
sudo systemctl start noder
```
- 仅回滚二进制通常安全（新列对旧代码多数忽略）；若已产生依赖新约束/新接口的数据，回滚需评估。**不要随意删除 `-wal`**（§10.5）。

## 7. 后续：迁移到 Docker Compose

计划后期改用 Docker Compose 部署（不再用 systemd）。届时按 §7 的 Compose/PG 路径与 `README`/`compose.prod.yaml` 规划：镜像来源、`DATABASE_URL`/SQLite 挂载、migrate 流程、停机窗口与数据迁移。当前生产在完成该迁移前，仍按本文 SQLite+systemd 运维。
