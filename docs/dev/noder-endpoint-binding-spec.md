# Noder Endpoint 与 Agent 绑定查询契约

> 状态：已实现（Noder `/api/service/endpoints`）
> 用途：供 Mall 等服务调用方读取可交付 Endpoint 及其 Agent 绑定状态。

## 1. 接口

```http
GET /api/service/endpoints
X-Service-Token: <token>
Accept: application/json
```

- 使用 Noder 的服务端鉴权；`token` 必须与 `mall_service_secret`（或部署配置中的服务密钥）一致。
- 缺少或错误的 Token 返回 HTTP `401`。
- 接口只读，不返回 UUID、密码、服务端配置等凭证或敏感字段。
- Endpoint 指 Noder 的 `node` 记录，其 `id` 用于 `POST /api/service/subscriptions/activate` 的 `endpoint_id`。

## 2. 成功响应

HTTP `200`：

```json
{
  "code": 200,
  "data": [
    {
      "id": 1,
      "tag": "tuic-01",
      "node_name": "TUIC 高速专线 01",
      "protocol": "tuic",
      "is_active": true,
      "status": "active",
      "binding_count": 0,
      "bound_agents": []
    },
    {
      "id": 2,
      "tag": "vless-02",
      "node_name": "VLESS REALITY 专线 02",
      "protocol": "vless",
      "is_active": true,
      "status": "active",
      "binding_count": 1,
      "bound_agents": [
        {
          "id": "ag_hk_01",
          "name": "香港线路机 01",
          "status": "online"
        }
      ]
    }
  ]
}
```

没有 Endpoint 时，`data` 返回空数组，不返回 `null`。列表应包含启用与停用的 Endpoint，便于调用方识别并排查不可用项。

| 字段 | 类型 | 定义 |
|---|---|---|
| `id` | integer | Noder Endpoint ID，对应 `node.id`。 |
| `tag` | string | Endpoint 的稳定标识，对应 `node.tag`。 |
| `node_name` | string | 展示名称，对应 `node.node_name`。 |
| `protocol` | string | 节点协议，如 `vless`、`tuic`。 |
| `is_active` | boolean | 管理开关，对应 `node.is_active`。 |
| `status` | string | Endpoint 生命周期状态，对应 `node.status`；可交付状态为 `active`。 |
| `binding_count` | integer | 此 Endpoint 上 `status=active` 的绑定记录数。它统计绑定关系，不代表 Agent 当前在线。 |
| `bound_agents` | array | 对应的 active binding 列表；无 active binding 时为空数组。 |
| `bound_agents[].id` | string | Agent ID，对应 `agent_nodes.id`。 |
| `bound_agents[].name` | string | Agent 名称，对应 `agent_nodes.name`。 |
| `bound_agents[].status` | string | Agent 当前状态，例如 `online`、`offline` 或 `disabled`。离线 Agent 仍可能有 active binding。 |

`binding_count` 应与 `bound_agents.length` 一致。列表不暴露绑定错误详情、控制 Token 或心跳时间。建议按 Endpoint 的 `sort_order`、再按 `id` 升序返回，保证结果稳定。

## 3. 判断是否可用于 landing 放行

查询方不得仅凭 `is_active=true` 判断 landing 已就绪。至少应检查：

1. Endpoint `is_active=true` 且 `status=active`；
2. `binding_count > 0`；
3. 至少一个绑定 Agent 的 `status=online`。

上述结果是查询时的快照，不保证后续放行一定成功。最终状态以 `POST /api/service/subscriptions/{id}/release` 为准；绑定在查询后被移除时，release 仍可能返回 `40902 BINDING_MISSING`。Agent 健康还应以 Noder 的在线判定为准（当前在线判定基于近 90 秒心跳），不应只信任调用方缓存。

## 4. 错误响应约定

| HTTP | JSON `code` / `error` | 含义 |
|---|---|---|
| `401` | `401` / `UNAUTHORIZED` | 服务 Token 缺失或无效。 |
| `500` | `500` / `INTERNAL_ERROR` | 查询失败；调用方可按自身重试策略处理。 |

Endpoint 列表查询成功但没有匹配项时仍返回 `200` 和空数组。不得将数据库查询失败伪装成空列表。

## 5. 方案 B：自动选择 Endpoint

当前 `activate` 契约要求 `endpoint_id > 0`。将来若启用方案 B，才允许：

