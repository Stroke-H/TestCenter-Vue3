<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { init, use, graphic } from 'echarts/core'
import { BarChart, LineChart, SankeyChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { ECharts, EChartsOption } from 'echarts'
import {
  Search,
  DataAnalysis,
  View,
  FullScreen,
  Delete,
  MagicStick
} from '@element-plus/icons-vue'
import { ElMessageBox, ElMessage } from 'element-plus'

import { storeToRefs } from 'pinia'
import { useReportStore } from '@/stores'
import { retryFetch } from '@/utils/retryFetch'
import { buildBackendUrl, normalizeBackendUrl } from '@/utils/runtimeUrl'

use([LineChart, BarChart, SankeyChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

// ---------- 1. 状态声明与 Store 挂载 ----------
const activeFilter = ref('K6 压测')
const searchQuery = ref('')
const dialogVisible = ref(false)
const analyticsLoading = ref(false)
const analyticsError = ref('')
const iframeUrl = ref('')
const selectedReport = ref<any>(null)
const selectedAnalyticsId = ref('')
const trendChartRef = ref<HTMLElement>()
const stackChartRef = ref<HTMLElement>()
const sankeyChartRef = ref<HTMLElement>()

let trendChart: ECharts | null = null
let stackChart: ECharts | null = null
let sankeyChart: ECharts | null = null

const filterOptions = ['Web 性能分析', 'K6 压测', '接口验证', 'UI 自动化', 'Monkey 测试']

// 挂载全局 Reports 仓库
const reportStore = useReportStore()
const { reports } = storeToRefs(reportStore)

const availablePerformanceReports = ref<string[]>([])

interface FailureBreakdownItem {
  key: string
  label: string
  value: number
  color: string
}

interface DramaFailedStat {
  id: string
  name: string
  createdAt: string
  reportUrl: string
  failedChecks: number
  failedRequests: number
  breakdown: FailureBreakdownItem[]
}

interface FailureCategory {
  key: string
  label: string
  color: string
  aliases: string[]
}

const failureCategories: FailureCategory[] = [
  { key: 'fetch', label: '接口抓取失败', color: '#ef4444', aliases: ['Fetch Failures', '接口抓取失败数', 'fetch_fail_count'] },
  { key: 'retry', label: '720p网络失败', color: '#f97316', aliases: ['Retry Candidates', '720p网络失败待复验数', 'retry_candidate_count'] },
  { key: 'continuity', label: '跳号剧集', color: '#eab308', aliases: ['Continuity Failures', '跳号剧集数', 'continuity_fail_count'] },
  { key: 'offline', label: '下架/异常章节', color: '#8b5cf6', aliases: ['Unhealthy Chapters', '下架/异常章节总数', 'offline_chapter_count'] },
  { key: 'mismatch', label: '计数不符', color: '#06b6d4', aliases: ['Total Mismatch', '计数不符剧集数', 'total_mismatch_count'] },
  { key: 'conversion', label: '转换失败', color: '#64748b', aliases: ['update_status_fail_count', '转换失败'] }
]

const dramaAnalytics = ref<DramaFailedStat[]>([])

const normalizeReportType = (type: string) => {
  if (type === '业务自动化') return 'K6 压测'
  if (type === 'Monkey测试') return 'Monkey 测试'
  return type
}

const getReportEnvironment = (row: any) => {
  const normalizedType = normalizeReportType(row.type)
  if (normalizedType !== 'K6 压测') return 'prod'

  const env = String(row.environment || '').toLowerCase()
  if (['prod', 'production', '正式服', '正式', '正服'].includes(env)) return 'prod'
  if (['test', 'testing', '测试服', '测试', '测服'].includes(env)) return 'test'
  return 'prod'
}

const getReportEnvBadge = (row: any) => {
  const env = getReportEnvironment(row)
  return {
    label: env === 'prod' ? 'Prod' : 'Test',
    className: env === 'prod' ? 'env-badge env-badge--prod' : 'env-badge env-badge--test'
  }
}

const showDramaAnalytics = computed(() => activeFilter.value === 'K6 压测')

// 获取后端 report 目录下真正存在的性能报告文件列表
const fetchAvailableReports = async () => {
  try {
    const response = await retryFetch(buildBackendUrl('/api/performance/reports'))
    availablePerformanceReports.value = await response.json()
  } catch (e) {
    console.error('获取性能报告列表失败', e)
  }
}

// ---------- 2. 核心计算属性与方法 ----------

// 根据顶栏的 Filter 与搜索框双重过滤列表
const filteredReports = computed(() => {
  return reports.value.filter(item => {
    // 1. 基础过滤：匹配侧边栏分类和搜索框
    const displayType = normalizeReportType(item.type)
    const matchesFilter = displayType === activeFilter.value
    const matchesSearch = item.name.toLowerCase().includes(searchQuery.value.toLowerCase()) || 
                          item.id.toLowerCase().includes(searchQuery.value.toLowerCase())
    
    if (!matchesFilter || !matchesSearch) return false

    // 2. 特殊逻辑：针对 “Web 性能分析”（Lighthouse），只有 report 文件夹中有文件才显示
    if (displayType === 'Web 性能分析') {
      if (!item.reportUrl) return false
      // 提取文件名，例如从 /performance-reports/baidu.report.html?t=... 提取出 baidu.report.html
      const fileName = item.reportUrl.split('/').pop()?.split('?')[0]
      return fileName && availablePerformanceReports.value.includes(fileName)
    }

    // 3. K6 压测等其他类型保持原始逻辑，直接显示记录
    return true
  })
})

const fetchDramaRunAnalytics = async () => {
  const response = await retryFetch(buildBackendUrl('/api/test-runs/analytics/drama-failed-checks'))
  if (!response.ok) throw new Error('无法读取剧集播放归档指标，请确认当前域名和后端端口可访问')
  const data = await response.json()
  if (!Array.isArray(data)) return []
  return data
}

const selectedAnalytics = computed<DramaFailedStat | null>(() => {
  return dramaAnalytics.value.find(item => item.id === selectedAnalyticsId.value) ||
    dramaAnalytics.value[dramaAnalytics.value.length - 1] ||
    null
})

const maxFailedChecks = computed(() => {
  return Math.max(1, ...dramaAnalytics.value.map(item => item.failedChecks))
})

const getBreakdownTotal = (item: DramaFailedStat | null) => {
  if (!item) return 0
  return item.breakdown.reduce((sum, part) => sum + part.value, 0)
}

const estimateSankeyLabelWidth = (label: string) => {
  return Array.from(label).reduce((width, char) => width + (char.charCodeAt(0) > 255 ? 13 : 7), 0)
}

const selectedSankeyItems = computed<FailureBreakdownItem[]>(() => {
  if (!selectedAnalytics.value) return []
  return selectedAnalytics.value.breakdown.filter(item => item.value > 0)
})

const sankeyLabelLayout = computed(() => {
  const maxLabelWidth = Math.max(0, ...selectedSankeyItems.value.map(item => estimateSankeyLabelWidth(item.label)))
  const labelWidth = Math.min(108, Math.max(48, Math.ceil(maxLabelWidth + 8)))
  return {
    labelWidth,
    right: Math.min(124, Math.max(58, labelWidth + 24))
  }
})

const disposeAnalyticsCharts = () => {
  trendChart?.dispose()
  stackChart?.dispose()
  sankeyChart?.dispose()
  trendChart = null
  stackChart = null
  sankeyChart = null
}

const loadDramaAnalytics = async () => {
  disposeAnalyticsCharts()
  analyticsLoading.value = true
  analyticsError.value = ''
  let shouldRenderCharts = false
  try {
    const sourceReports = await fetchDramaRunAnalytics()
    const stats = sourceReports.map((report) => ({
      id: report.id || report.runId,
      name: report.name,
      createdAt: report.createdAt,
      reportUrl: report.reportUrl,
      failedChecks: report.failedChecks || 0,
      failedRequests: report.failedRequests || 0,
      breakdown: Array.isArray(report.breakdown) ? report.breakdown : []
    }))

    dramaAnalytics.value = stats.sort((a, b) => Date.parse(a.createdAt) - Date.parse(b.createdAt))
    const peak = dramaAnalytics.value.reduce<DramaFailedStat | null>((current, item) => {
      if (!current || item.failedChecks >= current.failedChecks) return item
      return current
    }, null)
    selectedAnalyticsId.value = peak?.id || dramaAnalytics.value[dramaAnalytics.value.length - 1]?.id || ''
    shouldRenderCharts = true
  } catch (error: any) {
    analyticsError.value = error?.message || 'Failed Checks 视图加载失败'
  } finally {
    analyticsLoading.value = false
    if (shouldRenderCharts) {
      await nextTick()
      renderAnalyticsCharts()
    }
  }
}

const getReportShortLabel = (item: DramaFailedStat) => {
  return item.createdAt?.slice(5, 16) || item.id
}

const getTrendOption = (): EChartsOption => {
  const labels = dramaAnalytics.value.map(getReportShortLabel)
  return {
    color: ['#2563eb'],
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(15, 23, 42, 0.92)',
      borderWidth: 0,
      textStyle: { color: '#fff' },
      formatter: (params: any) => {
        const point = Array.isArray(params) ? params[0] : params
        const item = dramaAnalytics.value[point.dataIndex]
        if (!item) return ''
        return `${item.name}<br/>Failed Checks: <b>${item.failedChecks}</b><br/>Failed Requests: ${item.failedRequests}`
      }
    },
    grid: { left: 44, right: 24, top: 38, bottom: 44 },
    xAxis: {
      type: 'category',
      data: labels,
      boundaryGap: false,
      axisLine: { lineStyle: { color: '#cbd5e1' } },
      axisTick: { show: false },
      axisLabel: { color: '#64748b', fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: '#e2e8f0', type: 'dashed' } },
      axisLabel: { color: '#64748b' }
    },
    series: [
      {
        name: 'Failed Checks',
        type: 'line',
        data: dramaAnalytics.value.map(item => item.failedChecks),
        smooth: true,
        symbol: 'circle',
        symbolSize: 10,
        lineStyle: { width: 4, color: '#2563eb' },
        itemStyle: { color: '#ffffff', borderColor: '#2563eb', borderWidth: 3 },
        areaStyle: {
          color: new graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(37, 99, 235, 0.28)' },
            { offset: 1, color: 'rgba(37, 99, 235, 0.02)' }
          ])
        },
        emphasis: { scale: 1.25 }
      }
    ]
  }
}

