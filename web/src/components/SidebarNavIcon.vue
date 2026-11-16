<script setup lang="ts">
defineProps<{
  name: string
}>()
</script>

<template>
  <svg class="nav-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <!-- Nodes -->
    <rect v-if="name === 'nodes'" x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
    <rect v-if="name === 'nodes'" x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
    <line v-if="name === 'nodes'" x1="6" y1="6" x2="6.01" y2="6"></line>
    <line v-if="name === 'nodes'" x1="6" y1="18" x2="6.01" y2="18"></line>

    <!-- Users -->
    <path v-else-if="name === 'users'" d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
    <circle v-else-if="name === 'users'" cx="9" cy="7" r="4"></circle>
    <path v-else-if="name === 'users'" d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
    <path v-else-if="name === 'users'" d="M16 3.13a4 4 0 0 1 0 7.75"></path>

    <!-- Agents -->
    <rect v-else-if="name === 'agents'" x="3" y="3" width="7" height="7" rx="1"></rect>
    <rect v-else-if="name === 'agents'" x="14" y="3" width="7" height="7" rx="1"></rect>
    <rect v-else-if="name === 'agents'" x="3" y="14" width="7" height="7" rx="1"></rect>
    <rect v-else-if="name === 'agents'" x="14" y="14" width="7" height="7" rx="1"></rect>
    <circle v-else-if="name === 'agents'" cx="12" cy="12" r="1" fill="currentColor"></circle>

    <!-- Files -->
    <path v-else-if="name === 'files'" d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
    <polyline v-else-if="name === 'files'" points="14 2 14 8 20 8"></polyline>
    <line v-else-if="name === 'files'" x1="16" y1="13" x2="8" y2="13"></line>
    <line v-else-if="name === 'files'" x1="16" y1="17" x2="8" y2="17"></line>
    <polyline v-else-if="name === 'files'" points="10 9 9 9 8 9"></polyline>

    <!-- Help -->
    <circle v-else-if="name === 'help'" cx="12" cy="12" r="10"></circle>
    <path v-else-if="name === 'help'" d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"></path>
    <line v-else-if="name === 'help'" x1="12" y1="17" x2="12.01" y2="17"></line>

    <!-- Tools / default -->
    <path v-else d="M14.7 6.3a4 4 0 0 0-5.4 5.4L3 18v3h3l6.3-6.3a4 4 0 0 0 5.4-5.4l-2.1 2.1-2.1-2.1z" />
  </svg>
</template>

<style scoped>
.nav-svg {
  width: 18px;
  height: 18px;
}
</style>