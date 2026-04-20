import { defineStore } from 'pinia'
import { shallowRef } from 'vue'
import type { NovelBook, ReaderSettings } from '@/views/NovelReader/types'

export const useFishReaderStore = defineStore('fishReader', () => {
  const visible = shallowRef(false)
  const book = shallowRef<NovelBook | null>(null)
  const settings = shallowRef<ReaderSettings | null>(null)
  const chapterIndex = shallowRef(0)

  function open(payload: {
    book: NovelBook
    settings: ReaderSettings
    chapterIndex: number
  }) {
    book.value = payload.book
    settings.value = { ...payload.settings }
    chapterIndex.value = payload.chapterIndex
    visible.value = true
  }

  function close() {
    visible.value = false
  }

  function setChapterIndex(index: number) {
    if (!book.value) return
    chapterIndex.value = Math.min(Math.max(index, 0), book.value.chapters.length - 1)
  }

  return {
    visible,
    book,
    settings,
    chapterIndex,
    open,
    close,
    setChapterIndex
  }
})
