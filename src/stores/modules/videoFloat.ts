import { defineStore } from 'pinia'
import { shallowRef } from 'vue'

export const useVideoFloatStore = defineStore('videoFloat', () => {
  const visible = shallowRef(false)
  const title = shallowRef('视频播放器')
  const url = shallowRef('https://bbys.app/')

  function open(payload?: { title?: string; url?: string }) {
    title.value = payload?.title || '视频播放器'
    url.value = payload?.url || 'https://bbys.app/'
    visible.value = true
  }

  function close() {
    visible.value = false
  }

  return {
    visible,
    title,
    url,
    open,
    close
  }
})
