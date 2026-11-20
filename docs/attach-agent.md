# 对接 Agent：线路机控制面设计定稿

> 本文档用于noder和agent之间api，和mall解耦，负责mall的agent请跳过
> 状态：定稿 v5
> 日期：2026-11-05
> 涉及仓库：`Noder`（`/home/ryu/code/noder`）、新建 `noder-agent`
> 相关文档：[Mall ↔ Noder 交付设计](./attach-noder.md)、[流量计费与配额](./traffic-billing.md)、[Xray 流量统计实验记录](./xray-stats-experiment.md)、[代理节点自动交付与路由](../architecture/proxy_delivery_automation.md)、`Noder/doc/master_agent_architecture_plan.md`

**修订记录**

- v1：Agent 覆盖范围、通信协议、期望状态模型、冷热分离、故障降级。
- v2：修正落地机覆盖范围——纯隧道 landing 不装，socks 落地机也不装（因 `socks` 已定稿不计量、无限流量）；新增 `usage_report` / `quota_update` 消息与 `credential_usage` 表；Agent 职责从“只下发配置”扩展为“下发配置 + landing 计量 + 配额执行”。
- v3：T0 计量可行性实测通过（[实验记录](./xray-stats-experiment.md)）；新增生产 Xray 配置必含项（API dokodemo-door / policy.levels）与 freedom 同机回环黑洞禁忌。
- v4：D10（权益粒度 = 订单项）落实：明确同 endpoint 多 credential 合法、Agent 无需特殊处理、统计/配额按 credential 隔离。
- v5（2026-11-05）：UDP per-user 计量实测通过，T0 遗留项关闭。

本文定义 Agent 的职责边界、通信协议、期望状态模型与故障降级策略。它与 [attach-noder.md](./attach-noder.md) 成对使用：那份定义“卖什么、怎么开通”，这份定义“怎么真正落到机器上”。计量口径与配额模型见 [流量计费与配额](./traffic-billing.md)。

---

## 1. 结论摘要

- Agent 只装在线路机；落地机不装。
- 流量计量（Xray `StatsService` + 本地累计 + 上报）**只针对 `landing`**；`socks` 不计量、按无限流量售卖，落地机无需装任何 Agent。
- 通信方向是 **Agent 主动连 Noder**，不是 Noder 拨号调用 Agent。
- 控制模型是 **期望状态收敛（reconcile）**，不是命令式 RPC。
- 用户增删走 Xray `AlterInbound` 热更新，0 重启、0 断流；路由变更走低频 reload。
- 数据面**不以 3x-ui 为集成点**。
- Noder 不做多实例；单实例风险由 Agent 的 fail-static 兜底。

---

## 2. 现状核实

| 项 | 状态 |
| --- | --- |
| `agent_nodes` / `endpoint_agent_bindings` / `credential_deployments` 表 | 已建（`internal/model/models.go:140-170`） |
| Agent 相关 handler / 路由 | **不存在**（`internal/handler/` 内无任何 agent 引用） |
| `/api/agent/*` 端点 | 不存在 |
| 把凭证写进真实节点的代码 | **不存在** |
| `credential_deployments` 的写入点 | 仅 `RevokeCredential`（`internal/db/domain.go:310`） |

结论：**Noder 目前能渲染订阅，但从未把用户真正写进任何一台线路机。** 现在可用的原因，是运营在下单后手工登录机器加用户。

所以缺的不是一个客户端二进制，而是整条下发闭环：

```
期望状态生成 → 传输 → 应用 → 回报 → 就绪判定
```

`master_agent_architecture_plan.md` 里已有方向性设计，但未实现，且其“全量推送用户列表”的做法需要修正（见第 7 节）。

---

## 3. 通信方向：Agent 主动连 Noder

### 决策 A1：Agent 主动建连，Noder 通过已建连接下发