const getStackOption = (): EChartsOption => {
  return {
    color: failureCategories.map(item => item.color),
    legend: {
      top: 0,
      itemWidth: 10,
      itemHeight: 10,
      textStyle: { color: '#475569', fontSize: 11 }
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      backgroundColor: 'rgba(15, 23, 42, 0.92)',
      borderWidth: 0,
      textStyle: { color: '#fff' }
    },
    grid: { left: 44, right: 20, top: 54, bottom: 44 },
    xAxis: {
      type: 'category',
      data: dramaAnalytics.value.map(getReportShortLabel),
      axisLine: { lineStyle: { color: '#cbd5e1' } },
      axisTick: { show: false },
      axisLabel: { color: '#64748b', fontSize: 11 }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: '#e2e8f0', type: 'dashed' } },
      axisLabel: { color: '#64748b' }
    },
    series: failureCategories.map(category => ({
      name: category.label,
      type: 'bar',
      stack: 'failure',
      barMaxWidth: 34,
      emphasis: { focus: 'series' },
      data: dramaAnalytics.value.map(item => item.breakdown.find(part => part.key === category.key)?.value || 0)
    }))
  }
}

const getSankeyOption = (): EChartsOption => {
  const sourceValue = getBreakdownTotal(selectedAnalytics.value)
  const { labelWidth, right } = sankeyLabelLayout.value
  return {
    color: selectedSankeyItems.value.map(item => item.color),
    tooltip: {
      trigger: 'item',
      triggerOn: 'mousemove',
      backgroundColor: 'rgba(15, 23, 42, 0.92)',
      borderWidth: 0,
      textStyle: { color: '#fff' }
    },
    series: [
      {
        type: 'sankey',
        left: 18,
        right,
        top: 20,
        bottom: 20,
        nodeWidth: 18,
        nodeGap: 14,
        draggable: false,
        emphasis: { focus: 'adjacency' },
        lineStyle: {
          color: 'gradient',
          curveness: 0.55,
          opacity: 0.35
        },
        label: {
          color: '#0f172a',
          fontWeight: 700,
          fontSize: 12,
          width: labelWidth,
          overflow: 'truncate'
        },
        data: [
          { name: '异常分类合计', value: sourceValue, itemStyle: { color: '#0f172a' } },
          ...selectedSankeyItems.value.map(item => ({
            name: item.label,
            value: item.value,
            itemStyle: { color: item.color }
          }))
        ],
        links: selectedSankeyItems.value.map(item => ({
          source: '异常分类合计',
          target: item.label,
          value: item.value
        }))
      }
    ]
  }
}

