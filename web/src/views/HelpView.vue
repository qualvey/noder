<script setup lang="ts">
// 使用指引与 API 说明
import { useI18n } from 'vue-i18n'
import { apiLinkPrefix } from '../utils'

const { t } = useI18n()
const origin = location.origin + apiLinkPrefix()
const tokenHint = t('help.item4TokenHint')
const dlLink = `${origin}/dl/{file_id}?token={${tokenHint}}`
</script>

<template>
  <section class="tab-content help-tab">
    <div class="section-header">
      <div class="section-title">{{ t('help.sectionTitle') }}</div>
    </div>
    <div class="help-card">
      <h4 class="help-item-title">{{ t('help.item1Title') }}</h4>
      <div class="code-viewer code-viewer-spaced">{{ origin }}/sub?token={USER_TOKEN}</div>
      <div class="help-item-desc">{{ t('help.item1Desc') }}</div>

      <h4 class="help-item-title">{{ t('help.item1bTitle') }}</h4>
      <div class="code-viewer code-viewer-spaced">{{ origin }}/mihomo?token={USER_TOKEN}</div>
      <div class="help-item-desc">{{ t('help.item1bDesc') }}</div>

      <h4 class="help-item-title">{{ t('help.item2Title') }}</h4>
      <div class="code-viewer code-viewer-spaced-lg">{{ origin }}/node?token={USER_TOKEN}</div>

      <h4 class="help-item-title">{{ t('help.item3Title') }}</h4>
      <div class="code-viewer code-viewer-spaced">{{ origin }}/api/user/verify?token={USER_TOKEN}</div>

      <h4 class="help-item-title">{{ t('help.item4Title') }}</h4>
      <div class="code-viewer code-viewer-spaced">{{ dlLink }}</div>
      <div class="help-item-desc">
        <strong>{{ t('help.item4DescZipLabel') }}</strong>{{ t('help.item4DescZipText') }}
        <strong>{{ t('help.item4DescRegularLabel') }}</strong>{{ t('help.item4DescRegularText') }}
      </div>

      <h4 class="help-item-title">{{ t('help.item4bTitle') }}</h4>
      <div class="code-viewer code-viewer-spaced">GET /api/settings/shared-token &nbsp; POST /api/settings/shared-token/reset</div>
      <div class="help-item-desc">{{ t('help.item4bDesc') }}</div>

      <h4 class="help-item-title">{{ t('help.item5Title') }}</h4>
      <div class="code-viewer code-viewer-spaced">GET/POST /api/nodes &nbsp; PUT/DELETE /api/nodes/{id}</div>
      <div class="code-viewer code-viewer-spaced">GET/POST /api/users &nbsp; PUT/DELETE /api/users/{id}</div>
      <div class="code-viewer code-viewer-spaced">GET/POST /api/files &nbsp; PUT/DELETE /api/files/{id} &nbsp; POST /api/files/{id}/refresh</div>
      <div class="code-viewer">{{ t('help.swaggerDoc') }}: {{ origin }}/docs</div>

      <p class="help-footnote">{{ t('help.vlessNote') }}</p>
    </div>
  </section>
</template>

<style scoped>
.help-tab {
  display: block;
}
.help-card {
  background: var(--bg-card);
  padding: 24px;
  border-radius: var(--radius-lg);
  border: 1px solid var(--border-glass);
}
.help-item-title {
  margin-bottom: 12px;
  color: var(--primary);
}
.help-item-desc {
  font-size: 0.8rem;
  color: var(--text-muted);
  margin-bottom: 16px;
}
.code-viewer-spaced {
  margin-bottom: 8px;
}
.code-viewer-spaced-lg {
  margin-bottom: 16px;
}
.help-footnote {
  color: var(--text-muted);
  font-size: 0.9rem;
  margin-top: 16px;
}
</style>
