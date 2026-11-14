# Noder 免 Agent 节点 (Agentless Endpoint) 放行与订阅契约对齐规范

> 状态：提案 / 待实现  
> 对齐对象：Mall 履约系统 与 Noder 节点控制面  
> 核心目标：支持免 Agent / 手动维护落地节点的 VLESS/Trojan 等代理格式正常放行（Release），并输出合规客户端订阅链接。

---

## 1. 业务背景与问题定义

1. **客户端不接触裸参数**：
   在落点网络 LosDot 体系中，C 端买家仅通过导入 Noder 生成的订阅链接（`/singbox`、`/mihomo`、`/xray`）使用服务，**买家不接触也无法手工填写 VLESS/Reality 裸参数**。
2. **人工/免 Agent 节点的存在合理性**：
   部分专线、独享节点或高防落地机采用人工配置静态规则（如预配置多租户或固定回源），落地机不部署、也不需要常驻运行 `xray-agent` 自动热同步进程。
3. **当前 Noder 阻断点**：
   当前 Noder 在 `POST /api/service/subscriptions/{id}/release` 时对所有非 SOCKS5 节点执行两项强门禁：
   * 门禁 A（绑定校验）：`endpoint_agent_bindings` 必须有 active 记录，否则直接返回 `40902 BINDING_MISSING`。
   * 门禁 B（心跳校验）：绑定的 Agent 必须在近 90 秒内有心跳（`isAgentOnlineForEndpoint`），否则即使放行了也返回 `delivered: false`。
   * **影响**：Mall 收到 `delivered: false` 后，按安全防御契约阻断交付并报错 `40902 Noder 未确认订阅已生效`，导致人工维护的节点无法完成自动化交付闭环。

---

## 2. Noder 侧改造规格

### 2.1 数据模型扩展 (`model.Node` / `node` 表)

在 `node` 模型及数据表中新增 `is_agentless` 字段：

```go
type Node struct {
    // ... 既有字段 ...
    IsAgentless bool `bun:"is_agentless,notnull,default:false" json:"is_agentless"`
}
```

* **默认值**：`false`（默认依然走现有的 Agent 动态同步架构，保证 100% 向后兼容）。
* **数据库迁移**：
  在 `InitDB()` 或迁移逻辑中补充对 SQLite / Postgres 的平滑升级：
  ```sql
  ALTER TABLE node ADD COLUMN is_agentless BOOLEAN NOT NULL DEFAULT false;
  ```

### 2.2 放行接口逻辑优化 (`ReleaseSubscription`)

在 `internal/handler/service_subscription.go` 的 `ReleaseSubscription` 方法中适配 `is_agentless` 逻辑：

```go
// 1. 判断是否无需 Agent
isAgentless := isSocks || ep.IsAgentless

// 2. 绑定检查放行
if !isAgentless {
    bindingCount, _ := db.DB.NewSelect().Model((*model.EndpointAgentBinding)(nil)).
        Where("endpoint_id = ? AND status = ?", grant.EndpointID, "active").Count(r.Context())
    if bindingCount == 0 {
        RespondServiceCodedError(w, http.StatusConflict, 40902, "BINDING_MISSING",
            fmt.Sprintf("endpoint %d has no active agent binding", grant.EndpointID))
        return
    }
}

// 3. 交付状态与下发判断
var delivered bool
if isAgentless {
    // 免 Agent 节点：只要端点本身激活，即判定为交付成功
    delivered = endpointActive
} else {
    n := createDesiredDeployments(r.Context(), grant.CredentialID, grant.EndpointID, model.CredentialStatusActive)
    delivered = n > 0 && isAgentOnlineForEndpoint(r.Context(), grant.EndpointID) && endpointActive
}
```

### 2.3 查询接口对齐 (`GET /api/service/endpoints`)

在端点列表响应中，补充暴露 `is_agentless` 状态，便于 Mall 管理后台感知端点类型：

```json
{
  "id": 2,
  "tag": "vless-02",
  "node_name": "VLESS REALITY 专线 02",
  "protocol": "vless",
  "is_active": true,
  "is_agentless": true,
  "binding_count": 0,
  "bound_agents": []
}
```

---

## 3. 订阅渲染验证要求

确认在 `is_agentless = true` 场景下，买家请求订阅链接时：
1. `GET /singbox?token=<user_token>`
2. `GET /mihomo?token=<user_token>`
3. `GET /xray?token=<user_token>`

能够正常读取 `access_grant`（`status=active`）并拼接该 `Node` 的全部连接配置（包含分配给买家的 UUID / Password），格式与普通 Agent 节点完全一致。

---

## 4. 验收测试用例 (Test Cases)

1. **`TestRelease_AgentlessEndpoint_Success`**：
   * 构造一个 `is_agentless = true` 的 VLESS 端点，无任何 Agent 绑定（`binding_count = 0`）；
   * 创建一条待激活订阅（`pending_activation`）；
   * 调用 `POST /api/service/subscriptions/{id}/release`；
   * 断言：
     * HTTP 状态码为 `200`；
     * 返回数据中 `status == "active"`；
     * 返回数据中 `delivered == true`；
     * 数据库中 `grant.status == "active"`，`credential.status == "active"`。
2. **`TestSubscriptionRender_Agentless`**：
   * 验证拉取 `/sub`、`/singbox`、`/mihomo` 能够成功渲染该节点配置。
