<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Edit, Delete, Tickets, Search } from '@element-plus/icons-vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import type { ProcessItem } from './types'
import { buildBackendUrl } from '@/utils/runtimeUrl'

defineOptions({ name: 'TestProcessList' })

const router = useRouter()
const searchQuery = ref('')
const processList = ref<ProcessItem[]>([])

const LIST_STORAGE_KEY = 'test_process_list_v4'

// 加载后台数据
const loadList = async () => {
  try {
    const response = await fetch(buildBackendUrl('/api/processes'))
    if (response.ok) {
      processList.value = await response.json()
    }
  } catch (e) {
    ElMessage.error('无法连接到后台服务')
  }
}

// 迁移逻辑：将本地 localStorage 数据同步到后台
const migrateToBackend = async () => {
  const saved = localStorage.getItem(LIST_STORAGE_KEY)
  if (!saved) return

  try {
    const localList: ProcessItem[] = JSON.parse(saved)
    if (localList.length === 0) return

    // 如果后台目前没数据，则尝试同步
    if (processList.value.length === 0) {
      console.log('Migrating local data to backend...')
      for (const item of localList) {
        await fetch(buildBackendUrl('/api/processes'), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(item)
        })
      }
      // 同步完后重新加载
      await loadList()
      ElMessage.success('已自动同步本地数据到云端')
      // 标记已迁移
      localStorage.setItem(LIST_STORAGE_KEY + '_migrated', saved)
      localStorage.removeItem(LIST_STORAGE_KEY)
    }
  } catch (e) {
    console.error('Migration failed', e)
  }
}

const handleCreate = () => {
  router.push({ name: 'TestProcessEditor' })
}

const handleEdit = (id: string) => {
  router.push({ name: 'TestProcessEditor', params: { id } })
}

const handleDelete = (id: string) => {
  ElMessageBox.confirm('确定要删除这个流程吗？此操作不可逆。', '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    try {
      const response = await fetch(buildBackendUrl(`/api/processes/${id}`), {
        method: 'DELETE'
      })
      if (response.ok) {
        ElMessage.success('删除成功')
        loadList()
      }
    } catch (e) {
      ElMessage.error('删除失败')
    }
  })
}

onMounted(async () => {
  await loadList()
  await migrateToBackend()
})

const filteredList = ref<ProcessItem[]>([])
watch([processList, searchQuery], () => {
  if (!searchQuery.value) {
    filteredList.value = processList.value
  } else {
    filteredList.value = processList.value.filter(item => 
      item.name.toLowerCase().includes(searchQuery.value.toLowerCase())
    )
  }
}, { immediate: true, deep: true })
</script>

<template>
  <div class="test-process-page">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">必测流程仓库</h1>
          <span class="page-badge">Test Processes</span>
        </div>
        <p class="page-desc">标准化业务回归链路与核心关键节点管理，保障版本交付质量</p>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" class="add-btn" @click="handleCreate">新建流程</el-button>
      </div>
    </div>

    <!-- Toolbar / Search Card -->
    <div class="content-card">
      <div class="toolbar-actions">
        <el-input
          v-model="searchQuery"
          placeholder="搜索流程名称..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <div class="stat-badge">
          <span>共 <strong class="count-num">{{ filteredList.length }}</strong> 条业务流程</span>
        </div>
      </div>

      <!-- Process Grid -->
      <div v-if="filteredList.length > 0" class="process-grid">
        <div v-for="item in filteredList" :key="item.id" class="process-card">
          <div class="card-content" @click="handleEdit(item.id)">
            <div class="card-header">
              <h3 class="process-name">{{ item.name }}</h3>
              <div class="node-pill">
                <el-icon class="mr-1"><Tickets /></el-icon>
                <span>{{ item.data?.children?.length || 0 }} 个主节点</span>
              </div>
            </div>
            <div class="card-body">
              <p class="process-desc">最后更新: {{ item.updatedAt }}</p>
            </div>
          </div>
          <div class="card-actions">
            <el-button link type="primary" :icon="Edit" @click="handleEdit(item.id)">编辑流程</el-button>
            <el-button link type="danger" :icon="Delete" @click="handleDelete(item.id)">删除</el-button>
          </div>
        </div>
      </div>

      <!-- Empty State -->
      <div v-else class="empty-state">
        <el-empty :description="searchQuery ? '未找到匹配的流程' : '暂无流程，开启你的第一个测试流程吧'">
          <el-button v-if="!searchQuery" type="primary" @click="handleCreate">立即新建</el-button>
        </el-empty>
      </div>
    </div>
  </div>
</template>

<style scoped>
.test-process-page {
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
  margin-bottom: 20px;
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

/* Grid & Cards */
.process-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.process-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  overflow: hidden;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
  position: relative;
}

.process-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: linear-gradient(90deg, #3b82f6, #60a5fa);
  opacity: 0;
  transition: opacity 0.2s ease;
}

.process-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.08);
  border-color: #93c5fd;
}

.process-card:hover::before {
  opacity: 1;
}

.card-content {
  padding: 20px;
  cursor: pointer;
  flex: 1;
}

.card-header {
  margin-bottom: 12px;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.process-name {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  line-height: 1.4;
}

.node-pill {
  display: inline-flex;
  align-items: center;
  font-size: 11px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  padding: 2px 8px;
  border-radius: 6px;
  white-space: nowrap;
}

.card-body {
  margin-top: 8px;
}

.process-desc {
  font-size: 12px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  margin: 0;
}

.card-actions {
  padding: 10px 20px;
  background: #f8fafc;
  border-top: 1px solid #f1f5f9;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.empty-state {
  padding: 60px 0;
}

.mr-1 {
  margin-right: 4px;
}
</style>
