<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, shallowRef, useTemplateRef, watch } from 'vue'
import type { CSSProperties } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { useFishReaderStore } from '@/stores'
import ChapterDrawer from './ChapterDrawer.vue'
import ReaderSettingsPanel from './ReaderSettingsPanel.vue'
import type { NovelBook, NovelChapter, ReaderSettings } from '../types'

const props = defineProps<{
  book: NovelBook
  settings: ReaderSettings
  initialChapterIndex: number
  initialScrollTop: number
}>()

const emit = defineEmits<{
  exit: []
  updateSettings: [settings: Partial<ReaderSettings>]
  saveProgress: [payload: { chapterIndex: number; scrollTop: number; progress: number }]
}>()

const router = useRouter()
const fishReaderStore = useFishReaderStore()
const scrollArea = useTemplateRef<HTMLElement>('scrollArea')
const currentChapterIndex = shallowRef(Math.min(props.initialChapterIndex, props.book.chapters.length - 1))
const controlsVisible = shallowRef(true)
const chapterDrawerOpen = shallowRef(false)
const settingsPanelOpen = shallowRef(false)
const scrollTop = shallowRef(props.initialScrollTop)
const hideTimer = shallowRef<ReturnType<typeof window.setTimeout> | null>(null)
const pageTurnTimer = shallowRef<ReturnType<typeof window.setTimeout> | null>(null)
const pageTurn = shallowRef<{ direction: 'next' | 'previous'; key: number } | null>(null)
const chapterPageIndex = shallowRef(0)
const chapterPageCount = shallowRef(1)
const chapterPageStep = shallowRef(1)
const chapterPageAnchors = shallowRef<number[]>([0])

const currentChapter = computed<NovelChapter>(() => props.book.chapters[currentChapterIndex.value] ?? props.book.chapters[0]!)

const paragraphs = computed(() => {
  const content = currentChapter.value?.content || '本章暂无正文。'
  return content.split(/\n{2,}/).map((paragraph) => paragraph.trim()).filter(Boolean)
})

const chapterProgress = computed(() => {
  const element = scrollArea.value
  if (!element) return 0

  const maxScroll = Math.max(element.scrollHeight - element.clientHeight, 1)
  return Math.min(100, Math.max(0, (scrollTop.value / maxScroll) * 100))
})

const bookProgress = computed(() => {
  const chapterOffset = currentChapterIndex.value + chapterProgress.value / 100
  return Math.min(100, Math.max(0, (chapterOffset / props.book.chapters.length) * 100))
})

const readerStyle = computed<CSSProperties>(() => ({
  fontSize: `${props.settings.fontSize}px`,
  lineHeight: lineHeightMap[props.settings.lineHeight],
  fontFamily: fontMap[props.settings.font],
  scrollBehavior: props.settings.smoothScroll ? 'smooth' : 'auto'
}))

const contentClass = computed(() => [
  `reader-content--${props.settings.width}`,
  { 'reader-content--indent': props.settings.indent }
])

const themeClass = computed(() => `immersive-reader--${props.settings.theme}`)

const lineHeightMap = {
  compact: '1.65',
  standard: '1.85',
  comfortable: '2.05',
  loose: '2.25'
}

const fontMap = {
  system: '-apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif',
  serif: '"Songti SC", "Noto Serif SC", "Times New Roman", serif',
  sans: '"PingFang SC", "Microsoft YaHei", Arial, sans-serif'
}

function showControls() {
  controlsVisible.value = true
  if (hideTimer.value) {
    window.clearTimeout(hideTimer.value)
  }

  hideTimer.value = window.setTimeout(() => {
    if (!chapterDrawerOpen.value && !settingsPanelOpen.value) {
      controlsVisible.value = false
    }
  }, 3200)
}

function toggleControls() {
  controlsVisible.value = !controlsVisible.value
  if (controlsVisible.value) {
    showControls()
  }
}

