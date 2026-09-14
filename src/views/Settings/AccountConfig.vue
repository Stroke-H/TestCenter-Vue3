<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Edit, Refresh, Search, User, Check, Warning } from '@element-plus/icons-vue'
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

const unboundCount = computed(() => {
  return Math.max(0, accounts.value.length - boundCount.value)
})

const getAvatarChar = (username: string) => {
  if (!username) return 'U'
  return username.charAt(0).toUpperCase()
}

onMounted(fetchAccounts)
</script>

<template>
  <div class="account-config-container">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">账号配置</h1>
          <span class="page-badge">Account Directory</span>
        </div>
        <p class="page-desc">查看并维护系统统一认证账号、昵称、工作邮箱及飞书企业绑定状态</p>
      </div>
      <div class="header-right">
        <el-button :icon="Refresh" class="refresh-btn" @click="fetchAccounts">刷新列表</el-button>
      </div>
    </div>

    <!-- Summary Stat Cards -->
    <div class="stats-row">
      <div class="stat-card stat-total">
        <div class="stat-icon stat-icon-blue">
          <el-icon><User /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-label">可登录账号</span>
          <span class="stat-value">{{ accounts.length }}</span>
        </div>
      </div>

      <div class="stat-card stat-bound">
        <div class="stat-icon stat-icon-green">
          <el-icon><Check /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-label">飞书已绑定</span>
          <span class="stat-value">{{ boundCount }}</span>
        </div>
      </div>

      <div class="stat-card stat-unbound">
        <div class="stat-icon stat-icon-amber">
          <el-icon><Warning /></el-icon>
        </div>
        <div class="stat-info">
          <span class="stat-label">待绑定飞书</span>
          <span class="stat-value">{{ unboundCount }}</span>
        </div>
      </div>
    </div>

    <!-- Main Content Card -->
    <div class="content-card">
      <div class="toolbar-actions">
        <el-input
          v-model="searchQuery"
          placeholder="搜索用户名、昵称、邮箱或账号 ID..."
          class="search-input"
          :prefix-icon="Search"
          clearable
        />
        <div class="stat-badge">
          <span>共 <strong class="count-num">{{ filteredAccounts.length }}</strong> 位用户</span>
        </div>
      </div>

      <div class="table-container">
        <el-table :data="filteredAccounts" v-loading="loading" style="width: 100%" stripe>
          <el-table-column label="用户标识" min-width="200">
            <template #default="{ row }">
              <div class="user-profile-cell">
                <div class="user-avatar">
                  {{ getAvatarChar(row.username) }}
                </div>
                <div class="user-names">
                  <span class="username-text">{{ row.username }}</span>
                  <span class="user-id-sub">ID: {{ row.id.substring(0, 8) }}...</span>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="nickname" label="昵称" min-width="160">
            <template #default="{ row }">
              <span class="nickname-text">{{ row.nickname || '-' }}</span>
            </template>
          </el-table-column>

          <el-table-column prop="email" label="工作邮箱" min-width="240">
            <template #default="{ row }">
              <span class="email-text">{{ row.email || '-' }}</span>
            </template>
          </el-table-column>

          <el-table-column label="飞书绑定状态" width="160">
            <template #default="{ row }">
              <span 
                class="status-pill"
                :class="row.feishu_open_id ? 'status-pill-bound' : 'status-pill-unbound'"
              >
                <span class="status-dot"></span>
                {{ row.feishu_open_id ? '已绑定飞书' : '未绑定' }}
              </span>
            </template>
          </el-table-column>

          <el-table-column prop="created_at" label="创建时间" min-width="180">
            <template #default="{ row }">
              <span class="date-text">{{ row.created_at || '-' }}</span>
            </template>
          </el-table-column>

          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button link type="primary" :icon="Edit" @click="handleEdit(row)">编辑信息</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- Account Dialog -->
    <el-dialog 
      v-model="dialogVisible" 
      title="编辑账号信息" 
      width="520px"
      destroy-on-close
    >
      <el-form :model="form" label-width="90px" class="account-form">
        <el-form-item label="用户名">
          <el-input v-model="form.username" disabled />
        </el-form-item>
        <el-form-item label="用户昵称">
          <el-input v-model="form.nickname" placeholder="可选，便于平台内展示识别" />
        </el-form-item>
        <el-form-item label="工作邮箱">
          <el-input v-model="form.email" placeholder="可选，用于统一身份与通知" />
        </el-form-item>
        <el-form-item label="账号 ID">
          <el-input v-model="form.id" disabled />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitForm">保存修改</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.account-config-container {
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

.refresh-btn {
  border-radius: 8px;
  font-weight: 500;
}

/* Stats Row */
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
.stat-bound::before { background: #10b981; }
.stat-unbound::before { background: #f59e0b; }

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.06);
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

.stat-icon-amber {
  background: #fffbeb;
  color: #d97706;
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
}

.search-input {
  width: 340px;
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

/* User Profile Cell */
.user-profile-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 700;
  box-shadow: 0 2px 6px rgba(37, 99, 235, 0.25);
  flex-shrink: 0;
}

.user-names {
  display: flex;
  flex-direction: column;
}

.username-text {
  font-weight: 600;
  color: #0f172a;
  font-size: 14px;
}

.user-id-sub {
  font-size: 11px;
  color: #94a3b8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.nickname-text {
  font-weight: 500;
  color: #334155;
}

.email-text {
  font-size: 13px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Status Pill */
.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 9999px;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.status-pill-bound {
  background: #ecfdf5;
  color: #059669;
  border: 1px solid #a7f3d0;
}

.status-pill-bound .status-dot {
  background: #10b981;
}

.status-pill-unbound {
  background: #f8fafc;
  color: #64748b;
  border: 1px solid #e2e8f0;
}

.status-pill-unbound .status-dot {
  background: #94a3b8;
}

.date-text {
  font-size: 13px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.account-form {
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
