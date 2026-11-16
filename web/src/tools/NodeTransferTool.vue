<script setup lang="ts">
import { inject, ref } from 'vue'
import { useNodeTransfer } from '../composables/useNodeTransfer'
import TransferNodeCard from './TransferNodeCard.vue'
import TransferInputCard from './TransferInputCard.vue'
import TransferDetailsCard from './TransferDetailsCard.vue'
import { ToastKey } from '../injectionKeys'

const toast = inject(ToastKey)

const {
  inputText,
  supportedHandlers,
  conversionDirection,
  nodeResults,
  activeConfig,
  activeNodeIndex,
  nodeCount,
  parseError,
  configDetails,
  handlePasteSample,
  handleClear,
  handleDirectionChange,
  handleFormat,
  handleCopy,
  handleCopyAll,
  allXrayJsonText,
  allSingBoxJsonText,
  allDaeText,
  parseWarnings,
} = useNodeTransfer({
  onSuccess: (msg) => toast?.(msg, 'success'),
  onError: (msg) => toast?.(msg, 'error'),
})

const activeOutputTab = ref<'primary' | 'xray' | 'dae'>('primary')

function downloadSingBox() {
  if (!allSingBoxJsonText.value) return
  const blob = new Blob([allSingBoxJsonText.value], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = 'sing-box-config.json'
  link.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <section class="transfer-tool">
    <header class="tool-intro">
      <div>
        <h2>节点配置转换</h2>
        <p>在 Sing-box JSON 与 VLESS / TUIC 等分享链接之间转换，支持多节点独立解析与扫码。</p>
      </div>
      <div class="flex flex-1 justify-center">
        <div class="direction-switch">
          <button
            :class="{ active: conversionDirection === 'json2link' }"
            @click="handleDirectionChange('json2link')"
          >
            JSON <b>&rarr;</b> 分享链接
          </button>
          <button
            :class="{ active: conversionDirection === 'link2json' }"
            @click="handleDirectionChange('link2json')"
          >
            分享链接 <b>&rarr;</b> JSON
          </button>
        </div>
      </div>
      <div class="flex-1"></div>
    </header>

    <div v-if="parseWarnings.length" class="parse-warning" role="status">
      <strong>{{ parseWarnings.length }} 条输入未处理</strong>
      <span v-for="warning in parseWarnings" :key="warning">{{ warning }}</span>
    </div>

    <div
      v-if="parseError"
      class="parse-error flex-2 w-fit"
      role="alert"
      aria-live="polite"
    >
      <span class="parse-error-icon">!</span>
      <div>
        <strong>解析配置有误</strong><span>{{ parseError }}</span>
      </div>
    </div>

    <div class="transfer-grid">
      <TransferInputCard
        :direction="conversionDirection"
        :input-text="inputText"
        :supported-handlers="supportedHandlers"
        @update:input-text="(val) => (inputText = val)"
        @paste-sample="handlePasteSample"
        @format="handleFormat"
        @clear="handleClear"
      />

      <!-- 输出区域 (每个节点分开展示：独立文本 + 独立二维码) -->
      <section class="transfer-card output-card">
        <div class="card-heading">
          <div class="flex items-center gap-2">
            <h3>转换结果</h3>
            <span v-if="nodeCount > 0 && !parseError" class="valid-badge">
              {{ nodeCount > 1 ? `共 ${nodeCount} 个节点` : '已识别' }}
            </span>
          </div>
          <div v-if="nodeCount > 0 && !parseError" class="card-actions">
            <button class="copy-all-btn" @click="handleCopy(allSingBoxJsonText, '已复制完整 sing-box 配置')">
              复制完整配置
            </button>
            <button class="copy-all-btn" @click="downloadSingBox">下载 JSON</button>
            <button
              class="copy-all-btn"
              @click="
                activeOutputTab === 'xray'
                  ? handleCopy(allXrayJsonText, '已复制全部 Xray 配置')
                  : activeOutputTab === 'dae'
                    ? handleCopy(allDaeText, '已复制 DAE node 配置')
                    : handleCopyAll()
              "
            >
              {{
                activeOutputTab === 'xray'
                  ? '复制全部 Xray 配置'
                  : activeOutputTab === 'dae'
                    ? '复制 DAE node'
                    : nodeCount > 1
                      ? `复制全部 (${nodeCount})`
                      : '复制'
              }}
            </button>
          </div>
        </div>

        <nav v-if="nodeCount > 0 && !parseError" class="output-tabs" aria-label="转换结果格式">
          <button
            :class="{ active: activeOutputTab === 'primary' }"
            @click="activeOutputTab = 'primary'"
          >
            {{ conversionDirection === 'json2link' ? '分享链接' : 'Sing-box JSON' }}
          </button>
          <button
            :class="{ active: activeOutputTab === 'xray' }"
            @click="activeOutputTab = 'xray'"
          >
            Xray VLESS
          </button>
          <button
            :class="{ active: activeOutputTab === 'dae' }"
            @click="activeOutputTab = 'dae'"
          >
            DAE node
          </button>
        </nav>

        <!-- 多节点结果列表 (每个节点：文本 + 二维码分开) -->
        <div v-if="nodeCount > 0 && !parseError" class="node-results-list">
          <TransferNodeCard
            v-for="(item, idx) in nodeResults"
            :key="item.index"
            :item="item"
            :active="activeNodeIndex === idx"
            :output-tab="activeOutputTab"
            :direction="conversionDirection"
            @select="activeNodeIndex = idx"
            @copy="handleCopy"
          />
        </div>

        <div v-else class="empty-output">
          <span>◎</span>
          <p>输入有效配置后，结果会显示在这里</p>
        </div>
      </section>
    </div>

    <TransferDetailsCard
      v-if="activeConfig && !parseError"
      :node-count="nodeCount"
      :active-node-index="activeNodeIndex"
      :active-tag="activeConfig?.tag || ''"
      :active-server="activeConfig?.server || ''"
      :details="configDetails"
      @copy="handleCopy"
    />
  </section>
</template>

<style scoped>
.transfer-tool {
  display: grid;
  gap: 18px;
  max-width: 1180px;
  margin: 0 auto;
  color: var(--text-main);
}

.tool-intro {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.tool-intro h2 {
  margin: 5px 0;
  font-size: 1.2rem;
}

.tool-intro p {
  margin: 0;
  color: var(--text-muted);
  font-size: 0.74rem;
}

.valid-badge {
  padding: 4px 7px;
  border-radius: 6px;
  background: color-mix(in srgb, var(--primary) 12%, transparent);
  color: var(--primary);
  font-size: 0.62rem;
  font-weight: 800;
  white-space: nowrap;
}

.direction-switch {
  display: flex;
  gap: 4px;
  width: fit-content;
  padding: 4px;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  background: var(--bg-soft);
}

.direction-switch button,
.card-actions button {
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
  font-size: 0.68rem;
  font-weight: 700;
}

.direction-switch button {
  padding: 8px 13px;
}

.direction-switch button.active {
  background: var(--bg-elevated);
  color: var(--primary);
  box-shadow: 0 2px 8px rgba(2, 8, 23, 0.14);
}

.direction-switch b {
  margin: 0 4px;
  color: var(--text-dim);
}

.copy-all-btn {
  background: color-mix(in srgb, var(--primary) 14%, transparent) !important;
  color: var(--primary) !important;
  border-color: color-mix(in srgb, var(--primary) 30%, transparent) !important;
}

.transfer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.transfer-card {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--border-glass);
  border-radius: 14px;
  background: var(--bg-panel);
}

.output-card {
  min-height: 460px;
}

.card-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 15px;
}
.card-heading h3 {
  margin: 0;
  font-size: 0.9rem;
}
.card-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
}

