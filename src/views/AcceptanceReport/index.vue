<script setup lang="ts">
import { ref, onMounted, computed, watch, markRaw } from 'vue'
import { Plus, Search, Calendar, User, Money } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { retryFetch } from '@/utils/retryFetch'

defineOptions({ name: 'AcceptanceReport' })

// --- API Service ---
const API_BASE = 'http://localhost:8080/api'
const authStore = useAuthStore()

interface ProjectOption {
  id: string
  project_code: string
  project_name: string
  short_code?: string
}

interface DeviceOption {
  id: string
  device_name: string
  os: string
  model: string
  allowed_app: string
}

// ===== State =====
const stats = ref([
  { title: 'Total Report', value: 0, icon: markRaw(Calendar), color: '#3b82f6', bgColor: '#eff6ff' },
  { title: 'Tester', value: 0, icon: markRaw(User), color: '#eab308', bgColor: '#fefce8' },
  { title: 'Total Project', value: 0, icon: markRaw(Money), color: '#f97316', bgColor: '#fff7ed' }
])

const recentReports = ref<any[]>([])
const historyProjects = ref<any[]>([])
const searchQuery = ref('')
const categoryFilter = ref('')
const projects = ref<ProjectOption[]>([])
const devices = ref<DeviceOption[]>([])
const testTimeRange = ref<string[]>([])
const selectedTestDevices = ref<string[]>([])

// --- Preview State ---
const previewVisible = ref(false)
const currentPreview = ref<any>(null)
const previewMode = ref<'view' | 'create'>('view')
const sendingToFeishu = ref(false)
const reportForm = ref<any>({
  project_name: '',
  project_code: '',
  version: '',
  reporter: '',
  test_owner: '',
  test_time: '',
  test_env: '',
  test_devices: '',
  test_conclusion: 'Pass',
  update_requirements: '',
  bug_submission_status: '',
  bug_fix_status: '',
  status: 'Completed'
})

// --- Fetch Logic ---
const fetchReports = async () => {
  try {
    const res = await retryFetch(`${API_BASE}/acceptance-reports/list`, {
      credentials: 'include',
      headers: {
        'Authorization': authStore.token
      }
    })
    const data = await res.json()
    if (data && Array.isArray(data)) {
      recentReports.value = data.map(r => ({
        ...r, // Keep original data for preview
        avatar: r.reporter ? r.reporter.charAt(0).toUpperCase() : 'R',
        reporter_display: `${r.reporter} | ${r.project_name}`,
        date: r.created_at ? new Date(r.created_at).toISOString().split('T')[0] : '-',
        status: r.status || 'Completed'
      }))

      // Update Stats
      if (stats.value[0]) {
        stats.value[0].value = data.length
      }
      if (stats.value[1]) {
        const uniqueReporters = new Set(
          data
            .map((r: any) => (r.reporter || '').trim())
            .filter((reporter: string) => reporter)
        )
        stats.value[1].value = uniqueReporters.size
      }
      
      // Update History Projects (Count per Code)
      const counts: Record<string, number> = {}
      data.forEach((r: any) => {
        counts[r.project_code] = (counts[r.project_code] || 0) + 1
      })
      historyProjects.value = Object.entries(counts).map(([id, count]) => ({ id, count }))
      if (stats.value[2]) {
        stats.value[2].value = historyProjects.value.length
      }
    }
  } catch (err) {
    console.error('Failed to fetch reports', err)
  }
}

const fetchProjects = async () => {
  try {
    const res = await retryFetch(`${API_BASE}/config/projects`)
    const data = await res.json()
    projects.value = Array.isArray(data) ? data : []
  } catch (err) {
    console.error('Failed to fetch projects', err)
  }
}

const fetchDevices = async () => {
  try {
    const res = await retryFetch(`${API_BASE}/config/devices`)
    const data = await res.json()
    devices.value = Array.isArray(data) ? data : []
  } catch (err) {
    console.error('Failed to fetch devices', err)
  }
}

