<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, shallowRef, useTemplateRef, watch } from 'vue'
import { useFishReaderStore } from '@/stores'

defineOptions({ name: 'FishReaderFloat' })

const fishReaderStore = useFishReaderStore()
const contentRef = useTemplateRef<HTMLElement>('contentRef')
const controlsVisible = shallowRef(false)

const position = reactive({
  x: 0,
  y: 0,
  width: 420,
  height: 620
})

const dragState = reactive({
  active: false,
  startX: 0,
  startY: 0,
  originX: 0,
  originY: 0
})

const resizeState = reactive({
  active: false,
  startX: 0,
  startY: 0,
  originWidth: 0,
  originHeight: 0
})

const currentChapter = computed(() => {
  const book = fishReaderStore.book
  if (!book) return null
  return book.chapters[fishReaderStore.chapterIndex] ?? book.chapters[0] ?? null
})

const paragraphs = computed(() => {
  const chapter = currentChapter.value
  if (!chapter) return []

  return (chapter.content || '本章暂无正文。')
    .split(/\n{2,}/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean)
})

const shellClass = computed(() => `fish-reader-float--${fishReaderStore.settings?.theme ?? 'soft'}`)

const contentStyle = computed(() => {
  const settings = fishReaderStore.settings
  return {
    fontSize: `${Math.max(14, (settings?.fontSize ?? 18) - 2)}px`,
    lineHeight: lineHeightMap[settings?.lineHeight ?? 'comfortable'],
    fontFamily: fontMap[settings?.font ?? 'serif']
  }
})

const indentEnabled = computed(() => fishReaderStore.settings?.indent ?? true)

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

function initPosition() {
  position.width = Math.min(460, Math.max(360, window.innerWidth * 0.32))
  position.height = Math.min(640, Math.max(480, window.innerHeight * 0.72))
  position.x = Math.max(16, window.innerWidth - position.width - 32)
  position.y = Math.max(76, window.innerHeight - position.height - 32)
}

function goChapter(delta: number) {
  const book = fishReaderStore.book
  if (!book) return

  fishReaderStore.setChapterIndex(fishReaderStore.chapterIndex + delta)
  nextTick(() => {
    if (contentRef.value) {
      contentRef.value.scrollTop = 0
    }
  })
}

function goPage(delta: number) {
  const content = contentRef.value
  if (!content) return

  const targetScrollTop = delta > 0
    ? getNextPageScrollTop(content)
    : getPageAlignedScrollTop(content, delta)
  content.scrollTo({
    top: targetScrollTop,
    behavior: 'smooth'
  })
}

function getNextPageScrollTop(content: HTMLElement) {
  const targetLineOffset = findFirstIncompleteLineOffset(content)
  if (targetLineOffset === null) {
    return getPageAlignedScrollTop(content, 1)
  }

  const maxScrollTop = Math.max(0, content.scrollHeight - content.clientHeight)
  return clamp(content.scrollTop + targetLineOffset, 0, maxScrollTop)
}

function findFirstIncompleteLineOffset(content: HTMLElement) {
  const contentRect = content.getBoundingClientRect()
  const style = window.getComputedStyle(content)
  const viewportTop = contentRect.top + parseFloat(style.paddingTop)
  const viewportBottom = contentRect.bottom - parseFloat(style.paddingBottom)
  const lineRects = getTextLineRects(content)
  const incompleteLine = lineRects.find((rect) => rect.bottom > viewportBottom + 0.5 && rect.top > viewportTop + 0.5)

  return incompleteLine ? incompleteLine.top - viewportTop : null
}

function getTextLineRects(content: HTMLElement) {
  const walker = document.createTreeWalker(content, NodeFilter.SHOW_TEXT)
  const range = document.createRange()
  const rects: DOMRect[] = []
  let node = walker.nextNode()

  while (node) {
    if (node.textContent?.trim()) {
      range.selectNodeContents(node)
      rects.push(...Array.from(range.getClientRects()))
    }
    node = walker.nextNode()
  }

  range.detach()
  return rects.sort((a, b) => a.top - b.top || a.left - b.left)
}

