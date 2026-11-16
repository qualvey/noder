# Client API v1 Spec (Windows GUI 私有客户端)

> 面向 `Wails` Windows 桌面端. `BaseURL` 编译时注入 `ldflags -X noder/internal/config.ClientAPIBaseURL`, 客户端无硬编码、无设置页.

## 0. 全局约定

- **BaseURL**: `https://api.example.com`
- **鉴权**: `Authorization: Bearer <access_token>` (`JWT`, `exp 15min`). `refresh_token` (`30d`, 单设备单 token, 可吊销) `DPAPI` 加密落盘.
- **设备标识**: `device_id = uuid` 首次启动生成落盘, `device_name = hostname`, `platform = windows` (`enum: windows|macos|linux`), `app_version`.
- **Content-Type**: `application/json`
- **错误格式**:
```json
{ "code": 40001, "msg": "INVALID_CREDENTIAL", "detail": "username or password incorrect" }
```
| HTTP | code | msg |
|---|---|---|
| 401 | 40101 | AUTH_FAILED / TOKEN_EXPIRED / REFRESH_EXPIRED / DEVICE_KICKED |
| 403 | 40301 | DISABLED / REVOKED |
| 409 | 40901 | DEVICE_LIMIT_EXCEEDED |
| 429 | 42901 | RATE_LIMITED |

- **时间**: 均为 `RFC3339 UTC`.

---

## 1. POST /api/v1/client/auth/login

用户名密码换双 token, 并注册/更新设备.

**Req**
```json
{
  "username": "user1",
  "password": "P@ssw0rd",
  "device_id": "550e8400-e29b-41d4-a716-446655440000",
  "device_name": "DESKTOP-ABC",
  "platform": "windows"
}
```

**Res 200**
```json
{
  "access_token": "eyJ...",
  "refresh_token": "rt_...",
  "expires_in": 900,
  "user": { "id": 123, "username": "user1" },
  "device": { "id": "550e8400-...", "limit": 3, "current_count": 2 }
}
```

**Err**
- `401 AUTH_FAILED`
- `403 DISABLED`
- `409 DEVICE_LIMIT_EXCEEDED` (`msg: 已达设备上限, 请先踢掉其他设备`)
- `429 RATE_LIMITED`

**服务端逻辑**: `Bcrypt` 校验 -> 查 `AccessGrant`/`User.IsActive`/`ExpiresAt`/`TrafficQuotaBytes` -> `device_count < limit` -> 签发 `JWT`+`refresh_token` -> `upsert devices (device_id, last_seen=now, ip, platform)`.

---

## 2. POST /api/v1/client/auth/refresh

**Req**
```json
{ "refresh_token": "rt_...", "device_id": "550e8400-..." }
```

**Res 200**
```json
{ "access_token": "eyJ...", "refresh_token": "rt_...", "expires_in": 900 }
```
> 刷新后旧 `refresh_token` 作废 (rotation).

**Err** `401 REFRESH_EXPIRED` -> 前端清本地跳登录.

---

## 3. POST /api/v1/client/heartbeat (保活 + 准入闸门) `ClientAuth`

**周期 2min (允许 2-5min)**, 首连 `intent=connect`, 之后 `keepalive`. **唯一是否允许连接的判定**, `allowed==false` 必须立即断开.

**Req**
```json
{
  "device_id": "550e8400-...",
  "app_version": "1.0.0",
  "xray_version": "25.1.30",
  "intent": "connect"
}
```
`intent enum: connect | keepalive`

**Res 200**
```json
{
  "allowed": true,
  "reason": "OK",
  "reason_msg": "",
  "server_time": "2026-11-11T08:00:00Z",
  "ttl": 300
}
```
```json
{
  "allowed": false,
  "reason": "TRAFFIC_EXHAUSTED",
  "reason_msg": "流量已用尽, 请续费",
  "server_time": "2026-11-11T08:00:00Z",
  "ttl": 0
}
```
`reason enum: OK | EXPIRED | TRAFFIC_EXHAUSTED | DEVICE_LIMIT | KICKED | REVOKED | DISABLED`