const selectedProject = computed(() => {
  if (reportForm.value.project_code) {
    return projects.value.find(item => item.project_code === reportForm.value.project_code) || null
  }
  if (reportForm.value.project_name) {
    return projects.value.find(item => item.project_name === reportForm.value.project_name) || null
  }
  return null
})

const filteredDevices = computed(() => {
  if (!reportForm.value.project_code) return []
  return devices.value.filter((item) => {
    const allowedApps = (item.allowed_app || '').split(',').map(app => app.trim()).filter(Boolean)
    return allowedApps.includes(reportForm.value.project_code)
  })
})

const syncProjectByCode = (code: string) => {
  const matched = projects.value.find(item => item.project_code === code)
  if (matched) {
    reportForm.value.project_name = matched.project_name
  }
}

const syncProjectByName = (name: string) => {
  const matched = projects.value.find(item => item.project_name === name)
  if (matched) {
    reportForm.value.project_code = matched.project_code
  }
}

const handleRowClick = (row: any) => {
  previewMode.value = 'view'
  currentPreview.value = row
  previewVisible.value = true
}

const canSendToFeishu = computed(() => {
  return previewMode.value === 'view' &&
    !!currentPreview.value?.id &&
    !!authStore.user?.username &&
    authStore.user.username === currentPreview.value?.reporter
})

const openCreateReport = () => {
  previewMode.value = 'create'
  currentPreview.value = null
  reportForm.value = {
    project_name: '',
    project_code: '',
    version: '',
    reporter: authStore.user?.username || '',
    test_owner: authStore.user?.username || '',
    test_time: '',
    test_env: '',
    test_devices: '',
    test_conclusion: 'Pass',
    update_requirements: '',
    bug_submission_status: '',
    bug_fix_status: '',
    status: 'Completed'
  }
  testTimeRange.value = []
  selectedTestDevices.value = []
  previewVisible.value = true
}

const saveNewReport = async () => {
  if (!reportForm.value.project_name || !reportForm.value.project_code || !reportForm.value.version) {
    ElMessage.warning('请至少填写项目名称、项目代码和版本号')
    return
  }

  try {
    const payload = {
      ...reportForm.value,
      id: `AR_${Date.now()}`,
      reporter: reportForm.value.reporter || authStore.user?.username || 'Manual Report',
      test_owner: reportForm.value.test_owner || reportForm.value.reporter || authStore.user?.username || 'Manual Report',
      test_devices: selectedTestDevices.value.join('、'),
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
      status: reportForm.value.status || 'Completed'
    }

    const res = await fetch(`${API_BASE}/acceptance-reports/save`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify(payload)
    })

    if (!res.ok) {
      throw new Error('Save failed')
    }

    ElMessage.success('验收报告创建成功')
    previewVisible.value = false
    fetchReports()
  } catch (err) {
    console.error('Failed to save acceptance report', err)
    ElMessage.error('保存验收报告失败')
  }
}

const sendReportToFeishu = async () => {
  if (!currentPreview.value?.id) return

  sendingToFeishu.value = true
  try {
    const res = await fetch(`${API_BASE}/acceptance-reports/send-feishu`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({ id: currentPreview.value.id })
    })

    const rawText = await res.text()
    let data: any = null
    try {
      data = rawText ? JSON.parse(rawText) : null
    } catch {
      data = { error: rawText || 'Unexpected response format' }
    }

    if (!res.ok) {
      throw new Error(data?.error || 'Send failed')
    }

    ElMessage.success('已发送到飞书群')
  } catch (err: any) {
    console.error('Failed to send acceptance report to Feishu', err)
    ElMessage.error(err?.message || '发送到飞书失败')
  } finally {
    sendingToFeishu.value = false
  }
}

const getPreviewTestEnv = (report: any) => {
  const rawEnv = (report?.test_env || '').trim()
  if (!rawEnv) return 'N/A'

  const knownEnvs = ['测试服务器', '正式服务器']
  const matchedEnv = knownEnvs.find(item => rawEnv.includes(item))
  if (!matchedEnv) return rawEnv

  return matchedEnv
}

