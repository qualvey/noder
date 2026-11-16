# Noder 日志系统与可观测性设计规范 (Observability Spec)

> **状态**：Draft v1.0  
> **适用范围**：Noder 控制面核心服务（`cmd/server`、`internal/handler`、`internal/db` 等）  
> **参考技术选型**：Go 1.21+ 标准库 `log/slog`（零第三方重量依赖）

---

## 1. 目标与痛点定义

### 1.1 现状痛点
1. **无级别控制**：大量使用标准库 `log.Printf`，无法区分 `DEBUG` / `INFO` / `WARN` / `ERROR`，无法在联调排查时动态查看高信噪比排查信息，生产环境亦无法静音低价值信息。
2. **非结构化输出**：纯字符串拼接，日志解析需要复杂的正则表达式，难以使用 Loki / ELK 进行基于字段（如 `agent_id`, `credential_id`, `endpoint_id`, `revision`）的高效检索与告警。
3. **缺乏全链路关联**：Access Log 采用 Chi 框架默认的纯文本格式，与业务内部触发的 Error / DB 报错脱节，无法通过 `request_id` 快速回溯一个请求的完整调用上下文。
4. **启动“黑盒”**：启动时仅打印单行 `server listening on...`，系统运行时所需的数据库类型、数据目录、关键安全凭据配置状态、已挂载路由清单等关键信息完全不可知，导致配置错误难以及时发现。

### 1.2 建设目标
1. **统一门面与零冗余依赖**：基于 Go 1.21 官方引入的标准库 `log/slog`，不引入厚重的第三方框架，保证长期的代码兼容性与极佳的性能。
2. **多模式输出**：
   - **开发 / 联调模式（Dev/Terminal）**：输出对人类视觉友好、对齐、区分颜色与高亮关键属性的文本格式。
   - **生产模式（Production/Cloud）**：输出符合工业标准的单行 JSON 格式，包含 ISO8601/RFC3339 时间戳。
3. **启动自检看板（Boot Summary）**：在服务监听前，输出包含版本、环境、配置、数据库、安全凭据自检状态与路由表自检看板。
4. **全链路 RequestID 贯通**：HTTP 请求入口统一生成或传递 `X-Request-ID`，并将携带追踪属性的 Logger 注入 Context，实现请求日志与业务日志单链贯通。
5. **敏感信息自动脱敏**：杜绝明文打印 Token、私钥或密码。

---

## 2. 环境变量与配置定义

| 环境变量 | 可选值 | 默认值 | 说明 |
|---|---|---|---|
| `LOG_LEVEL` | `DEBUG`, `INFO`, `WARN`, `ERROR` | `INFO` | 最低输出日志等级。联调建议设为 `DEBUG` |
| `LOG_FORMAT` | `text`, `json`, `pretty` | `pretty`（开发）/ `json`（生产） | 日志输出格式。未显式指定时，根据 `APP_ENV` 自动判定 |
| `APP_ENV` | `development`, `production`, `test` | `development` | 运行环境标识 |
| `LOG_SOURCE` | `true`, `false` | `false` | 是否在日志中打印代码文件名和行号（`file:line`） |

---

## 3. 架构设计与包结构

新建 `internal/logger` 包作为全局可观测性入口，结构如下：

```
internal/logger/
├── logger.go          # 初始化 slog.Logger、全局 Logger 代理与 Level 解析
├── handler.go         # Pretty/Console 易读彩色日志处理器与 JSON 处理器适配
├── context.go         # Context 注入与提取（WithContext, FromContext）
└── middleware.go      # Chi HTTP 结构化访问日志中间件（替代 chi.Logger）
```

---

## 4. 详细规范与格式定义

### 4.1 启动看板规范（Boot Summary）

服务启动时，由 `logger.PrintBootSummary(info BootInfo)` 打印标准化看板，格式规范如下：

```
================================================================================
  Noder Control Plane (Go Edition v1.0.0-dev)
================================================================================
  [ENV]        Profile: development | Go: go1.27.3 | PID: 12345
  [CONFIG]     DataDir: /home/ryu/code/noder | Port: :8000
  [DATABASE]   Driver: SQLite (modernc) | DSN: /home/ryu/code/noder/noder.db | Status: OK
  [SECURITY]   Mall Secret: Configured (len=32) | JWT Secret: Configured
  [ROUTERS]    Auth, Node, User, Domain, Template, Service, AgentV1, SPA
  [AGENTS]     Active Endpoints: 3 | Heartbeat Timeout: 90s
================================================================================
2026-11-10T03:40:00.123+08:00 [INFO] HTTP server successfully started listening on http://0.0.0.0:8000
```

