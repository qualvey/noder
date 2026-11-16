<script setup lang="ts">
// 用户管理：表格 / 全选批量删除 / 右键复制订阅链接 + 下载配置包（新增/编辑表单在 UserFormModal 组件）
import { computed, inject, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { DistFile, Node, User, UserDownload } from '../types'
import UserFormModal from '../components/UserFormModal.vue'
import TemplateBranchModal from '../components/TemplateBranchModal.vue'
import ContextMenu from '../components/ContextMenu.vue'
import UserDownloadHistoryModal from '../components/UserDownloadHistoryModal.vue'
import UserTable from '../components/UserTable.vue'
import { ToastKey, PopoverKey, MetricsKey } from '../injectionKeys'
import { useFuzzySearch } from '../composables'
import { useUserCopyActions } from '../composables/useUserCopyActions'

const { t, locale } = useI18n()
const toast = inject(ToastKey)
const popover = inject(PopoverKey)
const updateMetrics = inject(MetricsKey)

const users = ref<User[]>([])
const nodes = ref<Node[]>([])
const zipFiles = ref<DistFile[]>([])
const selected = ref<Set<number>>(new Set())
const showModal = ref(false)
const editingUser = ref<User | null>(null)
const ctxMenu = ref<{ x: number; y: number; user: User } | null>(null)
const downloading = ref<string | null>(null) // waiting 页文案；null=不显示
const loading = ref(true)
const downloadHistoryUser = ref<User | null>(null)
const downloadHistory = ref<UserDownload[]>([])
const loadingDownloadHistory = ref(false)
const showTemplateBranchModal = ref(false)

// 右键菜单：复制订阅链接
function onRowContextMenu(e: MouseEvent, user: User) {
  e.preventDefault()
  e.stopPropagation() // 避免被 ContextMenu 的文档级 contextmenu 监听立即关闭
  ctxMenu.value = { x: e.clientX, y: e.clientY, user }
}

function closeCtxMenu() {
  ctxMenu.value = null
}

const { copyValue, ctxMenuItems } = useUserCopyActions(
  ctxMenu,
  zipFiles,
  closeCtxMenu,
  openTemplateBranchModal,
)

function requestDeleteUser(user: User, el: Element) {
  popover?.show(el, t('users.deleteConfirm'), () => removeUser(user.id))
}

function formatDownloadTime(value?: string | null) {
  if (!value) return t('users.noDownloads')
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

async function showDownloadHistory(user: User) {
  downloadHistoryUser.value = user
  downloadHistory.value = []
  loadingDownloadHistory.value = true
  try {
    downloadHistory.value = await api.users.downloads(user.id)
  } catch (e) {
    toast?.((e as Error).message, 'error')
  } finally {
    loadingDownloadHistory.value = false
  }
}

function openTemplateBranchModal() {
  closeCtxMenu()
  showTemplateBranchModal.value = true
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
    updateMetrics?.(n.length, u.length)
  } catch (e) {
    toast?.((e as Error).message, 'error')
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
  updateMetrics?.(nodes.value.length, users.value.length)

  try {
    await api.users.remove(id)
    toast?.(t('users.deleted'))
    // 静默对齐后端最新数据，不触发全屏 loading 闪烁
    await fetchData(true)
  } catch (e) {
    // 异常时优雅回滚
    users.value = prevUsers
    updateMetrics?.(nodes.value.length, prevUsers.length)
    toast?.((e as Error).message, 'error')
  }
}

function lookupNodeName(id: number) {
  const node = nodes.value.find((n) => n.id === id)
  return node?.node_name || node?.tag || `#${id}`
}

const { searchQuery, filteredItems: filteredUsers, clear: clearSearch } = useFuzzySearch(users, (u) => [
  u.name,
  u.remark,
  u.token,
  u.uuid,
  u.password,
  u.is_active ? t('common.enabled') : t('common.disabled'),
  ...(u.endpoints || []).map((ep) => ep.tag || ep.node?.node_name || lookupNodeName(ep.node_id)),
  ...(u.node_ids || []).map((id) => lookupNodeName(id)),
])

const isAllFilteredSelected = computed(() => {
  if (!filteredUsers.value.length) return false
  return filteredUsers.value.every((u) => selected.value.has(u.id))
})

function toggleSelect(id: number) {
  const s = new Set(selected.value)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  selected.value = s
}

function toggleSelectAll() {
  const s = new Set(selected.value)
  if (isAllFilteredSelected.value) {
    for (const u of filteredUsers.value) {
      s.delete(u.id)
    }
  } else {
    for (const u of filteredUsers.value) {
      s.add(u.id)
    }
  }
  selected.value = s
}

function bulkDelete(e: MouseEvent) {
  const count = selected.value.size
  if (!count) return
  popover?.show(e.currentTarget as Element, t('users.bulkDeleteConfirm', { count }), async () => {
    const idsToDelete = Array.from(selected.value)
    const prevUsers = [...users.value]

    // 乐观更新：批量从视图平滑移除
    users.value = users.value.filter((u) => !selected.value.has(u.id))
    selected.value = new Set()
    updateMetrics?.(nodes.value.length, users.value.length)

    try {
      for (const id of idsToDelete) {
        try {
          await api.users.remove(id)
        } catch {
          /* 继续 */
        }
      }
      toast?.(t('users.bulkDeleteSuccess', { count }))
      await fetchData(true)
    } catch {
      users.value = prevUsers
      updateMetrics?.(nodes.value.length, prevUsers.length)
    }
  })
}

onMounted(() => fetchData())
</script>

<template>
  <section class="tab-content users-tab">
    <div class="section-header">
      <div class="section-title">{{ t('users.headerTitle') }}</div>
      <div class="section-actions">
        <SearchInput
          v-model="searchQuery"
          :placeholder="t('users.searchPlaceholder')"
          :total-count="users.length"
          :filtered-count="filteredUsers.length"
          :disabled="!users.length"
        />
        <button v-if="selected.size" class="btn btn-danger btn-sm" @click="bulkDelete">
          🗑️ {{ t('users.bulkDelete', { count: selected.size }) }}
        </button>
        <button class="btn btn-primary" @click="openCreate">
          <span class="plus-mark">+</span> {{ t('users.addUser') }}
        </button>
      </div>
    </div>

    <UserTable
      :users="filteredUsers"
      :total-count="users.length"
      :loading="loading"
      :all-selected="isAllFilteredSelected"
      :selected-ids="selected"
      :nodes="nodes"
      :format-download-time="formatDownloadTime"
      @toggle-select="toggleSelect"
      @toggle-select-all="toggleSelectAll"
      @copy-value="copyValue"
      @show-downloads="showDownloadHistory"
      @edit="openEdit"
      @delete-request="requestDeleteUser"
      @row-contextmenu="onRowContextMenu"
      @clear-search="clearSearch"
    />
  </section>

  <UserFormModal :open="showModal" :editing="editingUser" :nodes="nodes" @close="showModal = false"
    @saved="() => fetchData(true)" />

  <TemplateBranchModal :open="showTemplateBranchModal" @close="showTemplateBranchModal = false" />

  <ContextMenu v-if="ctxMenu" :x="ctxMenu.x" :y="ctxMenu.y" :title="ctxMenu.user.name" :items="ctxMenuItems()"
    @close="closeCtxMenu" />

  <UserDownloadHistoryModal
    :user="downloadHistoryUser"
    :history="downloadHistory"
    :loading="loadingDownloadHistory"
    :format-time="formatDownloadTime"
    @close="downloadHistoryUser = null"
  />

  <!-- ZIP 生成等待页 -->
  <div v-if="downloading" class="download-waiting">
    <div class="waiting-box">
      <div class="waiting-spinner"></div>
      <div class="waiting-text">{{ downloading }}</div>
      <div class="waiting-hint">配置渲染需要一点时间，请稍候</div>
    </div>
  </div>
</template>

<style scoped>
.users-tab {
  display: block;
}
.section-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}
.plus-mark {
  font-size: 1.1rem;
  line-height: 1;
}
.waiting-hint {
  font-size: 0.75rem;
  color: var(--text-muted);
}
</style>
