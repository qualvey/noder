# Sing-Box Subscription Middleman (订阅中间件管理服务)

基于 FastAPI + SQLModel 打造的 Sing-Box 节点与订阅动态生成中间件系统。

采用 **方案 3：独立凭证与动态拼接** 架构设计：
- **`Node` 表**：仅存储服务器基础设施属性（IP、端口、协议类型、TLS SNI、传输方式等），**不存储任何用户鉴权信息（如 UUID 或密码）**。
- **`User` 表**：存储用户个人标识、鉴权 Token、**专属 UUID / Password** 以及绑定的节点 ID。
- **动态拼接引擎**：当用户请求订阅链接或验证接口时，服务端从 `Node` 表提取服务器信息，并结合 `User` 表该用户的专属 UUID 和密码，在内存中动态组装为完整的 Sing-Box 节点配置。

---

## 📦 Linux Systemd 一键安装部署

在 Linux 服务器 (Ubuntu / Debian / CentOS / AlmaLinux 等) 上运行以下一键部署命令即可自动完成安装：

```bash
curl -fsSL https://raw.githubusercontent.com/qualvey/noder/main/install.sh | sudo bash
```

或者使用 `wget`：

```bash
wget -qO- https://raw.githubusercontent.com/qualvey/noder/main/install.sh | sudo bash
```

脚本会自动为您：
1. 检查并安装必要环境及 `uv` 包管理器
2. 将代码部署到 `/opt/sub-server`
3. 交互设置 `ADMIN_SECRET_TOKEN` 与监听端口
4. 注册并启动 Systemd 服务 `sub-server.service`（支持开机自启与失败重启）

### ⚡ 单独热更新前端资源（无需重启后端）

如果您仅对 Web 前端界面（HTML/JS/CSS）进行了修改升级，无需重新安装 Python 依赖或重启后端 Systemd 服务，直接运行以下一键脚本即可秒级完成静态资源的独立热更新：

```bash
curl -fsSL https://raw.githubusercontent.com/qualvey/noder/main/update-frontend.sh | sudo bash
```

或者本地运行：
```bash
sudo ./install.sh --frontend
```

### ⚡ 单独热更新后端代码与依赖 (保全数据库)

如果修改了 Python 后端 API 代码或增加了依赖包，需要更新后端且**保全当前数据库 `data.db` 不受破坏**，运行以下一键更新脚本即可自动同步代码、更新 `uv` 依赖并重启 Systemd 服务：

```bash
curl -fsSL https://raw.githubusercontent.com/qualvey/noder/main/update-backend.sh | sudo bash
```

或者本地运行：
```bash
sudo ./install.sh --backend
```

---

## 🚀 本地开发运行方式

### 前端 (Vue 3 + TypeScript + Vite)

前端源码位于 `web/`，构建产物输出到 `static/`（后端直接挂载服务）。

```bash
cd web
pnpm install        # 安装依赖
pnpm dev            # 开发模式 (http://127.0.0.1:5273，API 代理到后端 8000)
pnpm build          # 构建 -> ../static/ (vue-tsc 类型检查 + vite build)
```

兼容根路径与子路径反代部署（vite base `./`，所有资源相对路径）。

### 方式 1：使用 `uv` 运行（推荐）

### 方式 1：使用 `uv` 运行（推荐）

项目根目录已包含 `pyproject.toml`，可直接使用 [uv](https://github.com/astral-sh/uv) 一键启动：

```bash
# 启动 Web 管理端服务 (默认监听 0.0.0.0:8000)
uv run python main.py

# 或者使用 uvicorn 热加载模式
uv run uvicorn main:app --reload --host 0.0.0.0 --port 8000
```

### 方式 2：使用传统 Python 虚拟环境

```bash
# 1. 创建并激活虚拟环境
python -m venv .venv
# Windows PowerShell:
.\.venv\Scripts\Activate.ps1

# 2. 安装依赖
pip install fastapi uvicorn sqlmodel pydantic

# 3. 运行服务
python main.py
```

服务启动后：
- **网页管理面板**：[http://127.0.0.1:8000](http://127.0.0.1:8000)
- **API Swagger 文档**：[http://127.0.0.1:8000/docs](http://127.0.0.1:8000/docs)

---

## 🔑 配置说明

### 环境变量

| 环境变量名 | 说明 | 默认值 |
| :--- | :--- | :--- |
| `ADMIN_SECRET_TOKEN` | 管理员 API 鉴权密钥（用于网页后台 CRUD 操作 header `X-Admin-Token`） | `admin-secret` |

---

## 📡 核心 API 端点说明

### 1. 客户端订阅导出 API (公开 / 用户侧)
- **请求方法**：`GET /sub?token={USER_TOKEN}`
- **说明**：Sing-Box 客户端导入该 URL，系统会自动校验 Token 有效性，提取绑定节点的服务器信息与用户的专属 UUID/密码，返回完整的 Sing-Box 配置文件 JSON。

### 2. 用户节点查询 API (用户侧)
- **请求方法**：`GET /node?token={USER_TOKEN}`
- **说明**：传入用户 Token，返回该用户绑定的所有节点元数据及动态拼接好的 Sing-Box Outbound 节点配置。

### 3. 用户 Token 验证与数据查询 API (用户侧/中间件侧)
- **请求方法**：`GET /api/user/verify?token={USER_TOKEN}`
- **说明**：验证 Token 是否有效，并返回绑定节点的基础信息与组装后的 Sing-Box 配置 JSON。
### 4. 文件分发 API (用户侧 / 管理侧)
- **用户下载 (Token 鉴权)**：GET /dl/{file_id}?token={USER_TOKEN}
  - APK：全员公用，原文件直出下载。
  - ZIP：内含 1 个 yaml 模板（支持 \{{uuid}}\ \{{password}}\ \{{token}}\ \{{name}}\ \{{node_list_yaml}}\ \{{node_list_json}}\ \{{outbounds_yaml}}\ \{{outbounds_json}}\ 占位符），下载时按用户凭证实时渲染，其余文件原样分发。
- **管理端 (Header: \X-Admin-Token\)**：\POST/GET/PUT/DELETE /api/files\ 上传、列表、更新元数据、删除分发文件。

### 3. 管理员 API (需要 Header: `X-Admin-Token`)
- **节点管理 (`/api/nodes`)**：
  - `GET /api/nodes` - 获取节点列表
  - `POST /api/nodes` - 创建代理节点 (无需填写 UUID 和密码)
  - `PUT /api/nodes/{id}` - 更新节点属性
  - `DELETE /api/nodes/{id}` - 删除节点
- **用户管理 (`/api/users`)**：
  - `GET /api/users` - 获取用户列表
  - `POST /api/users` - 创建用户 (支持手动设置或自动生成 Token、专属 UUID、Password，并绑定 `node_id`)
  - `PUT /api/users/{id}` - 更新用户信息或调整绑定的节点
  - `DELETE /api/users/{id}` - 删除用户

---

## 🧪 运行自动化验证测试

运行内置的集成测试脚本校验数据库模型与动态拼接逻辑：

```bash
uv run python test_verification.py
```
