<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Search } from '@element-plus/icons-vue'
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
    <div class="header-actions">
      <el-input
        v-model="searchQuery"
        placeholder="搜索设备、型号或所属项目"
        class="search-input"
        :prefix-icon="Search"
        clearable
      />
      <el-button type="primary" :icon="Plus" @click="handleAdd">添加设备</el-button>
    </div>

    <el-table :data="filteredDevices" v-loading="loading" style="width: 100%" border stripe>
      <el-table-column min-width="240">
        <template #header>
          <div class="device-info-header">
            <span>设备信息</span>
            <el-radio-group v-model="osFilter" size="small" class="os-quick-filter">
              <el-radio-button value="all">全部</el-radio-button>
              <el-radio-button value="iOS">iOS</el-radio-button>
              <el-radio-button value="Android">Android</el-radio-button>
            </el-radio-group>
          </div>
        </template>
        <template #default="{ row }">
          <div class="device-info">
            <el-tag :type="row.os === 'iOS' ? 'primary' : 'success'" size="small" class="mr-2">{{ row.os }}</el-tag>
            <span class="device-name">{{ row.device_name }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="model" label="型号/系统版本" width="150" />
      <el-table-column label="允许项目" min-width="300">
        <template #default="{ row }">
          <div class="tag-group">
            <el-tag 
              v-for="app in (row.allowed_app ? row.allowed_app.split(',') : [])" 
              :key="app"
              size="small"
              effect="plain"
              class="m-1"
            >
              {{ app }}
            </el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="200" />
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :icon="Edit" @click="handleEdit(row)">编辑</el-button>
          <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Device Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增设备' : '编辑设备'"
      width="600px"
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="设备名称" required>
          <el-input v-model="form.device_name" placeholder="请输入设备名称" />
        </el-form-item>
        <el-form-item label="操作系统" required>
          <el-radio-group v-model="form.os">
            <el-radio value="Android">Android</el-radio>
            <el-radio value="iOS">iOS</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="型号/版本">
          <el-input v-model="form.model" placeholder="如 18.5 或 13" />
        </el-form-item>
        <el-form-item label="允许项目">
          <el-select
            v-model="selectedApps"
            multiple
            filterable
            placeholder="请选择可选项目"
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
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitForm">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.device-config-container {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px 0 rgba(0,0,0,0.05);
}

.header-actions {
  display: flex;
  justify-content: space-between;
  margin-bottom: 20px;
}

.search-input {
  width: 300px;
}

.device-info {
  display: flex;
  align-items: center;
}

.device-info-header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.os-quick-filter {
  flex-shrink: 0;
}

.device-name {
  font-weight: 500;
  color: #1e293b;
}

.tag-group {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.mr-2 { margin-right: 8px; }
.m-1 { margin: 2px; }
</style>
