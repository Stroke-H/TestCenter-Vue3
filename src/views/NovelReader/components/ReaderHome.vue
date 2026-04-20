<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import type { ReadingRecord } from '../types'
import type { NovelEncoding } from '../utils/novelParser'

const props = defineProps<{
  records: ReadingRecord[]
  loading: boolean
  error: string
  encoding: NovelEncoding
}>()

const emit = defineEmits<{
  importFile: [file: File]
  openRecord: [record: ReadingRecord]
  removeRecord: [record: ReadingRecord]
  clearRecords: []
  updateEncoding: [encoding: NovelEncoding]
}>()

const isDragging = shallowRef(false)
const fileInput = shallowRef<HTMLInputElement | null>(null)
const searchKeyword = shallowRef('')
const addPanelOpen = shallowRef(false)
const manageMode = shallowRef(false)
const selectedIds = shallowRef<string[]>([])

const latestRecord = computed(() => props.records[0] ?? null)
const filteredRecords = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword) return props.records

  return props.records.filter((record) => record.name.toLowerCase().includes(keyword))
})

const selectedRecords = computed(() => props.records.filter((record) => selectedIds.value.includes(record.id)))
const shelfSummary = computed(() => {
  const latest = latestRecord.value
  if (!latest) return `已添加 ${props.records.length} 本`
  return `已添加 ${props.records.length} 本 · 最近阅读《${latest.name}》`
})

function openPicker() {
  fileInput.value?.click()
}

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (file) {
    emit('importFile', file)
    addPanelOpen.value = false
  }
  input.value = ''
}

function handleDrop(event: DragEvent) {
  isDragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) {
    emit('importFile', file)
    addPanelOpen.value = false
  }
}

function formatTime(timestamp: number) {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(timestamp)
}

function formatProgress(progress: number) {
  return `${Math.round(progress)}%`
}

function getCoverStyle(record: ReadingRecord) {
  const palettes = [
    ['#0f766e', '#14b8a6'],
    ['#334155', '#64748b'],
    ['#7f1d1d', '#b91c1c'],
    ['#3730a3', '#6366f1'],
    ['#365314', '#65a30d'],
    ['#111827', '#4b5563'],
    ['#155e75', '#0891b2']
  ]
  const index = Math.abs(hashText(record.id)) % palettes.length
  const [start, end] = palettes[index] ?? ['#0f766e', '#14b8a6']

  return {
    background: `linear-gradient(145deg, ${start}, ${end})`
  }
}

function getCoverTitle(name: string) {
  return name.slice(0, 6)
}

function hashText(text: string) {
  return text.split('').reduce((hash, char) => {
    return ((hash << 5) - hash + char.charCodeAt(0)) | 0
  }, 0)
}

function toggleManageMode() {
  manageMode.value = !manageMode.value
  selectedIds.value = []
}

function toggleSelected(record: ReadingRecord) {
  if (!manageMode.value) {
    emit('openRecord', record)
    return
  }

  selectedIds.value = selectedIds.value.includes(record.id)
    ? selectedIds.value.filter((id) => id !== record.id)
    : [...selectedIds.value, record.id]
}

function removeSelected() {
  selectedRecords.value.forEach((record) => emit('removeRecord', record))
  selectedIds.value = []
  manageMode.value = false
}

function selectAllVisible() {
  selectedIds.value = filteredRecords.value.map((record) => record.id)
}
</script>

