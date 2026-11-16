import { computed, nextTick, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { Node } from '../types'

/**
 * 节点选择下拉框：搜索过滤、视口边缘收拢、外部点击/滚动关闭。
 * 每张端点卡片独立实例；选中节点后通过 onPick 交还调用方
 * （调用方决定 tag 回填等表单语义），与卡片渲染解耦。
 */
export function useNodeDropdown(
  availableNodes: Ref<Node[]> | Node[],
  opts: {
    onPick: (node: Node) => void
  },
) {
  const nodesRef = Array.isArray(availableNodes) ? { value: availableNodes } : availableNodes

  const dropdownOpen = ref(false)
  const searchText = ref('')
  const inputRef = ref<HTMLInputElement | null>(null)
  const dropdownPosition = ref({ top: 0, left: 0, width: 0, maxHeight: 220 })

  const filteredOptions = computed(() => {
    const nodes = nodesRef.value
    const query = searchText.value.trim().toLowerCase()
    if (!query) return nodes
    return nodes.filter((node) =>
      [node.node_name, node.tag, node.protocol, node.server_address, node.server_port].some((value) =>
        String(value ?? '').toLowerCase().includes(query),
      ),
    )
  })

  const dropdownStyle = computed(() => ({
    top: `${dropdownPosition.value.top}px`,
    left: `${dropdownPosition.value.left}px`,
    width: `${dropdownPosition.value.width}px`,
    maxHeight: `${dropdownPosition.value.maxHeight}px`,
  }))

  function updateDropdownPosition() {
    const input = inputRef.value
    if (!input) return
    const rect = input.getBoundingClientRect()
    const viewportPadding = 8
    const spaceBelow = window.innerHeight - rect.bottom - viewportPadding
    const spaceAbove = rect.top - viewportPadding
    let top = rect.bottom + 3
    let maxHeight = Math.min(220, Math.max(80, spaceBelow))
    if (spaceBelow < 120 && spaceAbove > spaceBelow) {
      maxHeight = Math.min(220, Math.max(80, spaceAbove))
      top = rect.top - maxHeight - 3
    }
    dropdownPosition.value = {
      top: Math.max(viewportPadding, top),
      left: Math.max(viewportPadding, Math.min(rect.left, window.innerWidth - rect.width - viewportPadding)),
      width: rect.width,
      maxHeight,
    }
  }

  function stopTracking() {
    window.removeEventListener('resize', updateDropdownPosition)
    window.removeEventListener('scroll', updateDropdownPosition, true)
    window.removeEventListener('mousedown', onOutsideMouseDown)
  }

  function onOutsideMouseDown(event: MouseEvent) {
    if (!dropdownOpen.value) return
    const target = event.target
    if (!(target instanceof Element)) return
    if (inputRef.value?.contains(target) || target.closest('.endpoint-node-options')) return
    searchText.value = ''
    inputRef.value?.blur()
    dropdownOpen.value = false
  }

  function openDropdown() {
    dropdownOpen.value = true
    searchText.value = ''
    if (inputRef.value) inputRef.value.value = ''
  }

  function onSearchInput(event: Event) {
    if (!dropdownOpen.value) dropdownOpen.value = true
    searchText.value = (event.target as HTMLInputElement).value
    updateDropdownPosition()
  }

  function chooseNode(node: Node) {
    opts.onPick(node)
    searchText.value = ''
    dropdownOpen.value = false
    inputRef.value?.blur()
  }

  watch(dropdownOpen, async (open) => {
    stopTracking()
    if (!open) return
    await nextTick()
    updateDropdownPosition()
    window.addEventListener('resize', updateDropdownPosition)
    window.addEventListener('scroll', updateDropdownPosition, true)
    window.addEventListener('mousedown', onOutsideMouseDown)
  })

  onBeforeUnmount(() => {
    stopTracking()
  })

  return {
    dropdownOpen,
    searchText,
    inputRef,
    filteredOptions,
    dropdownStyle,
    openDropdown,
    onSearchInput,
    chooseNode,
  }
}
