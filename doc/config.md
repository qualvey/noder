# Noder 配置说明

## 结论

应用配置文件为 `config.yaml`（Debian 安装后为 `/etc/noder/config.yaml`），环境变量可覆盖
文件中的设置。配置主要分为以下几类：

1. **Noder 服务进程配置**：通过 `config.yaml` 和环境变量传入；
2. **运行目录和数据路径**：由进程启动时的工作目录决定；
3. **客户端配置模板**：`templates/` 下的 Sing-Box、Xray、Mihomo 配置；
4. **数据库配置**：保存在 `data.db` 的模板、模板分支和应用设置中。

## 服务端环境变量

生产环境的 Debian 包使用 `/etc/noder/noder.env`，systemd 服务通过
`EnvironmentFile` 加载该文件。源码本地运行时也可以直接设置 shell 环境变量。
环境变量只在进程启动时读取，修改后需要重启 Noder。

| 变量 | 默认值 | 作用 |
| --- | --- | --- |
| `HOST` | `0.0.0.0` | HTTP 监听地址。 |
| `PORT` | `8000` | HTTP 监听端口。 |
| `NODER_CONFIG_FILE` | `<BaseDir>/config.yaml` | 指定配置文件路径；Debian 默认是 `/etc/noder/config.yaml`。 |
| `LOG_LEVEL` | 配置文件中的值 | 覆盖 `logging.level`，可选 `debug`、`info`、`warn`、`error`。 |
| `LOG_FORMAT` | 配置文件中的值 | 覆盖 `logging.format`，可选 `text`、`json`。 |
| `LOG_OUTPUT` | 配置文件中的值 | 覆盖 `logging.output`，可选 `stdout`、`stderr`。 |
| `ADMIN_SECRET_TOKEN` | `admin-secret` | 首次初始化数据库时导入的管理 API 密钥。Debian 安装脚本会在配置为空时生成随机值。 |
| `MALL_SERVICE_SECRET` | `service-secret` | 微服务接口的 `X-Service-Token` 密钥。 |
| `SERVICE_SECRET_TOKEN` | 无 | `MALL_SERVICE_SECRET` 未设置时的兼容备用变量。 |

配置文件示例：

```yaml
server:
  host: 0.0.0.0
  port: 8000

logging:
  level: info
  format: text
  output: stderr
```

环境变量优先级高于配置文件；例如临时打开 debug 日志可以执行
`LOG_LEVEL=debug ./noder`，无需修改配置文件。

### 管理员密钥的实际来源

`ADMIN_SECRET_TOKEN` 只在数据库初始化时写入 `app_setting` 表的
`admin_secret_token`。数据库已经存在时，鉴权优先读取数据库中的值，所以仅修改
`/etc/noder/noder.env` 不会替换已有管理员密钥。可以通过管理设置接口或直接在数据库中
维护该设置；生产环境不要使用默认值。

## 数据目录和模板路径

服务启动时以当前工作目录作为 `BaseDir`。路径在 `internal/config/config.go` 中拼接：

| 内容 | 路径（相对于 `BaseDir`） |
| --- | --- |
| SQLite 数据库 | `data.db` |
| 文件分发内容 | `data/files/` |
| Sing-Box 模板 | `templates/sing-box.json` |
| Xray 模板 | `templates/xray.json` |
| Mihomo 模板 | `templates/mihomo.yml` |

Debian systemd 单元的 `WorkingDirectory` 是 `/var/lib/noder`，因此默认路径为：

```text
/var/lib/noder/data.db
/var/lib/noder/data/files/
/var/lib/noder/templates/sing-box.json
/var/lib/noder/templates/xray.json
/var/lib/noder/templates/mihomo.yml
```

前端静态文件通过 `embed` 编译进二进制，运行时不依赖 `static/` 路径。
当前 Debian 打包脚本也不会把仓库中的 `templates/` 目录复制进安装包；模板文件缺失时
使用代码内置的最小默认模板。需要使用仓库中的完整模板时，应将对应文件放到上述
`/var/lib/noder/templates/` 目录。

## 服务端日志