.card-actions button {
  padding: 6px 8px;
  border-color: var(--border-glass);
}

.card-actions button:hover:not(:disabled) {
  border-color: var(--primary);
  color: var(--primary);
}

.card-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.output-tabs {
  display: flex;
  gap: 4px;
  margin: -3px 0 12px;
  padding: 3px;
  border-radius: 8px;
  background: var(--bg-elevated);
}

.parse-warning {
  display: grid;
  gap: 4px;
  margin-bottom: 14px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--accent-amber) 55%, transparent);
  border-radius: 9px;
  background: color-mix(in srgb, var(--accent-amber) 10%, transparent);
  color: var(--accent-amber);
  font-size: 0.75rem;
}

.parse-warning span { color: var(--text-muted); }

.output-tabs button {
  flex: 1;
  padding: 7px 10px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font: inherit;
  font-size: 0.72rem;
  font-weight: 700;
}

.output-tabs button.active {
  background: var(--bg-panel);
  color: var(--primary);
  box-shadow: 0 1px 4px rgba(2, 8, 23, 0.12);
}

.parse-error {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin: 0 0 10px;
  padding: 9px 11px;
  border: 1px solid color-mix(in srgb, var(--accent-rose) 35%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--accent-rose) 10%, var(--bg-panel));
  color: var(--accent-rose);
  font-size: 0.68rem;
}

.parse-error-icon {
  display: grid;
  flex: 0 0 17px;
  place-items: center;
  width: 17px;
  height: 17px;
  border-radius: 50%;
  background: var(--accent-rose);
  color: white;
  font-size: 0.65rem;
  font-weight: 800;
}

.parse-error div {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.parse-error span:last-child {
  overflow-wrap: anywhere;
  line-height: 1.4;
}

/* 独立节点结果列表 */
.node-results-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 640px;
  overflow-y: auto;
  padding-right: 4px;
}

.empty-output {
  display: grid;
  min-height: 320px;
  place-items: center;
  align-content: center;
  gap: 10px;
  border: 1px dashed var(--border-glass);
  border-radius: 9px;
  color: var(--text-dim);
}

.empty-output span {
  font-size: 2rem;
  color: var(--primary);
}

.empty-output p {
  margin: 0;
  font-size: 0.7rem;
}

@media (max-width: 820px) {
  .transfer-grid {
    grid-template-columns: 1fr;
  }

  .output-card {
    min-height: 0;
  }
}

@media (max-width: 520px) {
  .tool-intro {
    align-items: flex-start;
    flex-direction: column;
  }

  .direction-switch {
    width: 100%;
  }

  .direction-switch button {
    flex: 1;
    padding-inline: 6px;
  }

  .card-heading {
    flex-direction: column;
    align-items: flex-start;
  }

  .card-actions {
    justify-content: flex-start;
  }
}
</style>
