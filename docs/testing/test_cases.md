# Noder 核心业务域测试用例规约

本文档详尽列出 **Noder** 系统的标准测试用例，覆盖 7 大核心业务域，包含正常流、异常流、边界值、安全性、幂等性及容错验证。

---

## 目录

- [1. 节点与协议契约域 (Node & Protocol Contracts)](#1-节点与协议契约域-node--protocol-contracts)
- [2. 用户、凭证与授权生命周期域 (User, Credential & Access Grant)](#2-用户凭证与授权生命周期域-user-credential--access-grant)
- [3. 多核心动态订阅与拼接引擎域 (Multi-Core Dynamic Subscription Engine)](#3-多核心动态订阅与拼接引擎域-multi-core-dynamic-subscription-engine)
- [4. 文件分发与个性化模板注入域 (File Distribution & Dynamic Templates)](#4-文件分发与个性化模板注入域-file-distribution--dynamic-templates)
- [5. Mall 商城服务级自动化交付域 (Mall Service Automation Delivery)](#5-mall-商城服务级自动化交付域-mall-service-automation-delivery)
- [6. Agent 控制面调度与双向同步协议域 (Agent Control Plane & Bi-directional Sync)](#6-agent-控制面调度与双向同步协议域-agent-control-plane--bi-directional-sync)
- [7. 系统管理、安全防护与健壮性域 (System Admin, Security & Robustness)](#7-系统管理安全防护与健壮性域-system-admin-security--robustness)

---

## 1. 节点与协议契约域 (Node & Protocol Contracts)

### TC-NODE-001: [P0] VLESS Reality 节点参数完整合法性创建
- **测试目的**：验证创建包含 Reality 完整参数的 VLESS 节点时，服务端校验通过并正确持久化。
- **前置条件**：具备管理员权限 Token。
- **测试步骤**：
  1. 向 `POST /api/nodes` 发送请求：
     ```json
     {
       "tag": "tokyo-vless-01",
       "node_name": "东京 01 Reality",
       "protocol": "vless",
       "server_address": "198.51.100.10",
       "server_port": 443,
       "security": "reality",
       "sni": "gateway.example.com",
       "public_key": "q1w2e3r4t5y6u7i8o9p0a1s2d3f4g5h6j7k8l9z0x1c=",
       "short_id": "0123456789abcdef",
       "fingerprint": "chrome",
       "flow": "xtls-rprx-vision"
     }
     ```
  2. 检查 HTTP 响应状态码及返回的 Node 对象。
- **预期结果**：
  - 响应 `200 OK`，返回带有自增 `id` 的 Node 数据。
  - 数据库中对应字段完全一致。
  - **核心准则**：节点数据中不包含任何用户凭证（UUID 或 Password）。

---

### TC-NODE-002: [P0] VLESS Reality 依赖参数缺失校验
- **测试目的**：验证当 `security="reality"` 时，缺少 `public_key`、`short_id`、`sni` 或 `fingerprint` 任意字段时，服务端拒绝入库。
- **前置条件**：具备管理员权限 Token。
- **测试步骤**：
  1. 分别发起 4 次创建请求，每次分别缺失 `public_key`、`short_id`、`sni`、`fingerprint`。
- **预期结果**：
  - 4 次请求均返回 `HTTP 400 Bad Request`。
  - 响应包含具体错误提示（如 `"reality security requires public_key, short_id, sni, fingerprint"`）。
  - 数据库中不产生脏记录。

---

### TC-NODE-003: [P0] TUIC 节点参数及拥塞控制算法校验
- **测试目的**：验证创建 TUIC 协议节点时固定与枚举参数的合法性校验。
- **前置条件**：具备管理员权限 Token。
- **测试步骤**：
  1. 提交 `protocol="tuic"` 且 `congestion_control="bbr"`，`security="tls"` 的有效节点。
  2. 提交 `protocol="tuic"` 但 `congestion_control="invalid_cc"` 的非法节点。
- **预期结果**：
  - 步骤 1 返回 `200 OK`，成功保存。
  - 步骤 2 返回 `400 Bad Request`，提示拥塞控制算法仅支持 `bbr`、`cubic`、`new_reno`。

---

### TC-NODE-004: [P1] XHTTP 传输层仅限 VLESS + TLS + SNI 约束校验
- **测试目的**：验证 `xhttp` 传输层的契约约束（不支持 Reality，必须配置 SNI 反代域名）。
- **测试步骤**：
  1. 提交 `transport_type="xhttp"`, `security="reality"` 节点。
  2. 提交 `transport_type="xhttp"`, `security="tls"`, 但缺失 `sni` 节点。
  3. 提交 `transport_type="xhttp"`, `security="tls"`, `sni="cdn.example.com"` 节点。
- **预期结果**：
  - 步骤 1 和 步骤 2 均返回 `400 Bad Request`。
  - 步骤 3 返回 `200 OK` 成功保存。

---

### TC-NODE-005: [P0] 节点删除及用户端点/绑定级联清理
- **测试目的**：验证删除物理基础设施节点时，自动级联清理关联的所有用户端点（Endpoints）与用户-节点绑定关系，防止悬空引用。
- **前置条件**：存在 Node ID=10，且 User A 与 User B 均绑定了该节点。
- **测试步骤**：
  1. 发送 `DELETE /api/nodes/10`。
  2. 查询 Node ID=10 是否存在。
  3. 查询 `endpoint` 表与 `user_node_link` 表中 `node_id = 10` 的记录数。
- **预期结果**：
  - 删除请求返回 `200 OK`。
  - 节点彻底被移除。
  - 关联的 `Endpoint` 与 `UserNodeLink` 计数归零（0 条残留）。

---

## 2. 用户、凭证与授权生命周期域 (User, Credential & Access Grant)

### TC-USER-001: [P0] 用户创建及默认凭证（UUID/Password/Token）自动生成
- **测试目的**：验证新建用户在不显式指定敏感凭证时，系统自动生成合法的 UUIDv4、专属密码与订阅 Token。
- **测试步骤**：
  1. 发送 `POST /api/users`：
     ```json
     { "name": "bob", "is_active": true }
     ```
  2. 验证响应中 `token`、`uuid`、`password` 字段。
- **预期结果**：
  - 响应 `200 OK`。
  - `token` 为非空字符串（至少 16 位）。
  - `uuid` 为标准 RFC 4122 UUID 格式。
  - `password` 为随机生成的强凭证字符串。

---

### TC-USER-002: [P0] 端点默认继承用户级 UUID 机制
- **测试目的**：验证用户下属 Endpoints 在未指定专属 UUID 时，自动继承所属 User 的全局 UUID。
- **测试步骤**：
  1. 创建用户并显式指定全局 `uuid: "user-uuid-default-1234"`。
  2. 为该用户添加两个端点（Endpoint 1 不填 uuid，Endpoint 2 填空格）。
  3. 校验入库后的两个端点的 UUID 值。
- **预期结果**：
  - 两个端点的 UUID 均被自动解析并持久化为 `"user-uuid-default-1234"`。

---

### TC-USER-003: [P0] 用户停用 (`is_active=false`) 全局订阅阻断
- **测试目的**：验证当用户被停用时，所有客户端订阅接口及个性化下载均拒绝服务。
- **前置条件**：用户拥有合法 Token 及 2 个正常节点。
- **测试步骤**：
  1. 发起 `PUT /api/users/{id}` 设置 `is_active: false`。
  2. 客户端请求 `GET /sub?token={token}`。
  3. 客户端请求 `GET /mihomo?token={token}`。
  4. 客户端请求 `GET /api/user/verify?token={token}`。
- **预期结果**：
  - 所有请求均返回 `401 Unauthorized`，拒绝下发任何节点及有效配置。

---

### TC-USER-004: [P0] 凭证生命周期状态流转与导出判定
- **测试目的**：验证领域模型对凭证状态（`active` / `revoked` / `expired`）的精准隔离。
- **前置条件**：User 绑定了 Credential 与 Endpoint。
- **测试步骤**：
  1. 凭证为 `active` 且未过期：调用 `GetActiveDomainEndpoints`。
  2. 凭证更新为 `revoked`：再次调用 `GetActiveDomainEndpoints`。
  3. 凭证更新为 `active` 但 `expires_at = time.Now() - 1 hour`：再次调用。
- **预期结果**：
  - 步骤 1 返回 1 个有效端点。
  - 步骤 2 返回 0 个有效端点。
  - 步骤 3 返回 0 个有效端点（过期自动剔除）。

---

### TC-USER-005: [P1] 用户级配置覆盖 (`config_override`) 校验
- **测试目的**：验证用户级高级路由规则覆盖的 JSON 合法性校验及字段安全白名单（仅允许 `route`、`dns`）。
- **测试步骤**：
  1. 提交非法 JSON 字符串（如 `{broken-json`）。
  2. 提交包含未开放高危字段（如 `inbounds`）。
  3. 提交合法 JSON：`{"route": {"auto_detect_interface": true}}`。
- **预期结果**：
  - 步骤 1 和 步骤 2 返回 `400 Bad Request`。
  - 步骤 3 返回 `200 OK` 并成功保存。

---

## 3. 多核心动态订阅与拼接引擎域 (Multi-Core Dynamic Subscription Engine)

### TC-SUB-001: [P0] Sing-Box JSON 订阅动态生成与策略组装配
- **测试目的**：验证根据用户绑定的节点，在内存中动态组装完整的 Sing-Box 配置文件。
- **前置条件**：用户拥有 1 个 VLESS 节点与 1 个 TUIC 节点。
- **测试步骤**：
  1. 发起 `GET /sub?token={user_token}`。
  2. 解析响应 JSON。
- **预期结果**：
  - 响应 `200 OK`，`Content-Type: application/json`。
  - `outbounds` 列表中包含对应的两个节点 outbound。
  - VLESS outbound 中自动装配该用户的专属 `uuid`，且不包含多余的 `"transport": {"type": "tcp"}` 字段（确保 Sing-box 客户端语法合规）。
  - 模板中的 `selector`（如 `Proxy`）与 `urltest` 自动将这两个节点的 `tag` 纳入成员列表。

---

### TC-SUB-002: [P0] Mihomo (Clash Meta) YAML 订阅生成与特殊传输层阻断
- **测试目的**：验证生成标准 Mihomo YAML 配置，且当遇到 Mihomo 不支持的传输层（如 `xhttp`）时正确阻断。
- **测试步骤**：
  1. 用户绑定正常 VLESS Reality 节点：请求 `GET /mihomo?token={user_token}`。
  2. 验证返回的 YAML 文本中 `proxies` 结构、`reality-opts`、`client-fingerprint` 及策略组配置。
  3. 用户绑定包含 `xhttp` 传输层的节点：请求 `GET /mihomo?token={user_token}`。
- **预期结果**：
  - 步骤 1 返回 `200 OK`，`Content-Type: text/yaml`，格式符合 Clash Meta 规范。
  - 步骤 3 返回 `400 Bad Request`，明确提示 `"Mihomo 暂不支持 xhttp 传输层，请使用 Xray 渲染"`。

---

### TC-SUB-003: [P0] Xray 订阅渲染：跳过 TUIC 协议并渲染 VLESS
- **测试目的**：验证 Xray 订阅引擎的协议适配策略——当用户混合绑定 VLESS 与 TUIC 节点时，自动跳过 Xray 不支持的 TUIC 协议，而不会直接报错中断。
- **前置条件**：用户绑定 Node A (VLESS) 与 Node B (TUIC)。
- **测试步骤**：
  1. 请求 `GET /xray?token={user_token}`。
  2. 检查生成的 Xray JSON 出站配置。
- **预期结果**：
  - 响应 `200 OK`。
  - Xray outbounds 中成功渲染 Node A 的 VLESS 配置。
  - 自动跳过 Node B (TUIC)，并在日志中记录或兼容处理，不导致整站 500/400 失败。

---

### TC-SUB-004: [P1] 端点级 `is_urltest` 与 `is_active` 精细控制
- **测试目的**：验证端点级别的 `is_active=false` 不参与任何订阅渲染，`is_urltest=false` 不加入 urltest 探测组。
- **测试步骤**：
  1. 用户设置 Endpoint 1 (`is_active: false`)，Endpoint 2 (`is_active: true, is_urltest: false`)，Endpoint 3 (`is_active: true, is_urltest: true`)。
  2. 请求 `/sub?token={token}`。
- **预期结果**：
  - 生成的出站中完全不包含 Endpoint 1。
  - `Proxy` 组包含 Endpoint 2 与 Endpoint 3。
  - `urltest` 自动测速组中仅包含 Endpoint 3。

---

### TC-SUB-005: [P0] 订阅接口空节点保护 (`404`)
- **测试目的**：当用户没有任何可用节点时，接口规范返回友好 404 错误。
- **测试步骤**：
  1. 用户未绑定任何节点，或绑定的节点全部为 `is_active: false`。
  2. 请求 `/sub?token={token}`。
- **预期结果**：
  - 返回 `404 Not Found`，错误体为 `{"detail": "No active nodes found"}`。

---

## 4. 文件分发与个性化模板注入域 (File Distribution & Dynamic Templates)

### TC-FILE-001: [P1] 文件上传与 SHA-256 哈希去重防御
- **测试目的**：验证服务端对上传文件的哈希去重逻辑，防止重复存储与覆盖风险。
- **测试步骤**：
  1. 通过 `POST /api/files` 上传 `client.apk`（内容: "version-1"）。
  2. 再次上传相同文件名的 `client.apk`（内容完全相同，SHA-256 相同），未携带 `force=true`。
  3. 第三次上传，携带 `force=true`。
- **预期结果**：
  - 步骤 1 返回 `200 OK`，保存成功。
  - 步骤 2 返回 `409 Conflict`，提示文件已存在。
  - 步骤 3 返回 `200 OK`，成功覆盖。

---

### TC-FILE-002: [P1] 远程文件拉取：URL 校验与防穿透保护
- **测试目的**：验证从第三方 URL 缓存下载文件时的安全与防护。
- **测试步骤**：
  1. 提交 `ftp://attacker.com/evil.bin`。
  2. 提交 `file:///etc/shadow`。
  3. 提交 `https://example.com/oversized.zip`（模拟响应体积超过 100MB 上限）。
- **预期结果**：
  - 步骤 1 和 步骤 2 立即被拒绝，返回 `400 Bad Request`。
  - 步骤 3 在流式读取超限后中断，返回 `413 Request Entity Too Large`。

---

### TC-FILE-003: [P0] ZIP 客户端模板个性化动态注入
- **测试目的**：验证用户使用专属 Token 下载预设 ZIP 客户端时，服务端自动解压、渲染内置模板配置文件并回填用户凭证。
- **前置条件**：上传了一个包含 `config.json` 模板的客户端分发包 `Xray-windows-64.zip`。
- **测试步骤**：
  1. 请求 `GET /dl/{file_id}?token={user_token}&download=1`。
  2. 解压下载到的 ZIP 流，检查其中的 `config.json` 内容。
- **预期结果**：
  - HTTP 响应标头包含正确的 `Content-Disposition: attachment; filename="..."`。
  - ZIP 内的 `config.json` 中，模板占位符已被替换为该用户的实际专属 UUID、加密凭证及当前可用节点配置。

---

### TC-FILE-004: [P1] 模板多版本管理与回滚
- **测试目的**：验证 Sing-Box / Mihomo 配置模板更新时自动记录版本历史，并支持一键安全回滚。
- **测试步骤**：
  1. 获取当前模板版本（版本为 v1）。
  2. 发起 `PUT /api/templates/{id}` 更新模板内容，版本自增为 v2。
  3. 调用 `GET /api/templates/{id}/history` 查询历史列表。
  4. 发起 `POST /api/templates/{id}/rollback/1` 回滚至 v1。
- **预期结果**：
  - 步骤 2 返回版本号为 2。
  - 步骤 3 查出 2 条历史记录。
  - 步骤 4 生成版本号为 3 的新记录，其内容与 v1 完全相同。

---

## 5. Mall 商城服务级自动化交付域 (Mall Service Automation Delivery)

### TC-MALL-001: [P0] 两阶段开通：POST /activate 只建待放行状态
- **测试目的**：验证商业系统调用开通接口时，只创建 `pending_activation` 记录，不触发边缘 Agent 下发。
- **测试步骤**：
  1. 携带 `X-Service-Token` 发送 `POST /api/service/subscriptions/activate`：
     ```json
     {
       "external_id": "mall_u_20001",
       "endpoint_id": 101,
       "format": "landing",
       "exit_route_tag": "exit-us-la-01",
       "duration_seconds": 2592000,
       "traffic_quota_bytes": 107374182400,
       "order_item_id": "order_item_20001"
     }
     ```
- **预期结果**：
  - 返回 `200 OK`，`code: 0`。
  - `data.status` 为 `"pending_activation"`。
  - `data.delivered` 为 `false`。
  - 边缘部署表（`credential_deployments`）中不产生下发任务。

---

### TC-MALL-002: [P0] 开通请求幂等性：同一 `order_item_id` 重复提交
- **测试目的**：验证网络重试或商城重试时，严格基于 `order_item_id` 幂等返回已有 Grant，不重复扣费与建单。
- **测试步骤**：
  1. 发送相同的 TC-MALL-001 请求 Payload 两次。
- **预期结果**：
  - 两次调用均返回 `HTTP 200`，`code: 0`。
  - 两次返回的 `subscription_id` 完全一致。
  - 数据库中仅存在 1 条 `AccessGrant` 记录。

---

### TC-MALL-003: [P0] 两阶段放行：POST /release 成功激活与期望状态下发
- **测试目的**：支付成功后调用 release 接口，激活订阅并向边缘 Agent 节点生成下发任务。
- **前置条件**：TC-MALL-001 已完成，且对应 Endpoint 已绑定在线的 AgentNode。
- **测试步骤**：
  1. 发送 `POST /api/service/subscriptions/{subscription_id}/release`。
  2. 查询 DB 中的 `AccessGrant` 状态。
  3. 查询 `credential_deployments` 表。
- **预期结果**：
  - 响应 `200 OK`，`code: 0`。
  - `status` 变为 `"active"`。
  - `credential_deployments` 表中新增一条记录，`desired_state="active"`。

---

### TC-MALL-004: [P0] 零绑定放行防御：无 Agent 节点时阻断放行
- **测试目的**：验证 Spec §7.1.1 核心防护——当 landing 端点没有任何 AgentNode 绑定时，禁止静默成功，必须返回 409 报警。
- **前置条件**：创建了一个没有关联任何 Agent 绑定的孤立 Node。
- **测试步骤**：
  1. 对该孤立 Node 调用 `activate` 成功获得 `subscription_id`。
  2. 调用 `release` 进行放行。
- **预期结果**：
  - 返回 `HTTP 409 Conflict`。
  - 响应内容包含 `{"code": 40902, "error": "BINDING_MISSING"}`。
  - 不会向数据库写入空或错误的部署期望状态。

---

### TC-MALL-005: [P0] 订阅生命周期扩展：续费 (`/extend`) 与加油包 (`/quota`)
- **测试目的**：验证订阅时长与流量配额的正确递增计算。
- **测试步骤**：
  1. 当前有效订阅 `expires_at = T0`。调用 `POST /extend`（延长 30 天）。
  2. 当前订阅已过期（`expires_at = T_past`）。调用 `POST /extend`（延长 30 天）。
  3. 调用 `POST /quota`，`delta_bytes: 53687091200`（50GB）。
- **预期结果**：
  - 步骤 1 的新 `expires_at` 为 `T0 + 30天`。
  - 步骤 2 的新 `expires_at` 为 `当前时间 now + 30天`（`max(now, expires_at) + duration` 逻辑生效）。
  - 步骤 3 流量配额在原有基础上严格累加 50GB。

---

### TC-MALL-006: [P0] 订阅撤销：POST /revoke 与即时下发阻断
- **测试目的**：退款或违规时撤销订阅，控制面即时将边缘节点部署期望改为 `revoked`。
- **测试步骤**：
  1. 发送 `POST /api/service/subscriptions/{id}/revoke`，携带 `{"reason": "refund"}`。
  2. 再次调用撤销接口验证幂等性。
  3. 检查 `credential_deployments` 表中的 `desired_state`。
- **预期结果**：
  - 首次调用返回 `200 OK`，状态变为 `revoked`。
  - 再次调用依然幂等返回 `200 OK`。
  - `credential_deployments` 期望状态变更为 `"revoked"`。

---

## 6. Agent 控制面调度与双向同步协议域 (Agent Control Plane & Bi-directional Sync)

### TC-AGENT-001: [P0] Agent 心跳维持与在线状态上报
- **测试目的**：验证边缘 Agent 与控制面之间的定时保活机制与身份鉴权。
- **测试步骤**：
  1. Agent 未带 Token 请求 `POST /api/agent/v1/heartbeat`。
  2. Agent 携带错误 Token 请求。
  3. Agent 携带正确 Control Token 请求，附带上报系统负载与运行状态。
- **预期结果**：
  - 步骤 1 和 步骤 2 返回 `401 Unauthorized`。
  - 步骤 3 返回 `200 OK`，数据库中 Agent 节点的 `last_heartbeat_at` 更新为当前时间，状态置为 `online`。

---

### TC-AGENT-002: [P0] Agent 凭证期望状态拉取 (`sync`)
- **测试目的**：Agent 定期或长轮询从控制面拉取属于该 Agent 负责端点的期望凭证列表。
- **前置条件**：控制面存在待下发的 active 凭证与待下线的 revoked 凭证。
- **测试步骤**：
  1. Agent 发起 `POST /api/agent/v1/sync`。
  2. 检查返回的 `deployments` 数组。
- **预期结果**：
  - 响应中准确包含该 Agent 所关联 Endpoints 的所有待部署和待撤销凭证数据（含 UUID、加密密码、流量限制等）。

---

### TC-AGENT-003: [P0] Agent 部署结果双向确认 (`confirm`)
- **测试目的**：Agent 将本地内核（Xray/Sing-box）实际加载结果回传控制面，完成最终一致性闭环。
- **测试步骤**：
  1. Agent 发送 `POST /api/agent/v1/confirm`，告知凭证部署成功。
  2. Agent 发送错误确认（如内核重载失败，附带错误堆栈）。
- **预期结果**：
  - 步骤 1 更新数据库中部署记录的 `actual_state = "active"`，`last_sync_at = now`。
  - 步骤 2 记录 `last_error` 字段，且控制面状态不盲目置为成功。

---

## 7. 系统管理、安全防护与健壮性域 (System Admin, Security & Robustness)

### TC-SEC-001: [P0] 管理员接口双轨认证防御
- **测试目的**：验证后台管理 API 对 `X-Admin-Token` 与 `Authorization: Bearer <token>` 的严格校验。
- **测试步骤**：
  1. 无 Header 访问 `GET /api/nodes`。
  2. 携带伪造 Token 访问。
  3. 分别使用 `X-Admin-Token` 与 `Bearer` 携带合法 Token 访问。
- **预期结果**：
  - 步骤 1 和 步骤 2 均被 `AdminAuth` 中间件拦截，返回 `401 Unauthorized`。
  - 步骤 3 均放行，返回 `200 OK`。

---

### TC-SEC-002: [P1] 管理员密钥强制修改 (`force-change`) 本地网络隔离保护
- **测试目的**：验证用于初始化/紧急重置管理员密码的 `/api/auth/force-change` 接口仅限本地主机（127.0.0.1 / ::1 / localhost）调用，防止公网未授权提权。
- **测试步骤**：
  1. 模拟外部来源 IP（如 `192.168.1.100` 或公网 IP）请求 `POST /api/auth/force-change`。
  2. 本地（`127.0.0.1`）请求该接口。
- **预期结果**：
  - 步骤 1 返回 `403 Forbidden`（提示 `"force change is only available from localhost"`）。
  - 步骤 2 返回 `200 OK`，成功更新管理员密钥。

---

### TC-SEC-003: [P0] 接口三权分立与越权访问防御
- **测试目的**：验证系统三套凭证体系（Admin Token、Mall Service Token、Agent Control Token）的绝对隔离。
- **测试步骤**：
  1. 使用 Mall 的 `X-Service-Token` 尝试访问管理员接口 `GET /api/nodes`。
  2. 使用 Admin Token 尝试调用 Agent 专属接口 `POST /api/agent/v1/heartbeat`。
  3. 使用普通用户订阅 Token 尝试访问 `/api/settings`。
- **预期结果**：
  - 所有的跨权限访问全部被中间件拒绝，返回 `401 Unauthorized`。

---

### TC-SEC-004: [P0] 敏感凭证日志脱敏与防泄漏
- **测试目的**：验证系统在记录访问日志与错误日志时，严格脱敏用户的敏感信息。
- **测试步骤**：
  1. 触发包含敏感参数的请求与异常（如包含 `uuid`、`password`、`token`）。
  2. 检查控制台及日志文件输出。
- **预期结果**：
  - 日志中不得输出用户私钥、明文密码或未经掩码的完整 Token。

---

### TC-SEC-005: [P1] 并发请求与数据库事务原子性保障
- **测试目的**：验证高并发下（如 50 个并发协程同时开通或续费）数据库无死锁、无脏写，数据总数保持严格一致。
- **测试步骤**：
  1. 启动 50 个并发 Goroutine，同时对同一个订阅调用 `/quota` 加油包接口增加 100MB。
- **预期结果**：
  - 全部请求执行成功或通过重试保证原子累加。
  - 最终增加的配额总量精确等于 `50 * 100MB = 5000MB`，无丢失更新。