```json
{
  "external_id": "mall-user-123",
  "endpoint_id": 0,
  "format": "landing",
  "region": "hk",
  "tag_override": "香港高速专线",
  "duration_seconds": 2592000,
  "traffic_quota_bytes": 107374182400,
  "order_item_id": "order-item-456"
}
```

### 5.1 候选池与调度

`endpoint_id=0` 表示由 Noder 调度，不是 ID 为 0 的真实 Endpoint。Noder 应在一个明确的、与 `format` 对应的候选池中执行筛选：

1. 排除 `is_active=false` 或 `status != active` 的 Endpoint；
2. 对 landing，仅保留至少存在一条 active binding 且 Agent 健康的 Endpoint。健康至少要求 Agent 未禁用、状态在线，并且心跳在最近 90 秒内；
3. `region` 为必填条件，按 Noder 统一维护的区域别名表匹配（例如 `hk` 与 `香港` 归一到同一区域）；未配置区域映射时请求应报参数错误，不得扩大为全局候选池；
4. `endpoint_id=0` 的方案 B 请求中，`tag_override` 作为可选的 Endpoint tag 精确匹配条件（区分大小写）；提供时只保留 `node.tag` 与之完全相同的候选项，不匹配时返回“无健康候选”，不得忽略该条件。当前正数 `endpoint_id` 请求里的 `tag_override` 仍表示订阅展示覆盖名。两种含义必须通过版本化契约明确区分；如自动选线订阅还需要自定义展示名，应另增 `display_tag` 字段；
5. 在满足条件的候选项中按 Noder 定义的健康度/容量/线路质量评分选出一条。评分数据和优先级必须可观测、可测试；如评分相同，使用确定性 tie-break（例如优先级后轮询），避免每次重试随机换线。

“最佳线路机”不能仅由 `is_active` 或 binding 数量推断。若 Noder 尚无 RTT、容量或出口探测数据，第一版只能按健康、管理员优先级和负载等可用信号选路，并应在接口文档中声明该排序不等价于端到端质量保证。

如果没有符合条件的候选项，Noder 应返回稳定的业务错误（建议 HTTP `409`、`NO_HEALTHY_ENDPOINT`），不能静默退回无 active binding 的节点，也不能创建半完成订阅。

### 5.2 幂等与放行

- 先按 `order_item_id` 查询已有订阅，再进行调度。幂等重放必须返回首次选择并持久化的 Endpoint，不能重新选线。
- 首次调度选出的真实 `endpoint_id` 必须写入 grant，并在 activate 响应中返回；后续查询、release、revoke 均使用该 ID。
- 自动选择只负责确定 Endpoint 并创建 `pending_activation`；landing 仍须经过 release 的 active binding 检查。未能放行时按既有规则返回 `40902 BINDING_MISSING`，不得把 pending 订阅标记为 active。
- 当前实现尚不接受 `endpoint_id=0`，`region` 也不是现有 activate 请求字段。该行为只有在 Noder 实现、测试并发布方案 B 后才生效。

## 6. 当前排查提示：Endpoint 1

目前 Endpoint `1` 缺少 `status=active` 的 Agent binding。因而它不适合作为 landing 放行目标；调用 `POST /api/service/subscriptions/{id}/release` 时会触发 HTTP `409`、业务码 `40902`、错误 `BINDING_MISSING`。

排查与修复：

1. 检查 Endpoint 1 对应的 `endpoint_agent_bindings`，确认存在指向正确 `agent_nodes.id` 的 active binding；
2. 确认 Agent 未被禁用，且持续向 Noder 上报心跳；
3. 检查 Agent 心跳/同步日志及 Endpoint 配置，修复后重新查询本接口；
4. 绑定恢复后，对原订阅安全重试 release；release 幂等，状态以 Noder 响应为准。

增加或恢复 binding 不会自动完成已有订阅的 release，调用方仍须重试 release。

## 7. 实现状态与兼容性

Noder 已在受 `ServiceAuth` 保护的服务路由中注册 `GET /api/service/endpoints`。自动选线方案 B 仍未实现；当前 `activate` 仍要求 `endpoint_id > 0`。

本文 endpoint-list 响应使用 JSON `code: 200`。现有部分 `/api/service/*` 成功响应使用 `code: 0`；调用方应按具体接口契约解析 JSON 业务码，不要将其与 HTTP 状态码混淆。

单元测试覆盖鉴权、无绑定 Endpoint、active/inactive binding、在线/离线 Agent、停用 Endpoint 的返回，以及稳定排序。
