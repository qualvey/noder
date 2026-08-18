<script setup lang="ts">
// 用户新增/编辑表单弹窗
// 自包含：凭证生成 / 快捷提取 / 节点多选 / config_override
// 契约：props { open, editing, nodes }，emits { close, saved }
import { inject, reactive, ref, watch } from 'vue'
import { api } from '../api'
import type { Node, User } from '../types'
import { parseCredentials, randomPassword, randomUUID } from '../utils'

const props = defineProps<{
  open: boolean
  editing: User | null
  nodes: Node[]
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void

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

function resetForm() {
  Object.assign(form, { name: '', remark: '', token: '', uuid: '', password: '', config_override: '', node_ids: [] })
  showExtract.value = false
  extractInput.value = ''
}

function loadForm(u: User) {
  Object.assign(form, {
    name: u.name, remark: u.remark || '', token: u.token, uuid: u.uuid || '',
    password: u.password || '', config_override: u.config_override || '', node_ids: [...u.node_ids],
  })
  showExtract.value = false
  extractInput.value = ''
}

// 打开弹窗时初始化表单（编辑回填 / 新增重置）
watch(
  () => props.open,
  (open) => {
    if (!open) return
    if (props.editing) loadForm(props.editing)
    else resetForm()
  },
)

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
    if (props.editing) {
      await api.users.update(props.editing.id, payload)
      toast('用户信息更新成功')
    } else {
      await api.users.create(payload)
      toast('新用户添加成功')
    }
    emit('close')
    emit('saved')
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}
</script>

<template>
  <div class="modal-overlay" :class="{ active: open }" @click.self="emit('close')">
    <div class="modal">
      <div class="modal-header">
        <div class="modal-title">{{ editing ? '编辑订阅用户' : '新增订阅用户' }}</div>
        <button class="modal-close" @click="emit('close')">&times;</button>
      </div>
      <form @submit.prevent="saveUser" class="modal-form">
        <div class="form-span" style="margin-bottom: 14px; background: rgba(59,130,246,0.1); padding: 10px; border-radius: 8px; border: 1px dashed rgba(59,130,246,0.4)">
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
          <div class="credential-row">
            <input v-model="form.token" class="form-control" placeholder="自动生成 Token" />
            <button type="button" class="btn btn-secondary" @click="form.token = randomUUID()">随机生成</button>
          </div>
        </div>
        <div class="form-group">
          <label>专属 UUID (用于 VLESS / TUIC / AnyTLS)</label>
          <div class="credential-row">
            <input v-model="form.uuid" class="form-control" placeholder="专属 UUID" />
            <button type="button" class="btn btn-secondary" @click="form.uuid = randomUUID()">随机 UUID</button>
          </div>
        </div>
        <div class="form-group">
          <label>专属 Password (用于 TUIC / AnyTLS)</label>
          <div class="credential-row">
            <input v-model="form.password" class="form-control" placeholder="专属密码" />
            <button type="button" class="btn btn-secondary" @click="form.password = randomPassword()">随机密码</button>
          </div>
        </div>
        <div class="form-group form-span">
          <label>config_override (JSON，可选，目前支持 route / dns 整体覆盖)</label>
          <textarea v-model="form.config_override" class="form-control" rows="4" style="font-family: var(--font-mono); font-size: 0.78rem"
            placeholder='{"route": {...}, "dns": {...}}'></textarea>
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">每用户独立覆盖 sing-box 配置的 route / dns 整段，其他字段不可覆盖。留空则使用默认模板。</div>
        </div>
        <div class="form-group form-span">
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
          <button type="button" class="btn btn-secondary" @click="emit('close')">取消</button>
          <button type="submit" class="btn btn-primary">保存用户</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
/* 凭证输入 + 随机按钮一行，输入框弹性伸缩 */
.credential-row {
  display: flex;
  gap: 8px;
}

.credential-row .form-control {
  flex: 1;
  min-width: 0;
}

/* 节点多选：宽度充足时多列排布 */
.checkboxes-group {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 6px;
}

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
