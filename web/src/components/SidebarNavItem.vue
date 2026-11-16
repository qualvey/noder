<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { RouterLink } from 'vue-router'
import SidebarNavIcon from './SidebarNavIcon.vue'

const props = defineProps<{
  item: { key: string; to: string; labelKey: string; icon: string; badge?: number | undefined }
  isExpanded: boolean
}>()

const { t } = useI18n()
const route = useRoute()

const isActive = computed(() => route.meta.navKey === props.item.key)
</script>

<template>
  <RouterLink
    :to="item.to"
    class="nav-item"
    :class="{ active: isActive }"
    :title="!isExpanded ? t(item.labelKey) : undefined"
  >
    <span class="nav-icon-wrapper">
      <SidebarNavIcon :name="item.icon" />
    </span>

    <span class="nav-label" v-show="isExpanded">{{ t(item.labelKey) }}</span>

    <span
      v-if="item.badge !== undefined"
      class="nav-badge"
      :class="{ 'badge-dot': !isExpanded }"
    >
      <span v-if="isExpanded">{{ item.badge }}</span>
    </span>
  </RouterLink>
</template>

<style scoped>
.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 44px;
  padding: 0 12px;
  border-radius: 10px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--text-muted);
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  white-space: nowrap;
  text-decoration: none;
}

.nav-item:hover {
  background: var(--bg-hover-weak);
  color: var(--text-main);
  border-color: rgba(255, 255, 255, 0.04);
}

.nav-item.active {
  background: linear-gradient(90deg, rgba(59, 130, 246, 0.16), rgba(99, 102, 241, 0.08));
  color: var(--primary);
  border-color: rgba(59, 130, 246, 0.3);
  font-weight: 600;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

:global([data-theme='light']) .nav-item.active {
  background: linear-gradient(90deg, rgba(37, 99, 235, 0.12), rgba(99, 102, 241, 0.05));
  border-color: rgba(37, 99, 235, 0.25);
}

.nav-icon-wrapper {
  width: 20px;
  height: 20px;
  min-width: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.nav-label {
  flex: 1;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-badge {
  background: var(--bg-soft);
  color: var(--text-dim);
  font-size: 0.72rem;
  font-weight: 600;
  padding: 1px 7px;
  border-radius: 10px;
  border: 1px solid var(--border-glass);
}

.nav-item.active .nav-badge {
  background: rgba(59, 130, 246, 0.2);
  color: var(--primary);
  border-color: rgba(59, 130, 246, 0.35);
}

.nav-badge.badge-dot {
  width: 6px;
  height: 6px;
  padding: 0;
  border-radius: 50%;
  background: var(--primary);
  position: absolute;
  top: 10px;
  right: 10px;
}
</style>