<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

const watermarkEnabled = defineModel<boolean>('watermarkEnabled', { required: true })
const watermarkText = defineModel<string>('watermarkText', { required: true })
const fontStyle = defineModel<string>('fontStyle', { required: true })
const fontSize = defineModel<number>('fontSize', { required: true })
const opacity = defineModel<number>('opacity', { required: true })
const angle = defineModel<number>('angle', { required: true })
const textColor = defineModel<string>('textColor', { required: true })
const blindEnabled = defineModel<boolean>('blindEnabled', { required: true })
const blindText = defineModel<string>('blindText', { required: true })
const blindBits = defineModel<number>('blindBits', { required: true })
const isMosaicMode = defineModel<boolean>('isMosaicMode', { required: true })
const isRectangleMode = defineModel<boolean>('isRectangleMode', { required: true })
const mosaicBrushSize = defineModel<number>('mosaicBrushSize', { required: true })
const rectangleColor = defineModel<string>('rectangleColor', { required: true })
const rectangleWidth = defineModel<number>('rectangleWidth', { required: true })

defineProps<{
  presets: string[]
  colorPresets: { name: string; value: string }[]
  watermarkedImageSrc: string
  extractedText: string | null
}>()

const emit = defineEmits<{
  (e: 'preset-click', text: string): void
  (e: 'select-tool', tool: 'none' | 'mosaic' | 'rectangle'): void
  (e: 'extract-preview'): void
  (e: 'extract-file', file: File): void
}>()

const { t } = useI18n()
const extractInput = ref<HTMLInputElement | null>(null)

function onExtractFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  emit('extract-file', file)
  ;(e.target as HTMLInputElement).value = ''
}
</script>

<template>
  <aside class="settings-panel">
    <div class="settings-heading"><span class="eyebrow">SETTINGS</span><h3>{{ t('tools.watermark.settings') }}</h3></div>
    <div class="setting-block">
      <div class="setting-label"><label>{{ t('tools.watermark.visibleLabel') }}</label><button class="toggle" :class="{ on: watermarkEnabled }" @click="watermarkEnabled = !watermarkEnabled"><i></i></button></div>
      <input v-model="watermarkText" class="text-input" :placeholder="t('tools.watermark.textPlaceholder')" />
      <div class="preset-list"><button v-for="preset in presets" :key="preset" @click="emit('preset-click', preset)">{{ preset }}</button></div>
    </div>
    <div class="setting-block">
      <div class="setting-label"><label>{{ t('tools.watermark.fontLabel') }}</label><select v-model="fontStyle"><option value="normal">{{ t('tools.watermark.fontNormal') }}</option><option value="bold">{{ t('tools.watermark.fontBold') }}</option><option value="italic">{{ t('tools.watermark.fontItalic') }}</option></select></div>
      <div class="range-row"><label>{{ t('tools.watermark.sizeLabel') }} <b>{{ fontSize }}px</b></label><input v-model.number="fontSize" type="range" min="12" max="120" /></div>
      <div class="range-row"><label>{{ t('tools.watermark.opacityLabel') }} <b>{{ Math.round(opacity * 100) }}%</b></label><input v-model.number="opacity" type="range" min="0.05" max="1" step="0.05" /></div>
      <div class="range-row"><label>{{ t('tools.watermark.angleLabel') }} <b>{{ angle }}°</b></label><input v-model.number="angle" type="range" min="-90" max="90" /></div>
    </div>
    <div class="setting-block">
      <div class="setting-label"><label>{{ t('tools.watermark.colorLabel') }}</label><span class="color-value"><input v-model="textColor" type="color" />{{ textColor }}</span></div>
      <div class="color-list"><button v-for="color in colorPresets" :key="color.value" :title="color.name" :style="{ background: color.value }" :class="{ selected: textColor === color.value }" @click="textColor = color.value"></button></div>
    </div>
    <div class="setting-block">
      <div class="setting-label"><label>{{ t('tools.watermark.blindLabel') }}</label><button class="toggle" :class="{ on: blindEnabled }" @click="blindEnabled = !blindEnabled"><i></i></button></div>
      <input v-model="blindText" class="text-input" :disabled="!blindEnabled" :placeholder="t('tools.watermark.blindPlaceholder')" />
      <div class="range-row"><label>{{ t('tools.watermark.blindStrengthLabel') }} <b>{{ blindBits }} bit</b></label><input v-model.number="blindBits" type="range" min="1" max="2" :disabled="!blindEnabled" /></div>
    </div>
    <div class="setting-block edit-tools-block">
      <div class="setting-label"><label>{{ t('tools.watermark.toolsLabel') }}</label><button v-if="isMosaicMode || isRectangleMode" class="tool-reset" @click="emit('select-tool', 'none')">{{ t('tools.watermark.exitTool') }}</button></div>
      <div class="edit-tool-buttons"><button :class="{ selected: isMosaicMode }" @click="emit('select-tool', isMosaicMode ? 'none' : 'mosaic')">{{ t('tools.watermark.mosaic') }}</button><button :class="{ selected: isRectangleMode }" @click="emit('select-tool', isRectangleMode ? 'none' : 'rectangle')">{{ t('tools.watermark.rectangle') }}</button></div>
      <div v-if="isMosaicMode" class="range-row"><label>{{ t('tools.watermark.brushSizeLabel') }} <b>{{ mosaicBrushSize }}px</b></label><input v-model.number="mosaicBrushSize" type="range" min="8" max="100" /></div>
      <template v-if="isRectangleMode"><div class="setting-label"><label>{{ t('tools.watermark.markColorLabel') }}</label><span class="color-value"><input v-model="rectangleColor" type="color" />{{ rectangleColor }}</span></div><div class="range-row"><label>{{ t('tools.watermark.lineWidthLabel') }} <b>{{ rectangleWidth }}px</b></label><input v-model.number="rectangleWidth" type="range" min="1" max="16" /></div></template>
    </div>
    <div class="extract-block">
      <div class="setting-label"><label>{{ t('tools.watermark.extractLabel') }}</label></div>
      <div class="extract-actions"><button :disabled="!watermarkedImageSrc" @click="emit('extract-preview')">{{ t('tools.watermark.extractFromCurrent') }}</button><button @click="extractInput?.click()">{{ t('tools.watermark.extractFromFile') }}</button><input ref="extractInput" type="file" accept="image/*" hidden @change="onExtractFileChange" /></div>
      <p v-if="extractedText !== null" class="extract-result">{{ extractedText || t('tools.watermark.extractEmpty') }}</p>
    </div>
  </aside>
