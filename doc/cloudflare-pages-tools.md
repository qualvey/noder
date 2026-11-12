# Cloudflare Pages 工具版

本分支将图片水印、节点配置转换、二维码三个工具抽成独立的纯前端页面。所有图片和配置仅在浏览器内处理，不请求 Noder API。

节点转换工具支持多条 VLESS 分享链接：有效链接会继续处理，重复 tag 自动追加 `-2`、`-3`，错误行单独提示，并可生成完整的 sing-box 客户端 JSON 配置或下载文件。

## Pages 配置

在 Cloudflare Pages 中使用以下设置：

- **Root directory**：`web`
- **Build command**：`pnpm install --frozen-lockfile && pnpm run build:pages`
- **Build output directory**：`pages-dist`
- **Node.js / pnpm**：使用项目锁定文件对应的 pnpm 版本

也可以在仓库根目录执行：

```bash
pnpm --dir web install --frozen-lockfile
pnpm --dir web exec vue-tsc --noEmit
pnpm --dir web exec vite build --config vite.pages.config.ts
```

生成目录为 `web/pages-dist`。

## 与完整 Noder 的关系

该入口使用 `tools/index.html` 和 `ToolsApp.vue`，不会加载管理员登录、SQLite 或 Go API。原有 `web/index.html` 和后端静态资源构建流程保持不变。