const renderAnalyticsCharts = async () => {
  if (!showDramaAnalytics.value || analyticsLoading.value || dramaAnalytics.value.length === 0) return
  await nextTick()

  if (trendChartRef.value) {
    if (trendChart && trendChart.getDom() !== trendChartRef.value) {
      trendChart.dispose()
      trendChart = null
    }
    trendChart ||= init(trendChartRef.value)
    trendChart.setOption(getTrendOption(), true)
    trendChart.off('click')
    trendChart.on('click', (params: any) => {
      const item = dramaAnalytics.value[params.dataIndex]
      if (item) selectedAnalyticsId.value = item.id
    })
  }

  if (stackChartRef.value) {
    if (stackChart && stackChart.getDom() !== stackChartRef.value) {
      stackChart.dispose()
      stackChart = null
    }
    stackChart ||= init(stackChartRef.value)
    stackChart.setOption(getStackOption(), true)
    stackChart.off('click')
    stackChart.on('click', (params: any) => {
      const item = dramaAnalytics.value[params.dataIndex]
      if (item) selectedAnalyticsId.value = item.id
    })
  }

  renderSankeyChart()
  resizeAnalyticsCharts()
}

const renderSankeyChart = async () => {
  if (!showDramaAnalytics.value || analyticsLoading.value) return
  if (selectedSankeyItems.value.length === 0) {
    sankeyChart?.dispose()
    sankeyChart = null
    return
  }
  await nextTick()
  if (!sankeyChartRef.value) return
  if (sankeyChart && sankeyChart.getDom() !== sankeyChartRef.value) {
    sankeyChart.dispose()
    sankeyChart = null
  }
  sankeyChart ||= init(sankeyChartRef.value)
  sankeyChart.setOption(getSankeyOption(), true)
}

