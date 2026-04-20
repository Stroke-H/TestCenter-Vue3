<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { NovelChapter } from '../types'

const props = defineProps<{
  open: boolean
  chapters: NovelChapter[]
  currentIndex: number
}>()

const emit = defineEmits<{
  close: []
  selectChapter: [index: number]
}>()

const keyword = shallowRef('')

const filteredChapters = computed(() => {
  const value = keyword.value.trim().toLowerCase()
  if (!value) {
    return props.chapters.map((chapter, index) => ({ chapter, index }))
  }

  return props.chapters
    .map((chapter, index) => ({ chapter, index }))
    .filter(({ chapter }) => chapter.title.toLowerCase().includes(value))
})
</script>

<template>
  <div v-if="open" class="chapter-drawer" role="dialog" aria-label="章节目录">
    <div class="chapter-drawer__header">
      <div>
        <p class="chapter-drawer__label">目录</p>
        <h2 class="chapter-drawer__title">{{ chapters.length }} 章</h2>
      </div>
      <button class="chapter-drawer__close" type="button" @click="emit('close')">关闭</button>
    </div>

    <input
      v-model="keyword"
      class="chapter-drawer__search"
      type="search"
      placeholder="搜索章节标题"
    >

    <div class="chapter-list">
      <button
        v-for="item in filteredChapters"
        :key="item.chapter.id"
        class="chapter-item"
        :class="{ 'chapter-item--active': item.index === currentIndex }"
        type="button"
        @click="emit('selectChapter', item.index)"
      >
        <span class="chapter-item__index">{{ item.index + 1 }}</span>
        <span class="chapter-item__title">{{ item.chapter.title }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.chapter-drawer {
  position: fixed;
  top: 0;
  left: 0;
  z-index: 42;
  width: min(380px, 92vw);
  height: 100vh;
  padding: 22px;
  overflow: auto;
  background: rgba(255, 255, 255, 0.96);
  border-right: 1px solid rgba(15, 23, 42, 0.1);
  box-shadow: 20px 0 50px rgba(15, 23, 42, 0.12);
  backdrop-filter: blur(18px);
}

.chapter-drawer__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin-bottom: 18px;
}

.chapter-drawer__label {
  margin: 0 0 4px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 900;
}

.chapter-drawer__title {
  margin: 0;
  color: #172033;
  font-size: 22px;
}

.chapter-drawer__close {
  border: 0;
  border-radius: 8px;
  padding: 8px 10px;
  background: #eef2f7;
  color: #334155;
  font-weight: 800;
  cursor: pointer;
}

.chapter-drawer__search {
  width: 100%;
  height: 40px;
  margin-bottom: 14px;
  padding: 0 12px;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  outline: none;
}

.chapter-drawer__search:focus {
  border-color: #0f766e;
}

.chapter-list {
  display: grid;
  gap: 6px;
}

.chapter-item {
  display: grid;
  grid-template-columns: 34px 1fr;
  gap: 10px;
  align-items: center;
  min-height: 42px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: #334155;
  text-align: left;
  cursor: pointer;
}

.chapter-item:hover {
  background: #f0fdfa;
}

.chapter-item--active {
  border-color: #5eead4;
  background: #ccfbf1;
  color: #0f766e;
  font-weight: 900;
}

.chapter-item__index {
  color: #94a3b8;
  font-size: 12px;
}

.chapter-item__title {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
