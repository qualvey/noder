# 状态与生命周期

## 状态字段

状态字段只表示明确的业务或运维意图，过期和健康状态通过独立字段表示。

```text
User:        active / disabled
Credential:  pending_binding / active / disabled / revoked
Endpoint:    active / disabled
AccessGrant: active / disabled / revoked
AgentNode:   online / offline / disabled
```

过期使用：

```text
expires_at
```

不要把 `expired` 同时作为所有实体的持久化状态值。

Endpoint 的 `unavailable` 属于运行健康状态，不属于 Endpoint 生命周期状态，使用健康检查字段或 Agent 上报状态表示。

## Credential 生命周期

```text
pending_binding
    ↓ 运维完成资源绑定
active
    ├── disabled
    ├── expires_at 到期
    └── revoked
```

Credential 可以在没有 AccessGrant 时处于 `pending_binding`。

## 有效访问条件

```text
User.active
&& Credential.active
&& Credential.expires_at 未到期
&& AccessGrant.active
&& AccessGrant.expires_at 未到期
&& Endpoint.active
```

## 撤销行为

Credential 撤销后：

1. 新订阅不再导出相关 Node；
2. 有 Agent 时，将撤销状态同步到相关 AgentNode；
3. 无 Agent 时，通知管理员手动从服务端代理内核撤销；
4. 已下载的本地 JSON 不会自动消失，只有服务端停止接受该 Credential 后才真正失效。

## Agent 运行状态

Agent 只负责将控制面的有效状态部署到服务器，不拥有 User、Credential 或 AccessGrant 的权威归属。

```text
控制面状态 → CredentialDeployment → AgentNode 实际状态
```
