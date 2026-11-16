<script setup lang="ts">
import { computed, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Node } from '../types'
import { randomUUID } from '../utils'
import { useNodeDropdown } from '../composables/useNodeDropdown'
import type { EndpointFormItem } from './UserFormEndpoints.vue'

const props = defineProps<{
  endpoint: EndpointFormItem
  index: number
  isFirst: boolean
  isLast: boolean
  availableNodes: Node[]
}>()

const emit = defineEmits<{
  (e: 'update', patch: Partial<EndpointFormItem>): void
  (e: 'remove'): void
  (e: 'move-top'): void
  (e: 'move-up'): void
  (e: 'move-down'): void
}>()

const { t } = useI18n()

function nodeDefaultTag(node: Node) {
  return node.node_name || node.tag || `Node-${node.id}`
}

function nodeLabel(node: Node) {
  return `${nodeDefaultTag(node)} (${node.protocol.toUpperCase()})`
}

const selectedLabel = computed(() => {
  const node = props.availableNodes.find((n) => n.id === props.endpoint.node_id)
  return node ? nodeLabel(node) : `Node-${props.endpoint.node_id}`
})

// ---- node-selector dropdown (state machine extracted to composable) ----
const {
  dropdownOpen,
  searchText,
  inputRef,
  filteredOptions,
  dropdownStyle,
  openDropdown,
  onSearchInput,
  chooseNode,
} = useNodeDropdown(toRef(props, 'availableNodes'), {
  onPick: (node) => {
    emit('update', {
      node_id: node.id,
      tag: props.endpoint.tagManuallyEdited ? props.endpoint.tag : nodeDefaultTag(node),
    })
  },
})
</script>

<template>
  <div
    class="endpoint-card"
    :class="{ 'is-primary': isFirst, 'is-node-select-open': dropdownOpen }"
  >
    <div class="endpoint-card-row1">
      <div class="endpoint-rank-wrap">
        <span class="endpoint-rank" :class="{ 'rank-first': isFirst }">#{{ index + 1 }}</span>
        <span v-if="isFirst" class="primary-badge">{{ t('users.form.primaryNodeBadge') }}</span>
      </div>

      <div class="endpoint-node-select" @keydown.esc="dropdownOpen = false">
        <input
          ref="inputRef"
          class="form-control form-control-sm"
          :value="dropdownOpen ? searchText : selectedLabel"
          :placeholder="t('users.form.endpointNodePlaceholder')"
          @focus="openDropdown"
          @input="onSearchInput"
        />
      </div>

      <div class="endpoint-tag-input">
        <input
          :value="endpoint.tag"
          class="form-control form-control-sm"
          :placeholder="t('users.form.endpointTagPlaceholder')"
          required
          @input="
            emit('update', { tag: ($event.target as HTMLInputElement).value, tagManuallyEdited: true })
          "
        />
      </div>

      <div class="endpoint-actions">
        <button
          v-if="!isFirst"
          type="button"
          class="btn-sort-action btn-pin"
          :title="t('users.form.pinToTop')"
          @click="emit('move-top')"
        >
          📌
        </button>
        <button
          type="button"
          class="btn-sort-action"
          :disabled="isFirst"
          :title="t('users.form.moveUp')"
          @click="emit('move-up')"
        >
          ↑
        </button>
        <button
          type="button"
          class="btn-sort-action"
          :disabled="isLast"
          :title="t('users.form.moveDown')"
          @click="emit('move-down')"
        >
          ↓
        </button>
        <button
          type="button"
          class="btn-sort-action btn-remove"
          :title="t('users.form.removeNode')"
          @click="emit('remove')"
        >
          ✕
        </button>
      </div>
    </div>

    <div class="endpoint-card-row2">
      <label class="endpoint-active-toggle">
        <input
          :checked="endpoint.is_active"
          type="checkbox"
          @change="emit('update', { is_active: ($event.target as HTMLInputElement).checked })"
        />
        {{ t('users.form.endpointActive') }}
      </label>
      <label class="endpoint-urltest-toggle">
        <input
          :checked="endpoint.is_urltest"
          type="checkbox"
          @change="emit('update', { is_urltest: ($event.target as HTMLInputElement).checked })"
        />
        {{ t('users.form.endpointUrltest') }}
      </label>

      <div class="endpoint-uuid-wrap">
        <span class="field-label-sm">UUID</span>
        <input
          :value="endpoint.uuid"
          class="form-control form-control-sm code-font"
          :placeholder="t('users.form.endpointUuidPlaceholder')"
          @input="emit('update', { uuid: ($event.target as HTMLInputElement).value })"
        />
        <button
          type="button"
          class="btn btn-secondary btn-sm btn-dice"
          :title="t('users.form.randomUuid')"
          @click="emit('update', { uuid: randomUUID() })"
        >
          🎲
        </button>
      </div>

      <div class="endpoint-pwd-wrap">
        <span class="field-label-sm">PWD</span>
        <input
          :value="endpoint.password"
          class="form-control form-control-sm code-font"
          :placeholder="t('users.form.endpointPassword')"
          @input="emit('update', { password: ($event.target as HTMLInputElement).value })"
        />
      </div>
    </div>
  </div>

  <Teleport to="body">
    <div v-if="dropdownOpen" class="endpoint-node-options" :style="dropdownStyle">
      <button
        v-for="node in filteredOptions"
        :key="node.id"
        type="button"
        class="endpoint-node-option"
        @mousedown.prevent="chooseNode(node)"
      >
        {{ nodeLabel(node) }}
      </button>
      <span v-if="!filteredOptions.length" class="endpoint-node-empty">
        {{ t('common.noResults') }}
      </span>
    </div>
  </Teleport>
