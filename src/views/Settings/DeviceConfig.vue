<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Search, Monitor, Apple, Platform } from '@element-plus/icons-vue'
import request from '@/api/request'

interface Device {
  id: string
  device_name: string
  os: string
  model: string
  allowed_app: string
  created_at?: string
}

interface Project {
  project_code: string
  project_name: string
}

const devices = ref<Device[]>([])
const projects = ref<Project[]>([])
const loading = ref(false)
const searchQuery = ref('')
const osFilter = ref<'all' | 'iOS' | 'Android'>('all')
const dialogVisible = ref(false)
const dialogType = ref<'add' | 'edit'>('add')
const form = ref<Device>({
  id: '',
  device_name: '',
  os: 'Android',
  model: '',
  allowed_app: ''
})

const selectedApps = ref<string[]>([])

const fetchDevices = async () => {
  loading.value = true
  try {
    const res = await request.get('/config/devices')
    devices.value = res as any
  } catch (err: any) {
    ElMessage.error('获取设备列表失败')
  } finally {
    loading.value = false
  }
}

const fetchProjects = async () => {
  try {
    const res = await request.get('/config/projects')
    projects.value = res as any
  } catch (err) {
    console.error('Failed to fetch projects for dropdown')
  }
}

const handleAdd = () => {
  dialogType.value = 'add'
  form.value = { id: '', device_name: '', os: 'Android', model: '', allowed_app: '' }
  selectedApps.value = []
  dialogVisible.value = true
}

const handleEdit = (row: Device) => {
  dialogType.value = 'edit'
  form.value = { ...row }
  selectedApps.value = row.allowed_app ? row.allowed_app.split(',').filter(s => s) : []
  dialogVisible.value = true
}

const handleDelete = (row: Device) => {
  ElMessageBox.confirm(`确定删除设备 ${row.device_name} 吗？`, '警告', {
    type: 'warning'
  }).then(async () => {
    try {
      await request.delete(`/config/devices/${row.id}`)
      ElMessage.success('删除成功')
      fetchDevices()
    } catch (err) {
      ElMessage.error('删除失败')
    }
  })
}

const submitForm = async () => {
  if (!form.value.device_name || !form.value.os) {
    ElMessage.warning('设备名称和操作系统不能为空')
    return
  }
  form.value.allowed_app = selectedApps.value.join(',')
  try {
    await request.post('/config/devices', form.value)
    ElMessage.success(dialogType.value === 'add' ? '添加成功' : '编辑成功')
    dialogVisible.value = false
    fetchDevices()
  } catch (err) {
    ElMessage.error('保存失败')
  }
}

const androidCount = computed(() => devices.value.filter(d => d.os === 'Android').length)
const iosCount = computed(() => devices.value.filter(d => d.os === 'iOS').length)

const filteredDevices = computed(() => {
  const q = searchQuery.value.toLowerCase()
  return devices.value.filter((device) => {
    const matchesOs = osFilter.value === 'all' || device.os === osFilter.value
    const matchesSearch = !q ||
      device.device_name.toLowerCase().includes(q) ||
      device.model.toLowerCase().includes(q) ||
      device.allowed_app.toLowerCase().includes(q)

    return matchesOs && matchesSearch
  })
})

onMounted(() => {
  fetchDevices()
  fetchProjects()
})
</script>