function playPageTurn(direction: 'next' | 'previous') {
  if (pageTurnTimer.value) {
    window.clearTimeout(pageTurnTimer.value)
  }

  pageTurn.value = {
    direction,
    key: Date.now()
  }

  pageTurnTimer.value = window.setTimeout(() => {
    pageTurn.value = null
  }, 560)
}

function setScrollTopInstantly(value: number) {
  const element = scrollArea.value
  if (!element) return

  const originalBehavior = element.style.scrollBehavior
  element.style.scrollBehavior = 'auto'
  element.scrollTop = value
  scrollTop.value = value

  window.requestAnimationFrame(() => {
    element.style.scrollBehavior = originalBehavior
  })
}

function getMaxScroll() {
  const element = scrollArea.value
  if (!element) return 0

  return Math.max(element.scrollHeight - element.clientHeight, 0)
}

function calculatePageStep() {
  const element = scrollArea.value
  if (!element) return 1

  return Math.max(240, element.clientHeight)
}

function calculatePageAnchors(maxScroll: number, pageStep: number) {
  if (maxScroll <= 0) return [0]

  const anchors: number[] = []
  for (let top = 0; top < maxScroll; top += pageStep) {
    anchors.push(Math.round(top))
  }

  if (anchors[anchors.length - 1] !== maxScroll) {
    anchors.push(maxScroll)
  }

  return anchors
}

function updateChapterPageMetrics() {
  const element = scrollArea.value
  if (!element) return

  const maxScroll = getMaxScroll()
  const pageStep = calculatePageStep()
  const pageAnchors = calculatePageAnchors(maxScroll, pageStep)
  const nextPageIndex = pageAnchors.reduce((nearestIndex, anchor, index) => {
    const nearestAnchor = pageAnchors[nearestIndex] ?? 0
    const nearestDistance = Math.abs(nearestAnchor - element.scrollTop)
    const distance = Math.abs(anchor - element.scrollTop)
    return distance < nearestDistance ? index : nearestIndex
  }, 0)

  chapterPageStep.value = pageStep
  chapterPageAnchors.value = pageAnchors
  chapterPageCount.value = pageAnchors.length
  chapterPageIndex.value = nextPageIndex
}

function getPageScrollTop(pageIndex: number) {
  return chapterPageAnchors.value[pageIndex] ?? getMaxScroll()
}

function schedulePageMetricsUpdate() {
  nextTick(() => {
    window.requestAnimationFrame(updateChapterPageMetrics)
  })
}

function goChapter(index: number, restoreScroll = 0) {
  if (index < 0 || index >= props.book.chapters.length) return

  const direction = index > currentChapterIndex.value ? 'next' : 'previous'
  playPageTurn(direction)
  currentChapterIndex.value = index
  chapterDrawerOpen.value = false
  nextTick(() => {
    setScrollTopInstantly(restoreScroll)
    updateChapterPageMetrics()
    saveProgress()
  })
}

function goPrevious() {
  goChapter(currentChapterIndex.value - 1)
}

function goNext() {
  goChapter(currentChapterIndex.value + 1)
}

function handleScroll() {
  scrollTop.value = scrollArea.value?.scrollTop ?? 0
  updateChapterPageMetrics()
  saveProgress()
}

function goChapterPage(delta: number) {
  const element = scrollArea.value
  if (!element) return

  updateChapterPageMetrics()
  const nextPageIndex = Math.min(
    chapterPageCount.value - 1,
    Math.max(0, chapterPageIndex.value + delta)
  )

  if (nextPageIndex === chapterPageIndex.value) return

  const targetTop = getPageScrollTop(nextPageIndex)
  chapterPageIndex.value = nextPageIndex
  scrollTop.value = targetTop
  element.scrollTo({
    top: targetTop,
    behavior: props.settings.smoothScroll ? 'smooth' : 'auto'
  })
  saveProgress()
}

async function openFishMode() {
  fishReaderStore.open({
    book: props.book,
    settings: props.settings,
    chapterIndex: currentChapterIndex.value
  })
  await router.push('/dashboard')
}

