// 应用级状态管理 — 控制侧边栏折叠、主题等全局状态
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAppStore = defineStore('app', () => {
  // 侧边栏是否折叠
  const sidebarCollapsed = ref(false)

  // 当前主题（预留扩展）
  const theme = ref<'dark' | 'light'>('dark')

  // 切换侧边栏折叠状态
  const toggleSidebar = () => {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  // 设置主题
  const setTheme = (newTheme: 'dark' | 'light') => {
    theme.value = newTheme
  }

  return {
    sidebarCollapsed,
    theme,
    toggleSidebar,
    setTheme
  }
})
