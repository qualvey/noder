<script setup lang="ts">
// 文件分发：先选类型（普通文件 / zip / 文本），按类型展示对应输入
// 普通文件与文本用共享 token 下载（可重置）；zip 按用户个性化渲染，在用户列表右键下载
import { inject, onMounted, ref } from 'vue'
import { api } from '../api'
import type { DistFile } from '../types'
import { apiLinkPrefix, formatFileSize } from '../utils'

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }

const files = ref<DistFile[]>([])
const showModal = ref(false)
const sharedToken = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

type FileType = 'apk' | 'zip' | 'text'
const form = ref({
  type: 'apk' as FileType,
  templateName: '',
  name: '',
  remark: '',
  sourceUrl: '',
  contentText: '',
})

const typeOptions: { value: FileType; label: string; desc: string }[] = [
  { value: 'apk', label: '普通文件', desc: 'URL 或上传文件二选一' },
  { value: 'zip', label: 'ZIP 配置包', desc: '内含 config.yaml 模板，按用户渲染' },
  { value: 'text', label: '文本', desc: '字符串原样分发' },
]

async function fetchData() {
  try {
    const [f, s] = await Promise.all([api.files.list(), api.settings.sharedToken()])
    files.value = f
    sharedToken.value = s.token
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

function openCreate() {
  Object.assign(form.value, { type: 'apk', templateName: '', name: '', remark: '', sourceUrl: '', contentText: '' })
  if (fileInput.value) fileInput.value.value = ''
  showModal.value = true
}

function pickType(t: FileType) {
  form.value.type = t
}

async function submitUpload() {
  const hasFile = fileInput.value?.files?.length
  const hasUrl = form.value.sourceUrl.trim().length > 0
  const hasContent = form.value.contentText.trim().length > 0

  // 按类型约束输入来源
  if (form.value.type === 'text') {
    if (!hasContent) {
      toast('文本类型需要输入字符串内容', 'error')
      return
    }
  } else if (form.value.type === 'zip') {
    if (!hasFile) {
      toast('ZIP 类型需要上传文件', 'error')
      return
    }
  } else {
    // 普通文件：URL 或上传文件二选一
    if (hasFile && hasUrl) {
      toast('普通文件：URL 与上传文件只能二选一', 'error')
      return
    }
    if (!hasFile && !hasUrl) {
      toast('普通文件需要上传文件或填写 URL', 'error')
      return
    }
  }

  const fd = new FormData()
  if (hasFile) fd.append('file', fileInput.value!.files![0])
  if (hasUrl) fd.append('source_url', form.value.sourceUrl.trim())
  if (hasContent) fd.append('content_text', form.value.contentText)
  fd.append('file_type', form.value.type)
  fd.append('template_name', form.value.templateName.trim())
  fd.append('name', form.value.name.trim())
  fd.append('remark', form.value.remark.trim())

  try {
    await api.files.create(fd)
    toast(hasUrl ? '远程文件已添加 (首次拉取完成)' : hasContent ? '文本内容已保存' : '分发文件上传成功')
    showModal.value = false
    fetchData()
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

// 普通文件/文本：复制共享 token 链接
function copySharedLink(f: DistFile) {
  if (!sharedToken.value) return
  const url = `${location.origin}${apiLinkPrefix()}/dl/${f.id}?token=${encodeURIComponent(sharedToken.value)}`
  navigator.clipboard.writeText(url).then(() => toast('共享下载链接已复制'), () => toast('复制失败', 'error'))
}

async function copySharedToken() {
  await navigator.clipboard.writeText(sharedToken.value)
  toast('共享 Token 已复制')
}

function resetSharedToken(btn: Element) {
  popover.show(btn, '⚠️ 重置共享 Token？旧链接将全部失效', async () => {
    try {
      const r = await api.settings.resetSharedToken()
      sharedToken.value = r.token
      toast('共享 Token 已重置，旧链接全部失效')
    } catch (e) {
      toast((e as Error).message, 'error')
    }
  })
}

function toggleActive(f: DistFile) {
  api.files.update(f.id, { is_active: !f.is_active })
    .then(() => {
      toast(f.is_active ? '文件已停用' : '文件已启用')
      fetchData()
    })
    .catch((e) => toast(e.message, 'error'))
}

function refreshRemote(f: DistFile, btn: HTMLButtonElement) {
  btn.disabled = true
  btn.textContent = '刷新中...'
  api.files.refresh(f.id)
    .then(() => {
      toast('远程文件已刷新')
      fetchData()
    })
    .catch((e) => {
      toast(e.message, 'error')
      btn.disabled = false
      btn.textContent = '🔄 刷新'
    })
}

function removeFile(f: DistFile) {
  api.files.remove(f.id)
    .then(() => {
      toast('分发文件已删除')
      fetchData()
    })
    .catch((e) => toast(e.message, 'error'))
}

onMounted(fetchData)
</script>

<template>
  <section class="tab-content" style="display: block">
    <div class="section-header">
      <div class="section-title">文件分发 (普通文件 / ZIP 配置包 / 文本)</div>
      <div style="display: flex; gap: 10px; align-items: center">
        <button class="btn btn-primary" @click="openCreate"><span>+</span> 上传分发文件</button>
      </div>
    </div>

    <div class="shared-token-bar">
      <span style="font-weight: 600">🔑 共享下载 Token</span>
      <span style="color: var(--text-muted); font-size: 0.8rem">(普通文件 / 文本文件下载鉴权，与用户 Token 独立)</span>
      <code class="shared-token-value">{{ sharedToken }}</code>
      <button class="btn btn-secondary btn-sm" @click="copySharedToken">复制</button>
      <button class="btn btn-danger btn-sm" @click="resetSharedToken($event.currentTarget as Element)">重置 (旧链接失效)</button>
    </div>

    <div class="table-container">
      <table class="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>名称</th>
            <th>类型</th>
            <th>大小</th>
            <th>来源</th>
            <th>ZIP 模板文件</th>
            <th>状态</th>
            <th>下载方式</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!files.length">
            <td colspan="9" style="text-align: center; color: var(--text-muted); padding: 30px">暂无分发文件，点击右上角上传</td>
          </tr>
          <tr v-for="f in files" :key="f.id">
            <td>{{ f.id }}</td>
            <td>
              {{ f.name }}
              <div v-if="f.remark" style="font-size: 0.72rem; color: var(--text-muted)">{{ f.remark }}</div>
            </td>
            <td>
              <span class="badge" :class="f.file_type === 'zip' ? 'badge-vless' : f.file_type === 'text' ? 'badge-anytls' : 'badge-tuic'">
                {{ f.file_type === 'apk' ? '普通文件' : f.file_type.toUpperCase() }}
              </span>
            </td>
            <td>{{ formatFileSize(f.size) }}</td>
            <td>
              <span v-if="f.source_url" :title="f.source_url" style="cursor: help">🔗 远程</span>
              <span v-else-if="f.file_type === 'text'" style="color: var(--text-muted)">📝 文本</span>
              <span v-else style="color: var(--text-muted)">📁 本地</span>
            </td>
            <td>{{ f.file_type === 'zip' ? f.template_name || '-' : '-' }}</td>
            <td>
              <span v-if="f.is_active" style="color: var(--accent-emerald)">🟢 启用</span>
              <span v-else style="color: var(--accent-rose)">🔴 停用</span>
            </td>
            <td>
              <template v-if="f.file_type === 'zip'">
                <span style="font-size: 0.75rem; color: var(--text-muted)">📦 用户列表右键下载（按用户个性化）</span>
              </template>
              <template v-else>
                <button class="btn btn-secondary btn-sm" @click="copySharedLink(f)">复制共享链接</button>
              </template>
            </td>
            <td>
              <div style="display: flex; gap: 6px">
                <button v-if="f.source_url" class="btn btn-secondary btn-sm" @click="refreshRemote(f, $event.currentTarget as HTMLButtonElement)">🔄 刷新</button>
                <button class="btn btn-secondary btn-sm" @click="toggleActive(f)">{{ f.is_active ? '停用' : '启用' }}</button>
                <button class="btn btn-danger btn-sm" @click="popover.show($event.currentTarget as Element, '⚠️ 确定删除该分发文件？(磁盘文件一并删除)', () => removeFile(f))">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-pre style="margin-top: 14px; background: var(--bg-card); padding: 14px; border-radius: var(--radius-lg); border: 1px solid var(--border-glass); font-size: 0.82rem; color: var(--text-muted)">
      <b>📄 ZIP 模板占位符：</b><code>{{uuid}}</code> <code>{{password}}</code> <code>{{token}}</code> <code>{{name}}</code> <code>{{node_list_yaml}}</code> <code>{{node_list_json}}</code> <code>{{outbounds_yaml}}</code> <code>{{outbounds_json}}</code> <code>{{mihomo_proxies_yaml}}</code> —
      下载时按用户凭证实时渲染；其余文件原样分发。硬编码 token/uuid 也会自动按用户替换。未指定模板文件名时自动取 ZIP 内第一个 .yaml/.yml 文件。文本类型为死字符原样分发。
    </div>
  </section>

  <!-- 上传分发文件 Modal -->
  <div class="modal-overlay" :class="{ active: showModal }" @click.self="showModal = false">
    <div class="modal">
      <div class="modal-header">
        <div class="modal-title">上传分发文件</div>
        <button class="modal-close" @click="showModal = false">&times;</button>
      </div>
      <form @submit.prevent="submitUpload" class="modal-form">
        <div class="form-group form-span">
          <label>类型（先选择）</label>
          <div class="type-segmented">
            <button
              v-for="opt in typeOptions"
              :key="opt.value"
              type="button"
              class="type-btn"
              :class="{ active: form.type === opt.value }"
              @click="pickType(opt.value)"
            >
              <span style="font-weight: 600">{{ opt.label }}</span>
              <span style="font-size: 0.72rem; opacity: 0.75">{{ opt.desc }}</span>
            </button>
          </div>
        </div>

        <!-- 普通文件：URL 或文件二选一 -->
        <template v-if="form.type === 'apk'">
          <div class="form-group">
            <label>上传文件 (方式一)</label>
            <input ref="fileInput" type="file" class="form-control" />
          </div>
          <div class="form-group">
            <label>远程 URL (方式二，每天自动刷新缓存)</label>
            <input v-model="form.sourceUrl" type="url" class="form-control" placeholder="https://example.com/app.apk" />
          </div>
          <div class="form-group form-span" style="font-size: 0.75rem; color: var(--text-muted)">
            ⚠️ 两种方式二选一，同时填写会拒绝。远程链接首次提交即拉取并缓存一天。
          </div>
        </template>

        <!-- ZIP 配置包 -->
        <template v-else-if="form.type === 'zip'">
          <div class="form-group form-span">
            <label>上传 ZIP 配置包（内含 config.yaml 模板）</label>
            <input ref="fileInput" type="file" class="form-control" accept=".zip" />
          </div>
          <div class="form-group form-span">
            <label>模板文件名 (留空自动识别第一个 .yaml/.yml)</label>
            <input v-model="form.templateName" class="form-control" placeholder="如 config.yaml" />
          </div>
        </template>

        <!-- 文本 -->
        <template v-else>
          <div class="form-group form-span">
            <label>文本内容（原样分发，不做渲染）</label>
            <textarea v-model="form.contentText" class="form-control" rows="6" style="font-family: var(--font-mono); font-size: 0.78rem"
              placeholder="直接粘贴文本内容，所有用户下载到完全相同的内容"></textarea>
          </div>
        </template>

        <div class="form-group">
          <label>显示名称 (留空用文件名)</label>
          <input v-model="form.name" class="form-control" placeholder="如 客户端安装包" />
        </div>
        <div class="form-group">
          <label>备注</label>
          <input v-model="form.remark" class="form-control" placeholder="如 仅供付费用户下载" />
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" @click="showModal = false">取消</button>
          <button type="submit" class="btn btn-primary">上传</button>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.shared-token-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  background: var(--bg-card);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 10px 14px;
  margin-bottom: 16px;
}

.shared-token-value {
  font-family: var(--font-mono);
  font-size: 0.8rem;
  color: var(--primary);
  background: var(--bg-input);
  padding: 3px 8px;
  border-radius: 6px;
}

.type-segmented {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}

.type-btn {
  display: flex;
  flex-direction: column;
  gap: 2px;
  align-items: flex-start;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-glass);
  background: var(--bg-input);
  color: var(--text-main);
  cursor: pointer;
  transition: all 0.2s ease;
  text-align: left;
}

.type-btn:hover {
  border-color: var(--primary);
}

.type-btn.active {
  border-color: var(--primary);
  background: rgba(59, 130, 246, 0.12);
  box-shadow: 0 0 0 1px var(--primary);
}
</style>
