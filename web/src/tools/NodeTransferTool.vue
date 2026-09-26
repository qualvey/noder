<script setup lang="ts">
import { inject } from 'vue'
import QrcodeVue from 'qrcode.vue'
import { useNodeTransfer } from '../composables/useNodeTransfer'

const toast =
  inject<(message: string, type?: 'info' | 'error' | 'success' | 'warning') => void>('toast')

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
} = useNodeTransfer({
  onSuccess: (msg) => toast?.(msg, 'success'),
  onError: (msg) => toast?.(msg, 'error'),
})
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
      <!-- 输入区域 -->
      <section class="transfer-card input-card">
        <div class="card-heading">
          <div>
            <h3>
              {{
                conversionDirection === 'json2link'
                  ? 'Sing-box 配置 JSON'
                  : 'VLESS / TUIC 分享链接'
              }}
            </h3>
          </div>
          <div class="card-actions">
            <button
              v-for="handler in supportedHandlers"
              :key="handler.protocol"
              @click="handlePasteSample(handler.protocol)"
            >
              {{
                conversionDirection === 'json2link'
                  ? `${handler.displayName} 示例`
                  : `${handler.displayName} 链接`
              }}
            </button>
            <button
              v-if="conversionDirection === 'json2link'"
              :disabled="!inputText"
              @click="handleFormat"
            >
              格式化
            </button>
            <button :disabled="!inputText" @click="handleClear">清空</button>
          </div>
        </div>

        <div class="editor-wrap">
          <textarea
            v-model="inputText"
            :placeholder="
              conversionDirection === 'json2link'
                ? '粘贴 sing-box 出站配置 JSON（支持单个对象、数组或逗号分隔的多节点）…'
                : '粘贴分享链接（支持单行或多行 vless://、tuic://）…'
            "
          ></textarea>
        </div>
        <footer>
          <span>{{ inputText.length }} 个字符</span>
          <span>{{
            conversionDirection === 'json2link'
              ? '支持 sing-box 1.18+ / 多节点数组'
              : '支持 vless:// / tuic:// (可多行)'
          }}</span>
        </footer>
      </section>

      <!-- 输出区域 (每个节点分开展示：独立文本 + 独立二维码) -->
      <section class="transfer-card output-card">
        <div class="card-heading">
          <div class="flex items-center gap-2">
            <h3>转换结果</h3>
            <span v-if="nodeCount > 0 && !parseError" class="valid-badge">
              {{ nodeCount > 1 ? `共 ${nodeCount} 个节点` : '已识别' }}
            </span>
          </div>
          <div v-if="nodeCount > 1 && !parseError" class="card-actions">
            <button class="copy-all-btn" @click="handleCopyAll">
              复制全部 ({{ nodeCount }})
            </button>
          </div>
        </div>

        <!-- 多节点结果列表 (每个节点：文本 + 二维码分开) -->
        <div v-if="nodeCount > 0 && !parseError" class="node-results-list">
          <article
            v-for="(item, idx) in nodeResults"
            :key="item.index"
            class="node-result-item"
            :class="{ active: activeNodeIndex === idx }"
            @click="activeNodeIndex = idx"
          >
            <!-- 节点头部 -->
            <header class="node-item-header">
              <div class="node-item-title">
                <span class="node-badge">#{{ item.index }}</span>
                <span class="node-tag">{{ item.config.tag || '未命名节点' }}</span>
                <span class="proto-tag">{{ (item.config.type as string).toUpperCase() }}</span>
              </div>
              <div class="node-item-server">
                {{ item.config.server }}:{{ item.config.server_port }}
              </div>
            </header>

            <!-- 节点主体：文本 + 二维码并排分开 -->
            <div class="node-item-body">
              <!-- 文本框 -->
              <div class="node-text-col">
                <textarea
                  readonly
                  :value="conversionDirection === 'json2link' ? item.shareLink : item.shareJson"
                ></textarea>
                <button
                  class="node-copy-btn"
                  title="复制此节点"
                  @click.stop="
                    handleCopy(
                      conversionDirection === 'json2link' ? item.shareLink : item.shareJson,
                      `已复制节点 #${item.index}`,
                    )
                  "
                >
                  复制
                </button>
              </div>

              <!-- 二维码 -->
              <div class="node-qr-col">
                <div class="node-qr-inner">
                  <qrcode-vue
                    :value="item.shareLink"
                    :size="104"
                    level="L"
                    render-as="svg"
                  />
                  <small>扫码导入</small>
                </div>
              </div>
            </div>
          </article>
        </div>

        <div v-else class="empty-output">
          <span>◎</span>
          <p>输入有效配置后，结果会显示在这里</p>
        </div>
      </section>
    </div>

    <!-- 详细参数列表 (展示当前选中的节点) -->
    <section v-if="activeConfig && !parseError" class="transfer-card details-card">
      <div class="card-heading">
        <div>
          <span class="card-kicker">DETAILS</span>
          <h3>
            配置参数
            <span v-if="nodeCount > 1" class="details-subtitle">
              — #{{ activeNodeIndex + 1 }} {{ activeConfig?.tag || activeConfig?.server }}
            </span>
          </h3>
        </div>
      </div>
      <div class="details-list">
        <div v-for="(detail, i) in configDetails" :key="i">
          <span>{{ detail.label }}</span>
          <strong :class="{ accent: detail.highlight }">{{ detail.value }}</strong>
          <button
            v-if="detail.copyable && detail.value !== '-'"
            @click="handleCopy(String(detail.value))"
          >
            复制
          </button>
        </div>
      </div>
    </section>
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

