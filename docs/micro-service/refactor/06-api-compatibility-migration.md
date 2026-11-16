# API 迁移原则

## 调用方

```text
商城服务 → 创建 User、创建 Credential
运维端   → 创建 Endpoint、绑定 AccessGrant、管理 AgentNode
客户端   → 使用 User.subscription_token 获取 Node
Agent    → 获取控制面期望状态并回报部署结果
```

## 资源生命周期 API 语义

```text
创建 User
    ↓
创建 Credential（可处于 pending_binding）
    ↓
创建 Endpoint
    ↓
绑定 Credential + Endpoint，创建 AccessGrant
    ↓
订阅接口动态解析 Node
```

## 兼容原则

1. 旧订阅地址和 `subscription_token` 保持有效；
2. 旧 `/sub`、`/xray`、`/node` 继续返回兼容格式；
3. 新管理 API 使用 User、Credential、Endpoint、AccessGrant 的明确资源边界；
4. 旧的 `uuid`、`password` 聚合字段在兼容层动态映射到默认 Credential；
5. API 不允许客户端直接修改 Endpoint 的协议字段；
6. 管理 API 的 AccessGrant 不接收或返回重复保存的 Credential 内容；订阅导出 Node 时按协议要求输出凭证；
7. 已撤销或过期 Credential 不得通过订阅导出；
8. 没有 Agent 的撤销操作必须产生运维告警。

## 幂等要求

商城接口必须使用 `external_id` 或幂等键避免重复创建 User、Credential。

绑定接口必须保证：

```text
同一 User + Endpoint 只有一个有效 AccessGrant
同一 AccessGrant 不得引用其他 User 的 Credential
```

## Agent API

Agent API 只处理部署状态，不改变控制面归属：

```text
获取某 AgentNode 的期望 CredentialDeployment
提交 applied_state、last_sync_at、last_error
```
