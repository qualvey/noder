# Noder ↔ Mall 代理订阅自动交付 —— 实施 Spec（Noder 侧）

> 状态：可执行（本文 = Mall 侧 `attach-noder.md` 的 Noder 物化版；冲突以本文为准）
> 日期：2026-11-07
> 目标读者：**Nopder 仓库的新 agent**。本文不引用 Mall 仓库文件；如需核对 Mall 行为，Mall 仓库只能读不能改。
> 涉及仓库：**只有** `/home/ryu/code/noder`（分支 `dev-refactor`，HEAD `f2873c2`）

---

## 0. 一句话目标

给 Noder 补齐：**订单粒度的权益模型（D10）+ Mall↔Noder 自动交付的 5 个 HTTP 接口 + 修复已知收入漏洞**，使得 Mall 侧（另一个 agent 同时在开发）能凭支付回调全自动完成“开通订阅 → 计费到期 → 退款撤销”，不再靠人肉登录机器加用户。

完成后 Mall 侧链路应当可端到端跑通：

```
用户下单 → 支付成功 → Mall 调 activate → Noder 创建 credential + grant
→ 用户拿到订阅（sing-box / mihomo / xray URL）→ 到期或超量停用 → 退款可撤销
```

---

## 1. 给接手 agent 的开场白（可直接粘贴）

```
你在 /home/ryu/code/noder（分支 dev-refactor）上开发。
先执行并核对：git status && git log --oneline -5 && go build ./cmd/server && go test ./...
你的任务全部写在 doc/d10-mall-delivery.md，先完整读完再动手。
铁律：
  1. 只改 noder 仓库；不修改 Mall 仓库任何文件；
  2. 不提交 data.db* / .env / 任何备份件（git status 里未跟踪的 data.db.bak-*、
     data.db.before-d10、data.db-wal.before-d10、tproxy.conf、xray.json 一律不加入）；
  3. 不猜生产形态（当前可能是 systemd + SQLite，也可能是 Docker Compose + PostgreSQL，
     README 已描述首次切换流程；见 §7）；
  4. 不自行决定对外契约（/api/service/*、code=0、X-Service-Token、
     external_ref 语义）——本文已定死，照做；
  5. 每完成一个任务就提交一个原子化 commit（Conventional Commits）。
```

---

## 2. 现状已核实（动手前先看一眼，不要假设）

```bash
git status --short          # 工作树干净，只有未跟踪的备份件（见上）
git log --oneline -5
# f2873c2 feat: PostgreSQL + Docker Compose deployment with SQLite migration
# d6dfae9 feat(noder): 权益粒度（D10）SQLite 热备迁移脚本
# a9b745e fix: config.json generator automatic / remove commit instead of throw error
```

**已经提交、你不必重做**：

| 事项 | 位置 | 状态 |
|---|---|---|
| `external_ref` 列（你要用它做幂等） | `internal/db/db.go` / `internal/model/models.go`（`AccessGrant.ExternalRef *string`） | ✅ 已提交（f2873c2） |
| 旧唯一索引 `access_grant_user_endpoint_idx` | **不再创建**（db.go 只建 `access_grant_credential_idx` 普通索引 + `access_grant_external_ref_idx` Unique IfNotExists） | ✅ 已提交 |
| 迁移脚本 `scripts/migrate_d10.sh` | 热备 + drop 旧索引 + 加列建索引 + 校验 + 回滚说明 | ✅ 已提交 |
| `/api/service/subscriptions/*` 路由 | **不存在** | ❌ 你要做 |
| socks5 到期未过滤（收入漏洞） | `internal/handler/service_socks5.go: GetGostNodeAuthList` / `GetUserSocks5List` | ❌ 你要做 |
| external_id 重复注册返回 400（幂等断裂） | `internal/handler/service.go: RegisterServiceUser`，约第 65–71 行 | ❌ 你要做 |
| 订阅 Token 只有 64 bit | `service.go` 与 `internal/db/db.go:203` 的 `randomHex(16)` | ❌ 你要做 |

**背景事实（写死，别试图“优化掉”）**：

- **legacy 投影里多个 grant 共享同一 credential 是合法状态**（`ProjectLegacyEndpoint` 按 `uuid_value` 复用 credential 渲染到多个节点）。**不要**给 `credential_id` 加全局唯一，也不要“清理重复”。
- D10 的唯一约束只有两条：`external_ref` 唯一（订单粒度）；`access_grants.external_ref` 允许多个 NULL（历史/手工授权）。
- 订阅渲染入口 `GetActiveDomainEndpoints` 只遍历 grants —— **没有 grant 的 credential 永远不会出现在 `/sub` 中**；D10 下每个购买订单项 = 1 条 grant = 1 个 credential。

