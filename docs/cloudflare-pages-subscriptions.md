# Cloudflare Pages 短期订阅

节点转换仍然在浏览器本地完成；用户主动点击“生成链接”时，才会把生成的 sing-box JSON 上传到 Pages Function。

## 部署 KV

```bash
wrangler kv namespace create SUBSCRIPTIONS
```

将命令输出的 `id` 填入 `web/wrangler.toml` 的 `[[kv_namespaces]]` 配置，然后部署：

```bash
pnpm --dir web install --frozen-lockfile
pnpm --dir web run build:pages
wrangler pages deploy web/pages-dist --project-name noder
```

Pages Functions 会随项目部署，提供：

- `POST /api/subscriptions`：创建临时订阅，支持 1 小时至 7 天
- `GET /sub/:token`：获取订阅 JSON

KV 使用 TTL 自动清理数据。服务端限制单个配置大小为 256 KiB，链接本身不包含节点配置。生产环境建议在 Cloudflare 控制台继续配置 WAF 或 Rate Limiting，避免接口被滥用。