<template>
  <div class="device-config-container">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">设备管理</h1>
          <span class="page-badge">Device Center</span>
        </div>
        <p class="page-desc">管理自动化与真机测试设备池、操作系统版本及关联项目白名单</p>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" class="add-btn" @click="handleAdd">添加设备</el-button>
      </div>
    </div>

    <!-- Quick Stat Cards -->
    <div class="stats-row">
      <div 
        class="stat-card stat-total"
        :class="{ 'stat-active': osFilter === 'all' }"
        @click="osFilter = 'all'"
      >
        <div class="stat-icon stat-icon-blue">
          <el-icon><Monitor /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-label">全部测试设备</span>
          <span class="stat-value">{{ devices.length }}</span>
        </div>
      </div>

      <div 
        class="stat-card stat-android"
        :class="{ 'stat-active': osFilter === 'Android' }"
        @click="osFilter = 'Android'"
      >
        <div class="stat-icon stat-icon-green">
          <el-icon><Platform /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-label">Android 设备</span>
          <span class="stat-value">{{ androidCount }}</span>
        </div>
      </div>

      <div 
        class="stat-card stat-ios"
        :class="{ 'stat-active': osFilter === 'iOS' }"
        @click="osFilter = 'iOS'"
      >
        <div class="stat-icon stat-icon-indigo">
          <el-icon><Apple /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-label">iOS 设备</span>
          <span class="stat-value">{{ iosCount }}</span>
        </div>
      </div>
    </div>

    <!-- Main Content Card -->
    <div class="content-card">
      <div class="toolbar-actions">
        <div class="filter-left">
          <el-input
            v-model="searchQuery"
            placeholder="搜索设备、型号或所属项目..."
            class="search-input"
            :prefix-icon="Search"
            clearable
          />
          <el-radio-group v-model="osFilter" size="default" class="os-quick-filter">
            <el-radio-button value="all">全部系统</el-radio-button>
            <el-radio-button value="Android">Android</el-radio-button>
            <el-radio-button value="iOS">iOS</el-radio-button>
          </el-radio-group>
        </div>

        <div class="stat-badge">
          <span>当前显示 <strong class="count-num">{{ filteredDevices.length }}</strong> 台设备</span>
        </div>
      </div>

      <div class="table-container">
        <el-table :data="filteredDevices" v-loading="loading" style="width: 100%" stripe>
          <el-table-column label="设备信息" min-width="240">
            <template #default="{ row }">
              <div class="device-cell">
                <span 
                  class="os-tag"
                  :class="row.os === 'iOS' ? 'os-tag-ios' : 'os-tag-android'"
                >
                  {{ row.os }}
                </span>
                <span class="device-name">{{ row.device_name }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="model" label="型号/系统版本" width="180">
            <template #default="{ row }">
              <span class="model-badge">{{ row.model || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="允许关联项目" min-width="320">
            <template #default="{ row }">
              <div class="tag-group" v-if="row.allowed_app">
                <el-tag 
                  v-for="app in row.allowed_app.split(',').filter((s: string) => s)" 
                  :key="app"
                  size="small"
                  effect="plain"
                  class="app-tag"
                >
                  {{ app }}
                </el-tag>
              </div>
              <span v-else class="text-placeholder">未限制（全部项目通用）</span>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="录入时间" width="180">
            <template #default="{ row }">
              <span class="date-text">{{ row.created_at || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="160" fixed="right">
            <template #default="{ row }">
              <div class="action-buttons">
                <el-button link type="primary" :icon="Edit" @click="handleEdit(row)">编辑</el-button>
                <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">删除</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- Device Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增设备' : '编辑设备'"
      width="580px"
      destroy-on-close
    >
      <el-form :model="form" label-width="96px" class="device-form">
        <el-form-item label="设备名称" required>
          <el-input v-model="form.device_name" placeholder="请输入设备名称，如 Pixel 7 Pro" />
        </el-form-item>
        <el-form-item label="操作系统" required>
          <el-radio-group v-model="form.os">
            <el-radio value="Android">Android</el-radio>
            <el-radio value="iOS">iOS</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="型号/版本">
          <el-input v-model="form.model" placeholder="如 14.0 或 iPhone 14 Pro" />
        </el-form-item>
        <el-form-item label="允许项目">
          <el-select
            v-model="selectedApps"
            multiple
            filterable
            placeholder="请选择可选项目（留空表示全部通用）"
            style="width: 100%"
          >
            <el-option
              v-for="proj in projects"
              :key="proj.project_code"
              :label="`${proj.project_name} (${proj.project_code})`"
              :value="proj.project_code"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitForm">确定</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.device-config-container {
  padding: 24px;
  max-width: 1680px;
  margin: 0 auto;
}

/* Standard Page Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  letter-spacing: -0.02em;
}

.page-badge {
  font-size: 12px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  padding: 3px 10px;
  border-radius: 9999px;
  letter-spacing: 0.02em;
}

.page-desc {
  margin: 6px 0 0;
  font-size: 13px;
  color: #64748b;
}

.add-btn {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  border: none;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.25);
  transition: all 0.2s ease;
}

.add-btn:hover {
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.35);
  transform: translateY(-1px);
}

/* Quick Stat Cards */
.stats-row {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.stat-card::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
}

.stat-total::before { background: #3b82f6; }
.stat-android::before { background: #10b981; }
.stat-ios::before { background: #6366f1; }

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.06);
}

.stat-card.stat-active {
  border-color: #3b82f6;
  background: linear-gradient(180deg, #ffffff 0%, #eff6ff 100%);
}

.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  flex-shrink: 0;
}

.stat-icon-blue {
  background: #eff6ff;
  color: #2563eb;
}

.stat-icon-green {
  background: #ecfdf5;
  color: #059669;
}

.stat-icon-indigo {
  background: #eef2ff;
  color: #4f46e5;
}

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-label {
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Content Card */
.content-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  padding: 20px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}

.toolbar-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}

.filter-left {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}

.search-input {
  width: 320px;
}

.stat-badge {
  font-size: 13px;
  color: #64748b;
  background: #f8fafc;
  padding: 6px 14px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
}

.count-num {
  color: #2563eb;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Table Elements */
.device-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.os-tag {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 6px;
  letter-spacing: 0.02em;
}

.os-tag-ios {
  background: #eff6ff;
  color: #1d4ed8;
  border: 1px solid #bfdbfe;
}

.os-tag-android {
  background: #ecfdf5;
  color: #047857;
  border: 1px solid #a7f3d0;
}

.device-name {
  font-weight: 600;
  color: #1e293b;
}

.model-badge {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  color: #475569;
  background: #f8fafc;
  padding: 2px 8px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
}

.tag-group {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.app-tag {
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.text-placeholder {
  color: #94a3b8;
  font-size: 13px;
}

.date-text {
  font-size: 13px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.action-buttons {
  display: flex;
  align-items: center;
  gap: 8px;
}

.device-form {
  padding: 10px 0;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 900px) {
  .stats-row {
    grid-template-columns: 1fr;
  }
}
</style>
