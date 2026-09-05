<script setup lang="ts">
// 用户管理：表格 / 全选批量删除 / 右键复制订阅链接 + 下载配置包（新增/编辑表单在 UserFormModal 组件）
import { inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { DistFile, Node, User } from '../types'
import { buildDownloadLink, buildMihomoLink, buildSubLink, copyText } from '../utils'
import UserFormModal from '../components/UserFormModal.vue'
import ContextMenu, { type ContextMenuItem } from '../components/ContextMenu.vue'
import { ToastType } from '@/App.vue'

const { t } = useI18n()
const toast = inject('toast') as (msg: string, type?: ToastType) => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }
const updateMetrics = inject('metrics') as (nodes: number, users: number) => void

const users = ref<User[]>([])
const nodes = ref<Node[]>([])
const zipFiles = ref<DistFile[]>([])
const selected = ref<Set<number>>(new Set())
const showModal = ref(false)
const editingUser = ref<User | null>(null)
const ctxMenu = ref<{ x: number; y: number; user: User } | null>(null)
const downloading = ref<string | null>(null) // waiting 页文案；null=不显示
const loading = ref(true)

// 右键菜单：复制订阅链接
function onRowContextMenu(e: MouseEvent, user: User) {
  e.preventDefault()
  e.stopPropagation() // 避免被 ContextMenu 的文档级 contextmenu 监听立即关闭
  ctxMenu.value = { x: e.clientX, y: e.clientY, user }
}

function closeCtxMenu() {
  ctxMenu.value = null
}

