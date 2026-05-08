<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
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

// ---------- 1. 状态声明与 Store 挂载 ----------
const activeFilter = ref('全部')
const searchQuery = ref('')
const dialogVisible = ref(false)
const iframeUrl = ref('')
const selectedReport = ref<any>(null)

const filterOptions = ['全部', 'Web 性能分析', 'K6 压测', '接口验证', 'UI 自动化']

// 挂载全局 Reports 仓库
const reportStore = useReportStore()
const { reports } = storeToRefs(reportStore)

const availablePerformanceReports = ref<string[]>([])

// 获取后端 report 目录下真正存在的性能报告文件列表
const fetchAvailableReports = async () => {
  try {
    const response = await retryFetch(buildBackendUrl('/api/performance/reports'))
    availablePerformanceReports.value = await response.json()
  } catch (e) {
    console.error('获取性能报告列表失败', e)
  }
}

onMounted(() => {
  fetchAvailableReports()
  reportStore.fetchReports()
})

// ---------- 2. 核心计算属性与方法 ----------

// 根据顶栏的 Filter 与搜索框双重过滤列表
const filteredReports = computed(() => {
  return reports.value.filter(item => {
    // 1. 基础过滤：匹配侧边栏分类和搜索框
    const matchesFilter = activeFilter.value === '全部' || item.type === activeFilter.value
    const matchesSearch = item.name.toLowerCase().includes(searchQuery.value.toLowerCase()) || 
                          item.id.toLowerCase().includes(searchQuery.value.toLowerCase())
    
    if (!matchesFilter || !matchesSearch) return false

    // 2. 特殊逻辑：针对 “Web 性能分析”（Lighthouse），只有 report 文件夹中有文件才显示
    if (item.type === 'Web 性能分析') {
      if (!item.reportUrl) return false
      // 提取文件名，例如从 /performance-reports/baidu.report.html?t=... 提取出 baidu.report.html
      const fileName = item.reportUrl.split('/').pop()?.split('?')[0]
      return fileName && availablePerformanceReports.value.includes(fileName)
    }

    // 3. K6 压测等其他类型保持原始逻辑，直接显示记录
    return true
  })
})

// 根据不同状态返回徽章对应的 Element-Plus type
const getStatusType = (status: string) => {
  switch (status) {
    case 'Passed': return 'success'
    case 'Failed': return 'danger'
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
</script>

<template>
  <div class="test-reports-page">
    
    <!-- 顶栏操作区：包含标题、筛选按钮和搜索框 -->
    <div class="page-header">
      <div class="header-title">
        <h2>报告大厅</h2>
        <span class="subtitle">共 {{ filteredReports.length }} 份测试报告</span>
      </div>
      
      <div class="header-actions">
        <!-- 过滤器 -->
        <el-radio-group v-model="activeFilter" size="large" class="filter-group">
          <el-radio-button 
            v-for="opt in filterOptions" 
            :key="opt" 
            :label="opt"
          />
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

    <!-- 主体表格区 -->
    <el-card class="table-card" shadow="never">
      <el-table 
        :data="filteredReports" 
        style="width: 100%" 
        height="100%"
        :row-class-name="'report-row'"
      >
        <!-- ID 与名称 -->
        <el-table-column label="报告信息" min-width="280">
          <template #default="{ row }">
            <div class="report-info-col">
              <el-icon class="file-icon" color="#94a3b8"><DataAnalysis /></el-icon>
              <div class="info-texts">
                <span class="report-id">{{ row.id }}</span>
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
              {{ row.type }}
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
  height: calc(100vh - 120px);
  gap: 20px;
}

/* 顶部信息和栏目操作 */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
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
  flex: 1;
  display: flex;
  flex-direction: column;
  border-radius: 12px;
  /* 去除默认 padding，让表格充满 */
  :deep(.el-card__body) {
    padding: 0;
    height: 100%;
    display: flex;
    flex-direction: column;
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
}

.report-id {
  font-size: 12px;
  color: #94a3b8;
  font-family: monospace;
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
</style>