| 方案 | 结论 | 原因 |
| --- | --- | --- |
| Noder 主动 HTTP/SSH 调 Agent | 否决 | 落地机在 NAT/住宅宽带后无公网入站；线路机若逐台暴露管理端口，攻击面随机器数线性增长，还要维护地址簿 |
| **Agent 主动连 Noder（采用）** | 采用 | Noder 只开一个入口；节点侧只放行出站；断线自动重连；新增节点零网络配置 |

注意语义：**决策权在 Noder**。Agent 不判断给谁开、何时到期，只把本地状态收敛到 Noder 给出的目标状态。

```
Noder:  desired_state(agent, endpoint) = {用户集合(含配额), 路由集合, generation=N}
Agent:  应用 → 上报 applied_generation=N + applied_hash
Noder:  desired == applied ? 就绪 : 标记 drift 并重发
```

Agent 在应用配置之外，还持续把本地观测（探测结果、用量）反向上报。**写方向由 Noder 决定，读方向由 Agent 主动上报**，两者共用同一条连接。

好处是天然幂等：重启 Agent、网络抖动、Noder 重发，都不会产生重复用户或孤儿配置。

---

## 4. Agent 覆盖范围

| 机器 | Agent | 原因 |
| --- | --- | --- |
| **线路机（transit）** | **必须** | 唯一需要热加/热删用户凭证的地方；同时负责 `landing` 的出口探测、按 `exit_route_tag` 的流量计量与配额硬停 |
| socks 落地机 | **不需要** | **socks 不做流量计量**（无限流量），且不经过线路机；保留 GOST 的到期过滤即可（见 attach-noder P0） |
| landing 落地机（纯隧道出口） | 不需要 | 不认识用户，只接受线路机隧道；静态出站配置一次即可 |

**Agent 的部署面**：只在线路机上部署一个 `transit` Agent。socks 与纯隧道落地机保持“哑设备”。

> 这条与第 4 节“v1 曾写落地机一律不装 Agent”的修正正好吻合：**现在更坚决——socks 也不装，因为不计量不需要装。** 若将来 socks 要做独立计量，再改这一行的决策。

落地机的健康由线路机 Agent 探测（第 7.2 节的 `probe_result`），符合“探测方必须能到达被探测方”的常识。这与 Agent 的“计量”职责不冲突：一个是健康探测，一个是流量计量，职责不同。

---

## 5. 数据面选型：3x-ui 还是 Xray + 自有 Agent

### 决策 A2：不走 3x-ui，线路机直接由 Agent 管理 Xray

#### 决定性事实

3x-ui 的工作方式是：**客户端数据存在它自己的数据库里，每次启动、每次修改 inbound，都会从 DB 重新生成 `config.json`。**

由此产生一个不可调和的问题：

- 绕过 3x-ui 直接调 Xray `AlterInbound` 加的用户，3x-ui 面板看不见；
- **3x-ui 一旦重启（升级/崩溃恢复/运维重启），config 被它的 DB 重写，这些用户全部消失。**

也就是说，凌晨三点一次自动重启，全站用户集体掉线，日志里还找不到删除动作。

因此只有两个干净选项，没有中间态：

| 选项 | 数据面权威 | 后果 |
| --- | --- | --- |
| A. 全部走 3x-ui API | 3x-ui | Noder 降级为第三方内部 API 的客户端，双份状态 |
| **B. Agent 直连 Xray（采用）** | Noder | 需要自建 Agent，无现成面板 |

#### 对比