function saveProgress() {
  emit('saveProgress', {
    chapterIndex: currentChapterIndex.value,
    scrollTop: scrollTop.value,
    progress: bookProgress.value
  })
}

function handleKeydown(event: KeyboardEvent) {
  if (isEditableTarget(event.target)) return

  if ((chapterDrawerOpen.value || settingsPanelOpen.value) && event.key !== 'Escape') {
    return
  }

  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    goPrevious()
  } else if (event.key === 'ArrowRight') {
    event.preventDefault()
    goNext()
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    goChapterPage(-1)
  } else if (event.key === 'ArrowDown') {
    event.preventDefault()
    goChapterPage(1)
  } else if (event.key === 'Escape') {
    if (chapterDrawerOpen.value || settingsPanelOpen.value) {
      chapterDrawerOpen.value = false
      settingsPanelOpen.value = false
    } else if (controlsVisible.value) {
      controlsVisible.value = false
    } else {
      emit('exit')
    }
  }
}

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false

  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable
}

watch(
  () => props.initialChapterIndex,
  (index) => {
    currentChapterIndex.value = Math.min(index, props.book.chapters.length - 1)
    schedulePageMetricsUpdate()
  }
)

watch(
  () => [
    currentChapterIndex.value,
    props.settings.fontSize,
    props.settings.lineHeight,
    props.settings.width,
    props.settings.font,
    props.settings.indent
  ],
  schedulePageMetricsUpdate
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', schedulePageMetricsUpdate)
  nextTick(() => {
    if (scrollArea.value) {
      scrollArea.value.scrollTop = props.initialScrollTop
      scrollTop.value = props.initialScrollTop
      updateChapterPageMetrics()
    }
  })
  showControls()
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', schedulePageMetricsUpdate)
  if (hideTimer.value) {
    window.clearTimeout(hideTimer.value)
  }
  if (pageTurnTimer.value) {
    window.clearTimeout(pageTurnTimer.value)
  }
})
</script>

<template>
  <section class="immersive-reader" :class="themeClass" @mousemove="showControls">
    <button class="side-zone side-zone--left" type="button" aria-label="上一章" @click="goPrevious" />
    <button class="side-zone side-zone--right" type="button" aria-label="下一章" @click="goNext" />

    <div
      ref="scrollArea"
      class="reader-scroll"
      :style="readerStyle"
      @scroll.passive="handleScroll"
      @click="toggleControls"
    >
      <article class="reader-content" :class="contentClass">
        <p class="reader-content__book">{{ book.name }}</p>
        <h1 class="reader-content__title">{{ currentChapter.title }}</h1>
        <p
          v-for="(paragraph, index) in paragraphs"
          :key="`${currentChapter.id}_${index}`"
          class="reader-content__paragraph"
        >
          {{ paragraph }}
        </p>
      </article>
    </div>

    <div class="progress-bar">
      <span class="progress-bar__fill" :style="{ width: `${bookProgress}%` }" />
    </div>

    <div
      v-if="pageTurn"
      :key="pageTurn.key"
      class="page-turn-layer"
      :class="`page-turn-layer--${pageTurn.direction}`"
      aria-hidden="true"
    >
      <span class="page-turn-dim" />
      <span class="page-turn-corner" />
      <span class="page-turn-ridge" />
      <span class="page-turn-shadow" />
    </div>

    <Transition name="fade">
      <div v-if="controlsVisible" class="reader-controls" @click.stop>
        <header class="reader-toolbar">
          <button class="ghost-btn" type="button" @click="emit('exit')">返回</button>
          <div class="reader-toolbar__title">
            <strong>{{ book.name }}</strong>
            <span>{{ currentChapter.title }}</span>
          </div>
          <div class="reader-toolbar__actions">
            <button class="ghost-btn" type="button" @click="openFishMode">
              摸鱼模式
            </button>
            <button class="ghost-btn" type="button" @click="chapterDrawerOpen = true">
              <el-icon><component :is="Icons.List" /></el-icon>
              目录
            </button>
            <button class="ghost-btn" type="button" @click="settingsPanelOpen = true">
              <el-icon><component :is="Icons.Setting" /></el-icon>
              设置
            </button>
          </div>
        </header>

        <footer class="reader-status">
          <button class="ghost-btn" type="button" :disabled="currentChapterIndex === 0" @click="goPrevious">上一章</button>
          <span>
            第 {{ currentChapterIndex + 1 }} / {{ book.chapters.length }} 章 ·
            本章 {{ chapterPageIndex + 1 }} / {{ chapterPageCount }} 屏 ·
            {{ Math.round(bookProgress) }}%
          </span>
          <button
            class="ghost-btn"
            type="button"
            :disabled="currentChapterIndex === book.chapters.length - 1"
            @click="goNext"
          >
            下一章
          </button>
        </footer>
      </div>
    </Transition>

    <ChapterDrawer
      :open="chapterDrawerOpen"
      :chapters="book.chapters"
      :current-index="currentChapterIndex"
      @close="chapterDrawerOpen = false"
      @select-chapter="goChapter"
    />

    <ReaderSettingsPanel
      :open="settingsPanelOpen"
      :settings="settings"
      @close="settingsPanelOpen = false"
      @update-settings="emit('updateSettings', $event)"
    />
  </section>
