<script setup lang="ts">
import type { ReaderFont, ReaderLineHeight, ReaderSettings, ReaderTheme, ReaderWidth } from '../types'

defineProps<{
  open: boolean
  settings: ReaderSettings
}>()

const emit = defineEmits<{
  close: []
  updateSettings: [settings: Partial<ReaderSettings>]
}>()

const fontSizeOptions = [14, 16, 18, 20, 22, 24]

const lineHeightOptions: Array<{ label: string; value: ReaderLineHeight }> = [
  { label: '紧凑', value: 'compact' },
  { label: '标准', value: 'standard' },
  { label: '舒适', value: 'comfortable' },
  { label: '宽松', value: 'loose' }
]

const widthOptions: Array<{ label: string; value: ReaderWidth }> = [
  { label: '窄', value: 'narrow' },
  { label: '标准', value: 'standard' },
  { label: '宽', value: 'wide' }
]

const themeOptions: Array<{ label: string; value: ReaderTheme }> = [
  { label: '白天', value: 'day' },
  { label: '柔和', value: 'soft' },
  { label: '护眼', value: 'green' },
  { label: '夜间', value: 'night' },
  { label: '黑色', value: 'black' }
]

const fontOptions: Array<{ label: string; value: ReaderFont }> = [
  { label: '系统', value: 'system' },
  { label: '衬线', value: 'serif' },
  { label: '无衬线', value: 'sans' }
]
</script>

<template>
  <div v-if="open" class="settings-panel" role="dialog" aria-label="阅读设置">
    <div class="settings-panel__header">
      <h2 class="settings-panel__title">阅读设置</h2>
      <button class="settings-panel__close" type="button" @click="emit('close')">关闭</button>
    </div>

    <div class="setting-group">
      <p class="setting-group__label">字号</p>
      <div class="segmented">
        <button
          v-for="size in fontSizeOptions"
          :key="size"
          class="segmented__btn"
          :class="{ 'segmented__btn--active': settings.fontSize === size }"
          type="button"
          @click="emit('updateSettings', { fontSize: size })"
        >
          {{ size }}
        </button>
      </div>
    </div>

    <div class="setting-group">
      <p class="setting-group__label">行距</p>
      <div class="segmented">
        <button
          v-for="option in lineHeightOptions"
          :key="option.value"
          class="segmented__btn"
          :class="{ 'segmented__btn--active': settings.lineHeight === option.value }"
          type="button"
          @click="emit('updateSettings', { lineHeight: option.value })"
        >
          {{ option.label }}
        </button>
      </div>
    </div>

    <div class="setting-group">
      <p class="setting-group__label">页面宽度</p>
      <div class="segmented">
        <button
          v-for="option in widthOptions"
          :key="option.value"
          class="segmented__btn"
          :class="{ 'segmented__btn--active': settings.width === option.value }"
          type="button"
          @click="emit('updateSettings', { width: option.value })"
        >
          {{ option.label }}
        </button>
      </div>
    </div>

    <div class="setting-group">
      <p class="setting-group__label">主题</p>
      <div class="segmented segmented--wrap">
        <button
          v-for="option in themeOptions"
          :key="option.value"
          class="segmented__btn"
          :class="{ 'segmented__btn--active': settings.theme === option.value }"
          type="button"
          @click="emit('updateSettings', { theme: option.value })"
        >
          {{ option.label }}
        </button>
      </div>
    </div>

    <div class="setting-group">
      <p class="setting-group__label">字体</p>
      <div class="segmented">
        <button
          v-for="option in fontOptions"
          :key="option.value"
          class="segmented__btn"
          :class="{ 'segmented__btn--active': settings.font === option.value }"
          type="button"
          @click="emit('updateSettings', { font: option.value })"
        >
          {{ option.label }}
        </button>
      </div>
    </div>

    <label class="toggle-row">
      <span>段首缩进</span>
      <input
        :checked="settings.indent"
        type="checkbox"
        @change="emit('updateSettings', { indent: ($event.target as HTMLInputElement).checked })"
      >
    </label>

    <label class="toggle-row">
      <span>平滑滚动</span>
      <input
        :checked="settings.smoothScroll"
        type="checkbox"
        @change="emit('updateSettings', { smoothScroll: ($event.target as HTMLInputElement).checked })"
      >
    </label>
  </div>
</template>

<style scoped>
.settings-panel {
  position: fixed;
  top: 0;
  right: 0;
  z-index: 42;
  width: min(360px, 92vw);
  height: 100vh;
  padding: 22px;
  overflow: auto;
  background: rgba(255, 255, 255, 0.96);
  border-left: 1px solid rgba(15, 23, 42, 0.1);
  box-shadow: -20px 0 50px rgba(15, 23, 42, 0.12);
  backdrop-filter: blur(18px);
}

.settings-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.settings-panel__title {
  margin: 0;
  color: #172033;
  font-size: 20px;
}

.settings-panel__close {
  border: 0;
  border-radius: 8px;
  padding: 8px 10px;
  background: #eef2f7;
  color: #334155;
  font-weight: 800;
  cursor: pointer;
}

.setting-group {
  display: grid;
  gap: 9px;
  margin-bottom: 18px;
}

.setting-group__label {
  margin: 0;
  color: #475569;
  font-size: 13px;
  font-weight: 800;
}

.segmented {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(58px, 1fr));
  gap: 8px;
}

.segmented--wrap {
  grid-template-columns: repeat(3, 1fr);
}

.segmented__btn {
  min-height: 36px;
  border: 1px solid #dbe3ef;
  border-radius: 8px;
  background: #ffffff;
  color: #475569;
  font-weight: 800;
  cursor: pointer;
}

.segmented__btn--active {
  border-color: #0f766e;
  background: #ccfbf1;
  color: #0f766e;
}

.toggle-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 42px;
  color: #334155;
  font-weight: 800;
}
</style>
