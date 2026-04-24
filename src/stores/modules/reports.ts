import { defineStore } from 'pinia'
import { ref } from 'vue'
import { retryFetch } from '@/utils/retryFetch'

export interface ReportItem {
  id: string
  name: string
  type: string
  status: 'Passed' | 'Failed' | 'Running'
  duration: string
  createdAt: string
  author: string
  reportUrl?: string
  analysisResult?: string
}

export const useReportStore = defineStore('reports', () => {
  // reports 现在主要保存从后端拉取的记录
  const reports = ref<ReportItem[]>([])

  // 辅助函数：获取后端基础地址
  const getBackendHost = () => {
    return `${window.location.protocol}//${window.location.hostname}:8080`
  }

  // 1. 从后端加载执行记录 (不再使用 LocalStorage)
  const fetchReports = async () => {
    try {
      const response = await retryFetch(`${getBackendHost()}/api/execution-reports`)
      if (!response.ok) throw new Error('Fetch reports failed')
      const data = await response.json()
      reports.value = data
    } catch (e) {
      console.error('加载执行记录失败:', e)
    }
  }

  // 2. 向后端添加记录
  const addReport = async (reportData: Omit<ReportItem, 'id' | 'createdAt'>) => {
    try {
      const response = await fetch(`${getBackendHost()}/api/execution-reports`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          ...reportData,
          author: reportData.author || 'tester'
        })
      })
      
      if (!response.ok) throw new Error('Add report failed')
      
      const newReport = await response.json()
      // 将新生成的记录推送到列表最前面 (前端即时更新)
      reports.value.unshift(newReport)
      return newReport
    } catch (e) {
      console.error('保存执行记录失败:', e)
      return null
    }
  }

  // 3. 从后端清空全部记录
  const clearReports = async () => {
    try {
      const response = await fetch(`${getBackendHost()}/api/execution-reports`, {
        method: 'DELETE'
      })
      if (response.ok) {
        reports.value = []
      }
    } catch (e) {
      console.error('清空执行记录失败:', e)
    }
  }

  return {
    reports,
    fetchReports,
    addReport,
    clearReports
  }
})
