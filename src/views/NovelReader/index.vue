<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import ImmersiveReader from './components/ImmersiveReader.vue'
import ReaderHome from './components/ReaderHome.vue'
import { useNovelLibrary } from './composables/useNovelLibrary'
import type { NovelBook, ReaderSettings, ReadingRecord } from './types'
import { clearCachedBooks, deleteCachedBook, getCachedBook, saveCachedBook } from './utils/bookCache'
import { isSupportedNovelFile, parseNovel, readTextFile } from './utils/novelParser'
import type { NovelEncoding } from './utils/novelParser'

defineOptions({ name: 'NovelReader' })

const { records, settings, saveSettings, upsertRecord, removeRecord, clearRecords } = useNovelLibrary()

const currentBook = shallowRef<NovelBook | null>(null)
const loading = shallowRef(false)
const error = shallowRef('')
const initialChapterIndex = shallowRef(0)
const initialScrollTop = shallowRef(0)
const selectedEncoding = shallowRef<NovelEncoding>('auto')

const isReading = computed(() => currentBook.value !== null)

async function importFile(file: File) {
  error.value = ''

  if (!isSupportedFile(file)) {
    error.value = '暂时支持 TXT、Markdown、HTML/XML 文本小说和 EPUB 文件。'
    return
  }

  loading.value = true

  try {
    const text = await readTextFile(file, selectedEncoding.value)
    const book = parseNovel(file, text)

    await saveCachedBook(book)
    currentBook.value = book
    initialChapterIndex.value = 0
    initialScrollTop.value = 0

    saveProgress({
      chapterIndex: 0,
      scrollTop: 0,
      progress: 0
    })
  } catch {
    error.value = '文件添加失败，请确认文件内容为可读取的文本，或检查浏览器本地存储空间。'
  } finally {
    loading.value = false
  }
}

async function openRecord(record: ReadingRecord) {
  error.value = ''
  loading.value = true

  try {
    const cachedBook = await getCachedBook(record.id)
    if (!cachedBook) {
      error.value = `「${record.name}」的本地缓存已失效，请重新添加这本书。`
      return
    }

    currentBook.value = cachedBook
    initialChapterIndex.value = Math.min(record.chapterIndex, cachedBook.chapters.length - 1)
    initialScrollTop.value = record.scrollTop
  } catch {
    error.value = '读取本地书籍缓存失败，请重新添加这本书。'
  } finally {
    loading.value = false
  }
}

function updateSettings(nextSettings: Partial<ReaderSettings>) {
  Object.assign(settings, nextSettings)
  saveSettings()
}

function saveProgress(payload: { chapterIndex: number; scrollTop: number; progress: number }) {
  const book = currentBook.value
  if (!book) return

  upsertRecord({
    id: book.id,
    name: book.name,
    fileName: book.fileName,
    fileSize: book.fileSize,
    lastModified: book.lastModified,
    chapterIndex: payload.chapterIndex,
    scrollTop: payload.scrollTop,
    chapterCount: book.chapters.length,
    progress: payload.progress,
    updatedAt: Date.now()
  })
}

function exitReader() {
  currentBook.value = null
  initialChapterIndex.value = 0
  initialScrollTop.value = 0
}

async function removeBook(record: ReadingRecord) {
  removeRecord(record.id)
  await deleteCachedBook(record.id)
}

async function clearShelf() {
  clearRecords()
  await clearCachedBooks()
}

function isSupportedFile(file: File) {
  return isSupportedNovelFile(file)
}
</script>

<template>
  <div class="novel-reader-page">
    <ReaderHome
      v-if="!isReading"
      :records="records"
      :loading="loading"
      :error="error"
      :encoding="selectedEncoding"
      @import-file="importFile"
      @open-record="openRecord"
      @remove-record="removeBook"
      @clear-records="clearShelf"
      @update-encoding="selectedEncoding = $event"
    />

    <ImmersiveReader
      v-else-if="currentBook"
      :book="currentBook"
      :settings="settings"
      :initial-chapter-index="initialChapterIndex"
      :initial-scroll-top="initialScrollTop"
      @exit="exitReader"
      @update-settings="updateSettings"
      @save-progress="saveProgress"
    />
  </div>
</template>

<style scoped>
.novel-reader-page {
  min-height: calc(100vh - 100px);
  background: #ffffff;
}
</style>
