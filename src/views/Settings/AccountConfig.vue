<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Edit, Refresh, Search, User } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import request from '@/api/request'

interface Account {
  id: string
  username: string
  nickname: string
  email: string
  created_at?: string
  feishu_open_id?: string
}

const loading = ref(false)
const searchQuery = ref('')
const accounts = ref<Account[]>([])
const dialogVisible = ref(false)
const form = ref<Account>({
  id: '',
  username: '',
  nickname: '',
  email: '',
  created_at: '',
  feishu_open_id: ''
})

const fetchAccounts = async () => {
  loading.value = true
  try {
    const res = await request.get('/config/accounts')
    accounts.value = Array.isArray(res) ? res : []
  } catch (err) {
    console.error('Failed to fetch accounts', err)
    ElMessage.error('获取账号列表失败')
  } finally {
    loading.value = false
  }
}

const handleEdit = (row: Account) => {
  form.value = { ...row }
  dialogVisible.value = true
}

const submitForm = async () => {
  if (!form.value.id) {
    ElMessage.warning('缺少账号标识，暂时无法保存')
    return
  }

  try {
    await request.post('/config/accounts', {
      id: form.value.id,
      nickname: form.value.nickname,
      email: form.value.email
    })
    ElMessage.success('账号信息已更新')
    dialogVisible.value = false
    fetchAccounts()
  } catch (err) {
    console.error('Failed to save account', err)
    ElMessage.error('保存账号失败')
  }
}

const filteredAccounts = computed(() => {
  if (!searchQuery.value) return accounts.value
  const keyword = searchQuery.value.toLowerCase()
  return accounts.value.filter((item) => {
    return item.username.toLowerCase().includes(keyword) ||
      (item.nickname || '').toLowerCase().includes(keyword) ||
      (item.email || '').toLowerCase().includes(keyword) ||
      (item.id || '').toLowerCase().includes(keyword)
  })
})

const boundCount = computed(() => {
  return accounts.value.filter(item => !!item.feishu_open_id).length
})

onMounted(fetchAccounts)
</script>

<template>
  <div class="account-config-container">
    <div class="summary-row">
      <div class="summary-card">
        <div class="summary-card__icon summary-card__icon--blue">
          <el-icon><User /></el-icon>
        </div>
        <div>
          <div class="summary-card__label">可登录账号</div>
          <div class="summary-card__value">{{ accounts.length }}</div>
        </div>
      </div>
      <div class="summary-card">
        <div class="summary-card__icon summary-card__icon--green">
          <el-icon><Refresh /></el-icon>
        </div>
        <div>
          <div class="summary-card__label">飞书已绑定</div>
          <div class="summary-card__value">{{ boundCount }}</div>
        </div>
      </div>
    </div>

    <div class="header-actions">
      <el-input
        v-model="searchQuery"
        placeholder="搜索用户名、昵称、邮箱或账号 ID"
        class="search-input"
        :prefix-icon="Search"
        clearable
      />
      <el-button :icon="Refresh" @click="fetchAccounts">刷新列表</el-button>
    </div>

    <el-table :data="filteredAccounts" v-loading="loading" style="width: 100%" border stripe>
      <el-table-column prop="username" label="用户名" min-width="160" />
      <el-table-column prop="nickname" label="昵称" min-width="140">
        <template #default="{ row }">
          <span>{{ row.nickname || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="email" label="邮箱" min-width="220">
        <template #default="{ row }">
          <span>{{ row.email || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="飞书绑定" width="120">
        <template #default="{ row }">
          <el-tag :type="row.feishu_open_id ? 'success' : 'info'" effect="plain">
            {{ row.feishu_open_id ? '已绑定' : '未绑定' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" min-width="200">
        <template #default="{ row }">
          <span>{{ row.created_at || '-' }}</span>
        </template>
      </el-table-column>
      <el-table-column label="管理" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :icon="Edit" @click="handleEdit(row)">编辑</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" title="账号管理" width="520px">
      <el-form :model="form" label-width="90px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" disabled />
        </el-form-item>
        <el-form-item label="昵称">
          <el-input v-model="form.nickname" placeholder="可选，便于平台内识别" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="可选，用于统一登录识别" />
        </el-form-item>
        <el-form-item label="账号 ID">
          <el-input v-model="form.id" disabled />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitForm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.account-config-container {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.05);
}

.summary-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.summary-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fafc 100%);
}

.summary-card__icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}

.summary-card__icon--blue {
  color: #2563eb;
  background: rgba(37, 99, 235, 0.12);
}

.summary-card__icon--green {
  color: #059669;
  background: rgba(5, 150, 105, 0.12);
}

.summary-card__label {
  font-size: 13px;
  color: #64748b;
  margin-bottom: 4px;
}

.summary-card__value {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
}

.header-actions {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}

.search-input {
  width: 340px;
}
</style>
