# 遗留数据审计与迁移

## 迁移映射

```text
旧 Node
    → 新 Endpoint

旧 Endpoint.UUID / Password / Username
    → 新 Credential

旧 Endpoint.UserID + NodeID
    → 新 AccessGrant

旧 Endpoint.Tag / Country / SortOrder / IsURLTest / Remark
    → 新 AccessGrant 的用户侧字段

旧 User.UUID / Password
    → 仅当该 User 没有有效旧 Endpoint 凭证时，生成默认 Credential
```

旧 Endpoint 不是直接迁移为新 Node，而是拆分为 Credential 和 AccessGrant。

## Credential 合并规则

同一 User 下：

```text
相同 scheme + 相同认证数据 → 复用一个 Credential
认证数据不同             → 创建不同 Credential
```

示例：

```text
Endpoint A: UUID=aaa, Password=111
Endpoint B: UUID=aaa, Password=111
→ 一个 Credential，两个 AccessGrant

Endpoint C: UUID=aaa, Password=222
→ 另一个 Credential
```

旧 Endpoint 的凭证优先于 User.UUID / User.Password。User 级凭证只作为没有旧 Endpoint 凭证时的回退来源。

## 待绑定数据

旧 Endpoint 中 `NodeID IS NULL` 的记录迁移为：

```text
Credential.status = 'pending_binding'
```

不创建 `endpoint_id IS NULL` 的 AccessGrant。

旧 Endpoint.Tag 迁移为 Credential.display_name；绑定完成后由 AccessGrant 提供用户侧展示名称。

## UserNodeLink 审计

迁移前必须执行只读审计：

1. 统计 UserNodeLink 总数；
2. 统计 Endpoint 总数；
3. 找出只有 Link 没有 Endpoint 的关系；
4. 找出只有 Endpoint 没有 Link 的关系；
5. 找出引用不存在 Node 的孤儿 Endpoint；
6. 找出同一 User + Node 的重复 Endpoint；
7. 对比 User、Endpoint 的 UUID、Username、Password；
8. 记录所有迁移冲突，不静默覆盖。

迁移完成后，UserNodeLink 不再作为权威关系，由 AccessGrant 取代。

## PostgreSQL 导入

旧 SQLite 使用整数 ID，目标 PostgreSQL 使用 UUID。迁移脚本必须保留：

```text
legacy_user_id
legacy_node_id
legacy_endpoint_id
```

作为迁移日志或临时映射，确保关系可追溯。迁移必须支持：

```text
备份 → 预览报告 → 导入 → 一致性检查 → 提交
```