---

## 3. 任务 A — P0 修复（不修就是漏钱）

### A1. socks5 到期根本没有被强制（收入漏洞）

**现象**：`GetGostNodeAuthList`（`internal/handler/service_socks5.go:281`）当前只过滤 `node_id/is_active/status`，**完全不看任何到期时间**，而 GOST 每 15 秒热加载一次名单（见 `doc/micro-service/design.md`）。结果是 `format=socks` 的订阅到期后账密永远有效。

**修复**：auth-list 查询 join `access_grants` + `credentials`，套用与 `/sub` 完全相同的判定（`grant.IsUsable(now) && credential.IsUsable(now)`；可参考 `internal/db/domain.go:251` 的 `GetActiveDomainEndpoints`）：

```
endpoint.node_id = ? AND endpoint.is_active AND endpoint.status = 'active'
  且 access_grants.user_id = endpoint.user_id AND access_grants.endpoint_id = endpoint.id
  且 access_grants.status = 'active' AND (expires_at IS NULL OR expires_at > now())
  且 credentials.status = 'active'   （及 credential.expires_at 判定）
```

修复后效果：到期 + GOST 15 秒重载 = 15 秒内断服。

**同样 必须修**：`GetUserSocks5List`（`internal/handler/service_socks5.go:198`）——补到期过滤，用户列表不应再展示已经失效的线路。

**验收**：构造一条 expires_at 已过期的 grant → `/api/socks5/nodes/{id}/auth-list` 不再返回其账号密码。

### A2. `external_id` 重复注册返回 400 → 改成幂等 200

**现象**（`internal/handler/service.go:65-71`）：

```go
exists, _ := db.DB.NewSelect().Model((*model.User)(nil)).Where("external_id = ?", extID).Exists(r.Context())
if exists {
    RespondServiceError(w, http.StatusBadRequest, ...已被使用)
    return
}
```

**修复**：幂等化——已存在则返回 **HTTP 200** + 同一个 user（及其 `sub_urls`）。**绝不**再报 400。

**理由**：支付回调重试、worker 重试、后台人工重开，任何一次重放都撞 400，整条幂等链断掉。幂等必须是接口语义，不是“调用方记得先查一下”。

### A3. 订阅 Token 只有 64 bit

`User.Token` 当前 `randomHex(16)`（16 hex = 64 bit），`/sub?token=` 是无鉴权裸 URL。提升到 **32 字节随机**。已有 token 不强制迁移；新签发的用新长度。`internal/db/db.go:225` 的 `randomHex(16)` 也一并提高为 `randomHex(32)`（该处疑似是初始化 admin/默认用户的 token，确认后统一改）。

---

## 4. 任务 B — 5 个 Mall↔Noder 接口（P1）

### 4.0 统一约定

- 路径统一 `POST /api/service/subscriptions/*`、`GET /api/service/subscriptions/{id}`；复用现有 `ServiceAuth`（`internal/handler/service.go` 的 `RegisterServiceRoutes`，挂在 `./api/service/...` 下）。
- 鉴权：`X-Service-Token`（与 Mall 的 `proxy_providers.service_token` 一致，加密存储 `security.EncryptedString`）。
- 响应信封 `{ "code": 0, "message": "success", "data": {...} }`；错误 `{ "code": 4xxxx, "message": "...", "error": "SOME_CODE" }`。
- **Noder 成功 code=0**（Mall 后端会转成自己的 code=200，别在 Noder 返回 200）。
- **幂等键 = `order_item_id`**（Mall 订单项，全局唯一）；落 `access_grants.external_ref UNIQUE`。重放 → 返回**同一条 grant**，HTTP 200，绝不重复创建。
- `duration_seconds` 由 Mall 传（秒）；**Noder 不引入 plan / 套餐概念**。
- 所有失败路径日志必须带 `order_item_id` / `endpoint_id`（`logger.Error` / `logger.Warn`，参照 `internal/logger`）。

### 4.1 `POST /api/service/subscriptions/activate`（开通）

请求体：

```json
{
  "external_id": "u_xxx",            // Mall 用户 ID（Noder 内唯一）
  "endpoint_id": 123,                // Noder endpoint（int64）
  "format": "landing",               // socks | landing
  "exit_route_tag": "exit-us-la-01", // socks → 空串
  "duration_seconds": 2592000,
  "traffic_quota_bytes": 107374182400, // 0 = 不限
  "order_item_id": "oi_xxx"
}
```

