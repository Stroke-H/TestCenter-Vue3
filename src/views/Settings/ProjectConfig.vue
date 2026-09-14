<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Search, Document } from '@element-plus/icons-vue'
import request from '@/api/request'

interface Project {
  id: string
  project_code: string
  project_name: string
  short_code: string
  wiki_url?: string
  workspace?: string
  created_at?: string
}

const projects = ref<Project[]>([])
const loading = ref(false)
const searchQuery = ref('')
const dialogVisible = ref(false)
const dialogType = ref<'add' | 'edit'>('add')
const form = ref<Project>({
  id: '',
  project_code: '',
  project_name: '',
  short_code: '',
  wiki_url: '',
  workspace: ''
})

const fetchProjects = async () => {
  loading.value = true
  try {
    const res = await request.get('/config/projects')
    projects.value = res as any
  } catch (err: any) {
    ElMessage.error('获取项目列表失败')
  } finally {
    loading.value = false
  }
}

const handleAdd = () => {
  dialogType.value = 'add'
  form.value = { id: '', project_code: '', project_name: '', short_code: '', wiki_url: '', workspace: '' }
  dialogVisible.value = true
}

const handleEdit = (row: Project) => {
  dialogType.value = 'edit'
  form.value = { ...row }
  dialogVisible.value = true
}

const handleDelete = (row: Project) => {
  ElMessageBox.confirm(`确定删除项目 ${row.project_name} (${row.project_code}) 吗？`, '警告', {
    type: 'warning'
  }).then(async () => {
    try {
      await request.delete(`/config/projects/${row.id}`)
      ElMessage.success('删除成功')
      fetchProjects()
    } catch (err: any) {
      ElMessage.error('删除失败')
    }
  })
}

const submitForm = async () => {
  if (!form.value.project_code || !form.value.project_name) {
    ElMessage.warning('项目代号和名称不能为空')
    return
  }
  try {
    await request.post('/config/projects', form.value)
    ElMessage.success(dialogType.value === 'add' ? '添加成功' : '编辑成功')
    dialogVisible.value = false
    fetchProjects()
  } catch (err: any) {
    ElMessage.error('保存失败')
  }
}

const filteredProjects = computed(() => {
  if (!searchQuery.value) return projects.value
  return projects.value.filter(p => 
    p.project_code.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
    p.project_name.toLowerCase().includes(searchQuery.value.toLowerCase())
  )
})

onMounted(fetchProjects)
</script>

<template>
  <div class="project-config-container">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">项目配置</h1>
          <span class="page-badge">Project Settings</span>
        </div>
        <p class="page-desc">维护平台项目代号、英文缩写及飞书 Wiki 关联信息，规范测试归属与空间分布</p>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" class="add-btn" @click="handleAdd">添加项目</el-button>
      </div>
    </div>

    <!-- Main Content Card -->
    <div class="content-card">
      <div class="toolbar-actions">
        <el-input
          v-model="searchQuery"
          placeholder="搜索项目代码或名称..."
          class="search-input"
          :prefix-icon="Search"
          clearable
        />
        <div class="stat-badge">
          <span>共 <strong class="count-num">{{ filteredProjects.length }}</strong> 个项目</span>
        </div>
      </div>

      <div class="table-container">
        <el-table :data="filteredProjects" v-loading="loading" style="width: 100%" stripe>
          <el-table-column prop="project_code" label="项目代码" width="160">
            <template #default="{ row }">
              <span class="code-badge">{{ row.project_code }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="project_name" label="项目名称" min-width="180">
            <template #default="{ row }">
              <span class="project-name">{{ row.project_name }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="short_code" label="项目缩写" width="120">
            <template #default="{ row }">
              <el-tag v-if="row.short_code" size="small" effect="plain" class="short-code-tag">{{ row.short_code }}</el-tag>
              <span v-else class="text-placeholder">-</span>
            </template>
          </el-table-column>
          <el-table-column prop="workspace" label="所属空间" width="150">
            <template #default="{ row }">
              <el-tag v-if="row.workspace" size="small" type="info" effect="light" class="workspace-tag">{{ row.workspace }}</el-tag>
              <span v-else class="text-placeholder">-</span>
            </template>
          </el-table-column>
          <el-table-column prop="wiki_url" label="项目文档关联" min-width="220">
            <template #default="{ row }">
              <el-link v-if="row.wiki_url" type="primary" :href="row.wiki_url" target="_blank" class="wiki-link">
                <el-icon class="mr-1"><Document /></el-icon>
                <span>查看项目文档</span>
              </el-link>
              <span v-else class="text-placeholder">未关联</span>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="180">
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

    <!-- Project Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增项目' : '编辑项目'"
      width="520px"
      destroy-on-close
    >
      <el-form :model="form" label-width="96px" class="project-form">
        <el-form-item label="项目代码" required>
          <el-input v-model="form.project_code" placeholder="如 A1106" :disabled="dialogType === 'edit'" />
        </el-form-item>
        <el-form-item label="项目名称" required>
          <el-input v-model="form.project_name" placeholder="请输入项目全名" />
        </el-form-item>
        <el-form-item label="项目缩写">
          <el-input v-model="form.short_code" placeholder="例如: swa, swi" />
        </el-form-item>
        <el-form-item label="所属空间">
          <el-select v-model="form.workspace" placeholder="请选择所属空间" clearable style="width: 100%;">
            <el-option label="海外短剧" value="海外短剧" />
            <el-option label="免费短剧" value="免费短剧" />
            <el-option label="iOS订阅产品" value="iOS订阅产品" />
            <el-option label="番茄短剧" value="番茄短剧" />
          </el-select>
        </el-form-item>
        <el-form-item label="项目文档">
          <el-input v-model="form.wiki_url" placeholder="请输入飞书文档或 Wiki 链接" />
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
.project-config-container {
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
.code-badge {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
  background: #f1f5f9;
  padding: 3px 8px;
  border-radius: 6px;
  border: 1px solid #e2e8f0;
}

.project-name {
  font-weight: 600;
  color: #1e293b;
}

.short-code-tag {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-weight: 600;
  color: #475569;
}

.workspace-tag {
  border-radius: 6px;
}

.wiki-link {
  font-size: 13px;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 4px;
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

.project-form {
  padding: 10px 0;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.mr-1 {
  margin-right: 4px;
}
</style>
