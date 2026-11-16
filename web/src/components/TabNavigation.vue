<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'

const props = defineProps<{
  tabs: { key: string; to: string; label: string }[]
  activeTab: string
  pinned: boolean
}>()

const tabIndex = computed(() => props.tabs.findIndex((tab) => tab.key === props.activeTab))

const tabSliderStyle = computed(() => ({
  width: 'calc((100% - 12px) / 6)',
  transform: `translateX(calc(${tabIndex.value} * 100%))`,
}))
const tabSliderInnerStyle = computed(() => ({
  transform: `translateX(calc(-${tabIndex.value} * 16.6667%))`,
}))
</script>

<template>
  <div class="tab-navigation mobile-tab-nav" :class="{ pinned }">
    <div class="tab-slider" :style="tabSliderStyle">
      <div class="tab-slider-inner" :style="tabSliderInnerStyle"></div>
    </div>
    <RouterLink
      v-for="tab in tabs"
      :key="tab.key"
      :to="tab.to"
      class="tab-btn"
      :class="{ active: activeTab === tab.key }"
    >
      {{ tab.label }}
    </RouterLink>
  </div>
</template>

<style scoped>
.tab-navigation {
  position: sticky;
  top: 0;
  z-index: 110;
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  margin: 18px 0 20px;
  padding: 6px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.025);
  backdrop-filter: blur(12px);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.2);
  isolation: isolate;
  transition:
    box-shadow 0.24s ease,
    border-color 0.24s ease,
    background 0.24s ease;
}
.tab-navigation.pinned {
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.35);
  border-color: rgba(99, 102, 241, 0.3);
  background: rgba(15, 23, 42, 0.92);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
}
:global([data-theme='light']) .tab-navigation {
  background: rgba(0, 0, 0, 0.03);
  border-color: rgba(0, 0, 0, 0.06);
  box-shadow: inset 0 1px 2px rgba(0, 0, 0, 0.04);
}
:global([data-theme='light']) .tab-navigation.pinned {
  background: rgba(255, 255, 255, 0.85);
  border-color: rgba(37, 99, 235, 0.2);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.08);
}
.tab-slider {
  position: absolute;
  top: 6px;
  bottom: 6px;
  left: 6px;
  border-radius: 12px;
  overflow: hidden;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.3),
    0 4px 16px rgba(79, 70, 229, 0.35);
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
  pointer-events: none;
  z-index: 0;
}
.tab-slider-inner {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 600%;
  height: 100%;
  background: linear-gradient(90deg, #6366f1 0%, #8b5cf6 30%, #38bdf8 70%, #06b6d4 100%);
  transition: transform 0.28s cubic-bezier(0.22, 1, 0.36, 1);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.28);
}
.tab-btn {
  position: relative;
  z-index: 1;
  border: none;
  background: transparent;
  color: var(--text-muted);
  font-weight: 600;
  font-size: 0.93rem;
  letter-spacing: 0.01em;
  padding: 10px 14px;
  border-radius: 12px;
  text-align: center;
  cursor: pointer;
  text-decoration: none;
  transition: color 0.2s ease;
}
.tab-btn:hover {
  color: var(--text-main);
}
.tab-btn.active {
  color: #fff;
}
@media (min-width: 768px) {
  .mobile-tab-nav {
    display: none !important;
  }
}
</style>
