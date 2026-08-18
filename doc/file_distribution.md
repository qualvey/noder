# 文件分发功能设计文档 (File Distribution)

## 1. 需求背景

原系统每个用户有独立订阅链接（`/sub?token=***` 返回动态拼接的 Sing-Box JSON）。
新增需求：**文件形态分发** —— 不是在线 JSON，而是真实文件下载：

- **APK**：全部用户共用同一份，直接静态分发；也可填远程链接自动拉取缓存。
- **ZIP**：一个文件夹，内含 **1 个 yaml 模板**（内容按用户个性化渲染）+ 其他公共文件（原样分发）。
- **文本 (text)**：粘贴一段文本内容即生成下载链接，下载时按用户渲染占位符（免打包的模板组件）。

## 2. 架构设计

```text
管理员上传 ──> POST /api/files (multipart)
                    │
                    ├── APK ──> data/files/{uuid}_{原名}  (静态存储)
                    │
                    └── ZIP ──> 校验 zip 完整性，确定模板文件
                                (显式指定 或 自动取第一个 .yaml/.yml)

用户下载 ──> GET /dl/{file_id}?token={USER_TOKEN}
                │
                ├── APK：原文件直出 (Content-Disposition: attachment)
                │
                └── ZIP：内存中解包 -> 渲染模板文件 -> 重新打包 -> 返回
```

- 鉴权复用用户 `token`（与 `/sub` 同一套），无效/停用 token → 401。
- 文件启用状态 `is_active=False` 时 → 404（不暴露存在性）。
- 模板渲染在内存完成（`render_zip_for_user`），磁盘上的原始 ZIP 永不被修改。

## 3. 数据库模型 (DistFile 表)

| 字段 | 类型 | 说明 |
| :--- | :--- | :--- |
| id | int PK | 文件 ID |
| name | str | 显示名称 |
| file_type | str | `apk` / `zip` |
| template_name | str? | ZIP 内模板文件相对路径（仅 zip） |
| original_name | str | 原始上传文件名（下载时还原） |
| stored_name | str | 磁盘存储文件名（uuid_原名，索引） |
| size | int | 字节数 |
| source_url | str? | 远程源链接（远程拉取模式，仅 http/https） |
| cached_at | str? | 最近一次成功拉取时间（ISO） |
| is_active | bool | 是否允许下载 |
| remark | str? | 管理员备注 |
| created_at | str | 创建时间 |

存储目录：`data/files/`（自动创建，已加入 .gitignore 规则）。

## 4. API 规范

### 管理端（需 `X-Admin-Token`）

| 方法 | 路径 | 说明 |
| :--- | :--- | :--- |
| POST | `/api/files` | multipart 上传，三选一：`file` / `source_url` / `content_text`(文本)；另可带 `file_type`(auto/apk/zip) + `template_name`(可选) + `name` + `remark`。ZIP 未指定模板名时自动取第一个 `.yaml/.yml` |
| POST | `/api/files/{id}/refresh` | 强制刷新远程文件缓存 |
| GET | `/api/files` | 文件列表（按 id 倒序） |
| PUT | `/api/files/{id}` | 更新元数据：`name` / `template_name` / `is_active` / `remark` |
| DELETE | `/api/files/{id}` | 删除记录 + 磁盘文件 |

校验规则：
- 文件大小上限 100MB（`MAX_FILE_SIZE`）。
- `file_type=auto` 时按扩展名推断（`.zip` → zip，其余 → apk）。
- ZIP 必须可正常打开且无损坏条目（`testzip()`）。
- 指定 `template_name` 时必须存在于 ZIP 中（按 basename 匹配）。
- 远程链接仅允许 `http/https`（防 SSRF），首次创建即拉取落盘。

### 远程拉取缓存

- 远程模式文件创建时立即拉取一次并记录 `cached_at`。
- **缓存有效期 1 天**（`REMOTE_CACHE_TTL = 86400` 秒）：用户下载时若缓存过期，自动重新拉取并更新缓存。
- 拉取失败时**保留旧缓存继续服务**，不影响用户下载。
- 管理端可随时 `POST /api/files/{id}/refresh` 强制刷新（前端表格「🔄 刷新」按钮）。

