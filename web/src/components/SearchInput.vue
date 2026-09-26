<script setup lang="ts">
defineProps<{
  modelValue: string
  placeholder?: string
  totalCount?: number
  filteredCount?: number
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'clear'): void
}>()

function onClear() {
  emit('update:modelValue', '')
  emit('clear')
}
</script>

<template>
  <div class="search-box">
    <svg
      class="search-icon"
      xmlns="http://www.w3.org/2000/svg"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
    >
      <circle cx="11" cy="11" r="8"></circle>
      <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
    </svg>
    <input
      :value="modelValue"
      type="text"
      class="search-input"
      :placeholder="placeholder"
      :disabled="disabled"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      @keydown.esc="onClear"
    />
    <button
      v-if="modelValue"
      type="button"
      class="search-clear-btn"
      title="Clear"
      @click="onClear"
    >
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="13"
        height="13"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <line x1="18" y1="6" x2="6" y2="18"></line>
        <line x1="6" y1="6" x2="18" y2="18"></line>
      </svg>
    </button>
    <span
      v-if="modelValue.trim() && totalCount != null && filteredCount != null"
      class="search-badge"
    >
      {{ filteredCount }}/{{ totalCount }}
    </span>
  </div>
</template>

<style scoped>
.search-box {
  position: relative;
  display: flex;
  align-items: center;
  background: var(--bg-input);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 0 10px;
  height: 34px;
  transition: all 0.2s ease;
}

.search-box:focus-within {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.search-icon {
  color: var(--text-muted);
  flex-shrink: 0;
  margin-right: 8px;
}

.search-input {
  background: transparent;
  border: none;
  outline: none;
  color: var(--text-main);
  font-size: 0.85rem;
  width: 220px;
  min-width: 140px;
}

.search-input::placeholder {
  color: var(--text-dim);
}

.search-clear-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border-radius: 50%;
  margin-left: 4px;
  transition: color 0.15s, background-color 0.15s;
}

.search-clear-btn:hover {
  color: var(--text-main);
  background-color: var(--bg-hover);
}

.search-badge {
  font-size: 0.72rem;
  color: var(--primary);
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.25);
  border-radius: 10px;
  padding: 1px 6px;
  margin-left: 6px;
  white-space: nowrap;
  font-family: var(--font-mono);
}

@media (max-width: 640px) {
  .search-box {
    width: 100%;
  }
  .search-input {
    width: 100%;
  }
}
</style>