<template>
  <section class="reader-home">
    <input
      ref="fileInput"
      class="reader-home__file"
      type="file"
      accept=".txt,text/plain"
      @change="handleFileChange"
    >

    <header class="shelf-header">
      <div class="shelf-header__copy">
        <p class="shelf-header__eyebrow">本地小说阅读器</p>
        <h1 class="shelf-header__title">我的书架</h1>
        <p class="shelf-header__summary">{{ shelfSummary }}</p>
      </div>

      <div class="shelf-header__actions">
        <label class="search-box">
          <el-icon :size="16"><component :is="Icons.Search" /></el-icon>
          <input v-model="searchKeyword" type="search" placeholder="搜索书名">
        </label>
        <button
          v-if="records.length > 0"
          class="secondary-btn"
          type="button"
          @click="toggleManageMode"
        >
          {{ manageMode ? '取消' : '管理' }}
        </button>
        <button class="primary-btn" type="button" @click="addPanelOpen = true">
          <el-icon :size="16"><component :is="Icons.Plus" /></el-icon>
          添加书籍
        </button>
      </div>
    </header>

    <p v-if="error" class="reader-error">{{ error }}</p>

    <section v-if="latestRecord" class="continue-card">
      <button class="continue-card__cover" type="button" :style="getCoverStyle(latestRecord)" @click="emit('openRecord', latestRecord)">
        <span>{{ getCoverTitle(latestRecord.name) }}</span>
        <small>{{ formatProgress(latestRecord.progress) }}</small>
      </button>

      <div class="continue-card__body">
        <p class="continue-card__label">继续阅读</p>
        <h2 class="continue-card__title">{{ latestRecord.name }}</h2>
        <p class="continue-card__meta">
          第 {{ latestRecord.chapterIndex + 1 }} / {{ latestRecord.chapterCount }} 章 ·
          {{ formatProgress(latestRecord.progress) }} ·
          {{ formatTime(latestRecord.updatedAt) }}
        </p>
        <span class="continue-card__progress">
          <span class="continue-card__progress-fill" :style="{ width: `${Math.round(latestRecord.progress)}%` }" />
        </span>
      </div>

      <button class="primary-btn" type="button" @click="emit('openRecord', latestRecord)">继续阅读</button>
    </section>

    <section v-if="manageMode" class="manage-bar">
      <span>已选择 {{ selectedIds.length }} 本</span>
      <div class="manage-bar__actions">
        <button class="secondary-btn" type="button" @click="selectAllVisible">全选</button>
        <button class="danger-btn" type="button" :disabled="selectedIds.length === 0" @click="removeSelected">删除所选</button>
      </div>
    </section>

    <section class="recent-section">
      <div class="recent-section__header">
        <h2 class="recent-section__title">我的书籍</h2>
        <button v-if="records.length > 0" class="link-btn" type="button" @click="emit('clearRecords')">清空书架</button>
      </div>

      <div v-if="filteredRecords.length > 0" class="book-grid">
        <button class="add-book-card" type="button" @click="addPanelOpen = true">
          <span class="add-book-card__cover">
            <el-icon :size="28"><component :is="Icons.Plus" /></el-icon>
          </span>
          <strong>添加书籍</strong>
        </button>

        <article
          v-for="record in filteredRecords"
          :key="record.id"
          class="book-card"
          :class="{ 'book-card--selected': selectedIds.includes(record.id), 'book-card--manage': manageMode }"
        >
          <button class="book-card__main" type="button" @click="toggleSelected(record)">
            <span class="book-card__cover" :style="getCoverStyle(record)">
              <span class="book-card__cover-title">{{ getCoverTitle(record.name) }}</span>
              <span class="book-card__cover-progress">{{ formatProgress(record.progress) }}</span>
              <span v-if="manageMode" class="book-card__check">
                <el-icon v-if="selectedIds.includes(record.id)" :size="14"><component :is="Icons.Check" /></el-icon>
              </span>
            </span>
            <span class="book-card__content">
              <strong class="book-card__name">{{ record.name }}</strong>
              <span class="book-card__meta">第 {{ record.chapterIndex + 1 }} / {{ record.chapterCount }} 章</span>
              <span class="book-card__progress">
                <span class="book-card__progress-fill" :style="{ width: `${Math.round(record.progress)}%` }" />
              </span>
              <span class="book-card__time">{{ formatProgress(record.progress) }} · {{ formatTime(record.updatedAt) }}</span>
            </span>
          </button>
          <div v-if="!manageMode" class="book-card__actions">
            <button class="primary-btn primary-btn--small" type="button" @click="emit('openRecord', record)">阅读</button>
            <button class="link-btn link-btn--danger" type="button" @click="emit('removeRecord', record)">移除</button>
          </div>
        </article>
      </div>

      <div v-else class="empty-shelf">
        <span class="empty-shelf__icon">
          <el-icon :size="28"><component :is="Icons.Reading" /></el-icon>
        </span>
        <p class="empty-shelf__title">书架还是空的</p>
        <p class="empty-shelf__text">{{ searchKeyword ? '没有找到这本书。' : '添加一本 TXT 小说后，它会出现在这里。' }}</p>
        <button v-if="!searchKeyword" class="primary-btn" type="button" @click="addPanelOpen = true">添加第一本书</button>
      </div>
    </section>

    <div v-if="addPanelOpen" class="modal-backdrop" @click.self="addPanelOpen = false">
      <section class="add-panel" role="dialog" aria-label="添加书籍">
        <div class="add-panel__header">
          <div>
            <p class="add-panel__eyebrow">添加到书架</p>
            <h2 class="add-panel__title">{{ loading ? '正在添加...' : '选择本地 TXT 小说' }}</h2>
          </div>
          <button class="icon-btn" type="button" @click="addPanelOpen = false">关闭</button>
        </div>

        <div
          class="import-zone"
          :class="{ 'import-zone--active': isDragging }"
          @dragover.prevent="isDragging = true"
          @dragleave.prevent="isDragging = false"
          @drop.prevent="handleDrop"
          @click="openPicker"
        >
          <div class="import-zone__icon">
            <el-icon :size="28"><component :is="Icons.UploadFilled" /></el-icon>
          </div>
          <h3 class="import-zone__title">拖入小说文件</h3>
          <p class="import-zone__text">文件会缓存在当前浏览器，之后可从书架直接打开。</p>
        </div>

        <label class="encoding-select">
          <span>文本编码</span>
          <select
            :value="encoding"
            @change="emit('updateEncoding', ($event.target as HTMLSelectElement).value as NovelEncoding)"
          >
            <option value="auto">自动识别</option>
            <option value="utf-8">UTF-8</option>
            <option value="gb18030">GBK / GB18030</option>
          </select>
        </label>
      </section>
    </div>
  </section>
