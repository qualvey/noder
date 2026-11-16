import { nextTick, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import type { TabKey } from '../navigation'

/**
 * 标签页吸顶导航：锚点触顶时固定并撑高容器，切 Tab 时还原滚动位置。
 * 监听 scroll/resize，卸载时自动清理。
 */
export function useTabStickyNav(activeTab: Ref<TabKey>) {
  const stickyAnchorRef = ref<HTMLElement | null>(null)
  const tabNavPinned = ref(false)
  const isHeightExpanded = ref(false)

  function getNavNaturalScrollY(): number {
    if (!stickyAnchorRef.value) return 0
    const anchorRect = stickyAnchorRef.value.getBoundingClientRect()
    return Math.max(0, window.scrollY + anchorRect.top)
  }

  function updateTabNavSticky() {
    if (!stickyAnchorRef.value) return
    const anchorRect = stickyAnchorRef.value.getBoundingClientRect()
    const currentScrollY = window.scrollY

    const isStuck = anchorRect.top <= 1 && currentScrollY > 2
    tabNavPinned.value = isStuck

    if (isStuck) {
      isHeightExpanded.value = true
    }

    if (currentScrollY <= 2) {
      isHeightExpanded.value = false
      tabNavPinned.value = false
    }
  }

  const handleTabNavScroll = () => updateTabNavSticky()

  watch(activeTab, async () => {
    const wasSticky = tabNavPinned.value || isHeightExpanded.value
    const targetScrollY = getNavNaturalScrollY()

    if (wasSticky) {
      isHeightExpanded.value = true
      window.scrollTo({ top: targetScrollY, behavior: 'instant' })
    }

    await nextTick()

    if (wasSticky) {
      window.scrollTo({ top: targetScrollY, behavior: 'instant' })
      tabNavPinned.value = true
      isHeightExpanded.value = true
    } else {
      updateTabNavSticky()
    }
  })

  onMounted(() => {
    updateTabNavSticky()
    window.addEventListener('scroll', handleTabNavScroll, { passive: true })
    window.addEventListener('resize', handleTabNavScroll, { passive: true })
  })

  onBeforeUnmount(() => {
    window.removeEventListener('scroll', handleTabNavScroll)
    window.removeEventListener('resize', handleTabNavScroll)
  })

  return { stickyAnchorRef, tabNavPinned, isHeightExpanded, updateTabNavSticky }
}
