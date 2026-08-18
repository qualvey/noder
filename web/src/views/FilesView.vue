<script setup lang="ts">
// 文件分发：APK 静态 / ZIP 模板渲染 / text 原样；本地文件 / 远程链接 / 文本内容三选一
import { inject, onMounted, ref } from 'vue'
import { api, apiBase } from '../api'
import type { DistFile, User } from '../types'
import { formatFileSize } from '../utils'

const toast = inject('toast') as (msg: string, type?: 'info' | 'error') => void
const popover = inject('popover') as { show: (el: Element, title: string, cb: () => void) => void }

const files = ref<DistFile[]>([])
const users = ref<User[]>([])
const showModal = ref(false)
const userTokens = ref<Record<number, string>>({})

// 上传表单
const form = ref({
  fileType: 'auto',
  templateName: '',
  name: '',
  remark: '',
  sourceUrl: '',
  contentText: '',
})
const fileInput = ref<HTMLInputElement | null>(null)

async function fetchData() {
  try {
    const [f, u] = await Promise.all([api.files.list(), api.users.list()])
    files.value = f
    users.value = u
    // 每行默认选第一个用户
    if (!Object.keys(userTokens.value).length && u.length) {
      const map: Record<number, string> = {}
      for (const file of f) map[file.id] = u[0].token
      userTokens.value = map
    }
  } catch (e) {
    toast((e as Error).message, 'error')
  }
}

function openCreate() {
  Object.assign(form.value, { fileType: 'auto', templateName: '', name: '', remark: '', sourceUrl: '', contentText: '' })
  if (fileInput.value) fileInput.value.value = ''
  showModal.value = true
}

async function submitUpload() {
  const hasFile = fileInput.value?.files?.length
  const hasUrl = form.value.sourceUrl.trim().length > 0
  const hasContent = form.value.contentText.trim().length > 0
  const chosen = [hasFile, hasUrl, hasContent].filter(Boolean).length
  if (chosen === 0) {
    toast('请选择文件、填写远程链接或输入文本内容', 'error')
    return
  }
  if (chosen > 1) {
    toast('本地文件 / 远程链接 / 文本内容只能三选一', 'error')
    return
  }

  const fd = new FormData()
  if (hasFile) fd.append('file', fileInput.value!.files![0])
  else if (hasUrl) fd.append('source_url', form.value.sourceUrl.trim())
  else fd.append('content_text', form.value.contentText)
  fd.append('file_type', form.value.fileType)
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

function copyLink(fileId: number) {
  const token = userTokens.value[fileId] || ''
  const url = `${location.origin}${apiBase()}/dl/${fileId}?token=${encodeURIComponent(token)}`
  navigator.clipboard.writeText(url)
  toast('下载链接已复制到剪贴板')
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
      <div class="section-title">文件分发 (APK 全员共用 / ZIP 模板按用户个性化渲染 / text 原样分发)</div>
      <div style="display: flex; gap: 10px; align-items: center">
        <button class="btn btn-primary" @click="openCreate"><span>+</span> 上传分发文件</button>
      </div>
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
            <th>下载链接 (带用户 Token)</th>
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
            <td><span class="badge" :class="f.file_type === 'zip' ? 'badge-vless' : 'badge-tuic'">{{ f.file_type.toUpperCase() }}</span></td>
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
              <div style="display: flex; gap: 6px; align-items: center">
                <select v-model="userTokens[f.id]" style="max-width: 150px; padding: 4px 6px; border-radius: 6px; background: var(--bg-card); color: var(--text); border: 1px solid var(--border-glass); font-size: 0.75rem">
                  <option v-for="u in users" :key="u.id" :value="u.token">{{ u.name }} ({{ u.token.slice(0, 8) }}…)</option>
                </select>
                <button class="btn btn-secondary btn-sm" @click="copyLink(f.id)">复制链接</button>
              </div>
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
      <b>📄 ZIP 模板占位符：</b><code>{{uuid}}</code> <code>{{password}}</code> <code>{{token}}</code> <code>{{name}}</code> <code>{{node_list_yaml}}</code> <code>{{node_list_json}}</code> <code>{{outbounds_yaml}}</code> <code>{{outbounds_json}}</code> —
      下载时按用户凭证实时渲染；其余文件原样分发。硬编码 token/uuid 也会自动按用户替换。未指定模板文件名时自动取 ZIP 内第一个 .yaml/.yml 文件。text 类型为死字符原样分发。
    </div>
  </section>

  <!-- 上传分发文件 Modal -->
  <div class="modal-overlay" :class="{ active: showModal }" @click.self="showModal = false">
    <div class="modal">
      <div class="modal-header">
        <div class="modal-title">上传分发文件</div>
        <button class="modal-close" @click="showModal = false">&times;</button>
      </div>
      <form @submit.prevent="submitUpload">
        <div class="form-group">
          <label>选择文件 (APK 或 ZIP，与远程链接/文本内容三选一)</label>
          <input ref="fileInput" type="file" class="form-control" />
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">APK：全员公用直接分发；ZIP：内含一个 yaml 模板，下载时按用户凭证渲染后再打包。</div>
        </div>
        <div class="form-group">
          <label>远程链接 (可选，每天自动刷新缓存)</label>
          <input v-model="form.sourceUrl" type="url" class="form-control" placeholder="https://example.com/app.apk" />
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">填写后服务端从远程拉取并缓存一天，过期自动重新拉取；可手动「刷新」强制更新。</div>
        </div>
        <div class="form-group">
          <label>文本内容 (可选，死字符原样分发)</label>
          <textarea v-model="form.contentText" class="form-control" rows="5" style="font-family: var(--font-mono); font-size: 0.78rem"
            placeholder="直接粘贴文本内容，原样分发给所有用户，如：&#10;token: 7df5db42-8df5-49ea-8700-a7b59d1b48d1&#10;config_file: config.json"></textarea>
          <div style="font-size: 0.72rem; color: var(--text-muted); margin-top: 4px">不做任何渲染/替换，所有用户下载到完全相同的内容。文件名取「显示名称」。</div>
        </div>
        <div class="form-group">
          <label>文件类型</label>
          <select v-model="form.fileType" class="form-control">
            <option value="auto">自动识别 (按扩展名)</option>
            <option value="apk">APK (静态分发)</option>
            <option value="zip">ZIP (模板渲染)</option>
          </select>
        </div>
        <div class="form-group">
          <label>ZIP 内模板文件名 (留空自动识别第一个 .yaml/.yml)</label>
          <input v-model="form.templateName" class="form-control" placeholder="如 config.yaml" />
        </div>
        <div class="form-group">
          <label>显示名称 (留空用上传文件名)</label>
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
