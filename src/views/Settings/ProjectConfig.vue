<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Search } from '@element-plus/icons-vue'
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
    <div class="header-actions">
      <el-input
        v-model="searchQuery"
        placeholder="搜索项目代码或名称"
        class="search-input"
        :prefix-icon="Search"
        clearable
      />
      <el-button type="primary" :icon="Plus" @click="handleAdd">添加项目</el-button>
    </div>

    <el-table :data="filteredProjects" v-loading="loading" style="width: 100%" border stripe>
      <el-table-column prop="project_code" label="项目代码" width="150" />
      <el-table-column prop="project_name" label="项目名称" min-width="150" />
      <el-table-column prop="short_code" label="项目缩写" width="100" />
      <el-table-column prop="workspace" label="所属空间" width="120" />
      <el-table-column prop="wiki_url" label="项目文档关联" min-width="200">
        <template #default="{ row }">
          <el-link v-if="row.wiki_url" type="primary" :href="row.wiki_url" target="_blank" style="font-size: 13px;">
            点击查看文档
          </el-link>
          <span v-else style="color: #999; font-size: 13px;">未关联</span>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180">
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :icon="Edit" @click="handleEdit(row)">编辑</el-button>
          <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Project Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogType === 'add' ? '新增项目' : '编辑项目'"
      width="500px"
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="项目代码" required>
          <el-input v-model="form.project_code" placeholder="如 A1106" :disabled="dialogType === 'edit'" />
        </el-form-item>
        <el-form-item label="项目名称">
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
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitForm">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.project-config-container {
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
</style>
