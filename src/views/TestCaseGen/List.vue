<template>
  <div class="history-container">
    <div class="header-section">
      <div class="title-area">
        <el-icon class="title-icon"><Collection /></el-icon>
        <div class="title-text">
          <h1>用例生成历史</h1>
          <p>查看并管理过往 AI 智能生成的测试用例记录</p>
        </div>
      </div>
      <el-button type="primary" size="large" @click="goToNew" :icon="Plus" class="create-btn">
        智能生成新用例
      </el-button>
    </div>

    <el-card class="list-card" shadow="never">
      <el-table 
        v-loading="loading" 
        :data="records" 
        style="width: 100%"
        :header-cell-style="{ background: '#f8fafc', fontWeight: 'bold', color: '#475569' }"
      >
        <el-table-column label="生成时间" width="200">
          <template #default="{ row }">
            <span class="time-text">{{ formatTime(row.created_at) }}</span>
          </template>
        </el-table-column>
        
        <el-table-column label="需求标题" min-width="200">
          <template #default="{ row }">
            <div class="title-cell">
              <span class="record-title">{{ row.title || '未命名需求' }}</span>
              <el-tag size="small" type="info" class="count-tag">{{ row.case_count }} 条用例</el-tag>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="requirement_summary" label="需求核心摘要" min-width="300" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="summary-text">{{ truncateSummary(row.requirement_text) }}</span>
          </template>
        </el-table-column>

        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button-group>
              <el-button type="primary" link @click="viewRecord(row)" :icon="View">
                查看
              </el-button>
              <el-button type="success" link @click="downloadAgain(row)" :icon="Download">
                下载 Excel
              </el-button>
              <el-popconfirm title="确定删除这条历史记录吗？" @confirm="deleteRecord(row.id)">
                <template #reference>
                  <el-button type="danger" link :icon="Delete">
                    删除
                  </el-button>
                </template>
              </el-popconfirm>
            </el-button-group>
          </template>
        </el-table-column>

        <template #empty>
          <el-empty description="暂无历史记录，快去生成第一个用例吧！">
            <el-button type="primary" @click="goToNew">立即尝试</el-button>
          </el-empty>
        </template>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Plus, Collection, View, Download, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import axios from 'axios'
import dayjs from 'dayjs'

const router = useRouter()
const API_BASE = '/api/testcase-gen'
const loading = ref(false)
const records = ref([])

const fetchRecords = async () => {
  loading.value = true
  try {
    const res = await axios.get(`${API_BASE}/records`)
    records.value = res.data
  } catch (error) {
    console.error('Failed to fetch records:', error)
    ElMessage.error('获取历史记录失败')
  } finally {
    loading.value = false
  }
}

const formatTime = (timeStr: string) => {
  return dayjs(timeStr).format('YYYY-MM-DD HH:mm:ss')
}

const truncateSummary = (text: string) => {
  if (!text) return '-'
  return text.length > 100 ? text.substring(0, 100) + '...' : text
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
    
    // 调用现有的导出接口
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
    ElMessage.success('正在打开下载...')
  } catch (error) {
    console.error('Download failed:', error)
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
})
</script>

<style scoped>
.history-container {
  padding: 24px;
}

.header-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.title-area {
  display: flex;
  align-items: center;
  gap: 16px;
}

.title-icon {
  font-size: 40px;
  color: #4f46e5;
  background: #f5f3ff;
  padding: 12px;
  border-radius: 12px;
}

.title-text h1 {
  font-size: 24px;
  margin: 0;
  color: #1e293b;
  font-weight: 700;
}

.title-text p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 14px;
}

.create-btn {
  height: 48px;
  padding: 0 24px;
  font-size: 16px;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.2);
}

.list-card {
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.title-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.record-title {
  font-weight: 600;
  color: #1e293b;
}

.count-tag {
  font-weight: 500;
}

.time-text {
  color: #64748b;
  font-family: monospace;
}

.summary-text {
  color: #475569;
  font-size: 13px;
}

:deep(.el-button--link) {
  padding: 4px 8px;
}
</style>
