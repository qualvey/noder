<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { PopoverState } from '../composables/usePopover'

defineProps<{
  popover: PopoverState | null
}>()

const emit = defineEmits<{
  (e: 'hide'): void
  (e: 'confirm'): void
}>()

const { t } = useI18n()
</script>

<template>
  <div v-if="popover" class="delete-confirm-mask" @click="emit('hide')" />
  <div
    v-if="popover"
    class="delete-confirm-popover active"
    :style="{ left: popover.x + 'px', top: popover.y + 'px' }"
  >
    <div class="delete-confirm-title">{{ popover.title }}</div>
    <div class="delete-confirm-actions">
      <button type="button" class="btn btn-secondary btn-sm" @click="emit('hide')">
        {{ t('common.cancel') }}
      </button>
      <button type="button" class="btn btn-danger btn-sm" @click="emit('confirm')">
        {{ t('common.confirm') }}
      </button>
    </div>
  </div>
</template>