const resizeAnalyticsCharts = () => {
  trendChart?.resize()
  stackChart?.resize()
  sankeyChart?.resize()
}

// 根据不同状态返回徽章对应的 Element-Plus type
const getStatusType = (status: string) => {
  switch (status) {
    case 'Passed': return 'success'
    case 'Failed': return 'danger'
    case 'Warning': return 'warning'
    case 'Running': return 'primary'
    default: return 'info'
  }
}

// 打开弹窗查看报告 (只针对压测)
const viewReport = (row: any) => {
  if (row.reportUrl) {
    selectedReport.value = row
    const normalizedUrl = normalizeBackendUrl(row.reportUrl)
    const separator = normalizedUrl.includes('?') ? '&' : '?'
    iframeUrl.value = `${normalizedUrl}${separator}t=${Date.now()}`
    dialogVisible.value = true
  }
}

const openInNewTab = (url: string) => {
  window.open(url, '_blank')
}

// 清分记录确认
const confirmClear = () => {
  ElMessageBox.confirm(
    '确定要清空所有测试报告记录吗？此操作不可撤销。',
    '提示',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(() => {
    reportStore.clearReports()
    ElMessage.success('历史记录已清空')
  }).catch(() => {})
}

onMounted(() => {
  fetchAvailableReports()
  reportStore.fetchReports()
  loadDramaAnalytics()
  window.addEventListener('resize', resizeAnalyticsCharts)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeAnalyticsCharts)
  disposeAnalyticsCharts()
})

watch(selectedAnalyticsId, () => {
  renderSankeyChart()
})