### 4.2 日志结构化输出规范

#### 本地终端模式（Text / Pretty）
```
2026-11-10 03:40:05.123 [INFO] HTTP request completed method=POST path=/api/agent/v1/heartbeat status=200 duration=1.24ms ip=127.0.0.1 req_id=req_01hx agent_id=ag_hk_01
2026-11-10 03:40:10.456 [WARN] deployment revision conflict credential_id=cred_01 endpoint_id=101 report_rev=1710000000050 db_rev=1710000000100 req_id=req_02hy
2026-11-10 03:40:12.789 [ERROR] failed to connect database error="connection refused" req_id=req_03hz
```

#### 生产容器/文件模式（JSON）
```json
{"time":"2026-11-10T03:40:05.123+08:00","level":"INFO","msg":"HTTP request completed","method":"POST","path":"/api/agent/v1/heartbeat","status":200,"duration_ms":1.24,"ip":"127.0.0.1","req_id":"req_01hx","agent_id":"ag_hk_01"}
{"time":"2026-11-10T03:40:10.456+08:00","level":"WARN","msg":"deployment revision conflict","credential_id":"cred_01","endpoint_id":101,"report_rev":"1710000000050","db_rev":"1710000000100","req_id":"req_02hy"}
```

### 4.3 HTTP 访问日志中间件规范
使用 `logger.HTTPMiddleware` 替换 `github.com/go-chi/chi/v5/middleware.Logger`：
1. **自动提取或生成 Request ID**：
   - 优先读取 `X-Request-ID` 请求头；若无则自动生成 UUID（如 `req_` 前缀短码）。
   - 回写 `X-Request-ID` 至响应头。
2. **Context 绑定**：
   - 构建附加了 `request_id` 属性的子 Logger 注入请求上下文 `r.Context()`。
3. **请求耗时与状态打标**：
   - 请求响应结束后记录单行汇总：
     - `status < 400`：输出 `INFO`
     - `400 <= status < 500`：输出 `WARN`
     - `status >= 500`：输出 `ERROR`
   - 字段包括：`method`, `path`, `status`, `duration_ms`, `ip`, `bytes_out`, `user_agent`。

### 4.4 业务模块打点规范

#### 1. Xray Agent 交互关键点
- **心跳（Heartbeat）**：
  - 正常心跳：`DEBUG`，附带 `agent_id`, `version`, `endpoints_count`；
  - 节点状态由离线转在线：`INFO`，记录 `agent_id`, `hostname`。
- **拉取部署（GetDeployments）**：
  - `DEBUG`，附带 `agent_id`, `since_revision`, `items_count`, `has_more`。
- **收敛上报（ReportDeployments）**：
  - 成功：`INFO`，附带 `accepted_count`, `agent_id`；
  - 冲突（Revision Conflict）：`WARN`，记录 `credential_id`, `endpoint_id`, `report_rev`, `db_rev`。
- **流量上报（ReportTraffic）**：
  - 首次处理：`INFO`，附带 `report_id`, `items_count`, `total_bytes`；
  - 幂等命中：`DEBUG`，附带 `report_id`, `msg: "duplicate report_id ignored"`。

#### 2. 安全与脱敏规范
- **禁止打印**：密码原文、未脱敏完整 Token（如 `control_token_ciphertext`、`MALL_SERVICE_SECRET`）。
- **脱敏策略**：
  - Token / Secret：仅打印前 4 位或长度，例如 `tok_****(len=32)`；
  - 凭证密码：打印 `[REDACTED]` 或 `password_set=true`。

---

## 5. 验收标准
1. **编译与单测**：新建的 `internal/logger` 单测覆盖率达到 80% 以上，全工程 `go test -count=1 ./...` 全部通过。
2. **启动自检**：执行 `go run cmd/server/main.go` 时，终端输出格式规整的 Boot Summary 看板。
3. **格式与级别自适应**：
   - 设置 `LOG_LEVEL=DEBUG` 可查看到精细心跳与 DB 迁移日志；
   - 设置 `LOG_FORMAT=json` 输出合法单行 JSON。
4. **全链路 RequestID 联通**：在业务逻辑中通过 `logger.FromContext(ctx)` 打出的日志自动包含该次请求的 `request_id`。
