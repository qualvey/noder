<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { Node, User } from '../types'
import IconButton from './IconButton.vue'

defineProps<{
  users: User[]
  totalCount: number
  loading: boolean
  allSelected: boolean
  selectedIds: Set<number>
  nodes: Node[]
  formatDownloadTime: (value?: string | null) => string
}>()

const emit = defineEmits<{
  (event: 'toggle-select', id: number): void
  (event: 'toggle-select-all'): void
  (event: 'copy-value', val: string, label: string): void
  (event: 'show-downloads', user: User): void
  (event: 'edit', user: User): void
  (event: 'delete-request', user: User, el: Element): void
  (event: 'row-contextmenu', e: MouseEvent, user: User): void
  (event: 'clear-search'): void
}>()

const { t } = useI18n()

function getNodeName(nodes: Node[], id: number) {
  const node = nodes.find((n) => n.id === id)
  return node?.node_name || node?.tag || `#${id}`
}

function getNodeProtocol(nodes: Node[], id: number) {
  const node = nodes.find((n) => n.id === id)
  return node?.protocol || ''
}

// 离开动画钩子：记录元素离开瞬间的几何尺寸与相对容器位移，保证表格其余行平滑滑向新位置 (FLIP)
function onBeforeLeave(el: Element) {
  const htmlEl = el as HTMLElement
  const rect = htmlEl.getBoundingClientRect()
  const parentRect = htmlEl.parentElement?.getBoundingClientRect()
  if (parentRect) {
    htmlEl.style.left = `${rect.left - parentRect.left}px`
    htmlEl.style.top = `${rect.top - parentRect.top}px`
    htmlEl.style.width = `${rect.width}px`
    htmlEl.style.height = `${rect.height}px`
  }
}
</script>

<template>
  <div class="table-container">
    <table class="data-table">
      <thead>
        <tr>
          <th class="col-check">
            <input type="checkbox" :checked="allSelected" @change="emit('toggle-select-all')" />
          </th>
          <th class="col-name">{{ t('users.colName') }}</th>
          <th class="col-creds">{{ t('users.colCredentials') }}</th>
          <th class="col-nodes">{{ t('users.colBoundNodes') }}</th>
          <th class="col-status">{{ t('users.colStatus') }}</th>
          <th class="col-downloads">{{ t('users.colDownloads') }}</th>
          <th class="col-actions">{{ t('users.colActions') }}</th>
        </tr>
      </thead>
      <tbody v-if="loading && !totalCount">
        <tr>
          <td :colspan="7" class="loading-cell">
            <div class="table-loading-container">
              <svg class="spinner" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle class="spinner-track" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                <path class="spinner-head" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
              </svg>
              <span class="loading-text">{{ t('users.loadingData') }}</span>
            </div>
          </td>
        </tr>
      </tbody>
      <tbody v-else-if="!totalCount">
        <tr>
          <td colspan="7" class="empty-cell">{{ t('users.emptyText') }}</td>
        </tr>
      </tbody>
      <tbody v-else-if="!users.length">
        <tr>
          <td colspan="7" class="empty-cell-lg">
            <div class="empty-icon">🔍</div>
            <div class="empty-text">{{ t('users.noMatchingUsers') }}</div>
            <button class="btn btn-secondary btn-sm" @click="emit('clear-search')">{{ t('common.clearFilter') }}</button>
          </td>
        </tr>
      </tbody>
      <TransitionGroup v-else tag="tbody" name="user-row" @before-leave="onBeforeLeave">
        <tr v-for="user in users" :key="user.id" @contextmenu.prevent="emit('row-contextmenu', $event, user)">
          <td class="cell-center">
            <input type="checkbox" :checked="selectedIds.has(user.id)" @change="emit('toggle-select', user.id)" />
          </td>
          <td>
            <div class="user-name">{{ user.name }}</div>
            <div v-if="user.remark" class="user-remark">{{ user.remark }}</div>
          </td>
          <td>
            <div
              v-if="user.uuid"
              class="copyable-cell"
              :title="t('users.copyUuidTitle')"
              @click.stop="emit('copy-value', user.uuid, 'UUID')"
            >
              <span class="cred-label">UUID</span>
              <code class="clickable-code">{{ user.uuid }}</code>
            </div>
            <div v-else class="copyable-cell copyable-empty">
              <span class="cred-label">UUID</span><code>-</code>
            </div>

            <div
              v-if="user.password"
              class="copyable-cell copyable-mt"
              :title="t('users.copyPwdTitle')"
              @click.stop="emit('copy-value', user.password, 'PWD')"
            >
              <span class="cred-label">PWD</span>
              <code class="clickable-code">{{ user.password }}</code>
            </div>
            <div v-else class="copyable-cell copyable-empty copyable-mt">
              <span class="cred-label">PWD</span><code>-</code>
            </div>
          </td>
          <td class="cell-center">
            <div v-if="(user.endpoints && user.endpoints.length) || user.node_ids.length" class="nodes-hover-trigger">
              <span class="badge badge-nodes">
                {{ t('users.boundNodesCount', { count: (user.endpoints && user.endpoints.length) || user.node_ids.length }) }} ▾
              </span>
              <div class="nodes-popover-menu">
                <div class="nodes-popover-title">{{ t('users.colBoundNodes') }} ({{ (user.endpoints && user.endpoints.length) || user.node_ids.length }})</div>
                <div class="nodes-popover-list">
                  <template v-if="user.endpoints && user.endpoints.length">
                    <div v-for="(ep, eIdx) in user.endpoints" :key="ep.id || eIdx" class="nodes-popover-item">
                      <span v-if="eIdx === 0" class="primary-star" :title="t('users.form.primaryNodeBadge')">★</span>
                      <span class="node-name-text">{{ ep.tag || ep.node?.node_name || getNodeName(nodes, ep.node_id) }}</span>
                      <span v-if="ep.node?.protocol || getNodeProtocol(nodes, ep.node_id)" class="badge-protocol">
                        {{ (ep.node?.protocol || getNodeProtocol(nodes, ep.node_id)).toUpperCase() }}
                      </span>
                    </div>
                  </template>
                  <template v-else>
                    <div v-for="(nid, nIdx) in user.node_ids" :key="nid" class="nodes-popover-item">
                      <span v-if="nIdx === 0" class="primary-star" :title="t('users.form.primaryNodeBadge')">★</span>
                      <span class="node-name-text">{{ getNodeName(nodes, nid) }}</span>
                      <span v-if="getNodeProtocol(nodes, nid)" class="badge-protocol">{{ getNodeProtocol(nodes, nid).toUpperCase() }}</span>
                    </div>
                  </template>
                </div>
              </div>
            </div>
            <span v-else class="unbound-text">{{ t('users.unbound') }}</span>
          </td>
          <td class="cell-center">
            <span class="status-badge" :class="user.is_active ? 'active' : 'inactive'">
              <span class="status-dot"></span>
              {{ user.is_active ? t('common.enabled') : t('common.disabled') }}
            </span>
          </td>
          <td class="cell-center">
            <button class="download-summary" :title="t('users.viewDownloadHistory')" @click.stop="emit('show-downloads', user)">
              <span>{{ formatDownloadTime(user.last_download_at) }}</span>
              <span class="download-count">{{ t('users.downloadCount', { count: user.download_count || 0 }) }}</span>
            </button>
          </td>
          <td class="cell-center">
            <div class="row-actions-nowrap">
              <IconButton icon="edit" :tip="t('common.edit')" variant="secondary" @click="emit('edit', user)" />
              <IconButton
                icon="delete"
                :tip="t('common.delete')"
                variant="danger"
                @click="emit('delete-request', user, $event.currentTarget as Element)"
              />
            </div>
          </td>
        </tr>
      </TransitionGroup>
    </table>
  </div>