.card-kicker {
  color: var(--primary);
  font-size: 0.62rem;
  font-weight: 800;
  letter-spacing: 0.13em;
}

.details-subtitle {
  font-size: 0.76rem;
  font-weight: 500;
  color: var(--text-muted);
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

.input-card,
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

.editor-wrap {
  position: relative;
}

.editor-wrap textarea {
  display: block;
  width: 100%;
  height: 340px;
  box-sizing: border-box;
  resize: none;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  padding: 12px;
  outline: none;
  background: var(--bg-input);
  color: var(--text-main);
  font: 0.72rem/1.7 var(--font-mono);
}

.editor-wrap textarea:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent);
}

.editor-wrap textarea::placeholder {
  color: var(--text-dim);
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

.transfer-card footer {
  display: flex;
  justify-content: space-between;
  margin-top: 10px;
  color: var(--text-dim);
  font-size: 0.64rem;
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

.node-result-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border-glass);
  border-radius: 10px;
  background: var(--bg-soft);
  transition: border-color 0.2s, box-shadow 0.2s;
  cursor: pointer;
}

.node-result-item:hover {
  border-color: color-mix(in srgb, var(--primary) 40%, var(--border-glass));
}

.node-result-item.active {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--primary) 18%, transparent);
}

.node-item-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 0.72rem;
}

.node-item-title {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow: hidden;
}

.node-badge {
  flex-shrink: 0;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--bg-panel);
  color: var(--text-dim);
  font: 0.62rem var(--font-mono);
  font-weight: 700;
}

.node-tag {
  font-weight: 700;
  color: var(--text-main);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.proto-tag {
  flex-shrink: 0;
  padding: 2px 6px;
  border-radius: 4px;
  background: color-mix(in srgb, var(--primary) 12%, transparent);
  color: var(--primary);
  font-size: 0.6rem;
  font-weight: 800;
}

.node-item-server {
  font: 0.65rem var(--font-mono);
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 节点主体：文本 + 二维码并排分开 */
.node-item-body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 130px;
  gap: 12px;
}

.node-text-col {
  position: relative;
  display: flex;
}

.node-text-col textarea {
  display: block;
  width: 100%;
  height: 140px;
  box-sizing: border-box;
  resize: none;
  border: 1px solid var(--border-glass);
  border-radius: 8px;
  padding: 10px 58px 10px 10px;
  outline: none;
  background: var(--bg-input);
  color: var(--text-main);
  font: 0.68rem/1.6 var(--font-mono);
}

.node-copy-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  border: 0;
  border-radius: 6px;
  padding: 5px 8px;
  background: var(--primary);
  color: white;
  cursor: pointer;
  font: inherit;
  font-size: 0.62rem;
  font-weight: 700;
}

.node-qr-col {
  display: flex;
  align-items: center;
  justify-content: center;
}

.node-qr-inner {
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 5px;
  width: 100%;
  height: 100%;
  padding: 8px;
  border: 1px solid var(--border-glass);
  border-radius: 8px;
  background: var(--bg-input);
}

.node-qr-inner svg {
  max-width: 100%;
  padding: 5px;
  background: white;
  border-radius: 4px;
}

.node-qr-inner small {
  color: var(--text-dim);
  font-size: 0.6rem;
}

.accent {
  color: var(--primary) !important;
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

.details-card {
  display: grid;
  gap: 3px;
}

.details-list {
  overflow: hidden;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
}

.details-list > div {
  display: grid;
  grid-template-columns: minmax(130px, 0.7fr) minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 9px 11px;
  border-bottom: 1px solid var(--border-glass);
  font-size: 0.68rem;
}

.details-list > div:last-child {
  border-bottom: 0;
}

.details-list span {
  color: var(--text-muted);
}

.details-list strong {
  overflow: hidden;
  font: 0.68rem var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.details-list button {
  border: 0;
  background: none;
  color: var(--primary);
  cursor: pointer;
  font: inherit;
  font-size: 0.62rem;
}

@media (max-width: 820px) {
  .transfer-grid {
    grid-template-columns: 1fr;
  }

  .input-card,
  .output-card {
    min-height: 0;
  }

  .editor-wrap textarea {
    height: 280px;
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

  .node-item-body {
    grid-template-columns: 1fr;
  }

  .details-list > div {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
</style>
