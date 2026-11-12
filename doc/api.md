# Noder API 参考

本文档描述 Noder 当前服务端实际注册的 HTTP API。示例中的
`http://localhost:8000` 均可替换为部署地址。

## 1. 基本约定

- 默认监听地址：`http://0.0.0.0:8000`；端口可通过 `PORT` 修改。
- JSON 接口使用 `Content-Type: application/json`。
- 管理接口鉴权：`X-Admin-Token: <ADMIN_SECRET_TOKEN>`，也支持
  `Authorization: Bearer <ADMIN_SECRET_TOKEN>`。
- 用户侧接口使用查询参数 `token`，不需要管理员请求头。
- 错误响应统一为 `{ "detail": "错误说明" }`。
- 常见状态码：`200` 成功，`400` 参数无效，`401` 鉴权失败，`404` 不存在或
  没有可用节点，`409` 冲突，`500` 服务端错误。

## 2. 资源模型

### Node（节点）

节点只保存服务器和协议参数，不保存用户凭证。字段如下：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | integer | 节点 ID，服务端生成 |
| `tag` | string | 节点标识，必填 |
| `node_name` | string | 展示名称；为空时回退为 `tag` |
| `protocol` | string | `vless`、`tuic` 或 `anytls`，默认 `vless` |
| `server_address` | string | 服务器地址，必填 |
| `server_port` | integer | 服务器端口，必填 |
| `security` | string | `vless` 支持 `tls`/`reality`；`tuic` 固定为 `tls` |
| `sni` | string | TLS SNI |
| `transport_type` | string | 传输方式；非 TUIC 默认 `direct`，支持 `xhttp` |
| `path` | string | 传输路径 |
| `public_key` | string | Reality 公钥 |
| `short_id` | string | Reality Short ID |
| `fingerprint` | string | uTLS 指纹，如 `chrome` |
| `flow` | string | VLESS 流控，常用 `xtls-rprx-vision` |
| `congestion_control` | string | TUIC：`bbr`、`cubic`、`new_reno` |
| `is_active` | boolean | 是否参与订阅，默认 `true` |
| `remark` | string | 备注 |

VLESS 使用 Reality 时需要同时提供 `public_key`、`short_id`、`sni` 和
`fingerprint`。服务端会校验必填字段、固定值和枚举值。

`xhttp` 仅支持 VLESS + TLS，必须填写反代域名 `sni`，不支持 Reality。
该传输层仅由 Xray 渲染；Sing-box 和 Mihomo 订阅会明确返回不支持错误。

### User（用户）

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | integer | 用户 ID |
| `name` | string | 用户名，必填 |
| `token` | string | 订阅/下载凭证，自动生成或手动指定且必须唯一 |
| `uuid` | string | 专属 UUID；不提供时自动生成 |
| `password` | string | 专属密码；不提供时自动生成 |
| `is_active` | boolean | 是否启用，默认 `true` |
| `node_ids` | integer[] | 绑定节点，数组顺序决定订阅顺序 |
| `config_override` | string | JSON 字符串，目前只允许 `route`、`dns` |
| `remark` | string | 备注 |

用户响应包含 UUID 和密码，应按敏感信息处理。

用户端点（`endpoints`）的 `is_active` 字段控制该端点是否参与订阅节点渲染；关闭后端点仍保留，但不会出现在 Sing-box、Mihomo、Xray 或节点列表输出中。`is_urltest` 仅控制端点是否加入 Sing-box 的 `urltest` 组。

### DistFile（分发文件）

`file_type` 为 `apk`、`zip` 或 `text`。ZIP 可指定 `template_name`，用户用
Token 下载时进行个性化渲染；远程文件还包含 `source_url`、`cached_at`。

## 3. 管理 API

以下接口均需要管理员鉴权。

### 节点 `/api/nodes`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/nodes` | 查询节点列表 |
| `GET` | `/api/nodes/{id}` | 查询单个节点 |
| `POST` | `/api/nodes` | 创建节点 |
| `PUT` | `/api/nodes/{id}` | 部分更新节点，未提交字段保留 |
| `DELETE` | `/api/nodes/{id}` | 删除节点并清理用户绑定 |

创建示例：

```json
{
  "tag": "东京-01",
  "protocol": "vless",
  "server_address": "example.com",
  "server_port": 443,
  "security": "reality",
  "sni": "www.example.com",
  "public_key": "PUBLIC_KEY",
  "short_id": "SHORT_ID",
  "fingerprint": "chrome"
}
```

### 用户 `/api/users`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/users` | 查询用户列表（含 `node_ids`） |
| `GET` | `/api/users/{id}` | 查询单个用户 |
| `POST` | `/api/users` | 创建用户并绑定节点 |
| `PUT` | `/api/users/{id}` | 更新用户；`node_ids` 会重建绑定关系 |
| `DELETE` | `/api/users/{id}` | 删除用户及其绑定 |

