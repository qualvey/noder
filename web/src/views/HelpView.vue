<!--HelpView.vue -->
<script setup lang="ts">
// 使用指引与 API 说明
import { useI18n } from 'vue-i18n'
import { apiLinkPrefix } from '../utils'

const { t } = useI18n()
const origin = location.origin + apiLinkPrefix()
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">{{ t('help.sectionTitle') }}</div>
    </div>
    <div style="background: var(--bg-card); padding: 24px; border-radius: var(--radius-lg); border: 1px solid var(--border-glass)">
      <h4 style="margin-bottom: 12px; color: var(--primary)">{{ t('help.item1Title') }}</h4>
      <div class="code-viewer" style="margin-bottom: 8px">{{ origin }}/sub?token={USER_TOKEN}</div>
      <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 16px">{{ t('help.item1Desc') }}</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">{{ t('help.item1bTitle') }}</h4>
      <div class="code-viewer" style="margin-bottom: 8px">{{ origin }}/mihomo?token={USER_TOKEN}</div>
      <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 16px">{{ t('help.item1bDesc') }}</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">{{ t('help.item2Title') }}</h4>
      <div class="code-viewer" style="margin-bottom: 16px">{{ origin }}/node?token={USER_TOKEN}</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">{{ t('help.item3Title') }}</h4>
      <div class="code-viewer" style="margin-bottom: 16px">{{ origin }}/api/user/verify?token={USER_TOKEN}</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">{{ t('help.item4Title') }}</h4>
      <div class="code-viewer" style="margin-bottom: 8px">{{ origin }}/dl/{file_id}?token={&#123; {{ t('help.item4TokenHint') }} &#125;&#125;}</div>
      <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 16px" v-html="t('help.item4Desc')"></div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">{{ t('help.item4bTitle') }}</h4>
      <div class="code-viewer" style="margin-bottom: 8px">GET /api/settings/shared-token &nbsp; POST /api/settings/shared-token/reset</div>
      <div style="font-size: 0.8rem; color: var(--text-muted); margin-bottom: 16px">{{ t('help.item4bDesc') }}</div>

      <h4 style="margin-bottom: 12px; color: var(--primary)">{{ t('help.item5Title') }}</h4>
      <div class="code-viewer" style="margin-bottom: 8px">GET/POST /api/nodes &nbsp; PUT/DELETE /api/nodes/{id}</div>
      <div class="code-viewer" style="margin-bottom: 8px">GET/POST /api/users &nbsp; PUT/DELETE /api/users/{id}</div>
      <div class="code-viewer" style="margin-bottom: 8px">GET/POST /api/files &nbsp; PUT/DELETE /api/files/{id} &nbsp; POST /api/files/{id}/refresh</div>
      <div class="code-viewer">{{ t('help.swaggerDoc') }}: {{ origin }}/docs</div>

      <p style="color: var(--text-muted); font-size: 0.9rem; margin-top: 16px" v-html="t('help.vlessNote')"></p>
    </div>
  </section>
</template>