function getPageAlignedScrollTop(content: HTMLElement, delta: number) {
  const lineHeight = getContentLineHeight(content)
  const readableHeight = Math.max(lineHeight, content.clientHeight)
  const fullLineCount = Math.max(1, Math.floor(readableHeight / lineHeight))
  const pageStep = fullLineCount * lineHeight
  const maxScrollTop = Math.max(0, content.scrollHeight - content.clientHeight)

  return clamp(content.scrollTop + delta * pageStep, 0, maxScrollTop)
}

function getContentLineHeight(content: HTMLElement) {
  const style = window.getComputedStyle(content)
  const parsedLineHeight = parseFloat(style.lineHeight)
  if (Number.isFinite(parsedLineHeight)) return parsedLineHeight

  const parsedFontSize = parseFloat(style.fontSize)
  return Number.isFinite(parsedFontSize) ? parsedFontSize * 1.8 : 28
}

function toggleControls() {
  controlsVisible.value = !controlsVisible.value
}

function startDrag(event: PointerEvent) {
  const target = event.target as HTMLElement
  if (target.closest('button')) return

  dragState.active = true
  dragState.startX = event.clientX
  dragState.startY = event.clientY
  dragState.originX = position.x
  dragState.originY = position.y
  window.addEventListener('pointermove', handlePointerMove)
  window.addEventListener('pointerup', stopPointerAction)
}

function startResize(event: PointerEvent) {
  event.stopPropagation()
  resizeState.active = true
  resizeState.startX = event.clientX
  resizeState.startY = event.clientY
  resizeState.originWidth = position.width
  resizeState.originHeight = position.height
  window.addEventListener('pointermove', handlePointerMove)
  window.addEventListener('pointerup', stopPointerAction)
}

function handlePointerMove(event: PointerEvent) {
  if (dragState.active) {
    position.x = clamp(dragState.originX + event.clientX - dragState.startX, 8, window.innerWidth - position.width - 8)
    position.y = clamp(dragState.originY + event.clientY - dragState.startY, 8, window.innerHeight - position.height - 8)
  }

  if (resizeState.active) {
    position.width = clamp(resizeState.originWidth + event.clientX - resizeState.startX, 320, Math.min(720, window.innerWidth - position.x - 8))
    position.height = clamp(resizeState.originHeight + event.clientY - resizeState.startY, 420, Math.min(820, window.innerHeight - position.y - 8))
  }
}

function stopPointerAction() {
  dragState.active = false
  resizeState.active = false
  window.removeEventListener('pointermove', handlePointerMove)
  window.removeEventListener('pointerup', stopPointerAction)
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
}

function handleKeydown(event: KeyboardEvent) {
  if (!fishReaderStore.visible || isEditableTarget(event.target)) return

  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    goChapter(-1)
  } else if (event.key === 'ArrowRight') {
    event.preventDefault()
    goChapter(1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    goPage(-1)
  } else if (event.key === 'ArrowDown') {
    event.preventDefault()
    goPage(1)
  }
}

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable
}

watch(
  () => fishReaderStore.visible,
  (visible) => {
    if (visible && position.x === 0 && position.y === 0) {
      initPosition()
    }
    if (visible) {
      controlsVisible.value = false
    }
  },
  { immediate: true }
)

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', initPosition)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', initPosition)
  stopPointerAction()
})
</script>

