# Noder 商城微服务与配置生成引擎设计方案

> 领域模型规范以 `doc/micro-service/refactor/` 和 `doc/domain_model_er.md` 为准。本文件中的旧 `Node` / `Endpoint` 表结构描述仅作为迁移前业务背景，不作为新模型定义。

## 部署标准

正式部署采用 Docker 化：

- Noder 作为应用容器运行；
- PostgreSQL 作为数据库容器运行；
- PostgreSQL 使用持久化 Volume；
- 配置、数据库连接、密钥通过环境变量或 Docker Secret 注入；
- 数据库迁移、备份和恢复纳入发布流程。

## 一、系统定位与职责边界

本项目（Noder）作为独立微服务运行，主要为外部商城后端及运维流程提供核心能力支撑：
1. **商城端职责（极简）**：仅负责向 Noder 注册用户（获取专属订阅链接）以及添加节点的子凭证（节点名与凭证挂钩）。不感知底层网络协议与物理配置。
2. **运维端职责**：负责服务器开机部署，在 Noder 中完成“子凭证”与“物理入口端点”的履约绑定。
3. **Noder 职责**：
   * 翻墙客户端配置：负责订阅分发（`/sub`、`/xray`）、多平台模板渲染，以及提供无状态配置即席生成能力。
   * **Socks5 独立业务线**：负责多母鸡 IP 反亲和防重调度、零 Agent 中心化鉴权、免模板纯净格式直出。

---

## 二、架构调用流程图

```mermaid
sequenceDiagram
    autonumber
    actor Customer as 终端用户
    participant Mall as 商城后端
    participant Noder as Noder 微服务
    actor Ops as 运维人员
    participant Client as Sing-box / Xray / 指纹浏览器

    Note over Mall,Noder: 1. 商城注册用户
    Mall->>Noder: POST /api/service/users/register (external_id, name)
    Noder-->>Mall: 返回成功及订阅链接 (token, sub_urls)
    Mall-->>Customer: 交付固定订阅链接 (此时无可用节点)

    Note over Mall,Noder: 2. 商城添加子凭证 (通用节点)
    Mall->>Noder: POST /api/service/users/{token}/credentials (node_tag, uuid)
    Noder-->>Mall: 返回成功/失败及原因 (凭证存根落库，状态待绑定)

    Note over Ops,Noder: 3. 运维开机并履约绑定
    Ops->>Ops: 云服务器开机部署内核与网络
    Ops->>Noder: 在管理后台/接口将该凭证与物理入口节点绑定
    Noder->>Noder: 凭证激活生效

    Note over Customer,Client: 4. 客户端订阅刷新
    Customer->>Client: 点击“更新订阅”
    Client->>Noder: GET /sub?token=xxx
    Noder-->>Client: 组装模板与激活节点，下发完整客户端配置
```

---

## 三、通用微服务 API 接口规范

微服务接口统一位于 `/api/service/*` 路由下，使用专属请求头进行鉴权：
`X-Service-Token: <MALL_SERVICE_SECRET>`

### 1. 注册用户 (Register User)
商城在用户下单购买套餐或创建账号时调用。

* **请求方式**：`POST /api/service/users/register`
* **请求体 (JSON)**：
```json
{
  "external_id": "mall_user_10086",   // 商城用户唯一ID（用于去重与幂等反查）
  "name": "user@example.com",          // 用户名/邮箱
  "token": "custom_token_123",         // 可选，不传则系统自动生成高强度 Token
  "remark": "定制独享套餐"             // 可选备注
}
```
* **响应结果 (JSON)**：
  * **成功 (200 OK)**：
  ```json
  {
    "code": 0,
    "message": "success",
    "data": {
      "user_id": 101,
      "token": "custom_token_123",
      "external_id": "mall_user_10086",
      "subscription_urls": {
        "singbox": "https://sub.domain.com/sub?token=custom_token_123",
        "xray": "https://sub.domain.com/xray?token=custom_token_123",
        "raw_nodes": "https://sub.domain.com/node?token=custom_token_123"
      }
    }
  }
  ```
  * **失败 (400 / 500)**：
  ```json
  {
    "code": 400,
    "message": "注册失败：external_id 'mall_user_10086' 已被使用"
  }
  ```

---

### 2. 添加用户子凭证 (Add User Credential - 通用定制节点)
商城为该用户添加一个节点子凭证（节点名字与凭证挂钩）。

* **请求方式**：`POST /api/service/users/{token}/credentials`
* **请求体 (JSON)**：
```json
{
  "node_tag": "HK-Dedicated-01",                      // 节点名字（唯一标识或展示名）
  "uuid": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",    // 专有子凭证 UUID（必填）
  "password": "optional_password",                    // 专有密码（可选，TUIC/AnyTLS 协议使用）
  "remark": "张三定制香港 100M"                        // 凭证备注（可选）
}
```
* **系统处理逻辑**：
  1. 校验目标用户 Token 是否有效；
  2. 录入 `Endpoint` 凭证记录，建立 `user_id` 与 `node_tag + uuid` 映射；
  3. 初始为“待绑定物理机”状态（`node_id = null`），等待运维后续关联。
