# Agent 管理面与端点绑定管理接口设计

> 状态：设计稿（待实现）
> 目标：补齐 Noder 内部缺失的 Agent 运营管理能力，明确“Agent 上线后归属哪个端点”的绑定链路
> 关联：`spec/noder-alignment.md`（Agent 控制面 `/api/agent/v1`）、`dev/noder-endpoint-binding-spec.md`（`GET /api/service/endpoints` 只读聚合）、`dev/agentless-endpoint-spec.md`（`is_agentless`）、`api.md`（对外总表）

## 1. 背景与问题

现状 `internal/model/models.go:163` `migrations/001_refactor_domain.sql:77` 已有 `agent_nodes` + `endpoint_agent_bindings` + `credential_deployments` 三张控制面表，但无 `AdminAuth` 管理 API：

- Agent 身份与 `control_token` 需手工 `INSERT INTO agent_nodes`（见 `docs/e2e/xray-agent-integration.md:49` 夹具）
- 归属关系 `endpoint_agent_bindings(endpoint_id,agent_node_id)` 需手工 `INSERT`（同文档:57），`POST /api/agent/v1/heartbeat` 仅更新 `last_heartbeat_at/last_sync_at`，不自动建绑定 `internal/handler/agent_sync.go:77` `internal/handler/agent_auth.go:60`
- 运维无法在后台查看在线状态、轮换 Token、解绑/禁用，`release` 失败 `40902 BINDING_MISSING` 后 Mall 无法自助排查

本文新增 **管理面** `AdminAuth` 接口族，**不改动** Agent 控制面 `/api/agent/v1/*` 契约。

## 2. 资源模型

### 2.1 AgentNode `agent_nodes`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string(UUID) | PK，服务端生成 `uuid.NewString()`；允许创建时指定，需校验唯一 |
| `name` | string | 展示名，必填 1..64，唯一性不强制但建议唯一 |
| `control_token_ciphertext` | string | 控制面 Token，明文仅创建/轮换时返回一次，DB 存密文（现状为明文 `control_token_ciphertext` 对比，见 `internal/handler/agent_auth.go:43`，后续可加加密） |
| `status` | string | `online`/`offline`/`disabled`，默认 `offline`，`heartbeat` 置 `online`，`disabled` 拒绝 `AgentAuth` |
| `last_heartbeat_at` | time | 最后心跳，由 `POST /api/agent/v1/heartbeat` 驱动 |
| `metadata` | json | `agent_version/xray_version/hostname/capabilities/endpoints` 快照 `internal/handler/agent_sync.go:55` |
| `created_at/updated_at` | time | |

在线判定：`status != disabled && last_heartbeat_at > now-90s` 复用 `internal/handler/service_subscription.go:77 isAgentOnlineForEndpoint`

### 2.2 EndpointAgentBinding `endpoint_agent_bindings`

PK `(endpoint_id, agent_node_id)`，`status` 仅 `active/disabled`（复用现有 `status` 字段），`last_sync_at/last_error`

- 一个 Agent 可绑多 Endpoint，一个 Endpoint 可绑多 Agent（HA）
- `credential_deployments` 按绑定展开 `createDesiredDeployments` `internal/handler/service_subscription.go:343`

## 3. 鉴权与通用约定

- 前缀：`/api/agents`、`/api/endpoints/{id}/bindings` 均 `AdminAuth`（`X-Admin-Token` / `Authorization: Bearer`），复用 `internal/handler/nodes.go:16 RegisterNodeRoutes` 模式
- 成功：`200 { code:0, message:"success", data:... }` 或纯 `RespondJSON`（与 `api.md:9` 管理 API 保持 `{detail}` 兼容；建议新接口统一用 `code/message/data`，前端兼容两者）
- 失败：`400/401/404/409/500` + `{code, error, message}` 或 `{detail}`，错误码见 §6
- 分页：`?page=1&limit=20`（默认 20，最大 100），`order=created_at desc`
- 排序：Agent 按 `status(online>offline>disabled), updated_at desc, id asc`；绑定按 `endpoint_id asc, agent_id asc`

## 4. 接口列表