watch(activeFilter, async () => {
  if (!showDramaAnalytics.value) {
    disposeAnalyticsCharts()
    return
  }
  await nextTick()
  if (dramaAnalytics.value.length === 0 && !analyticsLoading.value) {
    loadDramaAnalytics()
    return
  }
  renderAnalyticsCharts()
})
</script>

<template>
  <div class="test-reports-page">
    
    <!-- 顶栏操作区：包含标题、筛选按钮和搜索框 -->
    <div class="page-header">
      <div class="header-title">
        <div class="title-row">
          <h2>报告大厅</h2>
        </div>
        <span class="subtitle">共 {{ filteredReports.length }} 份测试报告</span>
      </div>
      
      <div class="header-actions">
        <!-- 过滤器 -->
        <el-radio-group v-model="activeFilter" size="large" class="filter-group">
          <el-radio-button 
            v-for="opt in filterOptions" 
            :key="opt" 
            :value="opt"
          >
            {{ opt }}
          </el-radio-button>
        </el-radio-group>
        
        <!-- 搜索框 -->
        <el-input
          v-model="searchQuery"
          placeholder="搜索报告名称或 ID..."
          class="search-input"
          :prefix-icon="Search"
          clearable
        />

        <!-- 清空按钮 -->
        <el-button 
          v-if="reports.length > 0"
          type="danger" 
          plain 
          :icon="Delete"
          @click="confirmClear"
        >
          清空记录
        </el-button>
      </div>
    </div>

    <section v-if="showDramaAnalytics" class="analytics-card">
      <div v-loading="analyticsLoading" class="failure-analytics failure-analytics--embedded">
        <el-alert
          v-if="analyticsError"
          :title="analyticsError"
          type="error"
          show-icon
          :closable="false"
        />

        <el-empty
          v-else-if="!analyticsLoading && dramaAnalytics.length === 0"
          description="暂无可分析的剧集播放接口测试报告"
        />

        <template v-else>
          <div class="analytics-summary">
            <div class="summary-card">
              <span class="summary-label">报告数</span>
              <strong>{{ dramaAnalytics.length }}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label">峰值 Failed Checks</span>
              <strong>{{ maxFailedChecks }}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label">选中异常分类合计</span>
              <strong>{{ getBreakdownTotal(selectedAnalytics) }}</strong>
            </div>
            <div class="summary-card">
              <span class="summary-label">桑基图选中报告</span>
              <strong>{{ selectedAnalytics ? getReportShortLabel(selectedAnalytics) : '-' }}</strong>
            </div>
          </div>

          <div class="analytics-layout">
            <div class="trend-panel">
              <div class="panel-title">
                <span>Failed Checks 数值变化</span>
                <small>点击峰值或任意节点查看失败来源</small>
              </div>
              <div ref="trendChartRef" class="echart-panel echart-panel--trend" />

              <div class="panel-title panel-title--stack">
                <span>失败类型构成</span>
                <small>堆叠柱用于比较每次测试的失败来源</small>
              </div>
              <div ref="stackChartRef" class="echart-panel echart-panel--stack" />
            </div>

            <div class="sankey-panel">
              <div class="panel-title">
                <span>失败来源流向</span>
                <small>{{ selectedAnalytics?.name || '选择一份报告' }}</small>
              </div>

              <div v-if="selectedSankeyItems.length === 0" class="sankey-empty">
                <strong>没有失败流向</strong>
                <span>当前报告没有可拆分的异常分类计数。</span>
              </div>

              <div v-else ref="sankeyChartRef" class="echart-panel echart-panel--sankey" />

              <div class="sankey-legend">
                <div
                  v-for="item in selectedAnalytics?.breakdown || []"
                  :key="item.key"
                  class="legend-item"
                >
                  <span class="legend-dot" :style="{ background: item.color }" />
                  <span>{{ item.label }}</span>
                  <strong>{{ item.value }}</strong>
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </section>

    <!-- 主体表格区 -->
    <el-card class="table-card" shadow="never">
      <el-table 
        :data="filteredReports" 
        style="width: 100%" 
        :row-class-name="'report-row'"
      >
        <!-- ID 与名称 -->
        <el-table-column label="报告信息" min-width="280">
          <template #default="{ row }">
            <div class="report-info-col">
              <el-icon class="file-icon" color="#94a3b8"><DataAnalysis /></el-icon>
              <div class="info-texts">
                <span class="report-id-line">
                  <span class="report-id">{{ row.id }}</span>
                  <span :class="getReportEnvBadge(row).className">{{ getReportEnvBadge(row).label }}</span>
                </span>
                <span class="report-name">{{ row.name }}</span>
              </div>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="author" label="测试人" width="140">
          <template #default="{ row }">
            <span>{{ row.author || '-' }}</span>
          </template>
        </el-table-column>

        <!-- 测试类型 -->
        <el-table-column prop="type" label="类型" width="140">
          <template #default="{ row }">
            <el-tag size="small" effect="light" class="type-tag">
              {{ normalizeReportType(row.type) }}
            </el-tag>
          </template>
        </el-table-column>

        <!-- 执行时间 -->
        <el-table-column prop="createdAt" label="生成时间" width="200" />
        
        <!-- 耗时 -->
        <el-table-column prop="duration" label="耗时" width="120" />

        <!-- 状态列 -->
        <el-table-column prop="status" label="状态" width="120">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" effect="dark" round size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>

        <!-- 操作列 -->
        <el-table-column label="操作" width="160" fixed="right" align="center">
          <template #default="{ row }">
            <div class="action-btns">
              <el-button 
                v-if="row.reportUrl"
                type="primary" 
                link 
                :icon="View"
                @click="viewReport(row)"
              >
                查看
              </el-button>
              <el-button v-else type="info" link disabled>暂无图表</el-button>
            </div>
          </template>
        </el-table-column>
        
        <!-- 空状态 -->
        <template #empty>
          <el-empty description="没有匹配的历史报告" />
        </template>
      </el-table>
    </el-card>

    <!-- 图表型报告的弹窗 / 抽屉查看器 -->
    <el-dialog
      v-model="dialogVisible"
      title="压测数据分析报告"
      width="90%"
      top="5vh"
      custom-class="report-dialog"
      :destroy-on-close="true"
    >
      <div class="iframe-container">
        <iframe v-if="dialogVisible" :src="iframeUrl" frameborder="0" class="report-iframe" />
      </div>

      <!-- AI 智能总结面板 -->
      <div v-if="selectedReport?.analysisResult" class="ai-analysis-panel">
        <div class="analysis-header">
          <el-icon class="magic-icon"><MagicStick /></el-icon>
          <span class="header-text">AI 智能报告总结</span>
        </div>
        <div class="analysis-content">
          {{ selectedReport.analysisResult }}
        </div>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">关闭</el-button>
          <el-button type="primary" :icon="FullScreen" @click="openInNewTab(iframeUrl)">
            全屏另存为
          </el-button>
        </span>
      </template>
    </el-dialog>

  </div>