* **响应结果 (JSON)**：
  * **成功 (200 OK)**：
  ```json
  {
    "code": 0,
    "message": "success"
  }
  ```
  * **失败 (400 / 404 / 500)**：
  ```json
  {
    "code": 400,
    "message": "添加凭证失败：该用户已存在同名节点凭证 'HK-Dedicated-01'"
  }
  ```

---

### 3. 即席无状态配置渲染 (Render Config Stateless)
不依赖数据库状态，外部调用方直接传入一组完整的节点参数 JSON，换取合法的、套用模板后的完整客户端配置文件。

* **请求方式**：`POST /api/service/render-config`
* **请求体 (JSON)**：
```json
{
  "platform": "sing-box",            // 目标平台："sing-box" | "xray"
  "template_id": 1,                  // 可选：指定模版 ID（不传使用默认模板）
  "nodes": [
    {
      "tag": "香港 Reality 01",
      "protocol": "vless",
      "server_address": "hk01.edge.net",
      "server_port": 443,
      "uuid": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
      "security": "reality",
      "sni": "gateway.icloud.com",
      "public_key": "x5vK7a...=",
      "short_id": "0123456789abcdef",
      "fingerprint": "chrome",
      "flow": "xtls-rprx-vision"
    }
  ]
}
```
* **响应结果 (JSON)**：
```json
{
  "code": 0,
  "platform": "sing-box",
  "config": {
    "log": { "level": "info" },
    "dns": { ... },
    "inbounds": [ ... ],
    "outbounds": [
      {
        "type": "selector",
        "tag": "Proxy",
        "outbounds": ["香港 Reality 01"]
      },
      {
        "type": "vless",
        "tag": "香港 Reality 01",
        "server": "hk01.edge.net",
        "server_port": 443,
        "uuid": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
        "flow": "xtls-rprx-vision",
        "tls": {
          "enabled": true,
          "server_name": "gateway.icloud.com",
          "utls": { "enabled": true, "fingerprint": "chrome" },
          "reality": { "enabled": true, "public_key": "x5vK7a...=", "short_id": "0123456789abcdef" }
        }
      }
    ],
    "route": { ... }
  }
}
```

---

## 四、Socks5 独立业务线设计与反亲和调度机制

### 1. 业务定位与三大原则
Socks5 主要面向跨境电商（指纹浏览器店群防关联）、社媒养号与自动化爬虫，遵循以下原则：
* **不进大模板渲染**：Socks5 节点从 `/sub` 客户端路由模板中严格剔除，避免被客户端 TUN/分流规则污染；
* **零 Agent 架构（HTTP 数据源热加载）**：母鸡无需安装复杂 Agent 或 Redis，利用 GOST 原生的 `http + reload` 机制定期拉取名单，并在**本地内存中完成 100% 鉴权（0 握手延迟、超高并发）**，实现秒级开通、秒级停机；
* **防关联反亲和性（Anti-Affinity）**：同一用户在同一国家购买多个 Socks5 节点时，**绝对不能分配到相同的物理 IP**。

---

### 2. 多母鸡反亲和性排重与负载均衡调度逻辑

```mermaid
flowchart TD
    A[商城请求为用户 101 分配 1 个 US 节点] --> B[查出用户 101 已持有的 US 节点 IP<br/>ExcludedIPs = ['45.1.1.1', '45.1.1.2']]
    B --> C[在活跃 US 母鸡池中过滤排除 ExcludedIPs]
    C --> D{候选母鸡池是否 > 0 ?}

    D -- 有候选母鸡 --> E[按最少在线租户优先选出最优母鸡<br/>Least-Connections]
    E --> F[生成用户账密，写入 endpoint 表<br/>状态: Active]
    F --> G[成功交付：保证每个节点独立 IP！]

    D -- 候选池为空 (无新IP) --> H[触发库存耗尽保护<br/>创建凭证存根, 状态: pending_pool_exhausted]
    H --> I[生成运维开机工单: US 需新开 1 台独立母鸡]
    I --> J[运维开机录入后自动绑定履约]
```

#### 核心数据库查询 (SQL)：
```sql
SELECT n.id, n.server_address, n.server_port, COUNT(ep.id) AS current_load
FROM node n
LEFT JOIN endpoint ep ON ep.node_id = n.id AND ep.is_active = true
WHERE n.protocol = 'socks5' 
  AND n.country = :country
  AND n.is_active = true
  -- 核心：排查该用户在该国家已经使用过的物理 IP
  AND n.server_address NOT IN (
      SELECT n2.server_address 
      FROM endpoint ep2
      JOIN node n2 ON ep2.node_id = n2.id
      WHERE ep2.user_id = :user_id AND n2.country = :country AND ep2.is_active = true
  )
GROUP BY n.id
ORDER BY current_load ASC   -- 优先挑选负载最轻的母鸡
LIMIT 1;
```

