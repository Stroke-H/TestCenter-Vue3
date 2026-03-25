<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Plus, Search, Calendar, User, Money } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'

defineOptions({ name: 'AcceptanceReport' })

// --- API Service ---
const API_BASE = 'http://localhost:8080/api'
const authStore = useAuthStore()

// ===== State =====
const stats = ref([
  { title: 'Total Report', value: 0, icon: Calendar, color: '#3b82f6', bgColor: '#eff6ff' },
  { title: 'Tester', value: 1, icon: User, color: '#eab308', bgColor: '#fefce8' },
  { title: 'Coming Soon', value: 'N/A', icon: Money, color: '#f97316', bgColor: '#fff7ed' }
])

const recentReports = ref<any[]>([])
const historyProjects = ref<any[]>([])
const searchQuery = ref('')
const categoryFilter = ref('')

// --- Preview State ---
const previewVisible = ref(false)
const currentPreview = ref<any>(null)

// --- Fetch Logic ---
const fetchReports = async () => {
  try {
    const res = await fetch(`${API_BASE}/acceptance-reports/list`, {
      headers: {
        'Authorization': authStore.token
      }
    })
    const data = await res.json()
    if (data && Array.isArray(data)) {
      recentReports.value = data.map(r => ({
        ...r, // Keep original data for preview
        avatar: r.reporter ? r.reporter.charAt(0).toUpperCase() : 'R',
        reporter: `${r.reporter} | ${r.project_name}`,
        date: r.created_at ? new Date(r.created_at).toISOString().split('T')[0] : '-',
        status: r.status || 'Completed'
      }))

      // Update Stats
      if (stats.value[0]) {
        stats.value[0].value = data.length
      }
      
      // Update History Projects (Count per Code)
      const counts: Record<string, number> = {}
      data.forEach((r: any) => {
        counts[r.project_code] = (counts[r.project_code] || 0) + 1
      })
      historyProjects.value = Object.entries(counts).map(([id, count]) => ({ id, count }))
    }
  } catch (err) {
    console.error('Failed to fetch reports', err)
  }
}

const handleRowClick = (row: any) => {
  currentPreview.value = row
  previewVisible.value = true
}

onMounted(() => {
  fetchReports()
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
        <el-button type="primary" :icon="Plus" class="new-report-btn">
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
              <el-table-column prop="reporter" label="Reporter/Project" min-width="250">
                <template #default="{ row }">
                  <span class="reporter-text">{{ row.reporter }}</span>
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
      :title="`Report Preview - ${currentPreview?.project_name || 'Detail'}`"
      width="600px"
      destroy-on-close
      class="preview-dialog"
    >
      <div v-if="currentPreview" class="preview-content">
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
            <span class="value">{{ currentPreview.test_env || 'N/A' }}</span>
          </div>
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
      <template #footer>
        <span class="dialog-footer">
          <el-button type="primary" @click="previewVisible = false">Close</el-button>
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

.preview-grid {
  display: flex;
  flex-direction: column;
  gap: 20px;
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