</template>

<style scoped>
/* 整个容器撑满外层 Layout 给的 Padding */
.test-reports-page {
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - 120px);
  gap: 20px;
  padding-bottom: 24px;
}

/* 顶部信息和栏目操作 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-title h2 {
  margin: 0;
  font-size: 24px;
  color: #1e293b;
  font-weight: 700;
}

.subtitle {
  font-size: 13px;
  color: #94a3b8;
  margin-top: 4px;
  display: inline-block;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.search-input {
  width: 260px;
}

/* 覆写 Radio 样式使其更贴合现代化质感 */
:deep(.el-radio-button__inner) {
  border-radius: 8px !important;
  border: 0 !important;
  background: transparent;
  color: #64748b;
  font-weight: 600;
  box-shadow: none !important;
  transition: background-color 0.2s ease, color 0.2s ease;
}

:deep(.filter-group) {
  display: flex;
  align-items: stretch;
  background: #f8fafc;
  height: 32px;
  padding: 2px;
  border-radius: 10px;
  border: 1px solid #e2e8f0;
  box-sizing: border-box;
}

:deep(.filter-group .el-radio-button) {
  margin-right: 2px;
  height: 100%;
}

:deep(.filter-group .el-radio-button:last-child) {
  margin-right: 0;
}

:deep(.filter-group .el-radio-button__inner:hover) {
  color: #334155;
  background: rgba(226, 232, 240, 0.7);
}