创建请求示例：

```json
{
  "name": "alice",
  "is_active": true,
  "node_ids": [1, 2],
  "config_override": "{\"route\":{\"auto_detect_interface\":true}}"
}
```

`config_override` 必须是合法 JSON 字符串；更新时传 `null` 可清空。传
`node_ids: []` 或 `null` 会解除全部节点绑定。

### 文件 `/api/files`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/files` | 查询文件列表 |
| `POST` | `/api/files` | `multipart/form-data` 上传文件 |
| `POST` | `/api/files/upload` | 上传别名 |
| `POST` | `/api/files/remote` | JSON：从 URL 创建远程缓存 |
| `POST` | `/api/files/text` | JSON：创建文本文件 |
| `GET` | `/api/files/{id}/text` | 读取文本内容 |
| `PUT` | `/api/files/{id}/text` | 更新文本内容 |
| `PUT` | `/api/files/{id}` | 更新元数据/启用状态 |
| `POST` | `/api/files/{id}/refresh` | 刷新远程缓存 |
| `DELETE` | `/api/files/{id}` | 删除文件 |

上传字段：`file`（必填）、`name`、`file_type`、`remark`、`template_name`、
`download_name`、`force`。`file_type` 可为 `apk`、`zip`、`text` 或 `auto`；
同名且 SHA-256 相同的文件再次上传返回 `409`，设置 `force=true` 才覆盖。
单文件默认上限为 100 MB。远程文件请求示例：

```json
{"url":"https://example.com/client.zip","name":"客户端","file_type":"zip"}
```

创建文本：`{"name":"说明","content":"hello","download_name":"说明.txt"}`；
更新文本：`{"content":"new content"}`。

### 模板 `/api/templates`

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/templates` | 查询模板列表 |
| `GET` | `/api/templates/{id}` | 查询模板详情 |
| `PUT` | `/api/templates/{id}` | 更新内容并生成新版本 |
| `GET` | `/api/templates/{id}/history` | 查询历史版本 |
| `POST` | `/api/templates/{id}/rollback/{version}` | 回滚并生成新版本 |

更新模板请求：`{"content":"...","remark":"调整 DNS"}`。

### 设置与共享下载凭证

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/api/settings` | 查询全部设置 |
| `PUT` | `/api/settings` | JSON 对象批量更新设置 |
| `GET` | `/api/settings/shared-token` | 查询共享下载 Token |
| `POST` | `/api/settings/shared-token/reset` | 重置共享下载 Token |
| `GET` | `/dl/token` | 查询共享 Token（兼容路径） |
| `POST` | `/dl/token/reset` | 重置共享 Token（兼容路径） |

## 4. 用户侧 API

### 订阅与节点

| 方法 | 路径 | 响应 |
| --- | --- | --- |
| `GET` | `/sub?token={token}` | Sing-Box JSON，`application/json` |
| `GET` | `/mihomo?token={token}` | Mihomo YAML，`text/yaml` |
| `GET` | `/xray?token={token}` | Xray JSON，追加 VLESS + Reality outbound |
| `GET` | `/node?token={token}` | 节点及 Sing-Box outbound 数组 |
| `GET` | `/api/user/nodes?token={token}` | Sing-Box outbound 数组 |
| `GET` | `/api/user/verify?token={token}` | 用户摘要及 `singbox_config` |


/xray?token={token} 

```json
{"detail":"Xray only supports vless, got 'tuic'"}
```
渲染xray 节点时，跳过tuic ,　而不是直接不渲染，有vless就可以渲染

以上接口只使用启用用户和启用节点。Token 缺失、无效或用户停用返回 `401`；
没有启用节点返回 `404`。

`/api/user/verify` 成功响应结构：

```json
{
  "valid": true,
  "user_name": "alice",
  "token": "...",
  "node_ids": [1, 2],
  "nodes": [{"id": 1, "node_name": "东京-01", "protocol": "vless"}],
  "singbox_config": {}
}
```

### 文件下载 `/dl/{id}`

```text
GET /dl/{id}?token={user_token}
```

`token` 可使用用户 Token 或共享下载 Token。启用用户 Token 下载 ZIP 时，
服务端会按用户凭证渲染模板；使用 `download=1` 或 `raw=1` 可跳过 ZIP 引导页
并直接返回文件。响应包含 `Content-Disposition`。

## 5. 服务侧 API（Mall 自动化交付，P1）

统一前缀 ` /api/service/*`，鉴权 `X-Service-Token`（`ServiceAuth`），外壳 `{code,message,data}`，`code!=0` 仍 `HTTP 200`。