</template>

<style scoped>
.immersive-reader {
  position: fixed;
  inset: 0;
  z-index: 30;
  overflow: hidden;
  color: #243040;
  background: #f4efe6;
}

.immersive-reader--day {
  background: #ffffff;
  color: #172033;
}

.immersive-reader--soft {
  background: #f4efe6;
  color: #28313f;
}

.immersive-reader--green {
  background: #e6f2e7;
  color: #1f3529;
}

.immersive-reader--night {
  background: #20242a;
  color: #d7dde7;
}

.immersive-reader--black {
  background: #050505;
  color: #d7d7d7;
}

.reader-scroll {
  position: relative;
  z-index: 2;
  height: 100vh;
  overflow-y: auto;
}

.reader-content {
  width: min(100% - 40px, 760px);
  min-height: 100vh;
  margin: 0 auto;
  padding: 88px 0 120px;
}

.reader-content--narrow {
  width: min(100% - 40px, 640px);
}

.reader-content--wide {
  width: min(100% - 40px, 920px);
}

.reader-content__book {
  margin: 0 0 10px;
  color: currentColor;
  opacity: 0.58;
  font-size: 0.78em;
  font-weight: 800;
}

.reader-content__title {
  margin: 0 0 34px;
  color: currentColor;
  font-size: 1.55em;
  line-height: 1.35;
}

.reader-content__paragraph {
  margin: 0 0 1.2em;
  white-space: pre-wrap;
}

.reader-content--indent .reader-content__paragraph {
  text-indent: 2em;
}

.side-zone {
  position: fixed;
  top: 0;
  z-index: 3;
  width: 18vw;
  height: 100vh;
  border: 0;
  background: transparent;
  cursor: pointer;
}

.side-zone--left {
  left: 0;
}

.side-zone--right {
  right: 0;
}

.progress-bar {
  position: fixed;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 9;
  height: 3px;
  background: rgba(15, 23, 42, 0.14);
}

.progress-bar__fill {
  display: block;
  height: 100%;
  background: #14b8a6;
}

.page-turn-layer {
  position: fixed;
  inset: 0;
  z-index: 40;
  overflow: hidden;
  pointer-events: none;
  --turn-size: clamp(300px, 62vmin, 680px);
  --turn-ridge: clamp(360px, 76vmin, 820px);
  --turn-shift: min(58vw, 780px);
  --turn-rise: min(13vh, 130px);
}

.page-turn-dim,
.page-turn-corner,
.page-turn-ridge,
.page-turn-shadow {
  position: absolute;
  display: block;
}

