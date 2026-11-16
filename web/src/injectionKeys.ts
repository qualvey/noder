import type { InjectionKey } from 'vue'
import type { ToastType } from './composables/useToast'

export type { ToastType }
export const ToastKey: InjectionKey<(msg: string, type?: ToastType) => void> = Symbol('toast')
export const PopoverKey: InjectionKey<{
  show: (el: Element, title: string, cb: () => void) => void
  hide: () => void
}> = Symbol('popover')
export const MetricsKey: InjectionKey<(nodes: number, users: number) => void> = Symbol('metrics')
