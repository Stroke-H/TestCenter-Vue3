<script setup lang="ts">
import { VideoCamera, FullScreen } from '@element-plus/icons-vue'
import { useRouter } from 'vue-router'
import { useVideoFloatStore } from '@/stores'

defineOptions({ name: 'VideoPlayer' })

const router = useRouter()
const videoFloatStore = useVideoFloatStore()
const targetUrl = 'https://bbys.app/'

function openInNewWindow() {
  window.open(targetUrl, '_blank', 'noopener,noreferrer')
}

function openFishMode() {
  videoFloatStore.open({
    title: 'BBYS 视频播放',
    url: targetUrl
  })
  router.push('/dashboard')
}
</script>

<template>
  <main class="video-player-page">
    <header class="video-player-page__header">
      <div class="video-player-page__title">
        <p class="video-player-page__eyebrow">网页包装</p>
        <h1>视频播放</h1>
        <p>直接在平台内打开 BBYS，保留一个摸鱼模式浮窗。</p>
      </div>
      <div class="video-player-page__actions">
        <el-button :icon="VideoCamera" @click="openFishMode">摸鱼模式</el-button>
        <el-button type="primary" :icon="FullScreen" @click="openInNewWindow">新窗口打开</el-button>
      </div>
    </header>

    <section class="video-player-page__frame-shell">
      <iframe :src="targetUrl" title="BBYS 视频播放" allowfullscreen referrerpolicy="no-referrer" />
    </section>
  </main>
</template>

<style scoped>
.video-player-page {
  display: grid;
  gap: 18px;
  height: 100%;
}

.video-player-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 20px 22px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.video-player-page__title h1 {
  margin: 0;
  font-size: 28px;
  color: #0f172a;
}

.video-player-page__title p {
  margin: 8px 0 0;
  color: #64748b;
}

.video-player-page__eyebrow {
  margin: 0 0 6px !important;
  color: #2563eb !important;
  font-size: 12px;
  font-weight: 700;
}

.video-player-page__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.video-player-page__frame-shell {
  height: calc(100vh - 220px);
  overflow: hidden;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.video-player-page__frame-shell iframe {
  width: 100%;
  height: 100%;
  border: 0;
}

@media (max-width: 900px) {
  .video-player-page__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .video-player-page__frame-shell {
    height: calc(100vh - 260px);
  }
}
</style>
