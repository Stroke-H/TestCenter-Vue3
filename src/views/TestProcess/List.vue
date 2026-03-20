<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Edit, Delete, Tickets, Search } from '@element-plus/icons-vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import type { ProcessItem } from './types'

defineOptions({ name: 'TestProcessList' })

const router = useRouter()
const searchQuery = ref('')
const processList = ref<ProcessItem[]>([])

// 协助定位后端地址
const getBackendHost = () => {
  return `${window.location.protocol}//${window.location.hostname}:8080`
}

const LIST_STORAGE_KEY = 'test_process_list_v4'

// 加载后台数据
const loadList = async () => {
  try {
    const response = await fetch(`${getBackendHost()}/api/processes`)
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
        await fetch(`${getBackendHost()}/api/processes`, {
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
      const response = await fetch(`${getBackendHost()}/api/processes/${id}`, {
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
  <div class="list-container">
    <!-- 顶部操作栏 -->
    <div class="list-header">
      <div class="header-left">
        <el-icon class="title-icon"><Tickets /></el-icon>
        <h2 class="title">必测流程仓库</h2>
        <span class="count-badge">{{ processList.length }}</span>
      </div>
      <div class="header-right">
        <el-input
          v-model="searchQuery"
          placeholder="搜索流程名称..."
          :prefix-icon="Search"
          class="search-input"
          clearable
        />
        <el-button type="primary" :icon="Plus" @click="handleCreate">
          新建流程
        </el-button>
      </div>
    </div>

    <!-- 列表区域 -->
    <div v-if="filteredList.length > 0" class="process-grid">
      <div v-for="item in filteredList" :key="item.id" class="process-card">
        <div class="card-content" @click="handleEdit(item.id)">
          <div class="card-header">
            <h3 class="process-name">{{ item.name }}</h3>
            <div class="status-summary">
              <span class="node-count">{{ item.data.children?.length || 0 }} 个主节点</span>
            </div>
          </div>
          <div class="card-body">
            <p class="process-desc">最后更新: {{ item.updatedAt }}</p>
          </div>
        </div>
        <div class="card-actions">
          <el-button link type="primary" :icon="Edit" @click="handleEdit(item.id)">编辑</el-button>
          <el-button link type="danger" :icon="Delete" @click="handleDelete(item.id)">删除</el-button>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else class="empty-state">
      <el-empty :description="searchQuery ? '未找到匹配的流程' : '暂无流程，开启你的第一个测试流程吧'">
        <el-button v-if="!searchQuery" type="primary" @click="handleCreate">立即新建</el-button>
      </el-empty>
    </div>
  </div>
</template>

<style scoped>
.list-container {
  padding: 32px;
  background: #f8fafc;
  min-height: calc(100vh - 56px);
  font-family: 'Inter', -apple-system, sans-serif;
}

.list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-icon {
  font-size: 24px;
  color: #3b82f6;
}

.title {
  font-size: 24px;
  font-weight: 800;
  color: #0f172a;
  margin: 0;
}

.count-badge {
  background: #e2e8f0;
  color: #64748b;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 700;
}

.header-right {
  display: flex;
  gap: 16px;
}

.search-input {
  width: 280px;
}

.process-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 24px;
}

.process-card {
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
}

.process-card:hover {
  transform: translateY(-8px);
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
  border-color: #3b82f6;
}

.card-content {
  padding: 24px;
  cursor: pointer;
  flex: 1;
}

.card-header {
  margin-bottom: 12px;
}

.process-name {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
  margin: 0 0 8px 0;
}

.status-summary {
  font-size: 12px;
  color: #64748b;
  font-weight: 600;
}

.process-desc {
  font-size: 13px;
  color: #94a3b8;
  margin: 0;
}

.card-actions {
  padding: 12px 24px;
  background: #f8fafc;
  border-top: 1px solid #e2e8f0;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.empty-state {
  margin-top: 100px;
}
</style>
