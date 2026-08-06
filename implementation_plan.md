# 项目实现计划：管理端开发（方案 3：独立凭证与动态拼接）

根据确定架构，采用 **方案 3** 推进管理端与数据模型设计：
1. **`Node` 表（节点公共表）**：仅存储服务器基础设施属性（`node_name`, `protocol`如 `tuic`/`vless`, `server_address`, `server_port`, `security`, `sni`, `transport_type` 等），**不存储任何用户鉴权字段（如 UUID 和密码）**。
2. **`User` 表（用户凭证表）**：存储用户个人信息与鉴权凭证（`id`, `name`, `token`, `is_active`, `uuid`, `password`, `node_id`）。
3. **动态拼接组装**：在生成/导出节点配置时，由服务端从 `Node` 表获取服务器基础信息，并从 `User` 表提取该用户的专属 `uuid` 和 `password` 动态拼接为完整的 Sing-Box 节点配置。

---

## Proposed Changes

### 1. 后端模型与 API 改造 (`main.py`)

#### [NEW] [main.py](file:///c:/Users/Ryu/Documents/workspace/sub-server/main.py)
- **更新 `Node` 数据库模型与 Schema**：
  - `NodeBase` 中去除用户鉴权字段 `uuid` 与 `password`。
  - 保留并完善 `protocol` (重点 `tuic`, `vless`), `server_address`, `server_port`, `security`, `sni`, `transport_type`, `path`, `is_active`。
- **更新 `User` 数据库模型与 Schema**：
  - 在 `UserBase` / `User` 模型中增加专属鉴权凭证字段：
    - `uuid`: `Optional[str]`（VLESS / TUIC 等使用的 UUID，建表/创建用户时可自动生成标准 UUID）
    - `password`: `Optional[str]`（TUIC / Trojan 等使用的密码，未填时可自动生成随机密码）
  - 维持 `node_id`: 外键关联到 `Node.id`。
- **优化 Sing-Box 节点构建引擎 `build_singbox_outbound(node, user)`**：
  - 接收 `node` 和 `user` 两个对象。
  - 根据协议动态拼接：
    - `tuic`: 提取 `node.server_address`, `node.server_port`, `node.sni`，结合 `user.uuid` (或 `user.password`)。
    - `vless`: 提取 `node.server_address`, `node.server_port`, `node.security`, `node.sni`，结合 `user.uuid`。
- **管理端 CRUD API**：
  - 节点 API (`/api/nodes`)：纯服务器信息增删改查。
  - 用户 API (`/api/users`)：用户创建/修改时可配置或自动生成 `uuid` 与 `password`，并绑定 `node_id`。

---

### 2. 前端管理界面改造 (`static/`)

#### [NEW] [index.html](file:///c:/Users/Ryu/Documents/workspace/sub-server/static/index.html)
- 节点 Modal 弹窗：移除 UUID 和密码输入框（节点表只维护 IP/端口/协议/SNI 等）。
- 用户 Modal 弹窗：增加专属 UUID 和密码设置列（支持一键生成随机 UUID 和密码），并支持下拉选择绑定的节点。

#### [NEW] [app.js](file:///c:/Users/Ryu/Documents/workspace/sub-server/static/app.js)
- 调整节点表单与卡片渲染逻辑。
- 调整用户表单，提交时传输用户专属的 `uuid` / `password` 及关联的 `node_id`。
- 预览 JSON 时传递完整参数并展现动态拼接效果。

#### [NEW] [styles.css](file:///c:/Users/Ryu/Documents/workspace/sub-server/static/styles.css)
- 前端管理界面的设计样式系统。

---

## Verification Plan

### Automated / Integration Tests
1. 使用 `uv run .\main.py` 启动服务。
2. 调用 `POST /api/nodes` 创建一个不含 UUID/密码的 TUIC 节点（如 server: `hk.example.com`, port: `8443`）。
3. 调用 `POST /api/nodes` 创建一个不含 UUID/密码的 VLESS 节点。
4. 调用 `POST /api/users` 创建用户“张三”，设置 `node_id=1`，为其配置（或自动生成）专属 UUID 和密码。
5. 调用 `GET /sub?token={user_token}`，验证导出的 Outbound JSON 是否成功从 `Node` 和 `User` 拼接出了完整的 `type: "tuic"`, `uuid`, `password`, `server` 等字段。

### Manual Verification
1. 访问网页管理端 `http://127.0.0.1:8000`。
2. 验证“节点管理”中创建/编辑节点不再需要输入 UUID 和密码。
3. 验证“用户管理”中每个用户可查看/编辑独立的 UUID 和密码，并绑定上游节点。
