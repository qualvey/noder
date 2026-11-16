# Noder 控制面 × Xray Agent 数据面 端到端 (E2E) 测试用例文档

> **文档版本**：v1.0  
> **适用范围**：Noder (`/home/ryu/code/noder`) 控制面与 Xray Agent (`/home/ryu/code/xray-agent`) 数据面的全流程集成验证与联调验收。  
> **生效契约**：基于 `doc/spec/noder-alignment.md` 与 `doc/spec/observability.md`。

---

## 1. 测试拓扑与角色定义

```
                  ┌──────────────────────────────────────────────┐
                  │                 Mall / 运维客户端             │
                  └──────────────────────┬───────────────────────┘
                                         │ HTTP (X-Service-Token)
                                         ▼
                                ┌─────────────────┐
                                │   Noder 控制面   │ (:8080)
                                │  (Authority DB) │
                                └────────▲────────┘
                                         │ HTTP Pull/Report (X-Agent-Token)
                                         │ /api/agent/v1/*
                                         ▼
                                ┌─────────────────┐
                                │   Xray Agent    │ (线路机守护进程)
                                └────────▲────────┘
                                         │ gRPC (:8900)
                                         │ HandlerService / StatsService
                                         ▼
                                ┌─────────────────┐
                                │    Xray-core    │ (数据面转发引擎)
                                └─────────────────┘
```

---

## 2. 前置测试环境准备

### 2.1 依赖与服务状态检查
1. **Xray-core 服务**：运行在本地或线路机，确保开放 gRPC API（默认 `127.0.0.1:8900`，包含 `HandlerService` 与 `StatsService`）。
2. **Noder 服务**：启动在 `http://127.0.0.1:8080`，使用 SQLite（`data.db`）。
3. **Xray Agent**：已根据 `config.yaml` 编译出二进制。

### 2.2 测试固定基准数据（Fixtures）

在 Noder 数据库中执行前置夹具初始化：
```sql
-- 1. 创建测试 Agent 节点
INSERT OR REPLACE INTO agent_nodes (id, name, control_token_ciphertext, status)
VALUES ('ag_hk_01', 'Line HK Agent 01', 'agt_live_9f8a7b6c5d4e3f2a1b0c', 'offline');

-- 2. 确保节点 2 (VLESS 端点) 存在
INSERT OR REPLACE INTO node (id, tag, inbound_tag, node_name, protocol, server_address, server_port, is_active, status)
VALUES (2, 'vless-02', 'in-443', 'VLESS REALITY 专线 02', 'vless', '127.0.0.1', 443, 1, 'active');

-- 3. 将端点 2 绑定到 Agent
INSERT OR REPLACE INTO endpoint_agent_bindings (endpoint_id, agent_node_id, status)
VALUES (2, 'ag_hk_01', 'active');
```

Agent 配置文件 `config.yaml` 基准：
```yaml
agent_id: "ag_hk_01"
hostname: "line-hk-01"
state_file: "./state.json"

log:
  level: "debug"
  format: "pretty"

noder:
  base_url: "http://127.0.0.1:8080"
  token: "agt_live_9f8a7b6c5d4e3f2a1b0c"
  sync_interval_sec: 5
  timeout_sec: 5

xray:
  grpc_addr: "127.0.0.1:8900"
  health_check_sec: 3

traffic:
  enabled: true
  interval_sec: 15

endpoints:
  - endpoint_id: 2
    inbound_tag: "in-443"
```

---

## 3. 测试用例全景清单 (Test Matrix)

| 用例 ID | 测试类别 | 测试场景 | 核心验证点 | 优先级 |
|---|---|---|---|---|
| **TC-01** | 身份与鉴权 | 非法 Token 访问拦截 | 401 Unauthorized，无凭证泄露 | P0 |
| **TC-02** | 探活与就绪 | 正常心跳发送与状态流转 | `status=online`，`agent_online=true` 动态感知 | P0 |
| **TC-03** | 凭证下发生效 | 订阅开通、释放至 Xray 注入 | `desired=active` -> Agent 拉取 -> Xray AddUser -> `applied=active` | P0 |
| **TC-04** | 增量与快照 | `since_revision` 增量拉取 | 无变更返回空，变更仅返回增量版本，分页安全 | P1 |
| **TC-05** | 凭证停用阻断 | 订阅撤销与下线收敛 | `desired=revoked` -> Xray RemoveUser -> `applied=revoked` | P0 |
| **TC-06** | 容灾与自愈 | Xray 重启全量自动灌入 | Xray 重启丢失内存用户后，Agent 自动检测重灌 | P0 |
| **TC-07** | 乱序与并发 | 单调 Revision 与版本冲突回退 | `report.rev < db.rev` 返回 `stale_revision` 冲突拒更 | P1 |
| **TC-08** | 流量统计幂等 | 流量定期上报与去重 | 相同 `report_id` 重复上报不重复累加计费 | P1 |
| **TC-09** | 权限边界隔离 | 越权访问非绑定 Endpoint | 访问/上报非绑定端点直接拦截返回 403 Forbidden | P0 |

