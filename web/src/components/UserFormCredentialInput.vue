<script setup lang="ts">
defineProps<{
  label: string
  modelValue: string
  placeholder?: string
  randomLabel: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', val: string): void
  (e: 'random'): void
}>()
</script>

<template>
  <div class="form-group">
    <label>{{ label }}</label>
    <div class="credential-row">
      <input
        :value="modelValue"
        class="form-control"
        :placeholder="placeholder"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
      />
      <button type="button" class="btn btn-secondary" @click="emit('random')">{{ randomLabel }}</button>
    </div>
  </div>
</template>

<style scoped>
.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 0.85rem;
  color: var(--text-main);
  font-weight: 500;
}

.credential-row {
  display: flex;
  gap: 8px;
}

.credential-row .form-control {
  flex: 1;
  min-width: 0;
}
</style>