**语义**（单次调用内完成）：

1. 按 `external_id` 查/建 `User`（new: 200 + 复用同一条；**幂等由 A2 保证**）；
2. 创建 `Credential`：
   - LANDING：`email = "c_<credential_id>@<exit_route_tag>"`（不可变；Xray 统计键）；
   - SOCKS：`email = "c_<credential_id>@socks"`；
3. 创建 `AccessGrant`（`external_ref = order_item_id`）；已有同 `external_ref` → 直接返回该 grant（HTTP 200），不新建；
4. 就绪校验（见 §4.6）→ `Delivered=true` 或 `delivered=false`；
5. `delivered_at/expires_at` 由 **Noder 自己的时钟**计算：`delivered_at = now`，`expires_at = now + duration_seconds`（**不是 Mall 传的**）；
6. 绑定凭证到 endpoint（落 `credential_deployments` desired_state，**由线路机 Agent 真正写入 Xray**；本阶段只发 desired_state，Agent 还没上线——**交付的就绪校验必须反映这一点，见 §4.6 接口行为说明**）。

成功响应（`code:0`）：

```json
{
  "delivered": true,
  "subscription_id": "<grant_id>",
  "user_id": 42,
  "credential_id": "<uuid>",
  "delivered_at": "2026-11-07T10:00:00Z",
  "expires_at":    "2026-12-07T10:00:00Z",
  "sub_urls": { "singbox": "...", "mihomo": "...", "xray": "..." },
  "readiness": { "endpoint_active": true, "agent_online": true, "route_ok": true, "egress_ip_match": true }
}
```

未就绪（**HTTP 202**，`code:202` 或 `delivered:false`）：

```json
{ "delivered": false, "reason": "route_probe_failed", "retryable": true }
```

**全部失败不会自动退款**（钱已收，状态就是“开通中”）；超时后由 Mall 转 `activation_failed`。

### 4.2 `GET /api/service/subscriptions/{id}`（对账 + 展示）

返回：状态、`expires_at`、`readiness`、`sub_urls`、`traffic { quota_bytes, used_bytes, updated_at }`。供 Mall 对账 worker 每 10 分钟拉一次，以及 C 端“我的订阅”展示。

### 4.3 `POST /api/service/subscriptions/{id}/extend`（续费）

```json
{ "duration_seconds": 2592000, "traffic_quota_bytes": 107374182400, "order_item_id": "oi_renew_xxx" }
```

- `expires_at = max(now, expires_at) + duration_seconds`；
- `traffic_quota_bytes` 可省略/0 = 本次续费不含额度；有则 **累加**（不清零）；
- 必须走同一条 grant（**不 new grant**）；同 `order_item_id` 重放幂等返回成功。
- 支持的订阅状态：`active / expired / traffic_exhausted`（过期半年后也可续）。

### 4.4 `POST /api/service/subscriptions/{id}/quota`（加油包）

```json
{ "delta_bytes": 107374182400, "order_item_id": "oi_topup_xxx" }
```

- `delta_bytes` 必须为**正数**；`quota_bytes = quota_bytes + delta_bytes`；
- 幂等键同 `order_item_id`（复用 `external_ref` 机制：**新建 grant 的思路完全一样**——以 `order_item_id` 为幂等键；实现时用 `quota_adjustments` 表或 grant 自身幂等；见下面“实现自由度”）；
- `traffic_exhausted` 订阅加额度后 → 自动重新激活（Agent 重新 AddUser + 状态回 `active`）；
- **只加不减**；扣减走后台人工并留审计。

### 4.5 `POST /api/service/subscriptions/{id}/revoke`（撤销，退款前置）

```json
{ "reason": "refund" }
```

- 撤销 grant + credential + 下发 `desired_state=revoked`；
- **已撤销再调必须返回 200**（退款流程会重试）；
- **绝不先退款、后撤销**（Mall 侧铁律；Noder 侧必须保证 revoke 可靠返回 200）。

### 4.6 就绪校验（landing vs socks，当前阶段怎么处理 Agent 未上线）

| format | 就绪条件 |
| --- | --- |
| `socks` | 落地机节点 active + 凭证 active + GOST 已加载该账密（`auth-list` 可查到） |
| `landing` | 线路机 active + Agent 在线 + 该 `exit_route_tag` 探针通过（exit IP == 落地机 IP） |