---

## 4. 详细测试步骤与预期结果

### TC-01: 非法 Token 访问拦截
- **测试目的**：验证 Noder Agent 接口边界安全，防止伪造线路机请求。
- **执行步骤**：
  ```bash
  # 1. 缺失 Token
  curl -i -X POST http://127.0.0.1:8080/api/agent/v1/heartbeat \
    -H "Content-Type: application/json" -d '{}'

  # 2. 错误 Token
  curl -i -X POST http://127.0.0.1:8080/api/agent/v1/heartbeat \
    -H "X-Agent-Token: invalid-token-xyz" \
    -H "Content-Type: application/json" -d '{}'
  ```
- **预期结果**：
  1. 两次请求均返回 HTTP `401 Unauthorized`；
  2. 响应 Body 为 `{"code":401,"error":"ERR_INVALID_TOKEN","message":"..."}`；
  3. Noder 控制台记录 `[WARN]` 级别的访问拦截日志。

---

### TC-02: 正常心跳发送与状态流转
- **测试目的**：验证 Agent 定时心跳上报，以及 Noder 动态计算 `grantReadiness.agent_online` 的能力。
- **执行步骤**：
  1. 启动 Xray Agent：`./bin/xray-agent -config ./bin/config.yaml`；
  2. 观察 Agent 日志输出；
  3. 查询 Noder 数据库状态：
     ```bash
     sqlite3 /home/ryu/code/noder/data.db \
       "SELECT id, status, last_heartbeat_at FROM agent_nodes WHERE id='ag_hk_01';"
     ```
  4. 触发一次订阅查询：
     ```bash
     curl -s -X GET http://127.0.0.1:8080/api/service/subscriptions/<sub_id> \
       -H "X-Service-Token: passwd" | jq .data.readiness
     ```
- **预期结果**：
  1. Agent 输出 `[DEBUG] heartbeat succeeded sync_interval_sec=15`；
  2. Noder 输出 `[DEBUG] received agent heartbeat agent_id=ag_hk_01`；
  3. 数据库 `agent_nodes.status` 变为 `online`，`last_heartbeat_at` 为当前时间戳；
  4. 订阅查询返回 `"agent_online": true`。

---

### TC-03: 订阅开通、释放至 Xray 注入（核心主流程）
- **测试目的**：验证完整的三段式生命周期：`activate(pending)` -> `release(active)` -> Agent 增量拉取 -> Xray 注入 -> `applied=active` 闭环。
- **执行步骤**：
  1. **开通订阅（activate）**：
     ```bash
     ACT_RESP=$(curl -s -X POST http://127.0.0.1:8080/api/service/subscriptions/activate \
       -H "X-Service-Token: passwd" \
       -H "Content-Type: application/json" \
       -d '{
         "external_id": "test_user_e2e_01",
         "order_item_id": "order_item_e2e_021",
         "endpoint_id": 2,
         "format": "landing",
         "duration_seconds": 86400,
         "traffic_quota_bytes": 10737418240
       }')
     SUB_ID=$(echo $ACT_RESP | jq -r .data.subscription_id)
     CRED_ID=$(echo $ACT_RESP | jq -r .data.credential_id)
     echo "Activated Subscription: $SUB_ID, Credential: $CRED_ID"
     ```
  2. **确认释放生效（release）**：
     ```bash
     curl -s -X POST "http://127.0.0.1:8080/api/service/subscriptions/${SUB_ID}/release" \
       -H "X-Service-Token: passwd" \
       -H "Content-Type: application/json" | jq
     ```
  3. 等待一个 Agent 同步周期（< 5 秒）；
  4. **核验 Noder 控制面状态**：
     ```bash
     sqlite3 /home/ryu/code/noder/data.db \
       "SELECT credential_id, endpoint_id, desired_state, applied_state, revision FROM credential_deployments WHERE credential_id='${CRED_ID}';"
     ```
  5. **核验 Agent 本地状态文件**：
     ```bash
     jq .users ./state.json
     ```
