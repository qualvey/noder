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

async function fetchData() {
  loading.value = true
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

function removeUser(id: number) {
  api.users.remove(id).then(() => {
    toast(t('users.deleted'))
    fetchData()
  }).catch((e) => toast(e.message, 'error'))
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
    for (const id of selected.value) {
      try {
        await api.users.remove(id)
      } catch {
        /* 继续 */
      }
    }
    selected.value = new Set()
    toast(t('users.bulkDeleteSuccess', { count }))
    fetchData()
  })
}

onMounted(fetchData)
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">{{ t('users.headerTitle') }}</div>
      <div style="display: flex; gap: 10px; align-items: center">
        <button v-if="selected.size" class="btn btn-danger btn-sm" @click="bulkDelete">🗑️ {{ t('users.bulkDelete', { count: selected.size }) }}</button>
        <button class="btn btn-primary" @click="openCreate"><span>+</span> {{ t('users.addUser') }}</button>
      </div>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th style="width: 40px; text-align: center">
              <input type="checkbox" :checked="selected.size === users.length && users.length > 0"
                @change="toggleSelectAll" />
            </th>
            <th>{{ t('users.colId') }}</th>
            <th>{{ t('users.colName') }}</th>
            <th>{{ t('users.colToken') }}</th>
            <th>{{ t('users.colCredentials') }}</th>
            <th>{{ t('users.colStatus') }}</th>
            <th>{{ t('users.colBoundNodes') }}</th>
            <th>{{ t('users.colActions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading">
            <td :colspan="8" style="text-align: center; padding: 40px 0;">
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
          <tr v-else-if="!users.length">
            <td colspan="8" style="text-align: center; color: var(--text-muted); padding: 30px">{{ t('users.emptyText') }}</td>
          </tr>
          <tr v-for="user in users" :key="user.id" @contextmenu.prevent="onRowContextMenu($event, user)">
            <td style="text-align: center">
              <input type="checkbox" :checked="selected.has(user.id)" @change="toggleSelect(user.id)" />
            </td>
            <td>{{ user.id }}</td>
            <td>{{ user.name }}<div v-if="user.remark" style="font-size: 0.72rem; color: var(--text-muted)">{{
              user.remark }}</div>
            </td>
            <td><code style="font-size: 0.72rem">{{ user.token }}</code></td>
            <td style="font-size: 0.75rem">
              <div
                v-if="user.uuid"
                class="copyable-cell"
                :title="t('users.copyUuidTitle')"
                @click.stop="copyValue(user.uuid, 'UUID')"
              >
                UUID: <code class="clickable-code">{{ user.uuid }}</code>
              </div>
              <div v-else style="color: var(--text-dim)">UUID: <code>-</code></div>

              <div
                v-if="user.password"
                class="copyable-cell"
                :title="t('users.copyPwdTitle')"
                @click.stop="copyValue(user.password, 'PWD')"
              >
                PWD: <code class="clickable-code">{{ user.password }}</code>
              </div>
              <div v-else style="color: var(--text-dim)">PWD: <code>-</code></div>
            </td>
            <td>
              <span v-if="user.is_active" style="color: var(--accent-emerald)">{{ t('users.statusActive') }}</span>
              <span v-else style="color: var(--accent-rose)">{{ t('users.statusInactive') }}</span>
            </td>
            <td style="font-size: 0.72rem; max-width: 220px">
              <template v-if="user.node_ids.length">
                <span
                  v-for="(nid, nIdx) in user.node_ids"
                  :key="nid"
                  class="badge"
                  :style="nIdx === 0 ? 'margin: 2px; border-color: rgba(59, 130, 246, 0.6); background: rgba(59, 130, 246, 0.15); color: #93c5fd; font-weight: 600;' : 'margin: 2px'"
                  :title="nIdx === 0 ? t('users.form.primaryNodeBadge') : ''"
                >
                  <template v-if="nIdx === 0">★ </template>{{nodes.find((n) =>
                  n.id === nid)?.node_name || `#${nid}`}}
                </span>
              </template>
              <span v-else style="color: var(--text-muted)">{{ t('users.unbound') }}</span>
            </td>
            <td>
              <div style="display: flex; gap: 6px">
                <button class="btn btn-secondary btn-sm" @click="openEdit(user)">{{ t('common.edit') }}</button>
                <button class="btn btn-danger btn-sm"
                  @click="popover.show($event.currentTarget as Element, t('users.deleteConfirm'), () => removeUser(user.id))">{{ t('common.delete') }}</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <UserFormModal :open="showModal" :editing="editingUser" :nodes="nodes" @close="showModal = false"
    @saved="fetchData" />

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
  gap: 3px;
  user-select: none;
}
.copyable-cell:hover .clickable-code {
  color: var(--primary);
  border-color: var(--primary);
  background: var(--bg-hover);
}
.clickable-code {
  transition: all 0.15s ease;
  cursor: pointer;
}
</style>