</template>

<style scoped>
.copyable-cell {
  cursor: pointer;
  padding: 1px 0;
  display: flex;
  align-items: center;
  gap: 6px;
  user-select: none;
  white-space: nowrap;
}
.download-summary {
  display: inline-flex;
  flex-direction: column;
  gap: 2px;
  border: 0;
  background: transparent;
  color: var(--text-main);
  cursor: pointer;
  font: inherit;
  font-size: 0.78rem;
}
.download-summary:hover { color: var(--primary); }
.download-count { color: var(--text-muted); font-size: 0.7rem; }
.cred-label {
  font-size: 0.7rem;
  font-weight: 700;
  color: var(--text-muted);
  width: 32px;
  flex-shrink: 0;
}
.clickable-code {
  font-family: var(--font-mono);
  font-size: 0.76rem;
  word-break: keep-all;
  white-space: nowrap;
  padding: 2px 7px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  transition: all 0.15s ease;
  color: var(--text-main);
}
:global([data-theme='light']) .clickable-code {
  background: rgba(0, 0, 0, 0.04);
  border-color: rgba(0, 0, 0, 0.08);
}
.copyable-cell:hover .clickable-code {
  color: var(--primary);
  border-color: var(--primary);
  background: var(--bg-hover);
}
.nodes-hover-trigger {
  position: relative;
  display: inline-block;
}
.badge-nodes {
  background: rgba(99, 102, 241, 0.15);
  color: #a5b4fc;
  border: 1px solid rgba(99, 102, 241, 0.3);
  cursor: pointer;
  padding: 3px 8px;
  border-radius: 12px;
  font-size: 0.75rem;
  font-weight: 500;
  transition: all 0.2s ease;
  user-select: none;
}
.nodes-hover-trigger:hover .badge-nodes {
  background: rgba(99, 102, 241, 0.25);
  border-color: rgba(99, 102, 241, 0.5);
  color: #fff;
}
.nodes-popover-menu {
  display: none;
  position: absolute;
  top: calc(100% + 4px);
  left: 50%;
  transform: translateX(-50%);
  background: var(--bg-elevated);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.45);
  padding: 10px;
  min-width: 190px;
  max-width: 280px;
  z-index: 1000;
  text-align: left;
  backdrop-filter: blur(12px);
}
.nodes-hover-trigger:hover .nodes-popover-menu {
  display: block;
}
.nodes-popover-title {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 600;
  margin-bottom: 6px;
  padding-bottom: 4px;
  border-bottom: 1px solid var(--border-glass);
}
.nodes-popover-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 180px;
  overflow-y: auto;
}
.nodes-popover-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 0.75rem;
  padding: 4px 6px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.03);
}
:global([data-theme='light']) .nodes-popover-item {
  background: rgba(0, 0, 0, 0.03);
}
.node-name-text {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-main);
}
.primary-star {
  color: #fbbf24;
  font-size: 0.8rem;
  line-height: 1;
}
.badge-protocol {
  font-size: 0.65rem;
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(59, 130, 246, 0.2);
  color: #93c5fd;
  font-family: var(--font-mono);
  font-weight: 600;
  flex-shrink: 0;
}
</style>