- **预期结果**：
  1. `release` 响应中 `status` 为 `active`，`readiness.agent_online` 为 `true`；
  2. Noder 日志记录 `createDesiredDeployments: written desired_state=active`；
  3. Agent 抓取到新增 item，日志输出 `[INFO] Upsert user to inbound in-443`；
  4. 数据库中 `credential_deployments.applied_state` 从 `unknown` 收敛为 `active`；
  5. 本地 `./state.json` 包含该用户，且标记为 `applied_state: active`。

---

### TC-04: 增量与快照拉取机制
- **测试目的**：验证 `since_revision` 过滤逻辑，避免每次轮询拉取全量打满数据库。
- **执行步骤**：
  1. 获取当前最新版本号：
     ```bash
     CUR_REV=$(sqlite3 /home/ryu/code/noder/data.db "SELECT MAX(revision) FROM credential_deployments;")
     ```
  2. 携带 `since_revision` 拉取：
     ```bash
     curl -s -X GET "http://127.0.0.1:8080/api/agent/v1/deployments?endpoint_id=2&since_revision=${CUR_REV}" \
       -H "X-Agent-Token: agt_live_9f8a7b6c5d4e3f2a1b0c" | jq .data
     ```
- **预期结果**：
  1. 返回体中的 `items` 为空数组 `[]`，`has_more: false`；
  2. 证明当没有状态变更时，Agent 轮询不会产生冗余数据处理。

---

### TC-05: 订阅撤销与下线收敛 (Revoke)
- **测试目的**：验证用户退款/到期撤销时，Xray 实时阻断用户。
- **执行步骤**：
  1. 发起撤销请求：
     ```bash
     curl -s -X POST "http://127.0.0.1:8080/api/service/subscriptions/${SUB_ID}/revoke" \
       -H "X-Service-Token: passwd" \
       -H "Content-Type: application/json" | jq
     ```
  2. 观察 Agent 日志（等待同步周期）；
  3. 核查 Noder 部署状态：
     ```bash
     sqlite3 /home/ryu/code/noder/data.db \
       "SELECT desired_state, applied_state FROM credential_deployments WHERE credential_id='${CRED_ID}';"
     ```
- **预期结果**：
  1. Noder 将 `desired_state` 更新为 `revoked`，并产生更大的单调递增 `revision`；
  2. Agent 增量拉取到该条目，调用 Xray gRPC `AlterInbound(RemoveUser)`；
  3. Agent 本地清除该用户缓存，并回传 `applied_state=revoked`；
  4. 数据库最终收敛为 `applied_state=revoked`。

---

### TC-06: Xray 重启全量自动灌入与自愈
- **测试目的**：验证由于 Xray 动态注入用户完全驻留在内存，在 Xray 重启进程后 Agent 能自愈重灌。
- **执行步骤**：
  1. 确保系统中至少有 1 个 `active` 状态的用户；
  2. 重启本地 Xray 进程：
     ```bash
     sudo systemctl restart xray
     ```
  3. 观察 Agent 控制台输出。
- **预期结果**：
  1. Agent 的健康探针探测到 gRPC 断开，触发重连；
  2. 重连成功后，触发 `RegisterOnReconnect` 监听器；
  3. Agent 打印：`[INFO] Xray connection recovered! Starting automatic re-apply of all active users...`；
  4. Agent 将本地 `state.json` 中的全部 `active` 用户自动调用 `AddUser` 重新注入 Xray；
  5. 整个自愈过程在 3~5 秒内自动完成，无需控制面参与。

---

### TC-07: 版本冲突与乱序防御 (Revision CAS)
- **测试目的**：验证并发网络乱序下，旧的 applied 报告不会覆盖最新的期望。
- **执行步骤**：
  1. 手动构造一个过时的上报请求（`revision="100"`，小于当前数据库最新 revision）：
     ```bash
     curl -s -X POST http://127.0.0.1:8080/api/agent/v1/deployments/report \
       -H "X-Agent-Token: agt_live_9f8a7b6c5d4e3f2a1b0c" \
       -H "Content-Type: application/json" \
       -d '{
         "items": [
           {
             "credential_id": "'"${CRED_ID}"'",
             "endpoint_id": 2,
             "applied_state": "active",
             "revision": "100",
             "applied_at": "2026-11-10T04:00:00Z",
             "xray_user_key": "c_test@line-hk-01"
           }
         ]
       }' | jq .
     ```