.page-turn-dim {
  inset: 0;
  opacity: 0;
  background: rgba(15, 23, 42, 0.1);
  animation: pageTurnDim 0.56s ease-out forwards;
}

.page-turn-corner {
  width: var(--turn-size);
  height: var(--turn-size);
  bottom: 0;
  opacity: 0;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(247, 242, 232, 0.94) 46%, rgba(188, 174, 151, 0.78)),
    repeating-linear-gradient(90deg, rgba(112, 94, 70, 0.05) 0 1px, transparent 1px 10px);
  box-shadow: 0 22px 54px rgba(15, 23, 42, 0.22);
  will-change: transform, opacity, clip-path;
}

.page-turn-ridge {
  width: var(--turn-ridge);
  height: clamp(10px, 1.6vmin, 16px);
  bottom: calc(var(--turn-size) * 0.43);
  opacity: 0;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.92), rgba(70, 58, 42, 0.16), transparent);
  filter: blur(0.2px);
  will-change: transform, opacity;
}

.page-turn-shadow {
  width: calc(var(--turn-size) * 1.25);
  height: calc(var(--turn-size) * 0.72);
  bottom: -5vh;
  opacity: 0;
  background: radial-gradient(ellipse at center, rgba(15, 23, 42, 0.28), transparent 68%);
  filter: blur(15px);
  will-change: transform, opacity;
}

.page-turn-layer--next .page-turn-corner {
  right: 0;
  clip-path: polygon(100% 100%, 100% 0, 0 100%);
  border-radius: 8px 0 0 0;
  transform-origin: 100% 100%;
  animation: pageCornerNext 0.56s cubic-bezier(0.18, 0.76, 0.22, 1) forwards;
}

.page-turn-layer--previous .page-turn-corner {
  left: 0;
  clip-path: polygon(0 100%, 0 0, 100% 100%);
  border-radius: 0 8px 0 0;
  transform-origin: 0 100%;
  animation: pageCornerPrevious 0.56s cubic-bezier(0.18, 0.76, 0.22, 1) forwards;
}

.page-turn-layer--next .page-turn-ridge {
  right: calc(var(--turn-size) * 0.18);
  transform-origin: 100% 50%;
  animation: pageRidgeNext 0.56s ease-out forwards;
}

.page-turn-layer--previous .page-turn-ridge {
  left: calc(var(--turn-size) * 0.18);
  transform-origin: 0 50%;
  animation: pageRidgePrevious 0.56s ease-out forwards;
}

.page-turn-layer--next .page-turn-shadow {
  right: -8vw;
  animation: pageShadowNext 0.56s ease-out forwards;
}

.page-turn-layer--previous .page-turn-shadow {
  left: -8vw;
  animation: pageShadowPrevious 0.56s ease-out forwards;
}

@keyframes pageCornerNext {
  0% {
    opacity: 0;
    transform: scale(0.12) rotate(0deg);
  }

  28% {
    opacity: 1;
    transform: scale(1.02) rotate(-4deg);
  }

  62% {
    opacity: 0.95;
    transform: translate3d(calc(var(--turn-shift) * -0.48), calc(var(--turn-rise) * -0.5), 0) scale(1.22) rotate(-9deg);
  }

  100% {
    opacity: 0;
    transform: translate3d(calc(var(--turn-shift) * -1), calc(var(--turn-rise) * -1), 0) scale(1.42) rotate(-14deg);
  }
}

@keyframes pageCornerPrevious {
  0% {
    opacity: 0;
    transform: scale(0.12) rotate(0deg);
  }

  28% {
    opacity: 1;
    transform: scale(1.02) rotate(4deg);
  }

  62% {
    opacity: 0.95;
    transform: translate3d(calc(var(--turn-shift) * 0.48), calc(var(--turn-rise) * -0.5), 0) scale(1.22) rotate(9deg);
  }

  100% {
    opacity: 0;
    transform: translate3d(var(--turn-shift), calc(var(--turn-rise) * -1), 0) scale(1.42) rotate(14deg);
  }
}

