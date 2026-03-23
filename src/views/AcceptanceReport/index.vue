<script setup lang="ts">
import { ref } from 'vue'
import { Plus, Search, Calendar, User, Money } from '@element-plus/icons-vue'

defineOptions({ name: 'AcceptanceReport' })

// ===== Mock Data =====
const stats = ref([
  { title: 'Total Report', value: 49, icon: Calendar, color: '#3b82f6', bgColor: '#eff6ff' },
  { title: 'Tester', value: 7, icon: User, color: '#eab308', bgColor: '#fefce8' },
  { title: 'Coming Soon', value: 'N/A', icon: Money, color: '#f97316', bgColor: '#fff7ed' }
])

const recentReports = ref([
  { id: 1, avatar: 'Y', reporter: 'yuerusong | A1157_powerfulcleaner-Andriod', date: '2026-03-20', status: 'Completed' },
  { id: 2, avatar: 'M', reporter: 'minghong.huang | A1203_browser-iOS', date: '2026-03-21', status: 'Completed' },
  { id: 3, avatar: 'L', reporter: 'liwei | A1099_cleaner-Pro', date: '2026-03-22', status: 'In Progress' },
  { id: 4, avatar: 'Z', reporter: 'zhangsan | B1022_music-Andriod', date: '2026-03-23', status: 'Completed' },
  { id: 5, avatar: 'Y', reporter: 'yuerusong | A1158_powerfulcleaner-iOS', date: '2026-03-24', status: 'Pending' }
])

const historyProjects = ref([
  { id: 'A1106', count: 9 },
  { id: 'A1107', count: 5 },
  { id: 'B1022', count: 12 },
  { id: 'C3011', count: 3 },
  { id: 'A1157', count: 7 },
  { id: 'A1158', count: 2 },
  { id: 'D4055', count: 1 }
])

const searchQuery = ref('')
const categoryFilter = ref('')
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
          
          <el-table :data="recentReports" style="width: 100%" class="custom-table" :row-style="{ height: '60px' }">
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
          
          <div class="history-list">
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

/* History List */
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
</style>
