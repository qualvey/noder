# 字段唯一权威来源

同一字段只能有一个权威来源。导出对象 Node 不保存为权威数据。

| 字段或字段组 | 权威归属 |
|---|---|
| 用户名称、外部系统 ID、订阅 Token、用户状态 | User |
| 用户级套餐、配额、全局配置 | UserPolicy（或 User 的用户级策略字段） |
| UUID、Username、Password、认证方案 | Credential |
| Credential 状态、过期时间、撤销时间 | Credential |
| 协议、实际地址、端口、默认名称、默认 Tag | Endpoint |
| TLS、Reality、传输层及其他协议参数 | Endpoint.config JSONB |
| 用户是否拥有 Endpoint | AccessGrant |
| 用户侧显示名称、Tag 覆盖、排序 | AccessGrant |
| URL Test、用户级备注、授权有效期 | AccessGrant |
| 用户对 Endpoint 的有限协议覆盖 | AccessGrant.override_config JSONB |
| VPS、母鸡、Agent 地址和心跳 | AgentNode |
| Endpoint 是否由 Agent 控制 | EndpointAgentBinding |
| 凭证是否已同步到 Agent、同步错误 | CredentialDeployment |
| 完整客户端节点 | Node（动态解析对象，不入库） |

## 禁止重复

以下字段不得同时存在于 User、Endpoint、AccessGrant 的多个位置：

```text
UUID
Password
Username
```

AccessGrant 只引用 Credential，不复制凭证内容。

## 协议配置规则

Endpoint.config 和 AccessGrant.override_config 使用 JSONB 存储，但不允许任意字段写入。
写入必须经过对应协议契约的强类型解析、必填字段、枚举值、依赖和冲突校验。