- **预期结果**：
  1. 接口返回 HTTP 200，但 `accepted: 0`；
  2. `conflicts` 列表中出现该条目，原因标注为 `"reason": "stale_revision"`；
  3. Noder 数据库中的状态与 revision 保持不变，拒绝被旧版本篡改。

---

### TC-08: 流量统计与计费幂等去重
- **测试目的**：验证流量数据正常累加，以及网络超时重试时依靠 `report_id` 防止重复计费。
- **执行步骤**：
  1. 记录当前已用流量基准值：
     ```bash
     BEFORE=$(sqlite3 /home/ryu/code/noder/data.db "SELECT traffic_used_bytes FROM access_grants WHERE id='${SUB_ID}';")
     ```
  2. 模拟上报 10MB 流量（生成固定 `report_id`）：
     ```bash
     REP_ID="rep_e2e_test_0001"
     curl -s -X POST http://127.0.0.1:8080/api/agent/v1/traffic/report \
       -H "X-Agent-Token: agt_live_9f8a7b6c5d4e3f2a1b0c" \
       -H "Content-Type: application/json" \
       -d '{
         "report_id": "'"${REP_ID}"'",
         "endpoint_id": 2,
         "observed_at": "2026-11-10T04:20:00Z",
         "items": [
           {
             "credential_id": "'"${CRED_ID}"'",
             "grant_id": "'"${SUB_ID}"'",
             "uplink_bytes": 5242880,
             "downlink_bytes": 5242880
           }
         ]
       }' | jq .
     ```
  3. 核验流量增加了 10485760 字节（10MB）；
  4. **重试相同的上报请求（模拟网络重发）**：重复执行第 2 步的 curl 命令；
  5. 再次查询流量：
     ```bash
     AFTER=$(sqlite3 /home/ryu/code/noder/data.db "SELECT traffic_used_bytes FROM access_grants WHERE id='${SUB_ID}';")
     ```
- **预期结果**：
  1. 第一次请求返回 `accepted: true`，流量正确累加 10MB；
  2. 第二次重复请求依然返回成功 `accepted: true`；
  3. `AFTER` 的值与第一次累加后完全一致，**绝不发生二次计费**。

---

### TC-09: 权限边界隔离 (越权访问拦截)
- **测试目的**：验证 Agent 仅能访问已绑定的端点，不能跨端点探测。
- **执行步骤**：
  1. 尝试以 `ag_hk_01` 的 Token 拉取未绑定的端点（例如 `endpoint_id: 3`）：
     ```bash
     curl -i -X GET "http://127.0.0.1:8080/api/agent/v1/deployments?endpoint_id=3" \
       -H "X-Agent-Token: agt_live_9f8a7b6c5d4e3f2a1b0c"
     ```
- **预期结果**：
  1. 响应返回 HTTP `403 Forbidden`；
  2. 错误码为 `"error": "ERR_FORBIDDEN"`，不返回任何数据项。

---

## 5. 验收通过标准判定表 (Sign-off Checklist)

| 验收项 | 验收标准 | 验证手段 | 结果 |
|---|---|---|---|
| **鉴权保护** | 非法 Token 100% 拦截，越权端点 403 阻断 | 执行 TC-01 & TC-09 | [ ] 通过 |
| **就绪感知** | 心跳中断 >90s 门禁告警，心跳恢复实时在线 | 执行 TC-02 | [ ] 通过 |
| **开通收敛** | 订阅释放后 5 秒内完成 Xray 数据面注入 | 执行 TC-03 | [ ] 通过 |
| **撤销阻断** | 订阅撤销后 5 秒内从 Xray 清除通行权 | 执行 TC-05 | [ ] 通过 |
| **崩溃自愈** | Xray-core 重启后 5 秒内全量恢复用户 | 执行 TC-06 | [ ] 通过 |
| **数据幂等** | 流量重复上报绝对去重，旧版本拒绝回滚 | 执行 TC-07 & TC-08 | [ ] 通过 |
| **可观测性** | 每次调用均带唯一 `req_id`，冲突与错误清晰可查 | 查看 Noder 日志输出 | [ ] 通过 |