**当前阶段 Noder 没有 Agent/无法真测探活**（A4 只提交了代码，Agent 侧 A1 只读阶段未开始；route 探测打哪个 URL 见 attach-agent.md A4）。因此：

- `activate` 的就绪判定**先按“能确认的部分”走**：endpoint active + credential active + grant 建成 → `delivered=true`；
- 探针逻辑留出接口位置（`probe_result` 写到 `agent_nodes`/后台暂用），**先不阻塞开通**；
- 文档里把“就绪=端到端探针”作为 Agent A4 的验收目标，本阶段允许 `endpoint_active=true` 且 Agent 缺位时 `delivered=true` + `readiness.agent_online=false`；
- 同一原则：Agent 未上线时 desired_state 先落库不下发；Agent 上线后即生效。

### 4.7 幂等落点

- `access_grants.external_ref text UNIQUE`（已建），存 `order_item_id`；
- 应用层“先查再插”在并发下必漏——靠 DB 约束收口；
- 所有 5 个写接口全部以 `external_ref`（或由其派生的副作用）收口幂等。

---

## 5. 业务关键决策（D1–D11 精简版，全部定稿）

| 决策 | 一句话 | 影响 |
| --- | --- | --- |
| D1 映射键 | Mall `ProxyNode → (endpoint_id, exit_route_tag)`；socks 的 `exit_route_tag` 为空 | 端点唯一键 `(node_id, format, exit_route_tag)` |
| D2 库存 | 库存仍挂 `ProxyPlan`（不按格式拆分） | 不改 |
| D4 时钟 | `delivered_at/expires_at` 由 Noder 的时钟计算；Mall 只传 `duration_seconds` | 防时钟漂移 → 对账不一致 |
| D6 超时 | 交付超时 24h、后台可配（Mall `site_settings` 现值 `proxy_activation_timeout_hours`） | Mall 侧行为，不涉及你 |
| D8 数据面 | 线路机用 Xray + 自有 Agent，不走 3x-ui | 你的 `desired_state` 只认 Agent |
| D9 socks | **socks 不做流量计量**（无限流量），只对 landing 配额 | 计量实现只做 landing |
| D10 权益 | **权益粒度 = 订单项**（1 order_item = 1 credential = 1 grant） | 你已落库 |
| D11 落地组 | Mall 上同组（同 landing_group）禁止重复购买；`traffic_exhausted`/`pending_activation` 占位 | Mall 行为 |

**模型要点（你的表）**：`access_grants` 每条 = 一份订阅（grant ↔ credential 1:1；credential 可被多个 grant 引用——legacy 合法，D10 新数据不会发生）。

---

## 6. 状态机（与 Mall 同步，全部“只进不退”）

```
pending_activation ──activate(成功)──→ active
pending_activation ──超过 24h──────→ activation_failed      (Mall 侧，本阶段 Noder 不参与)
active ──expires_at 到期──────────→ expired
active ──流量用尽（landing）──────→ traffic_exhausted
traffic_exhausted ──加额度(quota)──→ active                  (唯一“回头”，由 quota 接口触发)
active ──revoke 成功──────────────→ revoked
```

`revoked`/`expired` 再调接口 → HTTP 409（Mall 据此判定“不可再续/不可再买”）；已 `revoked` 再 `revoke` → 200（退款重试安全）。

---

## 7. 部署形态与备份（**不要猜，先问**）

生产可能是以下两种形态之一，**从这台开发机看不到生产现状**（`/var/lib/noder` 不存在 ⇒ 生产未在本机）：

| 形态 | 特征 | 你要做的 |
| --- | --- | --- |
| A. systemd + SQLite | `/opt/noder/data.db*`，README「Debian 安装部署」段 | `scripts/migrate_d10.sh`（.backup + 校验）+ 部署新二进制；备份在 `data.db.bak-<ts>` |
| B. Docker Compose + PG | `/opt/noder/.env` 有 `DATABASE_URL`；compose.prod.yaml + migrate 迁移完成 | **不要**在 Noder 跑 SQLite 迁移脚本；先确认 migrate 已执行完，再按 PG 校验（见下） |

确认方法（问人，别执行）：

```bash
ssh prod 'systemctl is-active noder; grep -E "^DATABASE_URL=" /opt/noder/.env'
```

PG 形态下 D10 校验：

```sql
-- migrate 完成后确认索引存在
SELECT indexname FROM pg_indexes WHERE tablename = 'access_grants';
-- 期望：access_grant_external_ref_idx（UNIQUE）、access_grant_credential_idx（plain）
-- 不再有 access_grant_user_endpoint_idx
```