const getPreviewTestDevices = (report: any) => {
  const explicitDevices = (report?.test_devices || '').trim()
  if (explicitDevices) return explicitDevices

  const rawEnv = (report?.test_env || '').trim()
  if (!rawEnv) return 'N/A'

  const knownEnvs = ['测试服务器', '正式服务器']
  for (const env of knownEnvs) {
    if (rawEnv.includes(env)) {
      const cleaned = rawEnv
        .replace(env, '')
        .replace(/^[：:、,\s/-]+/, '')
        .trim()
      return cleaned || 'N/A'
    }
  }

  return 'N/A'
}

watch(testTimeRange, (range) => {
  if (!Array.isArray(range) || range.length !== 2) {
    reportForm.value.test_time = ''
    return
  }
  reportForm.value.test_time = `${range[0]}～${range[1]}`
})

watch(selectedTestDevices, (list) => {
  reportForm.value.test_devices = list.join('、')
})

watch(() => reportForm.value.project_code, () => {
  const allowedNames = new Set(filteredDevices.value.map(item => item.device_name))
  selectedTestDevices.value = selectedTestDevices.value.filter(name => allowedNames.has(name))
})

onMounted(() => {
  fetchReports()
  fetchProjects()
  fetchDevices()
})
</script>

<template>
  <div class="acceptance-container">
    <!-- Header -->
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">Acceptance Report</h2>
        <div class="breadcrumb">Acceptance Report <span class="divider">/</span> Report</div>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" class="new-report-btn" @click="openCreateReport">
          New Report
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
      <!-- Left Column: Recent Report -->
      <el-col :span="16">
        <el-card class="content-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <h3 class="card-title">Recent Report</h3>
              <div class="header-actions">
                <el-input
                  v-model="searchQuery"
                  placeholder="Search by reporter..."
                  :prefix-icon="Search"
                  class="search-input"
                  clearable
                />
                <el-select v-model="categoryFilter" placeholder="Category" class="filter-select">
                  <el-option label="All Categories" value="" />
                  <el-option label="Mobile" value="mobile" />
                  <el-option label="Web" value="web" />
                </el-select>
              </div>
            </div>
          </template>
          
          <div class="table-scroll-container">
            <el-table 
              :data="recentReports" 
              style="width: 100%" 
              class="custom-table" 
              :row-style="{ height: '60px', cursor: 'pointer' }"
              @row-click="handleRowClick"
            >
              <el-table-column label="Img" width="70">
                <template #default="{ row }">
                  <div class="avatar-circle">{{ row.avatar }}</div>
                </template>
              </el-table-column>
              <el-table-column prop="reporter_display" label="Reporter/Project" min-width="250">
                <template #default="{ row }">
                  <span class="reporter-text">{{ row.reporter_display }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="date" label="Report Date" width="150" />
              <el-table-column prop="status" label="Status" width="120">
                <template #default="{ row }">
                  <span class="status-badge" :class="row.status.toLowerCase().replace(' ', '-')">
                    {{ row.status }}
                  </span>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-card>
      </el-col>

      <!-- Right Column: History Project -->
      <el-col :span="8">
        <el-card class="content-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <h3 class="card-title">History Project Report Count</h3>
            </div>
          </template>
          
            <div class="history-list scroll-container">
              <div v-for="item in historyProjects" :key="item.id" class="history-item">
                <div class="project-id">
                  <span class="id-dot"></span>
                  {{ item.id }}
                </div>
                <div class="count-bubble">{{ item.count }}</div>
              </div>
            </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Preview Dialog -->
    <el-dialog
      v-model="previewVisible"
      :title="previewMode === 'create' ? 'Create New Acceptance Report' : `Report Preview - ${currentPreview?.project_name || 'Detail'}`"
      width="600px"
      destroy-on-close
      class="preview-dialog"
    >
      <div v-if="previewMode === 'view' && currentPreview" class="preview-content">
        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Project Code:</span>
            <span class="value font-bold">{{ currentPreview.project_code }}</span>
          </div>
          <div class="preview-item">
            <span class="label">Version:</span>
            <span class="value">v{{ currentPreview.version }}</span>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Test Owner:</span>
            <span class="value">{{ currentPreview.reporter }}</span>
          </div>
          <div class="preview-item">
            <span class="label">Test Time:</span>
            <span class="value">{{ currentPreview.test_time || currentPreview.created_at?.split('T')[0] }}</span>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Test Env:</span>
            <span class="value">{{ getPreviewTestEnv(currentPreview) }}</span>
          </div>
          <div class="preview-item">
            <span class="label">Test Devices:</span>
            <span class="value">{{ getPreviewTestDevices(currentPreview) }}</span>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Conclusion:</span>
            <el-tag :type="currentPreview.test_conclusion === 'Pass' ? 'success' : 'danger'" size="small" effect="dark">
              {{ currentPreview.test_conclusion || 'Unknown' }}
            </el-tag>
          </div>
        </div>

        <el-divider border-style="dashed" />

        <div class="preview-grid">
          <div class="preview-detail">
            <h4 class="detail-title">测试需求点 (Acceptance Requirements)</h4>
            <pre class="detail-text">{{ currentPreview.update_requirements || '无需求说明' }}</pre>
          </div>

          <div class="preview-detail">
            <h4 class="detail-title">缺陷提交/修复情况 (Bug Status)</h4>
            <pre class="detail-text">{{ [currentPreview.bug_submission_status, currentPreview.bug_fix_status].filter(Boolean).join('\n') || '无缺陷记录' }}</pre>
          </div>
        </div>
      </div>
      <div v-else class="preview-content">
        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Project Name:</span>
            <el-select
              v-model="reportForm.project_name"
              filterable
              placeholder="请选择项目名称"
              style="width: 100%"
              @change="syncProjectByName"
            >
              <el-option
                v-for="project in projects"
                :key="project.id || project.project_code"
                :label="project.project_name"
                :value="project.project_name"
              />
            </el-select>
          </div>
          <div class="preview-item">
            <span class="label">Project Code:</span>
            <el-select
              v-model="reportForm.project_code"
              filterable
              placeholder="请选择项目代码"
              style="width: 100%"
              @change="syncProjectByCode"
            >
              <el-option
                v-for="project in projects"
                :key="project.id || project.project_code"
                :label="project.project_code"
                :value="project.project_code"
              />
            </el-select>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Version:</span>
            <el-input v-model="reportForm.version" placeholder="例如 2.58.0" />
          </div>
          <div class="preview-item">
            <span class="label">Test Time:</span>
            <el-date-picker
              v-model="testTimeRange"
              type="daterange"
              range-separator="~"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              style="width: 100%"
            />
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Reporter:</span>
            <el-input v-model="reportForm.reporter" placeholder="请输入报告人" />
          </div>
          <div class="preview-item">
            <span class="label">Test Owner:</span>
            <el-input v-model="reportForm.test_owner" placeholder="请输入测试负责人" />
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Test Env:</span>
            <el-select v-model="reportForm.test_env" placeholder="请选择测试环境" style="width: 100%">
              <el-option label="测试服务器" value="测试服务器" />
              <el-option label="正式服务器" value="正式服务器" />
            </el-select>
          </div>
          <div class="preview-item">
            <span class="label">Test Devices:</span>
            <el-select
              v-model="selectedTestDevices"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              placeholder="请选择测试设备"
              style="width: 100%"
            >
              <el-option
                v-for="device in filteredDevices"
                :key="device.id"
                :label="`${device.device_name}${device.model ? ` (${device.model})` : ''}`"
                :value="device.device_name"
              />
            </el-select>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">Conclusion:</span>
            <el-select v-model="reportForm.test_conclusion" style="width: 100%">
              <el-option label="Pass" value="Pass" />
              <el-option label="Fail" value="Fail" />
              <el-option label="Blocked" value="Blocked" />
            </el-select>
          </div>
          <div class="preview-item">
            <span class="label">Project Match:</span>
            <span class="value">{{ selectedProject?.project_name && selectedProject?.project_code ? `${selectedProject.project_name} / ${selectedProject.project_code}` : '请选择项目' }}</span>
          </div>
        </div>

        <el-divider border-style="dashed" />

        <div class="preview-grid">
          <div class="preview-detail">
            <h4 class="detail-title">测试需求点 (Acceptance Requirements)</h4>
            <el-input
              v-model="reportForm.update_requirements"
              type="textarea"
              :rows="6"
              placeholder="请输入测试需求点、需求链接或验收范围"
            />
          </div>

          <div class="preview-detail">
            <h4 class="detail-title">缺陷提交情况 (Bug Submission Status)</h4>
            <el-input
              v-model="reportForm.bug_submission_status"
              type="textarea"
              :rows="3"
              placeholder="请输入未修复缺陷、提单链接或说明"
            />

            <h4 class="detail-title detail-title--spaced">缺陷修复情况 (Bug Fix Status)</h4>
            <el-input
              v-model="reportForm.bug_fix_status"
              type="textarea"
              :rows="3"
              placeholder="请输入已修复缺陷、验证结果或说明"
            />
          </div>
        </div>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="previewVisible = false">{{ previewMode === 'create' ? '取消' : 'Close' }}</el-button>
          <el-button
            v-if="canSendToFeishu"
            type="success"
            :loading="sendingToFeishu"
            @click="sendReportToFeishu"
          >
            发送到飞书
          </el-button>
          <el-button v-if="previewMode === 'create'" type="primary" @click="saveNewReport">保存报告</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.acceptance-container {
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

.search-input {
  width: 220px;
}

.filter-select {
  width: 140px;
}

/* Table */
.custom-table {
  --el-table-border-color: #f1f5f9;
  --el-table-header-bg-color: #f8fafc;
  --el-table-header-text-color: #64748b;
}

.avatar-circle {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: #e2e8f0;
  color: #475569;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 14px;
}

.reporter-text {
  font-weight: 600;
  color: #334155;
}

/* Badges */
.status-badge {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  display: inline-block;
}

.status-badge.completed {
  background: #eff6ff;
  color: #3b82f6;
}

.status-badge.in-progress {
  background: #fefce8;
  color: #eab308;
}

.status-badge.pending {
  background: #f1f5f9;
  color: #64748b;
}

/* History List & Scroll */
.table-scroll-container,
.history-list {
  height: 480px; /* Approx 8 rows */
  overflow-y: auto;
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE/Edge */
}

.table-scroll-container::-webkit-scrollbar,
.history-list::-webkit-scrollbar {
  display: none; /* Chrome/Safari */
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 8px 0;
}

.history-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #f8fafc;
  border-radius: 12px;
  transition: background 0.2s;
}

.history-item:hover {
  background: #f1f5f9;
}

.project-id {
  display: flex;
  align-items: center;
  gap: 12px;
  font-weight: 600;
  color: #1e293b;
  font-size: 14px;
}

.id-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #3b82f6;
}

.count-bubble {
  background: #dbeafe;
  color: #2563eb;
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 700;
}

/* Preview Dialog */
.preview-content {
  color: #334155;
}

.preview-section {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  gap: 20px;
}

.preview-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.label {
  font-size: 12px;
  color: #64748b;
  font-weight: 600;
  text-transform: uppercase;
}

.value {
  font-size: 15px;
  color: #1e293b;
}

.detail-title {
  font-size: 14px;
  font-weight: 700;
  color: #6366f1;
  margin-bottom: 8px;
}

.detail-title--spaced {
  margin-top: 16px;
}

.preview-grid {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.preview-detail :deep(.el-textarea__inner),
.preview-item :deep(.el-input__wrapper),
.preview-item :deep(.el-select__wrapper) {
  border-radius: 10px;
}

.detail-text {
  background: #f8fafc;
  padding: 12px 16px;
  border-radius: 8px;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  color: #475569;
  border: 1px solid #e2e8f0;
  max-height: 200px;
  overflow-y: auto;
}
</style>