</template>

<style scoped>
.reader-home {
  min-height: calc(100vh - 120px);
  display: grid;
  align-content: start;
  gap: 22px;
  padding: 28px;
  color: #172033;
  background: #f5f7f8;
}

.reader-home__file {
  display: none;
}

.shelf-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18px;
}

.shelf-header__copy {
  min-width: 0;
}

.shelf-header__eyebrow {
  margin: 0 0 8px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 800;
}

.shelf-header__title {
  margin: 0;
  font-size: 34px;
  line-height: 1.2;
}

.shelf-header__summary {
  margin: 10px 0 0;
  color: #64748b;
  line-height: 1.7;
}

.shelf-header__actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.search-box {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: 220px;
  height: 40px;
  padding: 0 12px;
  border: 1px solid #d7dee8;
  border-radius: 8px;
  background: #ffffff;
  color: #64748b;
}

.search-box input {
  min-width: 0;
  width: 100%;
  border: 0;
  outline: none;
  color: #172033;
  background: transparent;
}

.import-zone {
  display: grid;
  justify-items: center;
  gap: 10px;
  padding: 34px 24px;
  border: 1px dashed #9aa8b9;
  border-radius: 8px;
  background: #f8fafc;
  cursor: pointer;
  transition: border-color 0.2s ease, background 0.2s ease, transform 0.2s ease;
}

.import-zone:hover,
.import-zone--active {
  border-color: #0f766e;
  background: #f0fdfa;
  transform: translateY(-1px);
}

.import-zone__input {
  display: none;
}

.import-zone__icon {
  width: 58px;
  height: 58px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: #ccfbf1;
  color: #0f766e;
}

.import-zone__title {
  margin: 4px 0 0;
  font-size: 20px;
}

.import-zone__text,
.import-zone__error {
  margin: 0;
  color: #64748b;
}

.encoding-select {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: #475569;
  font-size: 13px;
  font-weight: 800;
}

.encoding-select select {
  height: 34px;
  padding: 0 10px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  background: #ffffff;
  color: #334155;
  outline: none;
}

.reader-error {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid #fecaca;
  border-radius: 8px;
  background: #fef2f2;
  color: #dc2626;
  font-weight: 700;
}

.continue-card {
  display: grid;
  grid-template-columns: 116px minmax(0, 1fr) auto;
  align-items: center;
  gap: 18px;
  max-width: 1040px;
  padding: 18px;
  border: 1px solid #dbe7e4;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.06);
}

.continue-card__cover {
  position: relative;
  width: 116px;
  height: 156px;
  display: grid;
  place-items: center;
  padding: 16px;
  border: 0;
  border-radius: 8px;
  color: #ffffff;
  text-align: center;
  box-shadow: inset 9px 0 18px rgba(0, 0, 0, 0.16), 0 14px 26px rgba(15, 23, 42, 0.18);
  cursor: pointer;
}

.continue-card__cover span {
  font-size: 20px;
  font-weight: 900;
  line-height: 1.35;
}

.continue-card__cover small {
  position: absolute;
  right: 10px;
  bottom: 10px;
  font-weight: 900;
  opacity: 0.9;
}

.continue-card__body {
  min-width: 0;
}

.continue-card__label,
.continue-card__meta {
  margin: 0;
  color: #64748b;
  font-size: 13px;
}

.continue-card__title {
  margin: 8px 0;
  color: #172033;
  font-size: 24px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.continue-card__progress,
.book-card__progress {
  display: block;
  height: 5px;
  overflow: hidden;
  border-radius: 8px;
  background: #e2e8f0;
}

.continue-card__progress {
  max-width: 340px;
  margin-top: 14px;
}

.continue-card__progress-fill,
.book-card__progress-fill {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: #14b8a6;
}

.primary-btn,
.secondary-btn,
.danger-btn,
.link-btn,
.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 0;
  border-radius: 8px;
  font-weight: 800;
  cursor: pointer;
}

.primary-btn {
  padding: 11px 16px;
  background: #0f766e;
  color: #ffffff;
}

.primary-btn--small {
  padding: 8px 12px;
}

