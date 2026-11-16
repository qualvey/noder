<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Node } from '../types'
import UserFormEndpointCard from './UserFormEndpointCard.vue'

export interface EndpointFormItem {
  id?: number
  node_id: number
  tag: string
  tagManuallyEdited?: boolean
  uuid: string
  password?: string
  is_active: boolean
  is_urltest: boolean
}

const props = defineProps<{
  endpoints: EndpointFormItem[]
  availableNodes: Node[]
  defaultUuid?: string
  defaultPassword?: string
}>()

const emit = defineEmits<{
  (e: 'update:endpoints', value: EndpointFormItem[]): void
}>()

const { t } = useI18n()

function endpointKey(ep: EndpointFormItem, index: number) {
  return String(ep.id ?? `new-${index}`)
}

function nodeDefaultTag(node: Node) {
  return node.node_name || node.tag || `Node-${node.id}`
}

function setEndpoints(next: EndpointFormItem[]) {
  emit('update:endpoints', next)
}

function patchEndpoint(index: number, patch: Partial<EndpointFormItem>) {
  setEndpoints(props.endpoints.map((ep, idx) => (idx === index ? { ...ep, ...patch } : ep)))
}

function addEndpoint() {
  const targetNode = props.availableNodes[0]
  if (!targetNode) return
  setEndpoints([
    ...props.endpoints,
    {
      node_id: targetNode.id,
      tag: nodeDefaultTag(targetNode),
      tagManuallyEdited: false,
      uuid: props.defaultUuid || '',
      password: props.defaultPassword || '',
      is_active: true,
      is_urltest: true,
    },
  ])
}

function addAllNodes() {
  setEndpoints([
    ...props.endpoints,
    ...props.availableNodes.map((n) => ({
      node_id: n.id,
      tag: nodeDefaultTag(n),
      tagManuallyEdited: false,
      uuid: props.defaultUuid || '',
      password: props.defaultPassword || '',
      is_active: true,
      is_urltest: true,
    })),
  ])
}

function moveEndpointToTop(index: number) {
  if (index <= 0 || index >= props.endpoints.length) return
  const next = [...props.endpoints]
  const [target] = next.splice(index, 1)
  if (target) next.unshift(target)
  setEndpoints(next)
}

function moveEndpointUp(index: number) {
  if (index <= 0) return
  const next = [...props.endpoints]
  const temp = next[index]
  next[index] = next[index - 1]!
  next[index - 1] = temp!
  setEndpoints(next)
}

function moveEndpointDown(index: number) {
  if (index >= props.endpoints.length - 1) return
  const next = [...props.endpoints]
  const temp = next[index]
  next[index] = next[index + 1]!
  next[index + 1] = temp!
  setEndpoints(next)
}

function removeEndpoint(index: number) {
  setEndpoints(props.endpoints.filter((_, idx) => idx !== index))
}
</script>

<template>
  <div class="form-group form-span endpoints-section">
    <div class="endpoints-header">
      <div class="endpoints-header-left">
        <span class="endpoints-title">{{ t('users.form.endpoints') }}</span>
        <span class="endpoints-count">{{ endpoints.length }}</span>
      </div>
      <div class="endpoints-header-actions">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="!availableNodes.length"
          @click="addAllNodes"
        >
          {{ t('users.form.addAllNodes') }}
        </button>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :disabled="!availableNodes.length"
          @click="addEndpoint"
        >
          {{ t('users.form.addEndpoint') }}
        </button>
      </div>
    </div>
    <div class="endpoint-tip">{{ t('users.form.endpointTip') }}</div>

    <div v-if="!endpoints.length" class="empty-endpoints">
      {{ t('users.form.noEndpoints') }}
    </div>

    <div v-else class="endpoints-list">
      <UserFormEndpointCard
        v-for="(ep, idx) in endpoints"
        :key="endpointKey(ep, idx)"
        :endpoint="ep"
        :index="idx"
        :is-first="idx === 0"
        :is-last="idx === endpoints.length - 1"
        :available-nodes="availableNodes"
        @update="(patch) => patchEndpoint(idx, patch)"
        @remove="removeEndpoint(idx)"
        @move-top="moveEndpointToTop(idx)"
        @move-up="moveEndpointUp(idx)"
        @move-down="moveEndpointDown(idx)"
      />
    </div>
  </div>
</template>

<style scoped>
.endpoints-section {
  background: rgba(15, 23, 42, 0.4);
  border: 1px solid var(--border-glass);
  border-radius: 10px;
  padding: 14px;
  margin-top: 4px;
}
.endpoints-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  flex-wrap: wrap;
  gap: 8px;
}
.endpoints-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.endpoints-header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.endpoints-title {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--primary);
}
.endpoints-count {
  font-size: 0.72rem;
  background: rgba(59, 130, 246, 0.2);
  color: var(--primary);
  padding: 2px 8px;
  border-radius: 10px;
  font-weight: 600;
}
.endpoint-tip {
  font-size: 0.72rem;
  color: var(--text-muted);
  margin-bottom: 12px;
  line-height: 1.4;
}
.empty-endpoints {
  text-align: center;
  padding: 24px;
  color: var(--text-muted);
  font-size: 0.82rem;
  border: 1px dashed rgba(255, 255, 255, 0.1);
  border-radius: 8px;
}
.endpoints-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 340px;
  overflow-y: auto;
  padding-right: 4px;
}
</style>