| # | 方法 | 路径 | 说明 |
|---|---|---|---|
| 1 | `GET` | `/api/agents` | 列表 + 在线状态 |
| 2 | `POST` | `/api/agents` | 创建 Agent，返回一次性 `control_token` |
| 3 | `GET` | `/api/agents/{id}` | 详情 |
| 4 | `PUT` | `/api/agents/{id}` | 更新 `name/status` |
| 5 | `DELETE` | `/api/agents/{id}` | 删除（有 active 绑定时 409，需先解绑或 `?force=true` 置 disabled） |
| 6 | `POST` | `/api/agents/{id}/token/rotate` | 轮换 Token |
| 7 | `GET` | `/api/agents/{id}/bindings` | 该 Agent 绑定的端点列表 |
| 8 | `POST` | `/api/agents/{id}/bindings` | 绑定到端点 |
| 9 | `DELETE` | `/api/agents/{id}/bindings/{endpointId}` | 解绑 |
| 10 | `GET` | `/api/endpoints/{id}/bindings` | 该端点绑定的 Agent 列表（与 7 互为视图） |
| 11 | `POST` | `/api/endpoints/{id}/bindings` | 端点视角绑定 |
| 12 | `DELETE` | `/api/endpoints/{id}/bindings/{agentId}` | 端点视角解绑 |

> 10-12 为 7-9 的对称视图，二选一实现即可，建议同时提供以适配“按 Agent 配端点”和“按端点配 Agent”两种运维心智。实现层复用同一 service。

## 5. 详细契约

### 5.1 `GET /api/agents`

Query: `status=online|offline|disabled`, `q=keyword`（匹配 id/name）, `page, limit`

Response `200`:
```json
{
  "code": 0,
  "data": {
    "total": 2,
    "items": [
      {
        "id": "ag_hk_01",
        "name": "香港线路机 01",
        "status": "online",
        "last_heartbeat_at": "2026-11-14T08:00:00Z",
        "is_online": true,
        "metadata": {"agent_version":"1.0.0","xray_version":"1.8.24","hostname":"line-hk-01"},
        "binding_count": 2,
        "created_at": "2026-11-01T00:00:00Z",
        "updated_at": "2026-11-14T08:00:00Z"
      }
    ]
  }
}
```
`is_online` 由后端计算（90s 窗口），`binding_count` 为 `active` 绑定数，**不暴露** `control_token`。

### 5.2 `POST /api/agents`

Request:
```json
{"id": "ag_hk_01", "name": "香港线路机 01"}
```
- `id` 可选；不传则生成 UUID；传则校验 `^[a-zA-Z0-9_-]{1,64}$`
- `name` 必填 1..64

Response `200`（仅此一次返回明文 token）:
```json
{"code":0,"data":{"id":"ag_hk_01","name":"香港线路机 01","status":"offline","control_token":"agt_live_xxx","created_at":"..."}}
```
DB 写入 `control_token_ciphertext = token`（现状明文对比，后续可加密）。日志脱敏。

### 5.3 `GET /api/agents/{id}`

Response 同 5.1 单项，不含 token。`404` 若不存在。

### 5.4 `PUT /api/agents/{id}`

Request:
```json
{"name": "HK-01 新名", "status": "disabled"}
```
- `name` 可选
- `status` 可选 `online/offline/disabled`；`disabled` 后 `AgentAuth` 拒绝 `401` `internal/handler/agent_auth.go:44`

Response `200` 返回更新后对象。

### 5.5 `DELETE /api/agents/{id}`

- 若存在 `active` 绑定：默认 `409 BINDING_EXISTS`，提示先解绑
- `?force=true`：事务内将所有绑定 `status=disabled` 并删除 Agent（或置 `disabled` 软删，取决于是否需保留审计；建议硬删，`credential_deployments` 保留历史）

### 5.6 `POST /api/agents/{id}/token/rotate`

无 body。生成新 token，旧 token 立即失效。

Response `200`:
```json
{"code":0,"data":{"id":"ag_hk_01","control_token":"agt_live_new","updated_at":"..."}}
```

### 5.7 `GET /api/agents/{id}/bindings`

Response:
```json
{
  "code": 0,
  "data": [
    {"endpoint_id":2,"endpoint_tag":"vless-02","endpoint_name":"VLESS 02","status":"active","last_sync_at":"...","last_error":null},
    {"endpoint_id":3,"endpoint_tag":"tuic-01","status":"active","last_sync_at":null,"last_error":null}
  ]
}
```

