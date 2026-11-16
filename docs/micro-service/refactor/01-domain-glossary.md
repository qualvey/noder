# 领域词汇

本文为强约束定义。

## 核心对象

### User（用户）

资源授权主体，负责用户身份、订阅 Token、生命周期和用户级策略。

User 不保存 Endpoint 的协议参数，也不直接保存某个资源关系的排序、标签等字段。

### Credential（凭证）

用户的数据面鉴权身份，例如 VLESS UUID、TUIC UUID + Password、Socks5 Username + Password。

一个 User 可以拥有多个 Credential。一个 Credential 只能归属一个 User。

Credential 可以在多个 Endpoint 上复用，但不能被不同 User 共享。

Credential 可以脱离 Endpoint 独立存在，`pending_binding` 表示尚未绑定资源。

### Endpoint（端点）

不包含用户凭证的、拥有实际连接地址和完整协议参数的协议接入点。

```text
Endpoint = 实际地址 + 端口 + 协议 + 协议参数
```

Endpoint 不是协议模板，也不是用户节点。协议专属参数以结构化 JSONB 存储，并由协议契约严格校验。

### AccessGrant（资源授权）

User 获得某个 Endpoint 访问权的授权关系。

AccessGrant 必须同时引用 User、Endpoint 和 Credential。AccessGrant 只表示已经完成绑定的资源关系，不表示待履约资源。

### Node（完整节点）

Endpoint、Credential 和 AccessGrant 解析后形成的完整客户端节点：

```text
Node = Endpoint + Credential + AccessGrant
```

Node 是动态领域对象，不建立独立持久化表。格式化后交给 Sing-box、Xray、Mihomo 等客户端使用。

### AgentNode（Agent 运行节点）

真实 VPS、母鸡或部署 Agent 的物理运行节点。AgentNode 不是客户端 Node。

当前手动部署的 Endpoint 不绑定 AgentNode。未来通过 EndpointAgentBinding 建立控制关系。

### EndpointAgentBinding（端点 Agent 绑定）

表示某个 AgentNode 负责部署或控制某个 Endpoint。一个 Endpoint 可以绑定多个 AgentNode，以支持迁移和高可用。

### CredentialDeployment（凭证部署状态）

表示 Credential 应同步到某个 AgentNode、并应用于某个 Endpoint 的运行时投影。它不改变 User、Credential、AccessGrant 的权威关系。

## 权威关系

```text
User 1 ── N Credential
User 1 ── N AccessGrant
Credential 1 ── N AccessGrant
Endpoint 1 ── N AccessGrant
Endpoint N ── N AgentNode
```

## 名称迁移

```text
当前 Node       → Endpoint
当前 Endpoint   → Credential + AccessGrant
当前 User       → User
未来物理 Node   → AgentNode
完整客户端节点  → Node（动态对象）
```