### Noder 自身日志

服务端使用 Go 标准库 `log/slog` 提供结构化日志，HTTP 请求日志由项目自己的中间件统一
记录。日志输出到配置的 stdout/stderr；systemd 部署时推荐使用 stderr，由 journald 收集。

因此：

- 本地运行 `go run ./cmd/server` 时直接显示在终端；
- systemd 部署时由 journald 收集，可用以下命令查看：

  ```bash
  journalctl -u noder -f
  journalctl -u noder --since today
  ```

- 日志等级支持 `debug`、`info`、`warn`、`error`；
- `text` 格式便于终端和 journald 阅读，`json` 格式便于日志采集系统解析；
- 每个 HTTP 请求会记录 `request_id`、方法、路径（不记录 query，避免泄露 Token）、状态码、
  响应字节数、耗时和客户端地址；4xx 以 warn 记录，5xx 以 error 记录；
- 未处理的 panic 会被恢复，并记录 panic 内容和堆栈，同时返回 HTTP 500。
- ZIP 模板渲染在 `debug` 等级下会记录模板类型、选中的文件、Xray/Sing-Box 节点数量、
  占位符渲染前后数量和实际写入字节数；不会记录 Token、UUID 或密码。
- Xray/Sing-Box 启动器 ZIP 中识别出的 `config.json` 与 `config.yaml/config.yml` 会一并
  渲染，避免伴随启动器配置继续保留默认占位符。

### 生成给客户端的配置日志等级

下面的 `log` 字段不是 Noder 服务端日志，而是返回给客户端核心的配置：

| 客户端 | 配置文件 | 当前默认值 | 字段 |
| --- | --- | --- | --- |
| Sing-Box | `templates/sing-box.json` | `info`，启用时间戳 | `log.level`、`log.timestamp` |
| Xray | `templates/xray.json` | `warning` | `log.loglevel` |
| Mihomo | `templates/mihomo.yml` | `info` | `log-level` |

例如，调整 Sing-Box 日志等级应修改：

```json
"log": {
  "level": "debug",
  "timestamp": true
}
```

调整 Xray 应修改 `templates/xray.json` 的 `log.loglevel`，调整 Mihomo 应修改
`templates/mihomo.yml` 的 `log-level`。可用等级及具体行为以对应客户端版本为准。

模板文件不存在或内容为空时，代码会使用内置默认模板；内置默认值分别也是 Sing-Box
`info`、Xray `warning`、Mihomo `info`。

## 数据库中的可配置项

- 首次启动时，`templates/` 下的默认模板会写入数据库模板表；也可以通过管理 API
  `/api/templates` 修改模板并保留历史版本。需要注意：标准 `/sub`、`/mihomo`、`/xray`
  生成器优先读取文件系统模板；数据库模板主要用于服务端配置渲染接口和管理保存。
- 用户可以绑定模板分支（`/api/template-branches`），分支内容会优先用于对应用户的
  订阅渲染。
- `/api/settings` 是管理员保护的通用应用设置接口，目前主要保存管理员密钥和共享下载
  Token。它不是日志配置入口。
- 用户的 `config_override` 目前只允许覆盖 Sing-Box 的 `route` 和 `dns`，不能用它
  修改日志等级。

## 其它启动器配置

`xray-launcher/config.yaml`、`sing-launcher/config.yaml` 是随客户端启动器使用的配置，
用于指定订阅地址、Token、配置文件名和核心程序路径；它们不控制 Noder 服务端日志。
其中的订阅 Token 属于敏感信息，不应提交到公开仓库或写入日志。

## Debian 部署时常用位置

| 内容 | 位置 |
| --- | --- |
| 环境配置 | `/etc/noder/noder.env` |
| 应用配置 | `/etc/noder/config.yaml` |
| systemd 单元 | `noder.service` |
| 工作目录 | `/var/lib/noder` |
| 数据库 | `/var/lib/noder/data.db` |
| 服务日志 | `journalctl -u noder` |

修改端口或密钥后重启服务：

```bash
sudo systemctl restart noder
sudo systemctl status noder
```