### 5.8 `POST /api/agents/{id}/bindings`

Request:
```json
{"endpoint_id": 2}
```
或批量：
```json
{"endpoint_ids": [2,3]}
```
校验：
- `endpoint_id` 存在（`node.id`）
- 已存在 active 绑定 → `200` 幂等返回
- `status=disabled` 的历史绑定 → 置回 `active`

Response `200` 返回绑定列表。

错误：
- `404 ENDPOINT_NOT_FOUND`
- `409 BINDING_EXISTS` 仅当需要区分时，默认幂等 200

### 5.9 `DELETE /api/agents/{id}/bindings/{endpointId}`

幂等：不存在返回 `200`。成功 `200 {code:0}`。

### 5.10 `GET /api/endpoints/{id}/bindings`

对称于 5.7，返回该端点绑定的 Agent 列表：
```json
{"code":0,"data":[{"agent_id":"ag_hk_01","agent_name":"HK-01","agent_status":"online","status":"active","last_sync_at":"..."}]}
```

### 5.11 `POST /api/endpoints/{id}/bindings`

Request:
```json
{"agent_id": "ag_hk_01"}
```
或 `{"agent_ids": ["ag_hk_01","ag_sg_01"]}`

### 5.12 `DELETE /api/endpoints/{id}/bindings/{agentId}`

对称于 5.9

## 6. 错误码

| HTTP | code | error | 场景 |
|---|---|---|---|
| 401 | 401 | UNAUTHORIZED | `X-Admin-Token` 缺失/错误 |
| 400 | 40001 | INVALID_ARGUMENT | `name` 为空、id 格式非法、`endpoint_id` 非法 |
| 404 | 40401 | ENDPOINT_NOT_FOUND | 绑定的 endpoint 不存在 |
| 404 | 40402 | AGENT_NOT_FOUND | Agent 不存在 |
| 409 | 40901 | AGENT_ID_CONFLICT | `POST /api/agents` id 已存在 |
| 409 | 40902 | BINDING_EXISTS | 已绑定且需冲突提示时（默认幂等可不抛） |
| 409 | 40903 | BINDING_NOT_EMPTY | `DELETE /api/agents/{id}` 仍有 active 绑定且未 force |
| 500 | 500 | INTERNAL_ERROR | DB 失败 |

## 7. 与现有链路的联动

- `heartbeat` 后 `GET /api/agents` 的 `last_heartbeat_at/is_online` 实时变化
- `GET /api/service/endpoints` 的 `bound_agents` 即本模块写入的 `endpoint_agent_bindings` 聚合 `internal/handler/service_endpoints.go:48`
- `POST /api/service/subscriptions/{id}/release` 的 `BINDING_MISSING` 判定 `internal/handler/service_subscription.go:676` 来源于此绑定；`isAgentOnlineForEndpoint` 复用 90s 窗口

## 8. 前端与运维

- `web/src/views/NodesView.vue` 节点详情增加“绑定 Agent”区块，复用 `GET /api/endpoints/{id}/bindings`
- 新增 `web/src/views/AgentsView.vue` 列表页：`GET /api/agents` + 创建/轮换/禁用 + 绑定抽屉
- 菜单：`节点 -> Agent 管理` 独立一级或归入 `节点` 二级

## 9. 实施清单

| 步骤 | 文件 | 内容 |
|---|---|---|
| 1 | `internal/handler/agents.go` | 新增 `RegisterAgentAdminRoutes` + 上述 12 个 handler |
| 2 | `internal/handler/agents_test.go` | 覆盖 CRUD、幂等绑定、force 删除、鉴权 |
| 3 | `cmd/server/main.go` | 注册路由 |
| 4 | `web/src/api.ts` | `agentsApi` |
| 5 | `docs/api.md` | 同步 `### Agent /api/agents` 章节 |
| 6 | 可选 | `control_token` 加密存储（当前明文 `control_token_ciphertext = token` 可先保持一致，后续加 `crypto`） |

## 10. 非目标

- 不改动 `X-Agent-Token` 鉴权逻辑，仅增加管理面供给
- 不引入自动发现/自动绑定（Agent 上报 `endpoints` 仍不建绑定，需人工确认）
- 首版不做审计日志，`metadata` 仅透出