</template>

<style scoped>
.settings-panel { padding: 17px; }
.settings-heading { margin-bottom: 17px; }
.setting-block, .extract-block { display: grid; gap: 10px; padding: 15px 0; border-top: 1px solid var(--border-glass); }
.setting-label { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.setting-label label { color: var(--text-main); font-size: .72rem; font-weight: 700; }
.toggle { width: 31px; height: 18px; padding: 2px; border: 0; border-radius: 20px; background: var(--text-dim); cursor: pointer; }
.toggle i { display: block; width: 14px; height: 14px; border-radius: 50%; background: white; transition: .2s; }
.toggle.on { background: var(--primary); }
.toggle.on i { transform: translateX(13px); }
.text-input, .setting-label select { min-width: 0; border: 1px solid var(--border-glass); border-radius: 7px; padding: 8px 9px; outline: none; background: var(--bg-input); color: var(--text-main); font: inherit; font-size: .7rem; }
.text-input:focus, .setting-label select:focus { border-color: var(--primary); }
.preset-list { display: flex; flex-wrap: wrap; gap: 5px; }
.preset-list button { border: 1px solid var(--border-glass); border-radius: 5px; padding: 4px 6px; background: transparent; color: var(--text-muted); cursor: pointer; font-size: .6rem; }
.range-row { display: grid; gap: 6px; }
.range-row label { display: flex; justify-content: space-between; color: var(--text-muted); font-size: .65rem; }
.range-row b { color: var(--text-main); font-weight: 600; }
.range-row input[type=range] { width: 100%; accent-color: var(--primary); }
.color-value { display: inline-flex; align-items: center; gap: 5px; color: var(--text-muted); font: .62rem var(--font-mono); }
.color-value input { width: 22px; height: 22px; padding: 0; border: 0; background: transparent; cursor: pointer; }
.color-list { display: flex; gap: 8px; }
.color-list button { width: 22px; height: 22px; border: 2px solid transparent; border-radius: 50%; cursor: pointer; }
.color-list button.selected { border-color: var(--primary); box-shadow: 0 0 0 2px var(--bg-panel); outline: 1px solid var(--primary); }
.extract-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.extract-result { margin: 0; padding: 8px; border-radius: 6px; background: color-mix(in srgb, var(--accent-emerald) 10%, transparent); color: var(--accent-emerald); font-size: .66rem; overflow-wrap: anywhere; }
.edit-tools-block .edit-tool-buttons { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.edit-tool-buttons button, .tool-reset { border: 1px solid var(--border-glass); border-radius: 7px; padding: 7px 8px; background: transparent; color: var(--text-muted); cursor: pointer; font: inherit; font-size: .65rem; font-weight: 700; }
.edit-tool-buttons button.selected, .edit-tool-buttons button:hover, .tool-reset:hover { border-color: var(--primary); color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent); }
@media (max-width: 900px) { .settings-panel { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 16px; } .settings-heading { grid-column: 1 / -1; } .extract-block { grid-column: 1 / -1; } }
@media (max-width: 600px) { .settings-panel { display: block; width: 100%; box-sizing: border-box; } }
</style>