@keyframes pageRidgeNext {
  0% {
    opacity: 0;
    transform: rotate(-45deg) translateX(32%) scaleX(0.25);
  }

  34% {
    opacity: 0.9;
    transform: rotate(-45deg) translateX(-4%) scaleX(0.95);
  }

  100% {
    opacity: 0;
    transform: rotate(-45deg) translateX(-65%) scaleX(1.2);
  }
}

@keyframes pageRidgePrevious {
  0% {
    opacity: 0;
    transform: rotate(45deg) translateX(-32%) scaleX(0.25);
  }

  34% {
    opacity: 0.9;
    transform: rotate(45deg) translateX(4%) scaleX(0.95);
  }

  100% {
    opacity: 0;
    transform: rotate(45deg) translateX(65%) scaleX(1.2);
  }
}

@keyframes pageShadowNext {
  0% {
    opacity: 0;
    transform: translate3d(18%, 14%, 0) scale(0.55);
  }

  36% {
    opacity: 0.8;
    transform: translate3d(-12%, 4%, 0) scale(1);
  }

  100% {
    opacity: 0;
    transform: translate3d(-50%, -3%, 0) scale(1.15);
  }
}

@keyframes pageShadowPrevious {
  0% {
    opacity: 0;
    transform: translate3d(-18%, 14%, 0) scale(0.55);
  }

  36% {
    opacity: 0.8;
    transform: translate3d(12%, 4%, 0) scale(1);
  }

  100% {
    opacity: 0;
    transform: translate3d(50%, -3%, 0) scale(1.15);
  }
}

@keyframes pageTurnDim {
  0%,
  100% {
    opacity: 0;
  }

  42% {
    opacity: 1;
  }
}

.reader-controls {
  position: fixed;
  inset: 0;
  z-index: 12;
  pointer-events: none;
}

.reader-toolbar,
.reader-status {
  pointer-events: auto;
  position: absolute;
  left: 18px;
  right: 18px;
  display: flex;
  align-items: center;
  border: 1px solid rgba(15, 23, 42, 0.1);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.82);
  color: #172033;
  box-shadow: 0 16px 40px rgba(15, 23, 42, 0.12);
  backdrop-filter: blur(18px);
}

.reader-toolbar {
  top: 16px;
  justify-content: space-between;
  gap: 14px;
  padding: 10px;
}

.reader-toolbar__title {
  display: grid;
  min-width: 0;
  text-align: center;
}

.reader-toolbar__title strong,
.reader-toolbar__title span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reader-toolbar__title span {
  color: #64748b;
  font-size: 12px;
}

.reader-toolbar__actions {
  display: flex;
  gap: 8px;
}

.reader-status {
  bottom: 18px;
  justify-content: center;
  gap: 18px;
  width: fit-content;
  margin: 0 auto;
  padding: 10px;
}

.ghost-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 36px;
  padding: 0 13px;
  border: 1px solid #dbe3ef;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.9);
  color: #334155;
  font-weight: 800;
  cursor: pointer;
}

.ghost-btn:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.18s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

@media (max-width: 720px) {
  .page-turn-layer {
    --turn-size: clamp(240px, 74vmin, 420px);
    --turn-ridge: clamp(280px, 86vmin, 520px);
    --turn-shift: min(64vw, 390px);
    --turn-rise: min(10vh, 88px);
  }

  .reader-content {
    width: min(100% - 30px, 760px);
    padding-top: 76px;
  }

  .side-zone {
    width: 12vw;
  }

  .reader-toolbar {
    align-items: stretch;
    flex-wrap: wrap;
  }

  .reader-toolbar__title {
    order: 3;
    width: 100%;
    text-align: left;
  }

  .reader-status {
    left: 10px;
    right: 10px;
    width: auto;
    gap: 8px;
    font-size: 12px;
  }
}
</style>