| 维度 | 3x-ui API | 自有 Agent + Xray |
| --- | --- | --- |
| 单一事实源 | ❌ Noder + 3x-ui DB 两套 | ✅ 只有 Noder |
| API 稳定性 | ❌ 内部 API；`settings` 是 JSON 字符串套 JSON；升级可能变 | ✅ 自定义协议，可版本化 |
| 幂等 | ❌ 重复 addClient 报错，需先查再插，存在竞态 | ✅ generation + 期望状态天然幂等 |
| 路由组合 | ⚠️ 全局 `routing.rules` + email 后缀预埋，加出口要改全局配置 | ✅ 规则按出口下发，可程序化 |
| 到期强制 | ✅ 客户端带 `expiryTime` | ✅ 已有 `AccessGrant.ExpiresAt` |
| 流量统计 | ✅ 现成 | ⚠️ 需读 Xray `StatsService`（约 1 天） |
| 运维面板 | ✅ 现成 | ⚠️ 需在 Noder 后台自建 |
| 攻击面 | ❌ 面板是公网 Web 服务，历史上被扫/爆破案例多 | ✅ 仅一条出站 WSS |
| 故障域 | 3x-ui + 其 DB + xray | Agent + xray |
| 上手时间 | 约 1 周 | 约 3–4 周 |

**3x-ui 省下的是流量统计和面板 UI。** 现在流量统计**确实是 P1**（见 [流量计费与配额](./traffic-billing.md)），但 3x-ui 的统计模型与我们的要求并不匹配：它按面板自己的数据库展示总量，无法按 `exit_route_tag` 归集出口成本，也无法把配额下发到 Agent 本地做秒级硬停。自己读 `StatsService` 只需约 1 天，却换来与配额、路由同一份权威状态。运营同样不应该登录 3x-ui 手改客户——那正是 drift 的源头。

#### 如果必须先用 3x-ui 顶一阵

可以，但必须满足三个条件：

1. **所有写入只走 3x-ui API**，绝不与 `AlterInbound` 混用。
2. **Noder 侧定义 `DataPlane` 接口**（`Provision / Revoke / Extend`），3x-ui 只是其中一个实现；换 Agent 时不改业务层。
3. **给出退出时间点**，否则临时方案会长期存在。

同时接受两个约束：运维不再手改 3x-ui；client 的 `email` 由 Noder 生成且不可变，作为映射键。

---

## 6. 出口路由模型

一台线路机对外只开一个 inbound（443）。不同商品（不同出口）复用同一个 inbound，靠 `exit_route_tag` 区分。

```
线路机 endpoint #123（东京，443）
   ├── exit-us-la-01  → 美国 AS15169 原生
   ├── exit-us-la-02  → 美国 AS7922 住宅
   └── exit-uk-lon-01 → 英国 AS2856 原生
```

### 凭证命名约定

```
credential.email = "c_<credential_id>@<exit_route_tag>"
```

路由规则按**后缀**匹配，不按用户匹配：

```json
{
  "type": "field",
  "inboundTag": ["transit-in"],
  "user": ["regexp:@exit-us-la-01$"],
  "outboundTag": "exit-us-la-01"
}
```

**规则数量 = 出口数量，与用户数量无关。** 一万用户、几十个出口，规则仍然只有几十条。

同一 endpoint 上可以挂**任意数量**的 credential：一条 grant 挂一个 credential，粒度 = 订单项（见 [交付设计](./attach-noder.md) 决策 D10）。每个 credential 都是 Xray 里的一个独立用户（email 唯一），统计（`user>>>email>>>traffic`）与配额各自独立；**Agent 不需要为“同 endpoint 多订阅”做任何特殊处理**——多几行用户而已。`/sub` 里同出口的多份订阅靠 `tag_override` 区分展示。

### 决策 A3：email 必须从 credential id 派生且不可变

**为什么**：email 同时是 Xray 的用户标识和路由匹配键。一旦变更（比如改成“用户名”或“订单号”再改回来），该用户的路由会立刻失效，表现为“能连上但出口不对”或直接断流。

`c_<credential_id>@<tag>` 保证唯一、稳定、可反查，且不含 PII。

---

## 7. 通信协议

### 7.1 通道

```
WSS  wss://<noder>/api/agent/ws
Header: Authorization: Bearer <per-agent-token>
```

