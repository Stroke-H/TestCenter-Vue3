import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export interface ReportItem {
  id: string
  name: string
  type: string
  status: 'Passed' | 'Failed' | 'Running'
  duration: string
  createdAt: string
  author: string
  reportUrl?: string
}

export const useReportStore = defineStore('reports', () => {
  // 1. 初始化 state (尝试从 LocalStorage 读取历史记录)
  const storedReports = localStorage.getItem('testcenter_reports')
  const reports = ref<ReportItem[]>(storedReports ? JSON.parse(storedReports) : [])

  // 2. 监听变化并自动持久化
  watch(
    reports,
    (newVal) => {
      localStorage.setItem('testcenter_reports', JSON.stringify(newVal))
    },
    { deep: true }
  )

  // 3. Actions
  const addReport = (report: Omit<ReportItem, 'id' | 'createdAt'>) => {
    // 自动生成 ID 和当前时间
    const newReport: ReportItem = {
      ...report,
      id: `REP-${new Date().toISOString().replace(/\D/g, '').slice(0, 14)}`,
      createdAt: new Date().toLocaleString('zh-CN', { hour12: false })
    }
    
    // 采用 unshift 将最新记录推到最前面
    reports.value.unshift(newReport)
  }

  const clearReports = () => {
    reports.value = []
    localStorage.removeItem('testcenter_reports')
  }

  return {
    reports,
    addReport,
    clearReports
  }
})
