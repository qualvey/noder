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
  parsedConfigs,
  activeConfig,
  activeNodeIndex,
  nodeCount,
  parseError,
  allShareLinksText,
  allShareJsonText,
  activeShareLink,
  configDetails,
  handlePasteSample,
  handleClear,
  handleDirectionChange,
  handleFormat,
  handleCopy,
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
        <p>在 Sing-box JSON 与 VLESS / TUIC 等分享链接之间快速转换，支持单/多节点批量解析。</p>
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

      <!-- 输出区域 -->
      <section class="transfer-card output-card">
        <div class="card-heading">
          <div class="flex items-center gap-2">
            <h3>
              {{
                conversionDirection === 'json2link'
                  ? `${nodeCount > 1 ? '批量' : (activeConfig?.type as string)?.toUpperCase() || '节点'} 标准链接`
                  : 'Sing-box 配置 JSON'
              }}
            </h3>
            <span v-if="nodeCount > 0 && !parseError" class="valid-badge">
              {{ nodeCount > 1 ? `已识别 ${nodeCount} 个节点` : '已识别' }}
            </span>
          </div>
          <!-- 多节点预览切换器 -->
          <div v-if="nodeCount > 1" class="node-selector">
            <label for="node-select">当前节点:</label>
            <select id="node-select" v-model="activeNodeIndex">
              <option v-for="(cfg, idx) in parsedConfigs" :key="idx" :value="idx">
                #{{ idx + 1 }} {{ cfg.tag || cfg.server }} ({{ (cfg.type as string).toUpperCase() }})
              </option>
            </select>
          </div>
        </div>

        <div v-if="nodeCount > 0 && !parseError" class="output-content">
          <div class="result-box">
            <textarea
              readonly
              :value="conversionDirection === 'json2link' ? allShareLinksText : allShareJsonText"
            ></textarea>
            <button
              @click="
                handleCopy(conversionDirection === 'json2link' ? allShareLinksText : allShareJsonText)
              "
            >
              {{ nodeCount > 1 ? '复制全部' : '复制' }}
            </button>
          </div>
          <div class="output-meta">
            <div v-if="activeShareLink" class="qr-box">
              <qrcode-vue :value="activeShareLink" :size="126" level="L" render-as="svg" />
              <small>{{ nodeCount > 1 ? `扫码导入 (#${activeNodeIndex + 1})` : '扫码导入' }}</small>
            </div>
            <div class="summary text-2xl">
              <dl class="w-full">
                <div class="grid grid-cols-3 gap-4 w-full">
                  <div>
                    <dt>节点名称</dt>
                    <dd>{{ activeConfig?.tag || '未命名' }}</dd>
                  </div>
                  <div>
                    <dt>协议</dt>
                    <dd class="accent">
                      {{ (activeConfig?.type as string)?.toUpperCase() }}
                    </dd>
                  </div>
                  <div>
                    <dt>服务器</dt>
                    <dd>{{ activeConfig?.server }}:{{ activeConfig?.server_port }}</dd>
                  </div>
                </div>
              </dl>
            </div>
          </div>
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

.node-selector {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.68rem;
  color: var(--text-muted);
}

.node-selector select {
  padding: 4px 8px;
  border: 1px solid var(--border-glass);
  border-radius: 6px;
  background: var(--bg-soft);
  color: var(--text-main);
  font-size: 0.68rem;
  outline: none;
  cursor: pointer;
}

.node-selector select:focus {
  border-color: var(--primary);
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

.editor-wrap textarea,
.result-box textarea {
  display: block;
  width: 100%;
  height: fit-content;
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

.editor-wrap textarea {
  height: 340px;
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

.output-content {
  display: grid;
  gap: 14px;
}

.result-box {
  position: relative;
}

.result-box textarea {
  height: 340px;
  min-height: 220px;
  padding-right: 70px;
}

.result-box button {
  position: absolute;
  top: 8px;
  right: 8px;
  border: 0;
  border-radius: 6px;
  padding: 6px 9px;
  background: var(--primary);
  color: white;
  cursor: pointer;
  font: inherit;
  font-size: 0.65rem;
  font-weight: 700;
}

.output-meta {
  display: grid;
  grid-template-columns: 150px minmax(0, 1fr);
  gap: 12px;
}

.qr-box,
.summary {
  display: grid;
  align-content: center;
  justify-items: center;
  gap: 7px;
  padding: 12px;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  background: var(--bg-input);
}

.qr-box svg {
  max-width: 100%;
  padding: 7px;
  background: white;
}

.qr-box small {
  color: var(--text-dim);
  font-size: 0.62rem;
}

.summary {
  align-content: start;
  justify-items: stretch;
}

.summary dl {
  display: grid;
  gap: 9px;
  margin: 11px 0;
}

.summary dt {
  color: var(--text-dim);
  font-size: 0.62rem;
}

.summary dd {
  overflow: hidden;
  margin: 0;
  color: var(--text-main);
  font: 0.7rem var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
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

  .output-meta {
    grid-template-columns: 1fr;
  }

  .qr-box {
    justify-items: center;
  }

  .details-list > div {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
</style>