</template>

<style scoped>
.endpoint-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.2s ease;
}
.endpoint-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(255, 255, 255, 0.15);
}
.endpoint-card.is-primary {
  border-color: rgba(59, 130, 246, 0.45);
  background: rgba(59, 130, 246, 0.07);
}
.endpoint-card.is-node-select-open {
  position: relative;
  z-index: 100;
}
.endpoint-card-row1 {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.endpoint-rank-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}
.endpoint-rank {
  font-family: var(--font-mono);
  font-size: 0.75rem;
  color: var(--text-muted);
  width: 22px;
}
.endpoint-rank.rank-first {
  color: var(--primary);
  font-weight: 700;
}
.primary-badge {
  font-size: 0.68rem;
  padding: 1px 6px;
  border-radius: 4px;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  white-space: nowrap;
}
.endpoint-node-select {
  flex: 1.2;
  min-width: 170px;
  position: relative;
}
.endpoint-node-options {
  position: fixed;
  z-index: 10000;
  overflow-y: auto;
  padding: 4px;
  border: 1px solid var(--border-glass);
  border-radius: 6px;
  background: var(--bg-card, #172033);
  box-shadow: 0 8px 20px rgba(0, 0, 0, 0.35);
  box-sizing: border-box;
}
.endpoint-node-option {
  display: block;
  width: 100%;
  padding: 7px 8px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  color: var(--text-primary);
  text-align: left;
  font-size: 0.78rem;
  cursor: pointer;
}
.endpoint-node-option:hover {
  background: rgba(59, 130, 246, 0.2);
}
.endpoint-node-empty {
  display: block;
  padding: 8px;
  color: var(--text-muted);
  font-size: 0.78rem;
}
.endpoint-tag-input {
  flex: 1.3;
  min-width: 170px;
}
.endpoint-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
}
.endpoint-card-row2 {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-top: 4px;
  border-top: 1px dashed rgba(255, 255, 255, 0.05);
  flex-wrap: wrap;
}
.endpoint-active-toggle,
.endpoint-urltest-toggle {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  white-space: nowrap;
  font-size: 0.75rem;
  cursor: pointer;
}
.endpoint-active-toggle {
  color: var(--text-main);
}
.endpoint-urltest-toggle {
  color: var(--text-muted);
}
.endpoint-uuid-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1.8;
  min-width: 240px;
}
.endpoint-pwd-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 130px;
}
.field-label-sm {
  font-size: 0.68rem;
  font-weight: 700;
  color: var(--text-muted);
  width: 28px;
  flex-shrink: 0;
}
.code-font {
  font-family: var(--font-mono);
  font-size: 0.75rem !important;
}
.form-control-sm {
  padding: 4px 8px;
  font-size: 0.8rem;
  height: 28px;
}
.btn-dice {
  padding: 2px 7px;
  font-size: 0.8rem;
  height: 28px;
  line-height: 1;
  flex-shrink: 0;
}
.btn-sort-action {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-base);
  border-radius: 4px;
  padding: 2px 7px;
  font-size: 0.75rem;
  height: 26px;
  cursor: pointer;
  transition: all 0.15s ease;
}
.btn-sort-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.3);
}
.btn-sort-action:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}
.btn-pin {
  background: rgba(59, 130, 246, 0.2);
  border-color: rgba(59, 130, 246, 0.4);
  color: #93c5fd;
  font-weight: 600;
}
.btn-pin:hover {
  background: rgba(59, 130, 246, 0.35) !important;
  color: #fff !important;
}
.btn-remove {
  color: var(--accent-rose);
}
.btn-remove:hover {
  background: rgba(239, 68, 68, 0.2) !important;
  border-color: rgba(239, 68, 68, 0.4) !important;
}
</style>