:deep(.filter-group .el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-left-color: transparent !important;
}

:deep(.filter-group .el-radio-button__inner) {
  height: 100%;
  line-height: 26px;
  padding: 0 14px;
  background: transparent;
}

:deep(.el-radio-button.is-active .el-radio-button__inner) {
  background: #ffffff;
  color: #0f172a;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.08) !important;
}

/* 主体表格卡片 */
.table-card {
  flex: none;
  border-radius: 12px;
  /* 去除默认 padding，让表格充满 */
  :deep(.el-card__body) {
    padding: 0;
  }
}

/* 自定义表格样式 */
.report-info-col {
  display: flex;
  align-items: center;
  gap: 12px;
}

.file-icon {
  font-size: 24px;
  padding: 8px;
  background: #f8fafc;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
}

.info-texts {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.report-id-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.report-id {
  font-size: 12px;
  color: #94a3b8;
  font-family: monospace;
}

.env-badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 7px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 800;
  line-height: 18px;
  letter-spacing: 0;
}

.env-badge--test {
  color: #b45309;
  background: #fff7ed;
  border: 1px solid #fed7aa;
}

.env-badge--prod {
  color: #047857;
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
}

.report-name {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
  margin-top: 2px;
}

.type-tag {
  background-color: #f1f5f9;
  border-color: #e2e8f0;
  color: #475569;
}

/* 报表弹窗 iframe */
:deep(.el-dialog__body) {
  padding: 0; 
  height: 70vh;
}

.iframe-container {
  width: 100%;
  height: 100%;
  background: #f8fafc;
}

.report-iframe {
  width: 100%;
  height: 100%;
  border: none;
}
/* ==================== AI 总结面板 ==================== */
.ai-analysis-panel {
  margin-top: 24px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-left: 4px solid #6366f1;
  border-radius: 8px;
  padding: 20px;
}

.analysis-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  color: #4f46e5;
  font-weight: 600;
  font-size: 16px;
}

.magic-icon {
  font-size: 20px;
}

.analysis-content {
  color: #334155;
  line-height: 1.8;
  font-size: 14px;
  white-space: pre-wrap;
  background: #ffffff;
  padding: 16px;
  border-radius: 6px;
  border: 1px solid #f1f5f9;
}

.failure-analytics {
  min-height: 0;
}

.analytics-card {
  flex-shrink: 0;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 10px 28px rgba(15, 23, 42, 0.05);
}

.failure-analytics--embedded {
  padding: 14px 16px 16px;
}

.analytics-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.summary-card {
  border: 1px solid rgba(226, 232, 240, 0.75);
  border-radius: 10px;
  padding: 10px 12px;
  background: #f8fafc;
}

.summary-label {
  display: block;
  color: #64748b;
  font-size: 12px;
  margin-bottom: 6px;
}

.summary-card strong {
  color: #0f172a;
  font-size: 18px;
}

.analytics-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(360px, 0.9fr);
  gap: 12px;
}

.trend-panel,
.sankey-panel {
  border: 1px solid rgba(226, 232, 240, 0.78);
  border-radius: 12px;
  padding: 12px;
  background: #ffffff;
}

.panel-title {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 12px;
  margin-bottom: 12px;
}

.panel-title span {
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.panel-title small {
  color: #64748b;
  font-size: 12px;
}

.panel-title--stack {
  margin-top: 8px;
}

.echart-panel {
  width: 100%;
  border-radius: 10px;
  background: linear-gradient(180deg, #f8fafc 0%, #ffffff 100%);
}

.echart-panel--trend {
  height: 160px;
}

.echart-panel--stack {
  height: 170px;
}

.echart-panel--sankey {
  height: 270px;
  background: #f8fafc;
}

.sankey-empty {
  min-height: 180px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  gap: 8px;
  border: 1px dashed #cbd5e1;
  border-radius: 10px;
  color: #64748b;
  background: #f8fafc;
}

.sankey-empty strong {
  color: #0f172a;
}

.sankey-legend {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.legend-item {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  color: #475569;
  font-size: 12px;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

@media (max-width: 1180px) {
  .analytics-layout,
  .analytics-summary {
    grid-template-columns: 1fr;
  }
}
</style>
