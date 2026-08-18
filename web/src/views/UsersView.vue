<script setup lang="ts">
// 用户管理：表格 / 全选批量删除 / 新增编辑 (凭证生成 + 快捷提取 + 节点多选)
import { inject, onMounted, reactive, ref } from 'vue'
import { api } from '../api'
import type { Node, User } from '../types'
import { parseCredentials, randomPassword, randomUUID } from '../utils'

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }
const updateMetrics = inject('metrics') as (nodes: number, users: number) => void

const users = ref<User[]>([])
const nodes = ref<Node[]>([])
const selected = ref<Set<number>>(new Set())
const showModal = ref(false)
const editingId = ref<number | null>(null)
const showExtract = ref(false)
const extractInput = ref('')

const form = reactive({
  name: '',
  remark: '',
  token: '',
  uuid: '',
  password: '',
  config_override: '',
  node_ids: [] as number[],
})

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
  editingId.value = null
  Object.assign(form, { name: '', remark: '', token: '', uuid: '', password: '', config_override: '', node_ids: [] })
  showExtract.value = false
  extractInput.value = ''
  showModal.value = true
}

function openEdit(u: User) {
  editingId.value = u.id
  Object.assign(form, {
    name: u.name, remark: u.remark || '', token: u.token, uuid: u.uuid || '',
    password: u.password || '', config_override: u.config_override || '', node_ids: [...u.node_ids],
  })
  showExtract.value = false
  extractInput.value = ''
  showModal.value = true
}

function toggleNode(id: number) {
  const s = new Set(form.node_ids)
  if (s.has(id)) s.delete(id)
  else s.add(id)
  form.node_ids = [...s]
}

function applyExtract() {
  const { uuid, password } = parseCredentials(extractInput.value)
  if (uuid) form.uuid = uuid
  if (password) form.password = password
  toast(uuid || password ? '凭证已提取' : '未识别到 uuid/password', uuid || password ? 'info' : 'error')
}

async function saveUser() {
  const payload = {
    name: form.name.trim(),
    remark: form.remark.trim() || null,
    config_override: form.config_override.trim() || null,
    token: form.token || undefined,
    uuid: form.uuid || undefined,
    password: form.password || undefined,
    node_ids: form.node_ids,
    is_active: true,
  }
  try {
    if (editingId.value) {
      await api.users.update(editingId.value, payload)
      toast('用户信息更新成功')
    } else {
      await api.users.create(payload)
      toast('新用户添加成功')
    }
    showModal.value = false
    fetchData()
  } catch (e) {
    toast((e as Error).message, 'error')
  }
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

  <!-- 新增/编辑用户 Modal -->
  <div class="modal-overlay" :class="{ active: showModal }" @click.self="showModal = false">
    <div class="modal">
      <div class="modal-header">
        <div class="modal-title">{{ editingId ? '编辑订阅用户' : '新增订阅用户' }}</div>
        <button class="modal-close" @click="showModal = false">&times;</button>
      </div>
      <form @submit.prevent="saveUser">
        <div style="margin-bottom: 14px; background: rgba(59,130,246,0.1); padding: 10px; border-radius: 8px; border: 1px dashed rgba(59,130,246,0.4)">
          <div style="display: flex; justify-content: space-between; align-items: center">
            <span style="font-size: 0.8rem; color: var(--primary); font-weight: 600">📋 快捷提取凭证</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="showExtract = !showExtract">展开/折叠提取</button>
          </div>
          <div v-if="showExtract" style="margin-top: 10px">
            <textarea v-model="extractInput" class="form-control" rows="3" style="font-family: var(--font-mono); font-size: 0.78rem"
              placeholder='粘贴配置文本/JSON 片段，如：&#10;"uuid": "ef66463c-4bcb-4b20-bd42-9249758611ba",&#10;"password": "1bttcp…jjfw"'></textarea>
            <button type="button" class="btn btn-primary btn-sm" style="margin-top: 8px; width: 100%" @click="applyExtract">解析提取 UUID & Password</button>
          </div>
        </div>

        <div class="form-group">
          <label>用户姓名 / 简称</label>
          <input v-model="form.name" class="form-control" placeholder="如：张三" required />
        </div>
        <div class="form-group">
          <label>管理员内部备注 (仅管理员可见)</label>
          <input v-model="form.remark" class="form-control" placeholder="如：测试客户，2026年到期" />
        </div>
        <div class="form-group">
          <label>鉴权 Token (留空自动生成)</label>
          <div style="display: flex; gap: 8px">
            <input v-model="form.token" class="form-control" placeholder="自动生成 Token" />
            <button type="button" class="btn btn-secondary" @click="form.token = randomUUID()">随机生成</button>
          </div>
        </div>
        <div class="form-group">
          <label>专属 UUID (用于 VLESS / TUIC / AnyTLS)</label>
          <div style="display: flex; gap: 8px">
            <input v-model="form.uuid" class="form-control" placeholder="专属 UUID" />
            <button type="button" class="btn btn-secondary" @click="form.uuid = randomUUID()">随机 UUID</button>
          </div>
        </div>
        <div class="form-group">
          <label>专属 Password (用于 TUIC / AnyTLS)</label>
          <div style="display: flex; gap: 8px">
            <input v-model="form.password" class="form-control" placeholder="专属密码" />
            <button type="button" class="btn btn-secondary" @click="form.password = randomPassword()">随机密码</button>
          </div>
        </div>
        <div class="form-group">
          <label>config_override (JSON，可选，目前支持 route / dns 整体覆盖)</label>
          <textarea v-model="form.config_override" class="form-control" rows="4" style="font-family: var(--font-mono); font-size: 0.78rem"
            placeholder='{"route": {...}, "dns": {...}}'></textarea>
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">每用户独立覆盖 sing-box 配置的 route / dns 整段，其他字段不可覆盖。留空则使用默认模板。</div>
        </div>
        <div class="form-group">
          <label>绑定上游节点 (支持多选)</label>
          <div class="checkboxes-group">
            <label v-for="node in nodes" :key="node.id" class="checkbox-label">
              <input type="checkbox" :checked="form.node_ids.includes(node.id)" @change="toggleNode(node.id)" />
              {{ node.node_name }} <span style="color: var(--text-muted); font-size: 0.72rem">({{ node.protocol }})</span>
            </label>
            <div v-if="!nodes.length" style="color: var(--text-muted); font-size: 0.8rem">暂无节点，请先在节点管理创建</div>
          </div>
        </div>
        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="showModal = false">取消</button>
          <button type="submit" class="btn btn-primary">保存用户</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border: 1px solid var(--border-glass);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.82rem;
}
</style>
