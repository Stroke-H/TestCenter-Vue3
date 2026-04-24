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
        <h1>布布影视</h1>
      </div>
      <div class="video-player-page__actions">
        <el-button class="video-player-page__action-button" :icon="VideoCamera" @click="openFishMode">
          摸鱼模式
        </el-button>
        <el-button
          class="video-player-page__action-button"
          type="primary"
          :icon="FullScreen"
          @click="openInNewWindow"
        >
          新窗口打开
        </el-button>
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
  gap: 10px;
  height: 100%;
}

.video-player-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 58px;
  padding: 10px 14px;
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.video-player-page__title h1 {
  margin: 0;
  font-size: 20px;
  line-height: 1.2;
  color: #0f172a;
}

.video-player-page__actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.video-player-page__action-button {
  width: 116px;
  height: 32px;
  justify-content: center;
}

.video-player-page__frame-shell {
  height: calc(100vh - 164px);
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
    min-height: auto;
  }

  .video-player-page__frame-shell {
    height: calc(100vh - 210px);
  }
}
</style>
