# 分布式 Master 主控 + 边缘 Agent 架构升级计划方案

## 📖 1. 项目愿景与架构定位

本项目 `sub-server` 将升维为 **分布式网络控制面 (Control Plane / Master Controller)**。
通过在真正的 VPS 代理节点上部署轻量级 **`noder-agent`**，主控与节点之间建立加密 WebSocket 长连接通道，实现**用户状态毫秒级下发同步、节点实时断流风控**与**分布式探针监控**。

```text
                               +-----------------------------+
                               |     Master Controller       |
                               | (sub-server 主控控制面)     |
                               +-----------------------------+
                                       /              \
                          WebSocket (TLS)            WebSocket (TLS)
                                     /                  \
                                    v                    v
              +--------------------------+    +--------------------------+
              |   Edge Node Agent 01     |    |   Edge Node Agent 02     |
              | (HK VPS - Sing-Box 进程) |    | (US VPS - Sing-Box 进程) |
              +--------------------------+    +--------------------------+
```

---

## 🚦 2. 用户生命周期三态 (User Lifecycle Three-States)

在 Master 控制台操作用户时，状态实时映射至远程 Agent 节点：

| 用户状态 | 订阅 API (`/sub` / `/node`) | 边缘 Agent 行为 (Sing-Box Inbounds 节点控制) |
| :--- | :--- | :--- |
| **启用 (Active)** | 正常导出节点配置，客户端可一键连接 | 将该用户的专属 `uuid`/`password` 写入节点 Sing-Box `inbounds[].users` 列表中，允许建立代理通道。 |
| **停用 (Inactive)** | 自动从订阅中剔除该节点 | 从 Sing-Box `inbounds[].users` 接入列表中**剥离该用户**，并触发 Sing-Box 软重载（`systemctl reload sing-box`），**立刻断开连接**。 |
| **删除 (Deleted)** | 移除绑定的用户与节点关系 | 从对应节点的 Agent 存根中彻底清除该用户数据。 |

---

## 🗄️ 3. 数据库扩展设计 (Master Database Scheme)

在 Master 的 `Node` 数据库模型中增加分布式 Agent 字段：

```python
class NodeBase(SQLModel):
    # ... 现有节点基础属性 (node_name, protocol, server_address, server_port, security 等) ...

    # 🤖 边缘 Agent 扩展字段
    agent_token: Optional[str] = Field(default=None, index=True)   # Agent 通信秘钥 Token
    agent_status: Optional[str] = Field(default="offline")         # online / offline
    last_heartbeat: Optional[datetime] = None                       # 最后心跳时间戳
    cpu_usage: Optional[float] = None                               # CPU 占用率 %
    memory_usage: Optional[float] = None                            # 内存占用率 %
```

---

## 📡 4. 通信协议与 API 规范 (Master <-> Agent Protocol)

Master 与 Agent 之间建立双向全双工 WebSocket 长连接（支持断线重连与 HTTP 定时轮询兜底）：

### 4.1 WebSocket 端点
- **`WS /api/agent/ws?token={AGENT_TOKEN}`**

### 4.2 下发指令 (Master -> Agent)
当管理员在控制台触发用户【新增/编辑/启用/停用/删除】时，Master 实时推送 `USER_SYNC` 消息：

```json
{
  "event": "USER_SYNC",
  "timestamp": 1722000000,
  "active_users": [
    {
      "user_id": 1,
      "uuid": "ef66463c-4bcb-4b20-bd42-9249758611ba",
      "password": "1bttcp92ies-nfij_qjjfw"
    }
  ]
}
```

### 4.3 心跳与状态回报 (Agent -> Master)
Agent 每 10 秒发送一次心跳包：

```json
{
  "event": "HEARTBEAT",
  "cpu_usage": 12.5,
  "memory_usage": 45.2,
  "singbox_active": true
}
```

---

## 🤖 5. 边缘 Agent 节点设计 (`noder-agent`)

- **语言选型**：纯 Python 脚本或编译型 Go 二进制文件。
- **配置文件** (`/etc/noder-agent/config.json`)：
  ```json
  {
    "master_url": "wss://sub.yourdomain.com",
    "agent_token": "node-agent-secret-token",
    "singbox_config_path": "/etc/sing-box/config.json"
  }
  ```
- **核心动作**：
  1. 接收 `active_users` 列表。
  2. 提取并替换 `/etc/sing-box/config.json` 中 Inbounds 的用户列表。
  3. 执行 `systemctl reload sing-box`（无无缝切断违规用户连接，不影响其他合法用户）。

---

## 🗺️ 6. 实施路线阶段计划 (Roadmap)

- [ ] **阶段 1：Master 端数据库与 API 扩展**
  - 在 `Node` 表中加入 `agent_token` 与心跳字段。
  - 开发 `WS /api/agent/ws` WebSocket 控制管道与 HTTP 轮询兜底 API。
  - 在 Web 管理控制台中展示节点的 Agent 在线/离线状态灯 (🟢 在线 / 🔴 离线)。
- [ ] **阶段 2：Agent 客户端研发与发布**
  - 开发 `noder-agent` 客户端代码与 `install-agent.sh` 节点一键部署脚本。
  - 实现 Sing-Box Inbounds `users` 字典覆盖与软重载逻辑。
- [ ] **阶段 3：全链路联动联调与测试**
  - 测试实时新增/停用/删除用户时，节点断流与拉通效果。
