<script setup lang="ts">
import { computed } from 'vue'
import QrcodeVue from 'qrcode.vue'
import type { TransferNodeResult } from '../composables/useNodeTransfer'

export type OutputTab = 'primary' | 'xray' | 'dae'

const props = defineProps<{
  item: TransferNodeResult
  active: boolean
  outputTab: OutputTab
  direction: 'json2link' | 'link2json'
}>()

const emit = defineEmits<{
  (e: 'select'): void
  (e: 'copy', text: string, message: string): void
}>()

const displayText = computed(() => {
  if (props.outputTab === 'xray') return props.item.xrayJson || props.item.xrayError
  if (props.outputTab === 'dae') return props.item.daeNode
  return props.direction === 'json2link' ? props.item.shareLink : props.item.shareJson
})

const canCopy = computed(() => {
  if (props.outputTab === 'xray') return !!props.item.xrayJson
  if (props.outputTab === 'dae') return !!props.item.daeNode
  return !!(props.direction === 'json2link' ? props.item.shareLink : props.item.shareJson)
})

function copyThis() {
  if (!displayText.value) return
  emit('copy', displayText.value, `已复制节点 #${props.item.index}`)
}
</script>

<template>
  <article
    class="node-result-item"
    :class="{ active }"
    @click="emit('select')"
  >
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

    <div class="node-item-body">
      <div class="node-text-col">
        <textarea readonly :value="displayText"></textarea>
        <button
          v-if="canCopy"
          class="node-copy-btn"
          title="复制此节点"
          @click.stop="copyThis"
        >
          复制
        </button>
      </div>

      <div class="node-qr-col">
        <div class="node-qr-inner">
          <qrcode-vue
            :value="item.shareLink"
            :size="104"
            level="L"
            render-as="svg"
          />
          <small>{{ outputTab === 'xray' ? '分享链接二维码' : '扫码导入' }}</small>
        </div>
      </div>
    </div>
  </article>
</template>

<style scoped>
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
@media (max-width: 520px) {
  .node-item-body {
    grid-template-columns: 1fr;
  }
}
</style>