### 5.1 开通 POST /api/service/subscriptions/activate

只建 `pending_activation`，不下发。

**Request**
```json
{
  "external_id": "u_xxx",
  "endpoint_id": 123,
  "format": "landing",
  "exit_route_tag": "exit-us-la-01",
  "duration_seconds": 2592000,
  "traffic_quota_bytes": 107374182400,
  "order_item_id": "oi_xxx",
  "tag_override": "美国·洛杉矶 #2"
}
```
| 字段 | 必填 | 校验 | 说明 |
|---|---|---|---|
| `external_id` | 是 | `1..64` | Mall user_id |
| `endpoint_id` | 是 | 存在且 `is_active` | 否则 `40401 ENDPOINT_NOT_FOUND` |
| `format` | 是 | `socks\|landing` | `socks` 强制 `exit_route_tag=""` |
| `exit_route_tag` | 否 | `landing` 必填，`socks` 非空 `40001` | 组成 `(endpoint,tag)` 二元键 |
| `duration_seconds` | 是 | `>0` | 起点 `activate now`，`release` 不重算 |
| `traffic_quota_bytes` | 是 | `>=0`，`socks` 必须 `0` | `0=不限` |
| `order_item_id` | 是 | `UNIQUE external_ref` | 幂等键 |
| `tag_override` | 否 | `<=64` | 同用户同出口多份时区分，否则自动 `#N` |

**Response 200**
```json
{"code":0,"message":"success","data":{"subscription_id":"<grant.id>","status":"pending_activation","delivered":false,"delivered_at":"...","expires_at":"...","user_id":42,"credential_id":"...","sub_urls":{},"readiness":{"endpoint_active":true,"agent_online":false,"route_ok":false,"egress_ip_match":false}}}
```
幂等重放按 `order_item_id` 返回现 grant，`delivered = status==active && (socks?endpointActive:agentOnline&&hasDeployment)`。

**Failure（HTTP status + `{code, error, message}`）**
```json
// 400 参数错误
{"code":40001,"error":"INVALID_ARGUMENT","message":"exit_route_tag is required for landing"}
// 400 socks 额度非法
{"code":40001,"error":"INVALID_ARGUMENT","message":"traffic_quota_bytes must be 0 for socks (unlimited)"}
// 404 端点不存在
{"code":40401,"error":"ENDPOINT_NOT_FOUND","message":"endpoint not found"}
// 409 幂等冲突由DB兜底，重放返回200现grant而非409
```
`RespondServiceCodedError` 外壳见 `internal/handler/service_auth.go:84`，`RespondServiceError` 为裸 `{code:HTTP, message}`（如 `Invalid JSON body`）。

### 5.2 放行 POST /api/service/subscriptions/{id}/release

`{id}=grant.id`，无body。建议携带 `Idempotency-Key: <order_item_id>:release`。

| 前置状态 | 结果 |
|---|---|
| `pending_activation\|activation_failed` 且 `bindingCount>0` | `200 delivered=true status=active`，写 `credential_deployments(desired=active)` |
| `pending_activation\|activation_failed` 且 `bindingCount==0`（仅 landing） | `40902 BINDING_MISSING` |
| `active` | `200` 幂等 |
| `revoked` | `40901 SUBSCRIPTION_REVOKED` |
| `expired/traffic_exhausted` | `40902 INVALID_SUBSCRIPTION_STATE` |

成功后 `/sub` 渲染、`socks auth-list` 生效。错误 `40402 SUBSCRIPTION_NOT_FOUND`。

### 5.3 查询/续费/加油包/撤销

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/service/subscriptions/{id}` | 查询状态、`expires_at`、`readiness`、`traffic`、`sub_urls` |
| `POST` | `/api/service/subscriptions/{id}/extend` | `{"duration_seconds":..,"traffic_quota_bytes":..,"order_item_id":".."} ` 续费，`expires_at=max(now,expires)+duration` |
| `POST` | `/api/service/subscriptions/{id}/quota` | `{"delta_bytes":..,"order_item_id":".."} ` 加油包，`delta>0` |
| `POST` | `/api/service/subscriptions/{id}/revoke` | `{"reason":"refund"} ` 撤销，幂等 200 |

详见 `doc/attach-mall.md#7` 与 `doc/spec/mall-attach-noder.md#7`。

## 6. 配置与安全提示

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ADMIN_SECRET_TOKEN` | `admin-secret` | 管理 API 密钥 |
| `PORT` | `8000` | HTTP 监听端口 |

生产环境务必修改 `ADMIN_SECRET_TOKEN` 并使用 HTTPS。用户 Token、UUID、密码和
共享下载 Token 会出现在 API 响应或 URL 中，不应写入公开日志。