function copySubLink() {
  const user = ctxMenu.value?.user
  if (!user) return
  copyText(buildSubLink(user.token)).then((ok) => {
    toast(ok ? t('users.subLinkCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
  })
  closeCtxMenu()
}

function copyMihomoLink() {
  const user = ctxMenu.value?.user
  if (!user) return
  copyText(buildMihomoLink(user.token)).then((ok) => {
    toast(ok ? t('users.mihomoLinkCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
  })
  closeCtxMenu()
}

async function copyNodes() {
  const user = ctxMenu.value?.user
  if (!user) return
  closeCtxMenu()
  try {
    const nodes = await api.userNodes(user.token)
    const formattedText = nodes.map(node => JSON.stringify(node, null, 2)).join(',\n')
    const ok = await copyText(formattedText)
    toast(ok ? t('users.nodesJsonCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

async function copyUser() {
  const user = ctxMenu.value?.user
  if (!user) return
  closeCtxMenu()
  try {
    const payload = {
      name: user.name,
      uuid: user.uuid ?? "",
      password: user.password ?? ""
    }
    const textToCopy = JSON.stringify(payload, null, 2)
    await navigator.clipboard.writeText(textToCopy)
    toast(t('users.userCopied'), "success")
  } catch(e) {
    toast((e as Error).message, "error")
  }
}

function copyValue(val: string | undefined | null, label: string) {
  if (!val || val === '-') return
  copyText(val).then((ok) => {
    toast(ok ? t('users.copiedToClipboard', { label }) : t('common.copyFailed'), ok ? 'info' : 'error')
  })
}

function copyToken() {
  const user = ctxMenu.value?.user
  if (!user) return
  closeCtxMenu()
  copyText(user.token).then((ok) => {
    toast(ok ? t('users.tokenCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
  })
}

function getNodeName(id: number) {
  const node = nodes.value.find((n) => n.id === id)
  return node?.node_name || node?.tag || `#${id}`
}

function getNodeProtocol(id: number) {
  const node = nodes.value.find((n) => n.id === id)
  return node?.protocol || ''
}

// 复制 ZIP 配置包下载链接
function copyZipLink(f: DistFile) {
  const user = ctxMenu.value?.user
  if (!user) return
  closeCtxMenu()
  const url = buildDownloadLink(f.id, user.token)
  copyText(url).then((ok) => {
    toast(ok ? t('users.downloadLinkCopied', { name: f.name }) : t('common.copyFailed'), ok ? 'info' : 'error')
  })
}

const ctxMenuItems = (): ContextMenuItem[] => {
  const items: ContextMenuItem[] = [
    { label: t('users.menu.copyToken'), icon: '🔑', onClick: copyToken },
    { label: t('users.menu.copySingbox'), icon: '📋', onClick: copySubLink },
    { label: t('users.menu.copyMihomo'), icon: '🔄', onClick: copyMihomoLink },
    { label: t('users.menu.copyNodes'), icon: '📄', onClick: copyNodes },
    { label: t('users.menu.copyUser'), icon: '👤', onClick: copyUser }
  ]
  if (zipFiles.value.length) {
    items.push({ label: t('users.menu.configDlHeader'), icon: '📦', divider: true, onClick: () => { } })
    for (const z of zipFiles.value) {
      items.push({ label: t('users.menu.copyDl', { name: z.name }), icon: '🔗', onClick: () => copyZipLink(z) })
    }
  }
  return items
}

async function fetchData(silent = false) {
  if (!silent && !users.value.length) {
    loading.value = true
  }
  try {
    const [u, n, f] = await Promise.all([api.users.list(), api.nodes.list(), api.files.list()])
    users.value = u
    nodes.value = n
    zipFiles.value = f.filter((x) => x.file_type === 'zip' && x.is_active)
    updateMetrics(n.length, u.length)
  } catch (e) {
    toast((e as Error).message, 'error')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingUser.value = null
  showModal.value = true
}

function openEdit(u: User) {
  editingUser.value = u
  showModal.value = true
}

async function removeUser(id: number) {
  const prevUsers = [...users.value]
  // 乐观更新：直接从视图列表中剔除，触发 FLIP 平滑过渡与位移动画
  users.value = users.value.filter((u) => u.id !== id)
  if (selected.value.has(id)) {
    const s = new Set(selected.value)
    s.delete(id)
    selected.value = s
  }
  updateMetrics(nodes.value.length, users.value.length)

  try {
    await api.users.remove(id)
    toast(t('users.deleted'))
    // 静默对齐后端最新数据，不触发全屏 loading 闪烁
    await fetchData(true)
  } catch (e) {
    // 异常时优雅回滚
    users.value = prevUsers
    updateMetrics(nodes.value.length, prevUsers.length)
    toast((e as Error).message, 'error')
  }
}

function toggleSelect(id: number) {
  const s = new Set(selected.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selected.value = s
}

function toggleSelectAll() {
  if (selected.value.size === users.value.length) selected.value = new Set()
  else selected.value = new Set(users.value.map((u) => u.id))
}

function bulkDelete(e: MouseEvent) {
  const count = selected.value.size
  if (!count) return
  popover.show(e.currentTarget as Element, t('users.bulkDeleteConfirm', { count }), async () => {
    const idsToDelete = Array.from(selected.value)
    const prevUsers = [...users.value]

    // 乐观更新：批量从视图平滑移除
    users.value = users.value.filter((u) => !selected.value.has(u.id))
    selected.value = new Set()
    updateMetrics(nodes.value.length, users.value.length)

    try {
      for (const id of idsToDelete) {
        try {
          await api.users.remove(id)
        } catch {
          /* 继续 */
        }
      }
      toast(t('users.bulkDeleteSuccess', { count }))
      await fetchData(true)
    } catch {
      users.value = prevUsers
      updateMetrics(nodes.value.length, prevUsers.length)
    }
  })
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

onMounted(() => fetchData())
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">{{ t('users.headerTitle') }}</div>
      <div style="display: flex; gap: 12px; align-items: center">
        <button v-if="selected.size" class="btn btn-danger btn-sm" @click="bulkDelete">
          🗑️ {{ t('users.bulkDelete', { count: selected.size }) }}
        </button>
        <button class="btn btn-primary" @click="openCreate">
          <span style="font-size: 1.1rem; line-height: 1">+</span> {{ t('users.addUser') }}
        </button>
      </div>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th style="width: 44px; text-align: center">
              <input type="checkbox" :checked="selected.size === users.length && users.length > 0"
                @change="toggleSelectAll" />
            </th>
            <th style="min-width: 140px">{{ t('users.colName') }}</th>
            <th style="min-width: 260px">{{ t('users.colCredentials') }}</th>
            <th style="width: 120px; text-align: center">{{ t('users.colBoundNodes') }}</th>
            <th style="width: 100px; text-align: center">{{ t('users.colStatus') }}</th>
            <th style="width: 90px; text-align: center">{{ t('users.colActions') }}</th>
          </tr>
        </thead>
        <tbody v-if="loading && !users.length">
          <tr>
            <td :colspan="6" style="text-align: center; padding: 40px 0;">
              <div class="table-loading-container">
                <!-- SVG 转圈 -->
                <svg class="spinner" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle class="spinner-track" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                  <path class="spinner-head" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
                </svg>
                <span class="loading-text">{{ t('users.loadingData') }}</span>
              </div>
            </td>
          </tr>
        </tbody>
        <tbody v-else-if="!users.length">
          <tr>
            <td colspan="6" style="text-align: center; color: var(--text-muted); padding: 30px">{{ t('users.emptyText') }}</td>
          </tr>
        </tbody>
        <TransitionGroup
          v-else
          tag="tbody"
          name="user-row"
          @before-leave="onBeforeLeave"
        >
          <tr v-for="user in users" :key="user.id" @contextmenu.prevent="onRowContextMenu($event, user)">
            <td style="text-align: center">
              <input type="checkbox" :checked="selected.has(user.id)" @change="toggleSelect(user.id)" />
            </td>
            <td>
              <div style="font-weight: 600; color: var(--text-main)">{{ user.name }}</div>
              <div v-if="user.remark" style="font-size: 0.75rem; color: var(--text-muted)">{{ user.remark }}</div>
            </td>
            <td>
              <div
                v-if="user.uuid"
                class="copyable-cell"
                :title="t('users.copyUuidTitle')"
                @click.stop="copyValue(user.uuid, 'UUID')"
              >
                <span class="cred-label">UUID</span>
                <code class="clickable-code">{{ user.uuid }}</code>
              </div>
              <div v-else class="copyable-cell" style="color: var(--text-dim)">
                <span class="cred-label">UUID</span><code>-</code>
              </div>

              <div
                v-if="user.password"
                class="copyable-cell"
                :title="t('users.copyPwdTitle')"
                @click.stop="copyValue(user.password, 'PWD')"
                style="margin-top: 4px;"
              >
                <span class="cred-label">PWD</span>
                <code class="clickable-code">{{ user.password }}</code>
              </div>
              <div v-else class="copyable-cell" style="color: var(--text-dim); margin-top: 4px;">
                <span class="cred-label">PWD</span><code>-</code>
              </div>
            </td>
            <td style="text-align: center">
              <div v-if="user.node_ids.length" class="nodes-hover-trigger">
                <span class="badge badge-nodes">
                  {{ t('users.boundNodesCount', { count: user.node_ids.length }) }} ▾
                </span>
                <!-- 悬浮弹出的节点列表 -->
                <div class="nodes-popover-menu">
                  <div class="nodes-popover-title">{{ t('users.colBoundNodes') }} ({{ user.node_ids.length }})</div>
                  <div class="nodes-popover-list">
                    <div
                      v-for="(nid, nIdx) in user.node_ids"
                      :key="nid"
                      class="nodes-popover-item"
                    >
                      <span v-if="nIdx === 0" class="primary-star" :title="t('users.form.primaryNodeBadge')">★</span>
                      <span class="node-name-text">{{ getNodeName(nid) }}</span>
                      <span v-if="getNodeProtocol(nid)" class="badge-protocol">{{ getNodeProtocol(nid).toUpperCase() }}</span>
                    </div>
                  </div>
                </div>
              </div>
              <span v-else style="color: var(--text-dim); font-size: 0.8rem">{{ t('users.unbound') }}</span>
            </td>
            <td style="text-align: center">
              <span class="status-badge" :class="user.is_active ? 'active' : 'inactive'">
                <span class="status-dot"></span>
                {{ user.is_active ? (t('common.enabled') || '启用') : (t('common.disabled') || '停用') }}
              </span>
            </td>
            <td style="text-align: center">
              <div style="display: flex; gap: 8px; justify-content: center; align-items: center; white-space: nowrap">
                <IconButton icon="edit" :tip="t('common.edit')" variant="secondary" @click="openEdit(user)" />
                <IconButton icon="delete" :tip="t('common.delete')" variant="danger"
                  @click="popover.show($event.currentTarget as Element, t('users.deleteConfirm'), () => removeUser(user.id))" />
              </div>
            </td>
          </tr>
        </TransitionGroup>
      </table>
    </div>
  </section>

  <UserFormModal :open="showModal" :editing="editingUser" :nodes="nodes" @close="showModal = false"
    @saved="() => fetchData(true)" />

  <ContextMenu v-if="ctxMenu" :x="ctxMenu.x" :y="ctxMenu.y" :title="ctxMenu.user.name" :items="ctxMenuItems()"
    @close="closeCtxMenu" />

  <!-- ZIP 生成等待页 -->
  <div v-if="downloading" class="download-waiting">
    <div class="waiting-box">
      <div class="waiting-spinner"></div>
      <div class="waiting-text">{{ downloading }}</div>
      <div style="font-size: 0.75rem; color: var(--text-muted)">配置渲染需要一点时间，请稍候</div>
    </div>
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

/* 节点悬浮气泡 */
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
