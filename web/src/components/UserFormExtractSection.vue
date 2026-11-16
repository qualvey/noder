<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  showExtract: boolean
  extractInput: string
}>()

const emit = defineEmits<{
  (e: 'update:show-extract', val: boolean): void
  (e: 'update:extract-input', val: string): void
  (e: 'apply'): void
}>()

const { t } = useI18n()
</script>

<template>
  <div class="form-span extract-section">
    <div class="extract-section-header">
      <span class="extract-section-title">{{ t('users.form.extractTitle') }}</span>
      <button type="button" class="btn btn-secondary btn-sm" @click="emit('update:show-extract', !showExtract)">{{ t('users.form.toggleExtract') }}</button>
    </div>
    <div v-if="showExtract" class="extract-section-body">
      <textarea
        :value="extractInput"
        class="form-control extract-textarea"
        rows="3"
        :placeholder="t('users.form.extractPlaceholder')"
        @input="emit('update:extract-input', ($event.target as HTMLTextAreaElement).value)"
      ></textarea>
      <button type="button" class="btn btn-primary btn-sm extract-apply-btn" @click="emit('apply')">{{ t('users.form.extractBtn') }}</button>
    </div>
  </div>
</template>

<style scoped>
.extract-section {
  margin-bottom: 14px;
  background: rgba(59, 130, 246, 0.1);
  padding: 10px;
  border-radius: 8px;
  border: 1px dashed rgba(59, 130, 246, 0.4);
}
.extract-section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.extract-section-title {
  font-size: 0.8rem;
  color: var(--primary);
  font-weight: 600;
}
.extract-section-body {
  margin-top: 10px;
}
.extract-textarea {
  font-family: var(--font-mono);
  font-size: 0.78rem;
}
.extract-apply-btn {
  margin-top: 8px;
  width: 100%;
}
</style>
