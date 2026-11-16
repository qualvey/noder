<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  direction: 'json2link' | 'link2json'
  inputText: string
  supportedHandlers: { protocol: string; displayName: string }[]
}>()

const emit = defineEmits<{
  (e: 'update:input-text', val: string): void
  (e: 'paste-sample', protocol: string): void
  (e: 'format'): void
  (e: 'clear'): void
}>()

const { t } = useI18n()
</script>

<template>
  <section class="transfer-card input-card">
    <div class="card-heading">
      <div>
        <h3>
          {{ direction === 'json2link' ? t('tools.transfer.jsonTitle') : t('tools.transfer.linkTitle') }}
        </h3>
      </div>
      <div class="card-actions">
        <button
          v-for="handler in supportedHandlers"
          :key="handler.protocol"
          @click="emit('paste-sample', handler.protocol)"
        >
          {{
            direction === 'json2link'
              ? t('tools.transfer.sampleJson', { name: handler.displayName })
              : t('tools.transfer.sampleLink', { name: handler.displayName })
          }}
        </button>
        <button v-if="direction === 'json2link'" :disabled="!inputText" @click="emit('format')">
          {{ t('tools.transfer.format') }}
        </button>
        <button :disabled="!inputText" @click="emit('clear')">{{ t('tools.transfer.clear') }}</button>
      </div>
    </div>

    <div class="editor-wrap">
      <textarea
        :value="inputText"
        :placeholder="
          direction === 'json2link'
            ? t('tools.transfer.jsonPlaceholder')
            : t('tools.transfer.linkPlaceholder')
        "
        @input="emit('update:input-text', ($event.target as HTMLTextAreaElement).value)"
      ></textarea>
    </div>
    <footer>
      <span>{{ t('tools.transfer.charCount', { count: inputText.length }) }}</span>
      <span>{{
        direction === 'json2link' ? t('tools.transfer.jsonHint') : t('tools.transfer.linkHint')
      }}</span>
    </footer>
  </section>
</template>

<style scoped>
.input-card {
  min-height: 460px;
}
.editor-wrap {
  position: relative;
}
.editor-wrap textarea {
  display: block;
  width: 100%;
  height: 340px;
  box-sizing: border-box;
  resize: none;
  border: 1px solid var(--border-glass);
  border-radius: 9px;
  padding: 12px;
  outline: none;
  background: var(--bg-input);
  color: var(--text-main);
  font: 0.72rem/1.7 var(--font-mono);
}
.editor-wrap textarea:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent);
}
.editor-wrap textarea::placeholder {
  color: var(--text-dim);
}
.transfer-card footer {
  display: flex;
  justify-content: space-between;
  margin-top: 10px;
  color: var(--text-dim);
  font-size: 0.64rem;
}
@media (max-width: 820px) {
  .input-card {
    min-height: 0;
  }
  .editor-wrap textarea {
    height: 280px;
  }
}
</style>