### 用户侧（token 鉴权）

| 方法 | 路径 | 说明 |
| :--- | :--- | :--- |
| GET | `/dl/{file_id}?token=***` | APK：原文件直出；ZIP：渲染模板后返回 |

## 5. 模板渲染引擎

### 占位符（仅替换 ZIP 内指定的模板文件，其余文件原样）

| 占位符 | 渲染值 |
| :--- | :--- |
| `{{uuid}}` | 用户专属 UUID |
| `{{password}}` | 用户专属密码 |
| `{{token}}` | 用户订阅 Token |
| `{{name}}` / `{{user_name}}` | 用户名 |
| `{{node_list_yaml}}` | 绑定节点元数据（node_name/protocol/server/port）YAML 列表 |
| `{{node_list_json}}` | 同上 JSON 数组 |
| `{{outbounds_yaml}}` | 动态拼接的 Sing-Box Outbounds YAML 列表 |
| `{{outbounds_json}}` | 同上 JSON 数组 |

- 未知占位符**原样保留**，不误伤模板中其他 `{{ }}` 内容。
- **硬编码凭证智能替换**（无需改模板）：
  1. 键名感知：模板中 `token:`/`uuid:`/`password:` 键后的 UUID 格式硬编码值，自动替换为该用户的对应凭证（兼容 yaml `token: "..."` 与 json `"token": "..."` 写法）；
  2. 兜底：任何等于**已知用户 token** 的 UUID 值（如订阅 URL 里的 `?token=...`），替换为当前下载用户的 token。
- 模板文件按 UTF-8 解码渲染；非 UTF-8 文本（如二进制）跳过渲染原样打包。
- 渲染在 `build_template_context(user)` 中一次性构建上下文，节点仅取 `is_active` 的绑定节点。

### 示例模板

```yaml
# 客户端配置模板 config.yaml
client:
  user: "{{name}}"
  uuid: "{{uuid}}"
  password: "{{password}}"
  subscription_token: "{{token}}"

# 该用户的节点列表
nodes:
{{node_list_yaml}}

# 或直接嵌入完整 sing-box outbounds
# outbounds:
# {{outbounds_yaml}}
```

## 6. 前端

管理面板新增「文件分发 (Files)」tab：
- 上传弹窗：文件选择、类型（自动/APK/ZIP）、ZIP 模板文件名（可留空自动识别）、显示名、备注。
- 文件表格：ID / 名称(+备注) / 类型 / 大小 / 模板文件 / 状态灯 / 下载链接（用户下拉 + 一键复制带 token 链接）/ 操作（启停、删除）。
- 删除复用鼠标位置 Popover 二次确认。

## 7. 测试

`uv run python test_file_dist.py`（23 项断言，覆盖）：
1. APK 上传 → 内容原样下载 + Content-Disposition。
2. ZIP 上传 → 自动识别模板 → 下载后模板按用户渲染、公共文件原样。
3. 无效 token 401、停用文件 404、列表/删除/磁盘清理、管理端无 token 401。

回归：`uv run python test_verification.py`、`test_config_override.py`、`test_e2e_http.py` 均通过。

### 文本组件 (file_type=text)

- 提交 `content_text` 字段即可：内容存为 UTF-8 文本文件，`original_name` 取 `name`（无后缀自动补 `.txt`）。
- **死字符原样分发**：下载时不做任何渲染/替换，所有用户拿到完全相同的内容，`text/plain` 附件返回。
- 适合：静态配置文件、安装说明、公告等需要逐字分发场景。

## 8. 版本

- pyproject: 0.7.0 → 0.7.1
- FastAPI app version: 0.9.0 → 0.9.1
- 修正：text 类型改为死字符原样分发（不渲染占位符/不替换凭证）
- 新依赖：`python-multipart`（上传）、`pyyaml`（YAML 渲染）；dev 依赖：`httpx2`（TestClient）