<template>
  <section
    v-if="fishReaderStore.visible && fishReaderStore.book && currentChapter"
    class="fish-reader-float"
    :class="[shellClass, { 'fish-reader-float--immersive': !controlsVisible }]"
    :style="{
      left: `${position.x}px`,
      top: `${position.y}px`,
      width: `${position.width}px`,
      height: `${position.height}px`
    }"
    @click="toggleControls"
  >
    <header
      v-if="controlsVisible"
      class="fish-reader-float__header"
      @click.stop
      @pointerdown="startDrag"
    >
      <div class="fish-reader-float__meta">
        <span class="fish-reader-float__mode">摸鱼模式</span>
        <strong class="fish-reader-float__book">{{ fishReaderStore.book.name }}</strong>
      </div>
      <div class="fish-reader-float__actions">
        <button type="button" :disabled="fishReaderStore.chapterIndex === 0" @click="goChapter(-1)">上一章</button>
        <button
          type="button"
          :disabled="fishReaderStore.chapterIndex === fishReaderStore.book.chapters.length - 1"
          @click="goChapter(1)"
        >
          下一章
        </button>
        <button class="fish-reader-float__close" type="button" @click="fishReaderStore.close">关闭</button>
      </div>
    </header>

    <article
      ref="contentRef"
      class="fish-reader-float__content"
      :class="{ 'fish-reader-float__content--indent': indentEnabled }"
      :style="contentStyle"
    >
      <h1 class="fish-reader-float__title">{{ currentChapter.title }}</h1>
      <p
        v-for="(paragraph, index) in paragraphs"
        :key="`${currentChapter.id}_${index}`"
      >
        {{ paragraph }}
      </p>
    </article>

    <footer v-if="controlsVisible" class="fish-reader-float__footer" @click.stop>
      <button type="button" @click="goPage(-1)">上一屏</button>
      <span>第 {{ fishReaderStore.chapterIndex + 1 }} / {{ fishReaderStore.book.chapters.length }} 章</span>
      <button type="button" @click="goPage(1)">下一屏</button>
    </footer>

    <button
      class="fish-reader-float__resize"
      type="button"
      aria-label="调整大小"
      @click.stop
      @pointerdown="startResize"
    />
  </section>
</template>

<style scoped>
.fish-reader-float {
  position: fixed;
  z-index: 3000;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  overflow: hidden;
  border: 1px solid rgba(15, 23, 42, 0.14);
  border-radius: 8px;
  background: rgba(244, 239, 230, 0.52);
  color: #243040;
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.22);
  backdrop-filter: blur(16px);
}

.fish-reader-float--immersive {
  grid-template-rows: minmax(0, 1fr);
}

.fish-reader-float--day {
  background: rgba(255, 255, 255, 0.46);
  color: #172033;
}

.fish-reader-float--green {
  background: rgba(230, 242, 231, 0.5);
  color: #1f3529;
}

.fish-reader-float--night {
  background: rgba(32, 36, 42, 0.72);
  color: #d7dde7;
}

.fish-reader-float--black {
  background: rgba(5, 5, 5, 0.74);
  color: #d7d7d7;
}

.fish-reader-float__header,
.fish-reader-float__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px;
  background: rgba(255, 255, 255, 0.2);
}

.fish-reader-float__header {
  cursor: move;
  border-bottom: 1px solid rgba(15, 23, 42, 0.1);
}

.fish-reader-float__footer {
  border-top: 1px solid rgba(15, 23, 42, 0.1);
}

.fish-reader-float__meta {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.fish-reader-float__mode {
  color: #0f766e;
  font-size: 11px;
  font-weight: 900;
}

.fish-reader-float__book {
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fish-reader-float__actions {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

.fish-reader-float button {
  min-height: 30px;
  padding: 0 9px;
  border: 1px solid rgba(15, 23, 42, 0.14);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.68);
  color: inherit;
  font-size: 12px;
  font-weight: 800;
  cursor: pointer;
}

.fish-reader-float button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.fish-reader-float__close {
  color: #dc2626 !important;
}

.fish-reader-float__content {
  min-height: 0;
  overflow: auto;
  padding: 18px 20px 24px;
  scroll-behavior: smooth;
}

.fish-reader-float--immersive .fish-reader-float__content {
  padding-top: 22px;
  padding-bottom: 22px;
}

.fish-reader-float__content--indent p {
  text-indent: 2em;
}

.fish-reader-float__title {
  margin: 0 0 18px;
  font-size: 1.25em;
  line-height: 1.4;
}

.fish-reader-float__content p {
  margin: 0 0 1.1em;
  white-space: pre-wrap;
}

.fish-reader-float__footer span {
  overflow: hidden;
  color: currentColor;
  font-size: 13px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fish-reader-float__resize {
  position: absolute;
  right: 2px;
  bottom: 2px;
  width: 18px;
  height: 18px;
  min-height: 0 !important;
  padding: 0 !important;
  border: 0 !important;
  background:
    linear-gradient(135deg, transparent 0 45%, rgba(15, 23, 42, 0.28) 46% 53%, transparent 54%),
    linear-gradient(135deg, transparent 0 62%, rgba(15, 23, 42, 0.22) 63% 70%, transparent 71%) !important;
  cursor: nwse-resize;
}
</style>
