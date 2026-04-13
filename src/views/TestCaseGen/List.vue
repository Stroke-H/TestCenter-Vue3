<script setup lang="ts">
import { ref, onMounted, computed, markRaw } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Filter, Calendar, User, Collection, View, Download, Delete, Files } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import dayjs from 'dayjs'

defineOptions({ name: 'TestCaseHistory' })

const router = useRouter()
const API_BASE = '/api/testcase-gen'
const loading = ref(false)
const records = ref<any[]>([])
const projects = ref<any[]>([])
const filterProject = ref('')
const filterModule = ref('')

// --- Stats Logic ---
const stats = computed(() => [
  { 
    title: 'Total Records', 
    value: records.value.length, 
    icon: markRaw(Files), 
    color: '#3b82f6', 
    bgColor: '#eff6ff' 
  },
  { 
    title: 'Total Cases', 
    value: records.value.reduce((acc, r) => acc + (r.case_count || 0), 0), 
    icon: markRaw(Collection), 
    color: '#eab308', 
    bgColor: '#fefce8' 
  },
  { 
    title: 'Coverage Modules', 
    value: new Set(records.value.map(r => r.module || 'Default')).size,
    icon: markRaw(User), 
    color: '#f97316', 
    bgColor: '#fff7ed' 
  }
])

// --- Unique Filter Options ---
const uniqueProjects = computed(() => {
  const codes = new Set(records.value.map(r => r.project_code).filter(Boolean))
  return Array.from(codes).map(code => ({
    code,
    name: projects.value.find(p => p.project_code === code)?.project_name || code
  }))
})

const uniqueModules = computed(() => {
  const ms = new Set(records.value.map(r => r.module).filter(Boolean))
  return Array.from(ms).sort()
})

// --- Sidebar Logic (Project Summary) ---
const historySummary = computed(() => {
  const counts: Record<string, number> = {}
  records.value.forEach((r) => {
    const code = r.project_code || 'Manual'
    counts[code] = (counts[code] || 0) + 1
  })
  return Object.entries(counts)
    .sort((a, b) => b[1] - a[1])
    .map(([code, count]) => ({
      code,
      name: code === 'Manual' ? 'Manual' : (projects.value.find(p => p.project_code === code)?.project_name || code),
      count
    }))
})

const setProjectFilter = (code: string) => {
  filterProject.value = code === 'Manual' ? '' : code
}

// --- Filtered List ---
const filteredRecords = computed(() => {
  return records.value.filter(r => {
    const matchProject = !filterProject.value || r.project_code === filterProject.value
    const matchModule = !filterModule.value || r.module === filterModule.value
    return matchProject && matchModule
  })
})

const fetchRecords = async () => {
  loading.value = true
  try {
    const res = await axios.get(`${API_BASE}/records`)
    records.value = res.data || []
  } catch (error) {
    console.error('Failed to fetch records:', error)
    ElMessage.error('获取历史记录失败')
  } finally {
    loading.value = false
  }
}

const fetchProjects = async () => {
  try {
    const res = await axios.get('/api/config/projects')
    projects.value = res.data || []
  } catch (err) {
    console.error('Failed to fetch projects:', err)
  }
}

const formatTime = (timeStr: string) => {
  return dayjs(timeStr).format('YYYY-MM-DD HH:mm')
}

const truncateSummary = (text: string) => {
  if (!text) return '-'
  return text.length > 50 ? text.substring(0, 50) + '...' : text
}

const getProjectName = (code: string) => {
  return projects.value.find(p => p.project_code === code)?.project_name || code || '-'
}

const goToNew = () => {
  router.push('/testcase_gen/new')
}

const viewRecord = (row: any) => {
  router.push(`/testcase_gen/view/${row.id}`)
}

