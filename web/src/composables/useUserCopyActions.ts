import { inject, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '../api'
import type { DistFile, User } from '../types'
import { buildDownloadLink, buildMihomoLink, buildSubLink, buildXrayLink, copyText } from '../utils'
import { ToastKey } from '../injectionKeys'
import type { ContextMenuItem } from '../components/ContextMenu.vue'

export interface UserCtxMenu {
  x: number
  y: number
  user: User
}

/**
 * 用户行右键菜单的全部复制操作。
 * 收拢原本散落在 UsersView 中的 8 个 copy* 函数与菜单项装配。
 */
export function useUserCopyActions(
  ctxMenu: Ref<UserCtxMenu | null>,
  zipFiles: Ref<DistFile[]>,
  closeCtxMenu: () => void,
  openTemplateBranchModal: () => void,
) {
  const { t } = useI18n()
  const toast = inject(ToastKey)

  function copySubLink() {
    const user = ctxMenu.value?.user
    if (!user) return
    copyText(buildSubLink(user.token)).then((ok) => {
      toast?.(ok ? t('users.subLinkCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
    })
    closeCtxMenu()
  }

  function copyMihomoLink() {
    const user = ctxMenu.value?.user
    if (!user) return
    copyText(buildMihomoLink(user.token)).then((ok) => {
      toast?.(ok ? t('users.mihomoLinkCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
    })
    closeCtxMenu()
  }

  function copyXrayLink() {
    const user = ctxMenu.value?.user
    if (!user) return
    copyText(buildXrayLink(user.token)).then((ok) => {
      toast?.(ok ? t('users.xrayLinkCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
    })
    closeCtxMenu()
  }

  async function copyNodes() {
    const user = ctxMenu.value?.user
    if (!user) return
    closeCtxMenu()
    try {
      const nodes = await api.userNodes(user.token)
      const formattedText = nodes.map((node) => JSON.stringify(node, null, 2)).join(',\n')
      const ok = await copyText(formattedText)
      toast?.(ok ? t('users.nodesJsonCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
    } catch (e) {
      toast?.((e as Error).message, 'error')
    }
  }

  async function copyUser() {
    const user = ctxMenu.value?.user
    if (!user) return
    closeCtxMenu()
    try {
      const payload = {
        name: user.name,
        uuid: user.uuid ?? '',
        password: user.password ?? '',
      }
      await navigator.clipboard.writeText(JSON.stringify(payload, null, 2))
      toast?.(t('users.userCopied'), 'success')
    } catch (e) {
      toast?.((e as Error).message, 'error')
    }
  }

  // Copy the credential object accepted by Xray VLESS users/clients.
  async function copyXrayUser() {
    const user = ctxMenu.value?.user
    if (!user) return
    closeCtxMenu()
    const payload = {
      id: user.uuid ?? '',
      email: user.name,
    }
    const ok = await copyText(JSON.stringify(payload, null, 2))
    toast?.(ok ? t('users.xrayUserCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
  }

  function copyValue(val: string | undefined | null, label: string) {
    if (!val || val === '-') return
    copyText(val).then((ok) => {
      toast?.(ok ? t('users.copiedToClipboard', { label }) : t('common.copyFailed'), ok ? 'info' : 'error')
    })
  }

  function copyToken() {
    const user = ctxMenu.value?.user
    if (!user) return
    closeCtxMenu()
    copyText(user.token).then((ok) => {
      toast?.(ok ? t('users.tokenCopied') : t('common.copyFailed'), ok ? 'info' : 'error')
    })
  }

  // 复制 ZIP 配置包下载链接
  function copyZipLink(f: DistFile) {
    const user = ctxMenu.value?.user
    if (!user) return
    closeCtxMenu()
    const url = buildDownloadLink(f.id, user.token)
    copyText(url).then((ok) => {
      toast?.(ok ? t('users.downloadLinkCopied', { name: f.name }) : t('common.copyFailed'), ok ? 'info' : 'error')
    })
  }

  const ctxMenuItems = (): ContextMenuItem[] => {
    const items: ContextMenuItem[] = [
      { label: t('users.menu.copyToken'), icon: '🔑', onClick: copyToken },
      { label: t('users.menu.copySingbox'), icon: '📋', onClick: copySubLink },
      { label: t('users.menu.copyMihomo'), icon: '🔄', onClick: copyMihomoLink },
      { label: t('users.menu.copyXray'), icon: '⚡', onClick: copyXrayLink },
      { label: t('users.menu.copyNodes'), icon: '📄', onClick: copyNodes },
      { label: t('users.menu.copyUser'), icon: '👤', onClick: copyUser },
      { label: t('users.menu.copyXrayUser'), icon: '🛠️', onClick: copyXrayUser },
    ]
    if (zipFiles.value.length) {
      items.push({ label: t('users.menu.configDlHeader'), icon: '📦', divider: true, onClick: () => {} })
      for (const z of zipFiles.value) {
        items.push({ label: t('users.menu.copyDl', { name: z.name }), icon: '🔗', onClick: () => copyZipLink(z) })
      }
    }
    items.push({ label: t('users.menu.templateBranch'), icon: '📝', divider: true, onClick: openTemplateBranchModal })
    return items
  }

  return { copySubLink, copyMihomoLink, copyXrayLink, copyNodes, copyUser, copyXrayUser, copyValue, copyToken, copyZipLink, ctxMenuItems }
}
