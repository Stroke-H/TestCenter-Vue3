<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, shallowRef } from 'vue'
import { useVideoFloatStore } from '@/stores'

defineOptions({ name: 'VideoPlayerFloat' })

const videoFloatStore = useVideoFloatStore()

const position = reactive({
  x: 0,
  y: 0,
  width: 420,
  height: 260
})

const dragState = reactive({
  active: false,
  startX: 0,
  startY: 0,
  originX: 0,
  originY: 0
})

const hoverHeader = shallowRef(false)

const frameUrl = computed(() => videoFloatStore.url)

function initPosition() {
  position.width = Math.min(480, Math.max(380, Math.floor(window.innerWidth * 0.32)))
  position.height = Math.min(300, Math.max(220, Math.floor(position.width * 0.6)))
  position.x = Math.max(16, window.innerWidth - position.width - 28)
  position.y = Math.max(84, window.innerHeight - position.height - 28)
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max)
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
  window.addEventListener('pointerup', stopDrag)
}

function handlePointerMove(event: PointerEvent) {
  if (!dragState.active) return
  position.x = clamp(dragState.originX + event.clientX - dragState.startX, 8, window.innerWidth - position.width - 8)
  position.y = clamp(dragState.originY + event.clientY - dragState.startY, 8, window.innerHeight - position.height - 8)
}

function stopDrag() {
  dragState.active = false
  window.removeEventListener('pointermove', handlePointerMove)
  window.removeEventListener('pointerup', stopDrag)
}

function openInNewWindow() {
  window.open(videoFloatStore.url, '_blank', 'noopener,noreferrer')
}

onMounted(() => {
  initPosition()
  window.addEventListener('resize', initPosition)
})

onUnmounted(() => {
  stopDrag()
  window.removeEventListener('resize', initPosition)
})
</script>

<template>
  <section
    v-if="videoFloatStore.visible"
    class="video-player-float"
    :style="{
      left: `${position.x}px`,
      top: `${position.y}px`,
      width: `${position.width}px`,
      height: `${position.height}px`
    }"
  >
    <header
      class="video-player-float__header"
      :class="{ 'is-hovered': hoverHeader }"
      @pointerdown="startDrag"
      @mouseenter="hoverHeader = true"
      @mouseleave="hoverHeader = false"
    >
      <div class="video-player-float__meta">
        <span class="video-player-float__eyebrow">摸鱼模式</span>
        <strong>{{ videoFloatStore.title }}</strong>
      </div>
      <div class="video-player-float__actions">
        <button type="button" @click.stop="openInNewWindow">新窗口打开</button>
        <button type="button" @click.stop="videoFloatStore.close">关闭</button>
      </div>
    </header>

    <div class="video-player-float__body">
      <iframe :src="frameUrl" title="视频摸鱼模式" allowfullscreen referrerpolicy="no-referrer" />
    </div>
  </section>
</template>

<style scoped>
.video-player-float {
  position: fixed;
  z-index: 1600;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.08);
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 8px;
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.22);
  backdrop-filter: blur(8px);
}

.video-player-float__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  color: #fff;
  background: rgba(15, 23, 42, 0.52);
  cursor: grab;
  opacity: 0.58;
  transition: opacity 0.2s ease;
}

.video-player-float__header.is-hovered {
  opacity: 0.96;
}

.video-player-float__meta {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.video-player-float__eyebrow {
  color: rgba(191, 219, 254, 0.92);
  font-size: 11px;
}

.video-player-float__meta strong {
  overflow: hidden;
  max-width: 220px;
  font-size: 13px;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.video-player-float__actions {
  display: flex;
  gap: 8px;
}

.video-player-float__actions button {
  padding: 5px 10px;
  color: #e2e8f0;
  font: inherit;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.15);
  border-radius: 6px;
  cursor: pointer;
}

.video-player-float__body {
  height: calc(100% - 48px);
  background: rgba(255, 255, 255, 0.86);
}

.video-player-float__body iframe {
  width: 100%;
  height: 100%;
  border: 0;
}
</style>