---

### 3. Socks5 专属微服务 API

#### 接口 3.1：申请分配 Socks5 节点 (支持批量 & 自动排重)
* **请求方式**：`POST /api/service/users/{token}/socks5/allocate`
* **请求体 (JSON)**：
```json
{
  "country": "US",               // 目标国家代码（ISO 两位代码）
  "count": 2,                    // 需要购买/分配的数量（默认 1）
  "remark": "TikTok 运营店铺 01"  // 备注
}
```
* **响应结果 (JSON)**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "allocated_count": 2,
    "items": [
      {
        "endpoint_id": 3001,
        "status": "active",
        "country": "US",
        "ip": "45.76.12.34",
        "port": 1080,
        "username": "u101_us_01",
        "password": "pwd_secret_abc1",
        "proxy_format": "45.76.12.34:1080:u101_us_01:pwd_secret_abc1",
        "proxy_url": "socks5://u101_us_01:pwd_secret_abc1@45.76.12.34:1080#US-Shop01"
      },
      {
        "endpoint_id": 3002,
        "status": "active",
        "country": "US",
        "ip": "104.28.99.88",       // 确保与上一个 IP 绝对不同
        "port": 1080,
        "username": "u101_us_02",
        "password": "pwd_secret_abc2",
        "proxy_format": "104.28.99.88:1080:u101_us_02:pwd_secret_abc2",
        "proxy_url": "socks5://u101_us_02:pwd_secret_abc2@104.28.99.88:1080#US-Shop02"
      }
    ]
  }
}
```
* **若池中 IP 数量不足**：
  对应 Item 的 `status` 为 `"pending_pool_exhausted"`，系统自动生成运维开机提醒，不重复发放相同 IP。

---

#### 接口 3.2：获取用户 Socks5 节点列表 (纯净直出)
面向用户端或商城前端，支持指纹浏览器一键导入的标准纯文本。

* **请求方式**：`GET /api/service/users/{token}/socks5/list?format=text`
* **Query 参数**：
  * `format=text`：直接返回纯文本（Content-Type: text/plain），每行一个代理：
    ```text
    45.76.12.34:1080:u101_us_01:pwd_secret_abc1
    104.28.99.88:1080:u101_us_02:pwd_secret_abc2
    ```
  * `format=json`：返回结构化 JSON 列表。

#### 接口 3.3：GOST 母鸡端 HTTP 账密热加载接口 (零 Agent + 本地纯内存验证)
依据 GOST 官方认证器规范，GOST 原生支持“HTTP 数据源 + 热加载（`reload`）”。**所有 TCP 握手认证均由 GOST 在本地内存中完成（0 毫秒延迟、支持数十万 QPS）**，每隔设定周期向 Noder 拉取最新账密列表更新内存，实现秒级生效与欠费封禁。

* **请求方式**：`GET /api/service/socks5/nodes/{node_id}/auth-list`
* **鉴权方式**：母鸡专有 Token 或 IP 白名单
* **响应格式**（标准纯文本，`Content-Type: text/plain`，每行为空格分割的 `username password`）：
```text
# username password
u101_us_01 pwd_secret_abc1
u102_us_02 pwd_secret_abc2
```

##### 附：GOST 母鸡端原生配置示例 (`gost.yaml`)
母鸡服务器无需安装任何 Agent 或 Redis，只需配置如下官方原生文件：
```yaml
services:
  - name: socks-in
    addr: :1080
    handler:
      type: socks5
      auther: auther-0
    listener:
      type: tcp

authers:
  - name: auther-0
    reload: 15s   # 每 15 秒自动向 Noder 热同步一次最新名单，载入本地内存
    http:
      url: https://your-noder-domain.com/api/service/socks5/nodes/1/auth-list
      timeout: 5s
```

---

## 五、运维履约与数据模型设计

### 1. Endpoint 模型适配
系统已有的 `model.Endpoint` 表结构调整为支持待绑定态与 Socks5 专属字段：
* `NodeID`: 可空外键（或为 0），表示尚未分配物理服务器；
* 字段含义：
  * `UserID`: 归属用户
  * `Tag`: 节点名字（如 "香港定制01"、"US-Shop01"）
  * `UUID`: 子凭证 UUID（通用节点）
  * `Password`: 子凭证密码（Socks5 密码）
  * `NodeID`: 绑定的物理服务器（运维绑定或池化调度时填入）
  * `IsActive`: 是否激活

### 2. 订阅生成规则
* 用户访问 `/sub?token=xxx` 时，系统查询该用户的所有 `Endpoint`；
* **过滤机制**：仅提取已绑定物理机、物理机激活且**协议不是 socks5** 的端点注入 Sing-box / Xray 模板；
* 若用户只有 Socks5 节点或通用节点全在待绑定状态，返回基础安全模板。
