<script setup lang="ts">
// 用户管理：表格 / 全选批量删除（新增/编辑表单在 UserFormModal 组件）
import { inject, onMounted, ref } from 'vue'
import { api } from '../api'
import type { Node, User } from '../types'
import UserFormModal from '../components/UserFormModal.vue'

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }
const updateMetrics = inject('metrics') as (nodes: number, users: number) => void

const users = ref<User[]>([])
const nodes = ref<Node[]>([])
const selected = ref<Set<number>>(new Set())
const showModal = ref(false)
const editingUser = ref<User | null>(null)

async function fetchData() {
  try {
    const [u, n] = await Promise.all([api.users.list(), api.nodes.list()])
    users.value = u
    nodes.value = n
    updateMetrics(n.length, u.length)
  } catch (e) {
    toast((e as Error).message, 'error')
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
    toast('用户已删除')
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
  popover.show(e.currentTarget as Element, `⚠️ 确定批量删除已选中的 ${count} 个用户？`, async () => {
    for (const id of selected.value) {
      try {
        await api.users.remove(id)
      } catch {
        /* 继续 */
      }
    }
    selected.value = new Set()
    toast(`已批量删除 ${count} 个用户`)
    fetchData()
  })
}

onMounted(fetchData)
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">订阅用户列表 (支持多节点绑定)</div>
      <div style="display: flex; gap: 10px; align-items: center">
        <button v-if="selected.size" class="btn btn-danger btn-sm" @click="bulkDelete">🗑️ 批量删除 ({{ selected.size }})</button>
        <button class="btn btn-primary" @click="openCreate"><span>+</span> 新增用户</button>
      </div>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th style="width: 40px; text-align: center">
              <input type="checkbox" :checked="selected.size === users.length && users.length > 0" @change="toggleSelectAll" />
            </th>
            <th>ID</th>
            <th>用户备注</th>
            <th>鉴权 Token</th>
            <th>专属 UUID / 密码</th>
            <th>状态</th>
            <th>绑定节点</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!users.length">
            <td colspan="8" style="text-align: center; color: var(--text-muted); padding: 30px">暂无订阅用户，点击右上角新增</td>
          </tr>
          <tr v-for="user in users" :key="user.id">
            <td style="text-align: center">
              <input type="checkbox" :checked="selected.has(user.id)" @change="toggleSelect(user.id)" />
            </td>
            <td>{{ user.id }}</td>
            <td>{{ user.name }}<div v-if="user.remark" style="font-size: 0.72rem; color: var(--text-muted)">{{ user.remark }}</div></td>
            <td><code style="font-size: 0.72rem">{{ user.token }}</code></td>
            <td style="font-size: 0.75rem">
              <div>UUID: <code>{{ user.uuid || '-' }}</code></div>
              <div>PWD: <code>{{ user.password || '-' }}</code></div>
            </td>
            <td>
              <span v-if="user.is_active" style="color: var(--accent-emerald)">🟢 启用</span>
              <span v-else style="color: var(--accent-rose)">🔴 停用</span>
            </td>
            <td style="font-size: 0.72rem; max-width: 180px">
              <template v-if="user.node_ids.length">
                <span v-for="nid in user.node_ids" :key="nid" class="badge" style="margin: 2px">{{ nodes.find((n) => n.id === nid)?.node_name || `#${nid}` }}</span>
              </template>
              <span v-else style="color: var(--text-muted)">未绑定</span>
            </td>
            <td>
              <div style="display: flex; gap: 6px">
                <button class="btn btn-secondary btn-sm" @click="openEdit(user)">编辑</button>
                <button class="btn btn-danger btn-sm" @click="popover.show($event.currentTarget as Element, '⚠️ 确定删除该用户？', () => removeUser(user.id))">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>

  <UserFormModal :open="showModal" :editing="editingUser" :nodes="nodes" @close="showModal = false" @saved="fetchData" />
</template>
