<!--HelpView.vue -->
<script setup lang="ts">
// 使用指引与 API 说明
import { apiLinkPrefix } from '../utils'

const origin = location.origin + apiLinkPrefix()
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">Sing-Box 客户端导入指南 & 用户 API</div>
    </div>
    <div style="background: var(--bg-card); padding: 24px; border-radius: var(--radius-lg); border: 1px solid var(--border-glass)">
      <h4 style="margin-bottom: 12px; color: var(--primary)">1. 用户订阅导出 API</h4>
      <div class="code-viewer" style="margin-bottom: 8px">{{ origin }}/sub?token={USER_TOKEN}</div>
      <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 16px">Sing-Box 全量配置 JSON</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">1b. Mihomo (Clash) 完整配置 API</h4>
      <div class="code-viewer" style="margin-bottom: 8px">{{ origin }}/mihomo?token={USER_TOKEN}</div>
      <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 16px">完整配置 YAML：data/mihomo.yml 模板 + 节点注入 + 策略组接线（vless+reality+vision / tuic v5）</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">2. 用户节点列表查询 API</h4>
      <div class="code-viewer" style="margin-bottom: 16px">{{ origin }}/node?token={USER_TOKEN}</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">3. 用户 Token 验证与数据查询 API</h4>
      <div class="code-viewer" style="margin-bottom: 16px">{{ origin }}/api/user/verify?token={USER_TOKEN}</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">4. 文件分发下载 API (Token 鉴权)</h4>
      <div class="code-viewer" style="margin-bottom: 16px">{{ origin }}/dl/{file_id}?token={USER_TOKEN}</div>
      <p style="color: var(--text-muted); font-size: 0.9rem">
        APK：全员公用原样下载；ZIP：模板按用户渲染后打包；text：死字符原样分发。
      </p>

      <h4 style="margin-bottom: 12px; color: var(--primary)">5. 管理员 API (Header: X-Admin-Token)</h4>
      <div class="code-viewer" style="margin-bottom: 8px">GET/POST /api/nodes &nbsp; PUT/DELETE /api/nodes/{id}</div>
      <div class="code-viewer" style="margin-bottom: 8px">GET/POST /api/users &nbsp; PUT/DELETE /api/users/{id}</div>
      <div class="code-viewer" style="margin-bottom: 8px">GET/POST /api/files &nbsp; PUT/DELETE /api/files/{id} &nbsp; POST /api/files/{id}/refresh</div>
      <div class="code-viewer">Swagger 文档: {{ origin }}/docs</div>

      <p style="color: var(--text-muted); font-size: 0.9rem; margin-top: 16px">
        系统已全面兼容 doc/vless.md 定义的 VLESS REALITY 规格：支持 <code>public_key</code>、<code>short_id</code>、<code>flow: xtls-rprx-vision</code> 与 <code>uTLS: chrome</code> 指纹！
      </p>
    </div>
  </section>
</template>