- **token 放 Header，不放 query**。query 会进入 Nginx access log 和浏览器历史。
- **每台 Agent 独立 token**，复用 `AgentNode.ControlTokenCiphertext`，支持单独吊销。
- 兜底轮询：`GET /api/agent/desired-state?since=<generation>`，WS 断线期间使用。

### 7.2 消息

| 方向 | 类型 | 载荷 |
| --- | --- | --- |
| Agent → Noder | `hello` | agent_id、版本、内核类型与版本、承载的 endpoint 列表、能力位 |
| Agent → Noder | `heartbeat` | cpu/mem、连接数、applied_generation、applied_hash、uptime |
| Noder → Agent | `user_add` | endpoint_id、generation、credential（email/uuid/flow/password/**quota_bytes**） |
| Noder → Agent | `user_remove` | endpoint_id、generation、email |
| Noder → Agent | `route_upsert` | endpoint_id、generation、exit_route_tag、outbound 配置 |
| Noder → Agent | `quota_update` | endpoint_id、generation、credential_id、quota_bytes、used_base_bytes |
| Noder → Agent | `snapshot` | endpoint_id、generation、完整用户集合（含配额）+ 路由集合 |
| Agent → Noder | `apply_ack` | generation、ok、error、applied_hash |
| Agent → Noder | `probe_result` | exit_route_tag、ok、latency_ms、exit_ip |
| Agent → Noder | `usage_report` | seq、按 credential 的绝对累计值、按出口的绝对累计值 |

**当前只有一个 Agent 角色：`transit`**（在线路上路机）。socks 落地机因不做流量计量，**不部署 Agent**；纯隧道落地机也不部署。因此协议层**不引入 `landing-socks` 角色**——若将来 socks 要单独计量，再以 ADR 形式回炉设计（含端口分配器等）。

### 决策 A4：实时增量 + 定期全量，不推全量列表

`master_agent_architecture_plan.md` 原设计是 `USER_SYNC { active_users: [...] }` 全量广播。必须改：

1. **横向泄露**：一台线路机能拿到全站所有用户的 UUID/密码。
2. **O(N) 放大**：上万用户时每次变更推全量。
3. **无法确认**：推完不知道应用成功了没有。

采用：

- **增量事件**（`user_add` / `user_remove` / `route_upsert`）实时下发，正常路径下每次只传一条。
- **全量快照**只在 Agent 启动、重连、或检测到 drift 时拉取。
- 每条消息带单调递增 `generation`，Agent 应用后 `apply_ack`。

### 7.3 幂等

- 所有写操作以 `generation` 为准：Agent 丢弃小于当前 applied 的消息（乱序保护）。
- `user_add` 使用 upsert 语义：相同 email 重复下发视为覆盖，不报错。
- `user_remove` 删除不存在的用户视为成功。

### 7.4 计量消息语义

`usage_report` 上报的是**自 credential 创建以来的绝对累计值**，不是增量：

```json
{
  "type": "usage_report",
  "agent_id": "ag_xxx",
  "seq": 10231,
  "reports": [
    { "credential_id": "cred_1", "used_total_bytes": 123456789, "uplink": 23456789, "downlink": 99999999 }
  ],
  "exits": [
    { "exit_route_tag": "exit-us-la-01", "uplink": 1000, "downlink": 2000 }
  ]
}
```

- Noder 用 `GREATEST(existing, incoming)` 落库，天然幂等，不需要去重表。
- Agent 记录 `last_raw`；当 `raw < last_raw`（Xray 重启导致计数器归零）时按"本次 raw 即增量"处理。
- **单计数器读取返回 `NotFound` 时按 0 处理**：用户产生第一条流量前计数器不存在，属正常状态（实测证实）。
- 采集命令（实测可用）：

```bash
xray api statsquery --server=127.0.0.1:<api端口> -pattern 'user>>>'
xray api stats      --server=127.0.0.1:<api端口> -name "user>>><email>>>>traffic>>>downlink"
```

- `quota_update` 只改额度，不触发路由变更；`quota_bytes = 0` 表示不限。
- **配额判定在 Agent 本地**，Noder 只下发阈值。Noder 不可用时 Agent 仍必须执行硬停。

实测证据（计数精度、累加语义、按用户隔离、重启归零）见 [Xray 流量统计实验记录](./xray-stats-experiment.md) 第 3 节；生产配置必含项见本文 9.4 节。

---

## 8. 状态模型

### 8.1 持久化

现有三张表基本够用，补版本与内核信息：

```sql
ALTER TABLE agent_nodes ADD COLUMN role varchar(32) NOT NULL DEFAULT 'transit';  -- 当前仅单角色：transit（线路机）
ALTER TABLE agent_nodes ADD COLUMN kernel varchar(32);                 -- xray | singbox
ALTER TABLE agent_nodes ADD COLUMN agent_version varchar(32);
ALTER TABLE agent_nodes ADD COLUMN protocol_version int NOT NULL DEFAULT 1;
ALTER TABLE agent_nodes ADD COLUMN applied_generation bigint NOT NULL DEFAULT 0;
ALTER TABLE agent_nodes ADD COLUMN applied_hash varchar(64);

ALTER TABLE credential_deployments ADD COLUMN generation bigint NOT NULL DEFAULT 0;
ALTER TABLE credential_deployments ADD COLUMN last_applied_at timestamptz;

-- 每个 (agent, endpoint) 的期望版本
CREATE TABLE agent_endpoint_states (
  agent_node_id bigint NOT NULL,
  endpoint_id   bigint NOT NULL,
  generation    bigint NOT NULL DEFAULT 0,
  config_hash   varchar(64),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (agent_node_id, endpoint_id)
);

-- 按凭证的用量（Agent 上报的绝对累计值）
CREATE TABLE credential_usage (
  credential_id     varchar(64) PRIMARY KEY,
  agent_node_id     bigint      NOT NULL,
  used_total_bytes  bigint      NOT NULL DEFAULT 0,
  uplink_total      bigint      NOT NULL DEFAULT 0,
  downlink_total    bigint      NOT NULL DEFAULT 0,
  seq               bigint      NOT NULL DEFAULT 0,
  reported_at       timestamptz NOT NULL DEFAULT now()
);

-- 按出口的日流量（成本观测）
CREATE TABLE exit_traffic_daily (
  agent_node_id  bigint      NOT NULL,
  exit_route_tag varchar(64) NOT NULL,
  day            date        NOT NULL,
  uplink_bytes   bigint      NOT NULL DEFAULT 0,
  downlink_bytes bigint      NOT NULL DEFAULT 0,
  PRIMARY KEY (agent_node_id, exit_route_tag, day)
);
```

### 8.2 Agent 状态机

```
enrolled → online → applying → synced
              ↑        │
              └─ error ┘   (上报 last_error，等待重发)
```

`online` 的判定：`last_heartbeat_at > now() - 90s`。

### 8.3 就绪判定（供 attach-noder 的 activate 使用）

```
ready = endpoint.is_active
     && agent.status = 'online'
     && agent_endpoint_states.generation = agent.applied_generation
     && (format = 'landing' ? probe[exit_route_tag].ok
                            : gost 名单已包含该账密)
```

- 线路机分支要求 Agent（`agent.role = 'transit'`）在线；socks 分支的“凭证在线”由 GOST 名单加载判定。
- 流量配额只对 `landing` 生效，socks 不参与就绪判定。

这把“已交付”从“我发了个请求”变成“对方确认应用了，且链路探测通过”。

---

## 9. 配置应用：冷热分离

### 9.1 高频：用户增删 → AlterInbound 热更新

Xray 的 `HandlerService` 支持 `AddUserOperation` / `RemoveUserOperation`，通过本地 API 端口（`127.0.0.1:8900`）直接操作运行中的 inbound：

```
xray api adu --server=127.0.0.1:8900 transit-in '<client-json>'
xray api rmu --server=127.0.0.1:8900 transit-in 'c_xxx@exit-us-la-01'
```

效果：**0 重启、0 断流**，其他在线用户完全无感。

这是每单都会走的高频路径，必须用这条。

### 9.2 低频：路由变更 → 受控 reload

新增出口需要同时加 outbound 和 routing rule。Xray 运行时是否支持动态增删路由规则**取决于版本**，不要假设；实施前先在目标版本上验证。

- 若支持 `RoutingService` 动态加规则：Agent 直接调用。
- 若不支持：改写配置 + `xray run -test` 校验 + reload。

**关键约束：路由变更只在“上新出口”时发生，不在每单发生。** 因此可以安排低峰期执行，并提前通知。这也是为什么第 6 节的“规则按出口”如此重要——它把高频操作和低频操作彻底分开。

### 9.3 原子写与回滚

Agent 修改配置文件时必须：

1. 写临时文件；
2. `xray run -test -config <tmp>` 校验；
3. 校验通过才原子替换（`rename`）；
4. reload；
5. reload 失败则恢复上一份并上报 `apply_ack{ok:false}`。

**没有第 2 步的 Agent 不能上线。** 一次坏配置会让整台线路机所有用户断流，而线路机通常承载几百到几千人。

### 9.4 生产 Xray 配置必含项（T0 实测确认）

Agent 首次纳管一台线路机时，生成的 Xray 配置**必须**包含以下三块，缺一计量链路就不工作（实测见 [实验记录](./xray-stats-experiment.md) 第 2.2 节）：

```json
{
  "api": {"tag": "api", "services": ["HandlerService", "StatsService"]},
  "stats": {},
  "policy": {"levels": {"0": {"statsUserUplink": true, "statsUserDownlink": true}}}
}
```

外加两个容易踩的坑：

1. **API 是一个真正的 inbound**：`dokodemo-door` + `tag` 与 `api.tag` 一致 + 路由规则 `inboundTag:[api] → outboundTag:api`。只写 `api.services` 不会监听任何端口，`xray api` 会 dial 失败。
2. **`policy.levels` 与 `stats` 缺一不可**：只开 `stats` 不开 levels，计数器恒为 0 且不报错。

**禁忌：禁止同机混部。** Xray 26.x 的 Freedom 出站对"目标为本机 IP"的连接直接黑洞（实测日志：`blocked target: tcp:192.168.1.2:xxxx, blackholing connection for 1m22s`）。生产链路 transit → 落地机是跨机器，不受影响；但**不得把线路机 Xray 与落地机出口服务部署在同一台机器**，否则连接 accepted 却无数据回流，排查极其困难。

---

## 10. 首次纳管流程

```
1. Admin 在 Noder 创建 AgentNode → 一次性 enroll token（TTL 15 分钟，只展示一次）
2. 目标机器执行  noder-agent enroll --master wss://... --token <one-time>
3. Agent 换取长期 token，写入本地配置（0600），立即上报 hello
4. Admin 把 endpoint 绑定到 agent（endpoint_agent_bindings）
5. Noder 生成该 endpoint 的期望状态（snapshot）并下发
6. Agent 应用 → apply_ack → Noder 标记 synced
7. Noder 标记 endpoint ready → 允许 Mall 交付
```

### 决策 A5：enroll token 一次性，长期 token 单独签发

**为什么**：安装命令会进入 shell history、运维文档、聊天记录。一次性 token 即使泄露，15 分钟后失效，且只能换取一次长期 token；长期 token 存密文，可单独吊销。

第 7 步之前，endpoint 必须保持“未就绪”，Mall 不允许售卖。否则会出现“卖了但机器上没人”。

---

## 11. 故障与降级

| 故障 | 行为 | 理由 |
| --- | --- | --- |
| Noder 不可用 | Agent 保持最后一份配置继续服务（fail-static） | 控制面故障不能等于全站断网 |
| Agent 不可用 | endpoint 标记 `offline`，Mall 停止新交付 | 已交付用户的配置仍在机器上，继续可用 |
| 配置应用失败 | 回滚上一版，上报 `last_error` | 一次坏配置不能全节点断流 |
| 协议版本不兼容 | Agent 拒绝不认识的指令并上报 | 灰度升级 |
| 单 Agent 故障 | 只影响其承载的 endpoint | 故障隔离 |
| Noder 重启 | Agent 指数退避 + 随机抖动重连 | 避免几百台同时重连把 Noder 打崩 |
| 出站目标是本机 IP | Xray 26.x Freedom 直接黑洞连接（实测） | 防回环保护；**禁止同机混部线路机与落地机**（见 9.4） |
| Xray 重启 | 计数器归零（实测），上报中断数分钟 | Agent 用 `raw < last_raw` 分支处理，Noder 侧用量曲线出现台阶属正常 |

**fail-static 是硬要求。** 很多自建面板在主控重启时触发全量 reload 失败，导致全网断流。

---

## 12. 多实例问题

### 决策 A6：本期不做 Noder 多实例

**为什么**：多实例需要解决 WS 长连接的跨实例路由（共享 pub/sub 或 sticky）、generation 全局一致、Agent 重连归属等问题，成本显著。

而单实例的风险已被 fail-static 兜住：Noder 挂掉时，已交付用户不受影响，只有“新交付”暂停。这个降级是可接受的。

**触发重新评估的条件**（满足任意一条再考虑）：

- 新交付可用性要求 < 5 分钟；
- 单实例承载的 Agent 数超过 200；
- Noder 需要按地域部署。

在此之前，只做一件事：Agent 重连退避 + 抖动。

---

## 13. 安全

| 项 | 做法 | 理由 |
| --- | --- | --- |
| Agent 身份 | 每台独立 token，密文存储，可单独吊销 | 一台泄露不影响其他 |
| 传输 | 强制 TLS，校验证书 | 防中间人窃取用户凭证 |
| 配置完整性 | Noder 对下发内容做 HMAC 签名，Agent 校验后应用 | 防伪造下发 |
| 写入范围 | Agent 只允许写白名单路径 | 避免被当成任意文件写入器 |
| 上报内容 | 只上报统计与 hash，不上报用户密码 | 减少二次泄露面 |
| 运行权限 | 独立系统用户 + systemd 加固 | 不跑 root |
| 面板 | 线路机不暴露 3x-ui / 管理端口 | 缩小攻击面 |

---

## 14. 可观测性

Agent 上报：

- `cpu` / `mem` / `load`
- `xray_uptime`、`inbound_connection_count`
- `applied_generation`、`applied_hash`、`user_count`
- 每个 `exit_route_tag` 的 `probe_result`（ok、延迟、出口 IP）
- 每个 credential 的绝对累计用量；超量硬停事件（`credential_id`、`quota_bytes`、`used_total_bytes`）

Noder 后台展示：

- Agent 列表：在线状态、**角色（transit）**、版本、最后心跳、承载 endpoint
- 每个 endpoint：desired vs applied generation、drift 标记
- 每个出口：探测状态与最近一次失败原因、日流量与单位成本
- 每个订阅：额度、已用、剩余、最近上报时间

告警：

- Agent 离线 > 3 分钟
- drift 持续 > 10 分钟
- 出口探测连续失败 > 3 次
- 配置应用失败
- **用量上报中断 > 5 分钟（计量失联 = 无法限流）**
- **单用户日流量 > 套餐额度 3 倍（疑似共享/转售）**

---

## 15. 实施顺序

| 阶段 | 内容 | 风险 |
| --- | --- | --- |
| **A0** | 协议定稿 + `agent_endpoint_states` / `credential_usage` 建表 + generation 生成器 | 无 |
| **A1** | **只读 Agent**：enroll、hello、heartbeat、上报 applied | 极低（不下发） |
| **A2** | 下发 `user_remove` / 停用路径 | 低（先做减法） |
| **A3** | 下发 `user_add` + `AlterInbound` 热加 | 中 |
| **A4** | 出口探测上报 + 接入 `activate` 就绪判定 | 中 |
| **A5** | 计量与配额：`StatsService` 采集、本地累计、`usage_report`、`quota_update`、本地硬停 | 中 |
| **A6** | 配置原子写与回滚、自更新、灰度 | 中高 |

> A6 中提到的 `landing-socks` 已删除——socks 不计量，Agent 只有 `transit` 一个角色。

**强烈建议从 A1 开始**：只上报、不下发。先验证 Agent 在线率、版本分布、心跳稳定性，再动真正的配置。反过来做，第一次下发就可能打挂节点。

**A5 的前置条件已满足**：计量可行性 T0 已于 2026-11-04 在 Xray 26.9.30 上实测通过（[实验记录](./xray-stats-experiment.md)）——per-user 上下行计数精确、累加、按用户隔离、重启归零、单计数器 API 可用。剩余待验证项只有 **UDP 是否计入 per-user 统计**（见 traffic-billing §13）。

---

## 16. 待确认

1. **目标 Xray 版本是否支持运行时增删路由规则？** 实施 A2/A3 前必须在目标版本上验证；不支持则退回受控 reload。（计量侧 API 已实测可用；`RoutingService` 未测。）
2. **Agent 是否需要在离线时缓存期望状态？** 建议不缓存，fail-static 期间只维持现状，恢复后重新拉取。**注意：用量上报不能丢**——离线期间继续本地累计，恢复后补报。
3. **探测目标 URL 与判定标准**：建议自建 check 端点，比对出口 IP 而非依赖第三方服务可用性。
4. **单台线路机的用户上限**：决定是否需要在 Agent 侧做批量合并下发（例如 1 秒内的多个变更合并成一次）；同时需压测万级 `user>>>` 计数器的内存开销。
5. **socks 计量方案**：已定稿**不计量**（无限流量），本项关闭。附实测结论：SOCKS 的 `user` 字段即统计 email，技术上可计量（[实验记录](./xray-stats-experiment.md) 第 3.D 节）。
6. **用量上报缓冲策略**：环形缓冲保留多久？建议 24h，超过则丢弃最旧数据并告警（宁可少计，不可无限占用磁盘）。
7. **UDP 计量覆盖**：已验证通过（[实验记录](./xray-stats-experiment.md) §3.H，2026-11-05）：UDP 与 TCP 一样计入 `user>>>` 计数器，不需要按 inbound 兜底。

---

## 17. 与既有文档的关系

| 文档 | 状态 |
| --- | --- |
| `Noder/doc/master_agent_architecture_plan.md` | **部分被取代**。方向正确，但“全量 `USER_SYNC` 用户列表”和“agent_token 放 Node 表”需按本文修正（改为 scoped 增量 + 独立 `AgentNode` 表）。 |
| `architecture/proxy_delivery_automation.md` | **部分被取代**。其 3x-ui 作为集成点的假设不采用，理由见第 5 节；四层职责边界与“单端口 443”仍有效。 |
| `dev/attach-noder.md` | **配套文档**。本文就绪判定的输出，正是那份 `activate` 的输入。 |
| `dev/traffic-billing.md` | **配套文档**。计量原理、socks 计量决策、配额模型与成本观测。本文的 A5 阶段按其 T1 排期。 |
| `dev/xray-stats-experiment.md` | **实验记录**。T0 计量可行性的原始证据（Xray 26.9.30 实测），本文 7.4 / 9.4 / 11 节结论的数据来源。 |