**客户端行为**:
- `allowed==true` 才允许 `StartXray()`, 否则弹 `reason_msg` 拒绝启动.
- 已连接状态下每 `2min` 调一次, 一旦 `allowed==false` 立即 `StopXray()` + 弹提示.
- 断网/超时按 `ttl` 后视为 `allowed=false` 自动断开, 防止离线长期使用.
- 真正阻断靠服务端吊销节点侧 `Credential` (`model.CredentialStatusRevoked`), 客户端闸门仅为体验一致.

**服务端逻辑**: 校验 `JWT` -> 查 `device.last_seen=now` -> 查 `AccessGrant.Status/ExpiresAt/TrafficQuota` -> 查 `device_count` -> 返回 `allowed`.

---

## 4. GET /api/v1/client/nodes `ClientAuth`

返回裸节点参数, **客户端本地拼装** `xray.json` (复用 `templates/xray.json` 模板 + `internal/service/xray/xray.go:BuildXrayEndpointOutbound`).

**Query**: `?device_id=550e8400-...` (可选, 用于审计)

**Res 200**
```json
{
  "template_version": 12,
  "nodes": [
    {
      "id": 10,
      "tag": "HK-01",
      "protocol": "vless",
      "server_address": "cdn.example.com",
      "server_port": 443,
      "sni": "cdn.example.com",
      "public_key": "MvYkWqSUKK...",
      "short_id": "a854",
      "fingerprint": "chrome",
      "flow": "xtls-rprx-vision",
      "transport": "reality",
      "security": "reality",
      "path": null
    },
    {
      "id": 11,
      "tag": "JP-02",
      "protocol": "vless",
      "server_address": "test.ryugo.org",
      "server_port": 443,
      "sni": "test.ryugo.org",
      "transport": "xhttp",
      "security": "tls",
      "path": "/api/v1/data"
    }
  ]
}
```
> 若 `allowed==false`, 仍可返回 `nodes` 但客户端禁止 `Start`, 由 `heartbeat` 统一闸门.

**Err** `401 TOKEN_EXPIRED` -> 先 `refresh`.

---

## 5. GET /api/v1/client/devices `ClientAuth`

查询在线设备列表.

**Res 200**
```json
{
  "limit": 3,
  "devices": [
    {
      "id": "550e8400-...",
      "device_name": "DESKTOP-ABC",
      "platform": "windows",
      "ip": "1.2.3.4",
      "last_heartbeat_at": "2026-11-11T07:59:00Z",
      "is_current": true,
      "is_online": true
    }
  ]
}
```
`is_online = last_heartbeat_at > now-5min`.

---

## 6. POST /api/v1/client/devices/:id/kick `ClientAuth`

踢掉指定设备.

**Res 204** 无 body.

**服务端**: 吊销该 `device_id` 的 `refresh_token`, 下次 `heartbeat/nodes` 返回 `401 DEVICE_KICKED` / `allowed:false reason=KICKED`, 被踢设备前端清 `token` 跳登录.

---

## 7. 客户端流程

```
启动 -> 读 DPAPI token --有--> heartbeat(keepalive) -> allowed? --否--> 登录页
  |                               |是
  |                               v
  |                          GET /nodes -> 本地拼 xray.json -> heartbeat(connect) -> allowed? --否--> 弹 reason_msg
  |                                                                      |是
  |                                                                      v
  |                                                                 StartXray() -> 每2min heartbeat -> allowed==false => StopXray()
  --无--> 登录页 -> POST /login -> 存 token -> 同上
```

流量展示: 本地调 `http://127.0.0.1:8900` `StatsService` (`statsInboundUplink/Downlink`) 仅本地展示本次会话用量, 不上报服务端. 服务端计费以 `agent` 回传为准.

## 8. 编译注入

```bash
go build -ldflags "-X noder/internal/config.ClientAPIBaseURL=https://api.example.com -X noder/internal/config.ClientVersion=1.0.0" ./cmd/server
wails build -platform windows/amd64 -ldflags "-X main.APIBaseURL=https://api.example.com -H windowsgui"
```