**绝不在生产执行任何 DROP / 删 `-wal` / 覆盖 data.db / 停服未经授权的命令。** 任何生产操作前必须先得到明确许可。

---

## 8. 验证清单（交付前逐条打勾）

```bash
cd /home/ryu/code/noder
git status --short                 # 无意外修改；绝不提交 data.db* / .env
go test ./...                      # 全绿
go vet ./...                       # 无新警告
go build -o /tmp/noder-check ./cmd/server   # 构建通过
```

**本地冒烟**（用 dev SQLite；会往 dev 库写一条测试 grant，可接受）：

1. 用 `RegisterServiceUser` 注册一个测试用户（或复用现有）；
2. 调 `activate`（`order_item_id=oi_demo_1`）→ 期望 `delivered=true`，记下 grant id；
3. 重调同一个 `order_item_id` → 期望 **同一个 grant id**，不新建；
4. `GET /subscriptions/{id}` → 状态 `active`，`expires_at` 合理；
5. `POST /subscriptions/{id}/extend` → `expires_at` 增加；
6. `POST /subscriptions/{id}/quota` → 额度增加；`traffic_exhausted` 场景加额度后恢复 `active`；
7. `POST /subscriptions/{id}/revoke` → 返回 200；再调 → 200；`/sub?token=` 不再渲染该 grant；
8. A2：重复注册同一个 `external_id` → 200（不再是 400）；
9. A3：新用户 token 长度 64 hex；
10. A1：造一个过期 grant → auth-list 中该账密消失。

**提交前**：

```bash
git status --short                 # 目测无 data.db* / .env / 备份文件
git add internal/ ...
git commit -m "..."                # 每个任务恰好一个 commit
```

---

## 9. 禁止事项（写进记忆里）

1. **不修改** `/home/ryu/code/Mall/**` 任何文件（只读核对）。
2. **不提交** `data.db*`（含 `data.db.bak-*`、`data.db.before-d10*`、`data.db-wal.before-d10`、本地 `.env`、`tproxy.conf`、`xray.json`、`/dist/` 产物）。
3. **不重写**公共历史（无 `reset --hard` / `push --force` / 变基已有 commit）。
4. **不自行发明**：路由前缀 `/api/service/*`、`X-Service-Token` 语义、JSON 字段名、`code:0`、`external_ref` 机制、D9 socks 不计量、D10 权益粒度——全部照本文 / 照 mall 侧定稿，有疑问→问人。
5. **不给 `credential_id` 加全局唯一**（legacy 共享凭证合法）；不给外部表清理数据。
6. **不在生产执行**任何写操作（包括停服）。
7. **不改** `scripts/migrate_d10.sh` 的行为语义（可在该脚本上追加步骤，但必须先说明）。

---

## 10. 需要人拍板的事项（遇到就停）

1. 生产到底是 SQLite 还是 Docker+PG（决定 §7 哪条路径生效）；
2. `nokder` 镜像 / systemd 的部署方式是否按 README（Docker Compose + GHCR）走；
3. 交付的验收标准是否包含“真实支付端到端走一遍”（Mall 侧在开发中；本次交付**不要求**联调，只要求接口契约自洽）；
4. `probe_result` 探测 URL 与判定标准（若有人要改 §4.6 的“先放行”为“阻塞开通”，需负责人确认）；
5. 任何一次需要 `docker compose down -v` 或删 `-wal` 的操作 —— 停。

---

## 11. 与 Mall 侧的对账表（两侧各给一张镜子，方便核对）

| Mall 侧（Mall 仓库） | Noder 侧（本仓库） | 对齐点 |
| --- | --- | --- |
| `proxy_subscriptions` 一行一条订单 | `access_grants` 一个订单项一条 grant | `external_ref = order_item_id` |
| Mall 下单→支付成功 | `activate` | `order_item_id` 幂等 |
| 续费按钮（订阅管理页） | `extend` | 同 grant、同一份凭证 |
| 加油包 / 售后补偿 | `quota` | `quota_bytes` 累加 |
| 退款 | `revoke`（先撤后退） | Mall 幂等重试 |
| “我的订阅”展示 | `GET /subscriptions/{id}` | 拉 sub_urls + traffic |
| 到期 / 超量停用 | 到期过滤 + 15s GOST 重载 | A1 修复 |
| 订单唯一约束 `order_item_id` | `external_ref UNIQUE` | 双保险 |
