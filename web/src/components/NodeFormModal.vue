<script setup lang="ts">
// 节点新增/编辑表单弹窗
// 自包含：表单状态 / JSON 导入 / 协议联动 / Ctrl+V 快捷提取
// 契约：props { open, editing }，emits { close, saved }
import { inject, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { api } from '../api'
import type { Node } from '../types'
import { parseNodeJson } from '../utils'

const props = defineProps<{
  open: boolean
  editing: Node | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void

const showJsonImport = ref(false)
const jsonInput = ref('')

const form = reactive<{
  tag: string
  node_name: string
  protocol: 'tuic' | 'vless' | 'anytls'
  server_address: string
  server_port: number
  security: string
  sni: string
  transport_type: string
  path: string
  public_key: string
  short_id: string
  fingerprint: string
  flow: string
  congestion_control: string
  remark: string
}>({
  tag: '',
  node_name: '',
  protocol: 'vless',
  server_address: '',
  server_port: 8443,
  security: 'reality',
  sni: '',
  transport_type: 'direct',
  path: '',
  public_key: '',
  short_id: '',
  fingerprint: 'chrome',
  flow: 'xtls-rprx-vision',
  congestion_control: 'bbr',
  remark: '',
})

const securityOptions: Record<string, string[]> = {
  tuic: ['tls'],
  vless: ['reality'],
  anytls: ['tls', 'reality', 'auto', 'none'],
}

function resetForm() {
  Object.assign(form, {
    tag: '', node_name: '', protocol: 'vless', server_address: '', server_port: 8443,
    security: 'reality', sni: '', transport_type: 'direct', path: '',
    public_key: '', short_id: '', fingerprint: 'chrome', flow: 'xtls-rprx-vision',
    congestion_control: 'bbr', remark: '',
  })
  showJsonImport.value = false
  jsonInput.value = ''
}

function loadForm(n: Node) {
  Object.assign(form, {
    tag: n.tag || '', node_name: n.node_name || '', protocol: n.protocol, server_address: n.server_address,
    server_port: n.server_port, security: n.security || 'tls', sni: n.sni || '',
    transport_type: n.transport_type || 'direct', path: n.path || '',
    public_key: n.public_key || '', short_id: n.short_id || '',
    fingerprint: n.fingerprint || 'chrome', flow: n.flow || 'xtls-rprx-vision',
    congestion_control: n.congestion_control || 'bbr', remark: n.remark || '',
  })
  showJsonImport.value = false
  jsonInput.value = ''
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

// 协议联动：锁定 security 选项
function onProtocolChange() {
  const opts = securityOptions[form.protocol] || ['tls']
  if (!opts.includes(form.security)) form.security = opts[0]
}

async function saveNode() {
  const payload: Partial<Node> = {
    tag: form.tag.trim(),
    node_name: form.node_name.trim() || null,
    protocol: form.protocol,
    server_address: form.server_address.trim(),
    server_port: Number(form.server_port),
    security: form.security,
    sni: form.sni.trim() || null,
    transport_type: form.transport_type,
    path: form.path.trim() || null,
    public_key: form.public_key.trim() || null,
    short_id: form.short_id.trim() || null,
    fingerprint: form.fingerprint.trim() || null,
    flow: form.flow.trim() || null,
    congestion_control: form.protocol === 'tuic' ? (form.congestion_control || 'bbr') : null,
    remark: form.remark.trim() || null,
    is_active: true,
  }
  try {
    if (props.editing) {
      await api.nodes.update(props.editing.id, payload)
      toast('节点更新成功')
    } else {
      await api.nodes.create(payload)
      toast('新节点添加成功')
    }
    emit('close')
    emit('saved')
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

// JSON 导入：解析 outbound JSON 填充表单
function applyJsonImport() {
  const obj = parseNodeJson(jsonInput.value)
  if (!obj) {
    toast('JSON 解析失败', 'error')
    return
  }
  const tls = (obj.tls as Record<string, unknown>) || {}
  const reality = (tls.reality as Record<string, unknown>) || {}
  const utls = (tls.utls as Record<string, unknown>) || {}
  const transport = (obj.transport as Record<string, unknown>) || {}

  form.tag = String(obj.tag || '')
  form.node_name = String(obj.tag || '')
  const proto = (obj.type as string) || form.protocol
  form.protocol = proto as 'tuic' | 'vless' | 'anytls'
  form.server_address = String(obj.server || '')
  form.server_port = Number(obj.server_port || 8443)
  form.security = tls?.enabled ? (reality?.enabled ? 'reality' : 'tls') : 'none'
  form.sni = String(tls.server_name || '')
  form.public_key = String(reality.public_key || '')
  form.short_id = String(reality.short_id || '')
  form.fingerprint = String((utls.fingerprint as string) || 'chrome')
  form.flow = String(obj.flow || 'xtls-rprx-vision')
  form.congestion_control = String((obj.congestion_control as string) || 'bbr')
  form.transport_type = String(transport.type || 'direct')
  form.path = String(transport.path || '')
  onProtocolChange()
  toast('JSON 解析并填充完成')
}

// 全局 Ctrl+V 快捷提取（弹窗打开且无输入框聚焦时）
function onGlobalKeydown(e: KeyboardEvent) {
  if (!props.open) return
  const tag = (e.target as HTMLElement)?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'v') {
    navigator.clipboard.readText().then((text) => {
      jsonInput.value = text
      showJsonImport.value = true
      applyJsonImport()
    }).catch(() => {})
  }
}

onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onUnmounted(() => window.removeEventListener('keydown', onGlobalKeydown))
</script>

<template>
  <div class="modal-overlay" :class="{ active: open }" @click.self="emit('close')">
    <div class="modal">
      <div class="modal-header">
        <div class="modal-title">{{ editing ? '编辑节点' : '新增代理节点' }}</div>
        <button class="modal-close" @click="emit('close')">&times;</button>
      </div>
      <form @submit.prevent="saveNode" class="modal-form">
        <div class="form-span" style="margin-bottom: 14px; background: rgba(59,130,246,0.1); padding: 10px; border-radius: 8px; border: 1px dashed rgba(59,130,246,0.4)">
          <div style="display: flex; justify-content: space-between; align-items: center">
            <span style="font-size: 0.8rem; color: var(--primary); font-weight: 600">📋 从 JSON 导入 <span style="font-weight: normal; font-size: 0.75rem; color: var(--text-muted)">(按 Ctrl+V 快捷提取)</span></span>
            <button type="button" class="btn btn-secondary btn-sm" @click="showJsonImport = !showJsonImport">展开/折叠导入</button>
          </div>
          <div v-if="showJsonImport" style="margin-top: 10px">
            <textarea v-model="jsonInput" class="form-control" rows="4" style="font-family: var(--font-mono); font-size: 0.78rem"
              placeholder='粘贴 Outbound JSON 文本，如：&#10;{ "type": "vless", "tag": "my-node", "server": "1.2.3.4", "server_port": 443, "tls": { ... } }'></textarea>
            <button type="button" class="btn btn-primary btn-sm" style="margin-top: 8px; width: 100%" @click="applyJsonImport">解析并一键填充</button>
          </div>
        </div>

        <div class="form-group">
          <label>节点标识 Tag <span style="color: var(--accent-rose)">*</span></label>
          <input v-model="form.tag" class="form-control" placeholder="如 hk-hkt-01（配置内唯一标识）" required />
        </div>
        <div class="form-group">
          <label>展示名称 (可选)</label>
          <input v-model="form.node_name" class="form-control" placeholder="如：香港 HKT 专线 01" />
        </div>
        <div class="form-group">
          <label>管理员备注 (仅管理员可见)</label>
          <input v-model="form.remark" class="form-control" placeholder="如：香港 HKT 物理机 2026到期" />
        </div>
        <div class="form-group">
          <label>协议类型 (仅限 3 种)</label>
          <select v-model="form.protocol" class="form-control" @change="onProtocolChange">
            <option value="tuic">TUIC</option>
            <option value="vless">VLESS</option>
            <option value="anytls">AnyTLS</option>
          </select>
        </div>
        <div class="form-group">
          <label>端口</label>
          <input v-model.number="form.server_port" type="number" class="form-control" placeholder="8443" required />
        </div>
        <div class="form-group">
          <label>服务器地址 (IP / 域名)</label>
          <input v-model="form.server_address" class="form-control" placeholder="1.2.3.4 或 hk.example.com" required />
        </div>
        <div class="form-group">
          <label>传输安全 (Security)</label>
          <select v-model="form.security" class="form-control">
            <option v-for="opt in securityOptions[form.protocol] || ['tls']" :key="opt" :value="opt">{{ opt.toUpperCase() }}</option>
          </select>
        </div>
        <div class="form-group">
          <label>TLS SNI / ServerName</label>
          <input v-model="form.sni" class="form-control" placeholder="aws.amazon.com" />
        </div>

        <div v-if="form.protocol === 'tuic'" class="form-group">
          <label>拥塞控制 (Congestion Control)</label>
          <select v-model="form.congestion_control" class="form-control">
            <option value="bbr">bbr</option>
            <option value="cubic">cubic</option>
            <option value="new_reno">new_reno</option>
          </select>
        </div>

        <div v-if="form.security === 'reality'" class="form-span reality-panel">
          <div class="form-group">
            <label>Public Key (REALITY 公钥 <span style="color: var(--accent-rose)">*</span>)</label>
            <input v-model="form.public_key" class="form-control" placeholder="如 99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo" />
          </div>
          <div class="form-group">
            <label>Short ID (简短 ID <span style="color: var(--accent-rose)">*</span>)</label>
            <input v-model="form.short_id" class="form-control" placeholder="如 1a91" />
          </div>
          <div class="form-group">
            <label>uTLS 指纹 (Fingerprint)</label>
            <input v-model="form.fingerprint" class="form-control" placeholder="chrome" />
          </div>
          <div class="form-group">
            <label>Flow (流控方式)</label>
            <input v-model="form.flow" class="form-control" placeholder="xtls-rprx-vision" />
          </div>
        </div>

        <div v-if="['vless', 'anytls'].includes(form.protocol)" class="form-group">
          <label>传输层 (Transport)</label>
          <select v-model="form.transport_type" class="form-control">
            <option value="direct">direct</option>
            <option value="ws">WebSocket (ws)</option>
            <option value="grpc">gRPC</option>
            <option value="http">HTTP</option>
          </select>
        </div>
        <div v-if="['vless', 'anytls'].includes(form.protocol)" class="form-group">
          <label>路径 (Path)</label>
          <input v-model="form.path" class="form-control" placeholder="/path" />
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="emit('close')">取消</button>
          <button type="submit" class="btn btn-primary">保存节点</button>
        </div>
      </form>
    </div>
  </div>
</template>
