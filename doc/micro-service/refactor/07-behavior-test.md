# 行为锁定测试

行为锁定测试用于保证重构前后的外部业务行为一致，不测试具体表结构。

## User 与订阅

```text
有效 User + 有效 Credential + 有效 AccessGrant + 有效 Endpoint
→ 订阅包含一个完整 Node

User disabled
→ 订阅不包含任何 Node

Credential pending_binding
→ 订阅不包含 Node

Credential revoked 或已过期
→ 关联 AccessGrant 不再导出

Endpoint disabled
→ 关联 Node 不再导出

AccessGrant revoked
→ 只移除该 User 对该 Endpoint 的访问
```

## 凭证复用

```text
同一 User 的一个 Credential 绑定多个 Endpoint
→ 订阅返回多个 Node，使用同一 Credential

不同 User 尝试引用同一 Credential
→ 数据库或服务层拒绝

同一 User 的不同 Endpoint 使用不同 Credential
→ 每个 Node 使用各自 Credential
```

## 端点绑定

```text
Credential 无 Endpoint
→ 可以保存为 pending_binding

绑定 Endpoint 后
→ 创建 AccessGrant
→ Credential 变为 active

同一 User + Endpoint 重复绑定
→ 幂等，不产生第二条有效 AccessGrant
```

## Agent

```text
Endpoint 无 AgentNode 绑定
→ 允许手动部署，不自动同步

Credential 撤销且 Endpoint 有 AgentNode
→ 生成 CredentialDeployment 的撤销期望状态

Credential 撤销且 Endpoint 无 AgentNode
→ 不假装已完成撤销，产生管理员告警

Agent 上报同步失败
→ 保留 last_error，控制面状态仍以 Credential 为准
```

## 遗留数据迁移

```text
旧 Endpoint 凭证与 User 凭证不一致
→ 以旧 Endpoint 凭证为准

多个旧 Endpoint 使用相同认证数据
→ 复用一个 Credential

旧 Endpoint.NodeID 为空
→ 迁移为 pending Credential，不创建空 AccessGrant

UserNodeLink 与 Endpoint 不一致
→ 迁移报告记录冲突，不静默覆盖
```