const downloadAgain = async (row: any) => {
  try {
    const res = await axios.get(`${API_BASE}/records/${row.id}`)
    const fullRecord = res.data
    const exportRes = await axios.post(`${API_BASE}/export`, {
      cases: fullRecord.cases
    }, { responseType: 'blob' })
    
    const url = window.URL.createObjectURL(new Blob([exportRes.data]))
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', `TestCase_${row.title || 'History'}.xlsx`)
    document.body.appendChild(link)
    link.click()
    link.remove()
    ElMessage.success('正在下载...')
  } catch (error) {
    ElMessage.error('导出失败')
  }
}

const deleteRecord = async (id: string) => {
  try {
    await axios.delete(`${API_BASE}/records/${id}`)
    ElMessage.success('已删除历史记录')
    fetchRecords()
  } catch (error) {
    ElMessage.error('删除失败')
  }
}

onMounted(() => {
  fetchRecords()
  fetchProjects()
})
</script>

<template>
  <div class="page-container">
    <!-- Header -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">TestCase Generation History</h2>
        <div class="breadcrumb">TestCase Gen <span class="divider">/</span> History</div>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" class="new-report-btn" @click="goToNew">
          Generate New
        </el-button>
      </div>
    </div>

    <!-- Stats Grid -->
    <el-row :gutter="24" class="stats-row">
      <el-col :span="8" v-for="(stat, index) in stats" :key="index">
        <el-card class="stat-card" shadow="hover">
          <div class="stat-content">
            <div class="stat-info">
              <span class="stat-title">{{ stat.title }}</span>
              <span class="stat-value">{{ stat.value }}</span>
            </div>
            <div class="stat-icon-wrapper" :style="{ backgroundColor: stat.bgColor, color: stat.color }">
              <el-icon class="stat-icon"><component :is="stat.icon" /></el-icon>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Main Data Area -->
    <el-row :gutter="24" class="content-row">
      <!-- Left Column: History Table -->
      <el-col :span="16">
        <el-card class="content-card" shadow="hover" v-loading="loading">
          <template #header>
            <div class="card-header">
              <h3 class="card-title">Recent History</h3>
              <div class="header-actions">
                <el-select
                  v-model="filterProject"
                  placeholder="Filter by Project"
                  clearable
                  class="filter-select"
                >
                  <template #prefix><el-icon><Filter /></el-icon></template>
                  <el-option
                    v-for="p in uniqueProjects"
                    :key="p.code"
                    :label="p.name"
                    :value="p.code"
                  />
                </el-select>
                <el-select
                  v-model="filterModule"
                  placeholder="Filter by Module"
                  clearable
                  class="filter-select"
                >
                   <template #prefix><el-icon><Calendar /></el-icon></template>
                  <el-option
                    v-for="m in uniqueModules"
                    :key="m"
                    :label="m"
                    :value="m"
                  />
                </el-select>
              </div>
            </div>
          </template>
          
          <div class="table-scroll-container">
            <el-table 
              :data="filteredRecords" 
              style="width: 100%" 
              class="custom-table" 
              :row-style="{ height: '64px' }"
              @row-click="viewRecord"
            >
              <el-table-column label="Icon" width="70">
                <template #default>
                  <div class="avatar-circle">AI</div>
                </template>
              </el-table-column>
              <el-table-column label="Title / Summary" min-width="240">
                <template #default="{ row }">
                  <div class="title-cell">
                    <span class="record-title">{{ row.title || 'Untitled Requirement' }}</span>
                    <span class="record-summary">{{ truncateSummary(row.requirement_text) }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="Project" width="140">
                <template #default="{ row }">
                  <el-tag v-if="row.project_code" size="small" effect="plain">{{ getProjectName(row.project_code) }}</el-tag>
                  <span v-else class="empty-text">-</span>
                </template>
              </el-table-column>
              <el-table-column label="Module" width="140" show-overflow-tooltip>
                <template #default="{ row }">
                  <span class="module-text">{{ row.module || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="Date" width="140">
                <template #default="{ row }">
                  <span class="date-text">{{ formatTime(row.created_at) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="Actions" width="120" fixed="right">
                <template #default="{ row }">
                  <div class="action-btns" @click.stop>
                    <el-tooltip content="Download Excel" placement="top">
                      <el-button link :icon="Download" @click="downloadAgain(row)" />
                    </el-tooltip>
                    <el-popconfirm title="Delete this record?" @confirm="deleteRecord(row.id)">
                      <template #reference>
                        <el-button link type="danger" :icon="Delete" />
                      </template>
                    </el-popconfirm>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>
          <template v-if="!records.length && !loading">
            <el-empty description="No records found" />
          </template>
        </el-card>
      </el-col>

      <!-- Right Column: Project Summary -->
      <el-col :span="8">
        <el-card class="content-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <h3 class="card-title">Generation Project Summary</h3>
            </div>
          </template>
          
          <div class="history-list scroll-container">
            <div 
              v-for="item in historySummary" 
              :key="item.code" 
              class="history-item"
              @click="setProjectFilter(item.code)"
            >
              <div class="project-id">
                <span class="id-dot"></span>
                {{ item.name }}
              </div>
              <div class="count-bubble">{{ item.count }}</div>
            </div>
            <div v-if="!historySummary.length" class="empty-state">No data available</div>
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.page-container {
  padding: 8px;
  font-family: 'Inter', -apple-system, sans-serif;
}

/* Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: #1e293b;
}

.breadcrumb {
  margin-top: 4px;
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
}

.divider {
  margin: 0 4px;
  color: #cbd5e1;
}

.new-report-btn {
  border-radius: 8px;
  padding: 10px 20px;
  font-weight: 600;
  box-shadow: 0 4px 6px -1px rgba(59, 130, 246, 0.2);
}

/* Stats */
.stats-row {
  margin-bottom: 24px;
}

.stat-card {
  border-radius: 16px;
  border: 1px solid #f1f5f9;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
  border-color: #3b82f6;
}

.stat-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px;
}

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-title {
  font-size: 14px;
  color: #64748b;
  font-weight: 600;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 32px;
  font-weight: 800;
  color: #0f172a;
}

.stat-icon-wrapper {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-icon {
  font-size: 28px;
}

/* Main Content */
.content-row {
  margin-bottom: 24px;
}

.content-card {
  border-radius: 16px;
  border: 1px solid #f1f5f9;
  height: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.filter-select {
  width: 180px;
}

/* Table Customization */
.custom-table :deep(.el-table__row) {
  cursor: pointer;
  transition: background-color 0.2s;
}

.custom-table :deep(.el-table__row:hover) {
  background-color: #f8fafc !important;
}

.avatar-circle {
  width: 40px;
  height: 40px;
  background-color: #f1f5f9;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 700;
  color: #64748b;
  border: 2px solid #fff;
  box-shadow: 0 2px 4px rgba(0,0,0,0.05);
}

.title-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.record-title {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
}

.record-summary {
  font-size: 12px;
  color: #64748b;
}

.module-text {
  font-size: 13px;
  color: #475569;
  font-weight: 500;
}

.date-text {
  font-size: 13px;
  color: #64748b;
}

.empty-text {
  color: #cbd5e1;
  font-style: italic;
}

.action-btns {
  display: flex;
  gap: 8px;
}

/* Sidebar History Items */
.scroll-container {
  max-height: 400px;
  overflow-y: auto;
  padding-right: 4px;
}

.history-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-radius: 12px;
  margin-bottom: 8px;
  background-color: #f8fafc;
  transition: all 0.2s;
  cursor: pointer;
}

.history-item:hover {
  background-color: #f1f5f9;
}

.project-id {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

.id-dot {
  width: 8px;
  height: 8px;
  background-color: #3b82f6;
  border-radius: 50%;
}

.count-bubble {
  background-color: #fff;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 20px;
  box-shadow: 0 1px 2px rgba(0,0,0,0.05);
  border: 1px solid #e2e8f0;
}

.empty-state {
  text-align: center;
  color: #94a3b8;
  padding: 40px 0;
  font-size: 14px;
}
</style>