.secondary-btn {
  min-height: 40px;
  padding: 0 13px;
  border: 1px solid #d7dee8;
  background: #ffffff;
  color: #334155;
}

.danger-btn {
  min-height: 36px;
  padding: 0 13px;
  background: #dc2626;
  color: #ffffff;
}

.danger-btn:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.link-btn {
  padding: 7px 10px;
  background: transparent;
  color: #0f766e;
}

.link-btn--danger {
  color: #dc2626;
}

.icon-btn {
  min-height: 36px;
  padding: 0 12px;
  background: #eef2f7;
  color: #334155;
}

.manage-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  max-width: 1040px;
  padding: 12px 14px;
  border: 1px solid #dbe7e4;
  border-radius: 8px;
  background: #ffffff;
  color: #334155;
  font-weight: 800;
}

.manage-bar__actions {
  display: flex;
  gap: 10px;
}

.recent-section {
  max-width: 1040px;
}

.recent-section__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.recent-section__title {
  margin: 0;
  font-size: 18px;
}

.book-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 24px 20px;
}

.add-book-card {
  display: grid;
  justify-items: center;
  align-content: start;
  gap: 11px;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: #64748b;
  text-align: center;
  cursor: pointer;
}

.add-book-card__cover {
  width: 120px;
  height: 164px;
  display: grid;
  place-items: center;
  border: 1px dashed #9aa8b9;
  border-radius: 8px;
  background: #ffffff;
  color: #0f766e;
}

.add-book-card:hover .add-book-card__cover {
  border-color: #0f766e;
  background: #f0fdfa;
}

.book-card {
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
}

.book-card--selected .book-card__cover {
  outline: 3px solid #14b8a6;
  outline-offset: 3px;
}

.book-card__main {
  display: grid;
  justify-items: center;
  gap: 11px;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.book-card__cover {
  position: relative;
  width: 120px;
  height: 164px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 18px;
  border-radius: 8px;
  color: #ffffff;
  box-shadow: inset 9px 0 18px rgba(0, 0, 0, 0.18), 0 12px 24px rgba(15, 23, 42, 0.16);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.book-card:hover .book-card__cover {
  transform: translateY(-3px);
  box-shadow: inset 9px 0 18px rgba(0, 0, 0, 0.18), 0 18px 30px rgba(15, 23, 42, 0.2);
}

.book-card__cover-title {
  font-size: 18px;
  font-weight: 900;
  line-height: 1.35;
  text-align: center;
  word-break: break-all;
}

.book-card__cover-progress {
  position: absolute;
  right: 9px;
  bottom: 9px;
  font-size: 12px;
  font-weight: 900;
  opacity: 0.9;
}

.book-card__check {
  position: absolute;
  top: 9px;
  right: 9px;
  width: 22px;
  height: 22px;
  display: grid;
  place-items: center;
  border: 2px solid rgba(255, 255, 255, 0.82);
  border-radius: 50%;
  background: rgba(15, 23, 42, 0.18);
  color: #ffffff;
}

.book-card__content {
  display: grid;
  align-content: start;
  justify-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
  text-align: center;
}

.book-card__name {
  overflow: hidden;
  color: #172033;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.book-card__meta,
.book-card__time {
  color: #64748b;
  font-size: 12px;
}

.book-card__actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 120px;
  margin: 0 auto;
}

.empty-shelf {
  display: grid;
  justify-items: center;
  gap: 6px;
  padding: 26px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
  text-align: center;
}

.empty-shelf__icon {
  width: 54px;
  height: 54px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: #ccfbf1;
  color: #0f766e;
}

.empty-shelf__title,
.empty-shelf__text {
  margin: 0;
}

.empty-shelf__title {
  color: #172033;
  font-size: 18px;
  font-weight: 900;
}

.empty-shelf__text {
  color: #64748b;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(15, 23, 42, 0.36);
}

.add-panel {
  display: grid;
  gap: 18px;
  width: min(520px, 100%);
  padding: 22px;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.24);
}

.add-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.add-panel__eyebrow {
  margin: 0 0 5px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 900;
}

.add-panel__title {
  margin: 0;
  color: #172033;
  font-size: 22px;
}

@media (max-width: 720px) {
  .reader-home {
    padding: 20px;
  }

  .shelf-header {
    display: grid;
  }

  .shelf-header__title {
    font-size: 28px;
  }

  .shelf-header__actions {
    justify-content: stretch;
  }

  .search-box {
    width: 100%;
  }

  .continue-card {
    grid-template-columns: 1fr;
    justify-items: start;
  }

  .book-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 20px 14px;
  }

  .book-card__cover,
  .add-book-card__cover {
    width: 108px;
    height: 150px;
  }

  .book-card__actions {
    width: 108px;
  }
}
</style>
