# Sing-Box Subscription Middleman (订阅中间件管理服务 - Noder)

基于 Go + Chi + SQLite + Vue 3 打造的 Sing-Box 与 Mihomo 节点及订阅动态生成中间件系统。

采用 **方案 3：独立凭证与动态拼接** 架构设计：
- **`Node` 表**：仅存储服务器基础设施属性（IP、端口、协议类型、TLS SNI、传输方式等），**不存储任何用户鉴权信息（如 UUID 或密码）**。
- **`User` 表**：存储用户个人标识、鉴权 Token、**专属 UUID / Password** 以及绑定的节点 ID。
- **动态拼接引擎**：当用户请求订阅链接或验证接口时，服务端从 `Node` 表提取服务器信息，并结合 `User` 表该用户的专属 UUID 和密码，在内存中动态组装为完整的 Sing-Box / Mihomo 节点配置。
- **单二进制自包含**：Go 内嵌前端静态页面（`//go:embed`），开箱即用，无 Python 运行时与虚拟环境依赖，秒级冷启动与极低内存开销。

## 🧭 项目概览

- `cmd/server/`：服务启动、路由注册和嵌入式 SPA 托管
- `internal/handler/`：节点、用户、文件、模板、设置和订阅 API
- `internal/service/`：Sing-Box/Mihomo 配置生成、文件分发和模板渲染
- `internal/model/`、`internal/db/`：SQLite 数据模型、初始化与迁移
- `internal/contract/`：协议字段校验及核心能力注册
- `web/`：Vue 3 + TypeScript 管理面板源码
- `templates/`、`static/`：默认配置模板与前端构建产物

完整 API 字段、鉴权方式、请求示例和状态码见 **[API 参考](doc/api.md)**。

---

## 📦 Debian / Ubuntu 安装部署 (.deb)

项目提供原生的 Debian 打包方案，支持 `amd64` 与 `arm64` 架构：

```bash
# 1. 构建 Debian 安装包 (或从 GitHub Releases 下载)
make deb        # amd64
make deb-arm64  # arm64

# 2. 安装软件包
sudo dpkg -i dist/noder_*_amd64.deb

# 3. 启停与状态管理
sudo systemctl status noder
sudo systemctl restart noder
sudo journalctl -u noder -f
```

安装后服务会自动注册为 `noder.service`，监听 `0.0.0.0:8000`，工作目录位于 `/var/lib/noder`，配置文件位于 `/etc/noder/noder.env`。

---

## 🚀 本地开发与运行方式

### 1. 前端开发 (Vue 3 + TypeScript + Vite)

前端源码位于 `web/`：

```bash
cd web
pnpm install        # 安装依赖
pnpm dev            # 开发模式 (http://127.0.0.1:5273，API 代理到后端 8000)
pnpm build          # 构建产物输出至 static/ (供 Go 内嵌)
```

### 2. 后端开发 (Go 1.22+)

```bash
# 运行单元测试
go test ./...

# 编译并运行服务端 (默认监听 0.0.0.0:8000)
go run ./cmd/server

# 或者编译二进制可执行文件
go build -ldflags "-s -w" -o noder ./cmd/server
./noder
```

### 3. 全量构建与快捷脚本

项目提供快捷 Makefile 与跨平台构建脚本：

```bash
make build          # 全量构建前端与后端
make release        # 交叉编译全平台二进制 (Linux / macOS / Windows)
make deb-all        # 构建 amd64 与 arm64 架构 deb 包
```

服务启动后：
- **网页管理面板**：[http://127.0.0.1:8000](http://127.0.0.1:8000)
- **静态前端与 API**：单一二进制一体化托管提供服务

---

## 🔑 配置说明

### 环境变量

| 环境变量名 | 说明 | 默认值 |
| :--- | :--- | :--- |
| `ADMIN_SECRET_TOKEN` | 管理员 API 鉴权密钥（用于网页后台 CRUD 操作 header `X-Admin-Token`） | `admin-secret` |

---

## 📡 核心 API 端点说明

> 本节为快速索引；完整接口以 **[doc/api.md](doc/api.md)** 为准。

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

运行内置的 Go 单元与集成测试校验数据库模型与动态拼接逻辑：

```bash
go test -v ./...
```
