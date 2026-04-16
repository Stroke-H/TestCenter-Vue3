<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import axios from 'axios'
import { ElMessage, ElMessageBox } from 'element-plus'

defineOptions({ name: 'SandboxAccounts' })

interface ProjectItem {
  id?: string
  project_code: string
  project_name: string
}

interface TestAccountItem {
  id: string
  account_type: 'sandbox' | 'dataqa'
  account: string
  password: string
  project_code: string
}

interface AccountGroup {
  title: string
  projectCode: string
  items: TestAccountItem[]
}

const ACCOUNT_TYPE_OPTIONS = [
  { label: '全部账号', value: 'all' },
  { label: '沙盒账号', value: 'sandbox' },
  { label: '数说测试账号', value: 'dataqa' }
] as const

const projects = ref<ProjectItem[]>([])
const testAccounts = ref<TestAccountItem[]>([])
const showCreateDialog = ref(false)
const editingProjectCode = ref('')
const activeType = ref<'all' | 'sandbox' | 'dataqa'>('all')
const activeProjectFilter = ref('all')

const showEditDialog = ref(false)
const editForm = ref({
  id: '',
  account_type: 'sandbox' as 'sandbox' | 'dataqa',
  account: '',
  password: '',
  project_code: ''
})

const newAccountForm = ref({
  account_type: 'sandbox' as 'sandbox' | 'dataqa',
  account: '',
  password: '',
  project_code: ''
})

const filteredAccounts = computed(() => {
  return testAccounts.value.filter((item) => {
    const matchType = activeType.value === 'all' || item.account_type === activeType.value
    const matchProject = activeProjectFilter.value === 'all' || item.project_code === activeProjectFilter.value
    return matchType && matchProject
  })
})

const accountStats = computed(() => {
  const projectCodes = new Set(testAccounts.value.map((item) => item.project_code).filter(Boolean))
  const sandboxCount = testAccounts.value.filter((item) => item.account_type === 'sandbox').length
  const dataqaCount = testAccounts.value.filter((item) => item.account_type === 'dataqa').length

  return [
    { title: '总账号数', value: testAccounts.value.length, tone: 'blue' },
    { title: '沙盒账号', value: sandboxCount, tone: 'green' },
    { title: '数说账号', value: dataqaCount, tone: 'amber' },
    { title: '已分配项目数', value: projectCodes.size, tone: 'slate' }
  ]
})

const projectNavItems = computed(() => {
  const counts = new Map<string, number>()
  filteredAccounts.value.forEach((item) => {
    const key = item.project_code || 'unassigned'
    counts.set(key, (counts.get(key) || 0) + 1)
  })

  const items = [
    {
      key: 'all',
      label: '全部',
      count: filteredAccounts.value.length
    }
  ]

  Array.from(counts.entries())
    .sort((a, b) => a[0].localeCompare(b[0]))
    .forEach(([projectCode, count]) => {
      const matchedProject = projects.value.find((project) => project.project_code === projectCode)
      items.push({
        key: projectCode,
        label: matchedProject?.project_name || (projectCode === 'unassigned' ? '未分配' : projectCode),
        count
      })
    })

  return items
})

const groupedAccounts = computed<AccountGroup[]>(() => {
  const groupMap = new Map<string, AccountGroup>()

  filteredAccounts.value.forEach((item) => {
    const matchedProject = projects.value.find((project) => project.project_code === item.project_code)
    const title = matchedProject?.project_name || '未分配项目'
    const projectCode = matchedProject?.project_code || item.project_code || 'unassigned'
    const key = projectCode || 'unassigned'

    if (!groupMap.has(key)) {
      groupMap.set(key, {
        title,
        projectCode,
        items: []
      })
    }

    groupMap.get(key)?.items.push(item)
  })

  return Array.from(groupMap.values())
})

const getProjectName = (projectCode: string) => {
  return projects.value.find((project) => project.project_code === projectCode)?.project_name || projectCode
}

const getTypeBadgeLabel = (type: TestAccountItem['account_type']) => {
  return type === 'dataqa' ? 'DataQA' : 'Sandbox'
}

const getTypeBadgeClass = (type: TestAccountItem['account_type']) => {
  return type === 'dataqa' ? 'account-badge account-badge--dataqa' : 'account-badge account-badge--sandbox'
}

const getPrimaryFieldLabel = (type: TestAccountItem['account_type']) => {
  return type === 'dataqa' ? '设备' : '账号'
}

const getSecondaryFieldLabel = (type: TestAccountItem['account_type']) => {
  return type === 'dataqa' ? '标识' : '密码'
}

const getAccountPlaceholder = (type: 'sandbox' | 'dataqa') => {
  return type === 'dataqa' ? '请输入设备名称' : '请输入账号'
}

const getPasswordPlaceholder = (type: 'sandbox' | 'dataqa') => {
  return type === 'dataqa' ? '请输入设备标识' : '请输入密码'
}

const resetCreateForm = () => {
  newAccountForm.value = {
    account_type: activeType.value === 'dataqa' ? 'dataqa' : 'sandbox',
    account: '',
    password: '',
    project_code: ''
  }
}

const resetEditForm = () => {
  editForm.value = {
    id: '',
    account_type: 'sandbox',
    account: '',
    password: '',
    project_code: ''
  }
}

const fetchProjects = async () => {
  try {
    const res = await axios.get('/api/config/projects')
    projects.value = res.data || []
  } catch (err) {
    console.error('Failed to fetch projects:', err)
    ElMessage.error('项目数据加载失败')
  }
}

const fetchAccounts = async () => {
  try {
    const res = await axios.get('/api/config/sandbox-accounts')
    testAccounts.value = (res.data || []).map((item: TestAccountItem) => ({
      ...item,
      account_type: item.account_type || 'sandbox'
    }))
  } catch (err) {
    console.error('Failed to fetch test accounts:', err)
    ElMessage.error('测试账号加载失败')
  }
}

const handleCreateAccount = async () => {
  const payload = {
    account_type: newAccountForm.value.account_type,
    account: newAccountForm.value.account.trim(),
    password: newAccountForm.value.password.trim(),
    project_code: newAccountForm.value.project_code
  }

  if (!payload.account) {
    ElMessage.warning('请输入测试账号')
    return
  }
  if (!payload.password) {
    ElMessage.warning('请输入密码')
    return
  }
  if (!payload.project_code) {
    ElMessage.warning('请选择项目')
    return
  }

  try {
    const res = await axios.post('/api/config/sandbox-accounts', payload)
    testAccounts.value.push(res.data)
    showCreateDialog.value = false
    resetCreateForm()
    ElMessage.success('测试账号已新增')
  } catch (err) {
    console.error('Failed to create test account:', err)
    ElMessage.error('测试账号新增失败')
  }
}

const toggleEditProject = (projectCode: string) => {
  editingProjectCode.value = editingProjectCode.value === projectCode ? '' : projectCode
}

const handleDeleteAccount = async (accountId: string) => {
  const target = testAccounts.value.find((item) => item.id === accountId)
  if (!target) return

  try {
    await ElMessageBox.confirm(`确定删除账号 ${target.account} 吗？`, '删除测试账号', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })

    await axios.delete(`/api/config/sandbox-accounts/${accountId}`)
    testAccounts.value = testAccounts.value.filter((item) => item.id !== accountId)
    ElMessage.success('测试账号已删除')
  } catch (err) {
    if (err !== 'cancel' && err !== 'close') {
      console.error('Failed to delete test account:', err)
      ElMessage.error('测试账号删除失败')
    }
  }
}

const openEditDialog = (account: TestAccountItem) => {
  editForm.value = {
    id: account.id,
    account_type: account.account_type || 'sandbox',
    account: account.account,
    password: account.password,
    project_code: account.project_code
  }
  showEditDialog.value = true
}

const handleUpdateAccount = async () => {
  if (!editForm.value.id) return

  const payload = {
    account_type: editForm.value.account_type,
    account: editForm.value.account.trim(),
    password: editForm.value.password.trim(),
    project_code: editForm.value.project_code
  }

  if (!payload.account) {
    ElMessage.warning('请输入测试账号/设备名称')
    return
  }
  if (!payload.password) {
    ElMessage.warning('请输入密码/设备标识')
    return
  }
  if (!payload.project_code) {
    ElMessage.warning('请选择项目')
    return
  }

  try {
    const res = await axios.put(`/api/config/sandbox-accounts/${editForm.value.id}`, {
      id: editForm.value.id,
      ...payload
    })

    const updated = res.data as TestAccountItem
    const idx = testAccounts.value.findIndex((item) => item.id === updated.id)
    if (idx !== -1) {
      testAccounts.value[idx] = {
        ...testAccounts.value[idx],
        id: updated.id,
        account_type: updated.account_type || 'sandbox',
        account: updated.account,
        password: updated.password,
        project_code: updated.project_code
      }
    }

    showEditDialog.value = false
    resetEditForm()
    ElMessage.success('测试账号已更新')
  } catch (err) {
    console.error('Failed to update test account:', err)
    ElMessage.error('测试账号更新失败')
  }
}

onMounted(async () => {
  await Promise.all([fetchProjects(), fetchAccounts()])
  resetCreateForm()
})
</script>

<template>
  <div class="account-console">
    <div class="console-header">
      <div>
        <p class="console-kicker">Test Process Tools</p>
        <h1 class="console-title">测试账号管理台</h1>
        <p class="console-desc">从仪表盘的沙盒账号管理入口进入，在这里统一维护沙盒账号、数说测试账号、项目归属与删除操作。</p>
      </div>
      <el-button type="primary" class="create-btn" @click="showCreateDialog = true">
        新增账号
      </el-button>
    </div>

    <div class="stats-grid">
      <el-card
        v-for="stat in accountStats"
        :key="stat.title"
        shadow="hover"
        :class="['stat-card', `stat-card--${stat.tone}`]"
      >
        <span class="stat-title">{{ stat.title }}</span>
        <strong class="stat-value">{{ stat.value }}</strong>
      </el-card>
    </div>

    <div class="type-switch">
      <button
        v-for="item in ACCOUNT_TYPE_OPTIONS"
        :key="item.value"
        type="button"
        :class="['type-switch__item', { 'is-active': activeType === item.value }]"
        @click="activeType = item.value"
      >
        {{ item.label }}
      </button>
    </div>

    <div class="console-body">
      <aside class="project-sidebar">
        <div class="sidebar-card">
          <h2 class="sidebar-title">项目筛选</h2>
          <button
            v-for="item in projectNavItems"
            :key="item.key"
            type="button"
            :class="['project-filter', { 'is-active': activeProjectFilter === item.key }]"
            @click="activeProjectFilter = item.key"
          >
            <span>{{ item.label }}</span>
            <span class="project-filter__count">{{ item.count }}</span>
          </button>
        </div>
      </aside>

      <section class="content-panel">
        <div v-if="groupedAccounts.length === 0" class="empty-panel">
          当前筛选条件下暂无账号
        </div>

        <div v-else class="project-sections">
          <section
            v-for="group in groupedAccounts"
            :key="group.projectCode"
            class="project-section"
          >
            <div class="project-section__header">
              <div class="project-heading">
                <h2 class="project-title">{{ group.title }}</h2>
                <span class="project-code">{{ group.projectCode === 'unassigned' ? '未分配' : group.projectCode }}</span>
              </div>
              <el-button
                size="small"
                plain
                class="project-edit-btn"
                @click="toggleEditProject(group.projectCode)"
              >
                {{ editingProjectCode === group.projectCode ? '完成' : '编辑' }}
              </el-button>
            </div>

            <div class="account-grid">
              <el-card
                v-for="account in group.items"
                :key="account.id"
                class="account-card"
                shadow="hover"
                @dblclick.stop="openEditDialog(account)"
              >
                <div class="account-card__title">
                  <span class="account-name">{{ account.account }}</span>
                  <span :class="getTypeBadgeClass(account.account_type)">
                    {{ getTypeBadgeLabel(account.account_type) }}
                  </span>
                </div>

                <div class="account-card__body">
                  <div class="account-row">
                    <span class="field-label">{{ getPrimaryFieldLabel(account.account_type) }}</span>
                    <span class="field-value">{{ account.account }}</span>
                  </div>
                  <div class="account-row">
                    <span class="field-label">{{ getSecondaryFieldLabel(account.account_type) }}</span>
                    <span class="field-value">{{ account.password }}</span>
                  </div>
                  <div class="account-meta">
                    <span class="project-chip">{{ getProjectName(account.project_code) }}</span>
                  </div>
                </div>

                <div class="account-card__footer">
                  <el-button
                    v-if="editingProjectCode === group.projectCode"
                    type="danger"
                    text
                    class="account-delete-btn"
                    @click="handleDeleteAccount(account.id)"
                  >
                    删除
                  </el-button>
                </div>
              </el-card>
            </div>
          </section>
        </div>
      </section>
    </div>

    <el-dialog
      v-model="showCreateDialog"
      title="新增测试账号"
      width="480px"
      @closed="resetCreateForm"
    >
      <div class="dialog-form">
        <el-select v-model="newAccountForm.account_type" placeholder="请选择账号类型" style="width: 100%">
          <el-option label="沙盒账号" value="sandbox" />
          <el-option label="数说测试账号" value="dataqa" />
        </el-select>

        <el-select v-model="newAccountForm.project_code" placeholder="请选择所属项目" style="width: 100%">
          <el-option
            v-for="project in projects"
            :key="project.id || project.project_code"
            :label="`${project.project_name} (${project.project_code})`"
            :value="project.project_code"
          />
        </el-select>

        <el-input
          v-model="newAccountForm.account"
          :placeholder="getAccountPlaceholder(newAccountForm.account_type)"
        />
        <el-input
          v-model="newAccountForm.password"
          type="password"
          show-password
          :placeholder="getPasswordPlaceholder(newAccountForm.account_type)"
          @keyup.enter="handleCreateAccount"
        />
      </div>

      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" @click="handleCreateAccount">新增</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="showEditDialog"
      title="编辑测试账号"
      width="480px"
      @closed="resetEditForm"
    >
      <div class="dialog-form">
        <el-select v-model="editForm.account_type" placeholder="请选择账号类型" style="width: 100%">
          <el-option label="沙盒账号" value="sandbox" />
          <el-option label="数说测试账号" value="dataqa" />
        </el-select>

        <el-select v-model="editForm.project_code" placeholder="请选择所属项目" style="width: 100%">
          <el-option
            v-for="project in projects"
            :key="project.id || project.project_code"
            :label="`${project.project_name} (${project.project_code})`"
            :value="project.project_code"
          />
        </el-select>

        <el-input
          v-model="editForm.account"
          :placeholder="getAccountPlaceholder(editForm.account_type)"
        />

        <el-input
          v-model="editForm.password"
          type="password"
          show-password
          :placeholder="getPasswordPlaceholder(editForm.account_type)"
          @keyup.enter="handleUpdateAccount"
        />
      </div>

      <template #footer>
        <el-button @click="showEditDialog = false">取消</el-button>
        <el-button type="primary" @click="handleUpdateAccount">保存修改</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.account-console {
  padding: 24px;
}

.console-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.console-kicker {
  margin: 0 0 8px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.console-title {
  margin: 0;
  color: #0f172a;
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.console-desc {
  max-width: 760px;
  margin: 10px 0 0;
  color: #64748b;
  font-size: 14px;
  line-height: 1.7;
}

.create-btn {
  flex-shrink: 0;
  min-height: 42px;
  border-radius: 12px;
  padding: 0 18px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 18px;
}

.stat-card {
  border-radius: 18px;
  border: 1px solid #e2e8f0;
}

.stat-card :deep(.el-card__body) {
  display: grid;
  gap: 10px;
}

.stat-card--blue {
  background: linear-gradient(180deg, #ffffff, #eff6ff);
}

.stat-card--green {
  background: linear-gradient(180deg, #ffffff, #ecfdf5);
}

.stat-card--amber {
  background: linear-gradient(180deg, #ffffff, #fffbeb);
}

.stat-card--slate {
  background: linear-gradient(180deg, #ffffff, #f8fafc);
}

.stat-title {
  color: #64748b;
  font-size: 13px;
  font-weight: 600;
}

.stat-value {
  color: #0f172a;
  font-size: 30px;
  font-weight: 800;
}

.type-switch {
  display: inline-flex;
  gap: 6px;
  padding: 6px;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  background: #ffffff;
  margin-bottom: 18px;
}

.type-switch__item {
  border: 0;
  background: transparent;
  border-radius: 999px;
  padding: 9px 14px;
  color: #475569;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.type-switch__item.is-active {
  background: #0f172a;
  color: #ffffff;
}

.console-body {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 18px;
}

.sidebar-card {
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 18px;
  background: #ffffff;
}

.sidebar-title {
  margin: 0 0 12px;
  color: #0f172a;
  font-size: 15px;
  font-weight: 800;
}

.project-filter {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: #f8fafc;
  color: #334155;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.project-filter.is-active {
  border-color: #bfdbfe;
  background: #eff6ff;
  color: #1d4ed8;
}

.project-filter__count {
  min-width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: #ffffff;
  color: #64748b;
  font-size: 12px;
}

.content-panel {
  min-width: 0;
}

.empty-panel {
  padding: 48px 24px;
  border: 1px dashed #cbd5e1;
  border-radius: 18px;
  background: #ffffff;
  color: #94a3b8;
  text-align: center;
}

.project-sections {
  display: grid;
  gap: 24px;
}

.project-section {
  padding: 16px 18px 18px;
  border: 1px solid #e2e8f0;
  border-radius: 20px;
  background: #ffffff;
}

.project-section__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.project-heading {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.project-title {
  margin: 0;
  color: #0f172a;
  font-size: 20px;
  font-weight: 800;
}

.project-code {
  padding: 4px 10px;
  border-radius: 999px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
}

.project-edit-btn {
  border-radius: 999px;
}

.account-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.account-card {
  width: min(100%, 420px);
  border: 1px solid #dbeafe;
  border-radius: 18px;
  background: radial-gradient(circle at top right, rgba(16, 185, 129, 0.10), transparent 28%), linear-gradient(135deg, #ffffff, #f8fafc);
}

.account-card :deep(.el-card__body) {
  padding: 18px 18px 16px;
}

.account-card__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.account-name {
  color: #0f172a;
  font-size: 16px;
  font-weight: 800;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.account-badge {
  padding: 4px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.account-badge--sandbox {
  background: rgba(16, 185, 129, 0.12);
  color: #059669;
}

.account-badge--dataqa {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}

.account-card__body {
  display: grid;
  gap: 10px;
}

.account-row {
  display: flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
}

.field-label {
  color: #94a3b8;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.field-value {
  color: #0f172a;
  font-size: 16px;
  font-weight: 800;
}

.account-meta {
  margin-top: 2px;
}

.project-chip {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  background: #f8fafc;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.account-card__footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 14px;
}

.account-delete-btn {
  padding: 0;
}

.dialog-form {
  display: grid;
  gap: 14px;
}

@media (max-width: 1080px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .console-body {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .account-console {
    padding: 18px;
  }

  .console-header {
    flex-direction: column;
    align-items: stretch;
  }

  .console-title {
    font-size: 26px;
  }

  .stats-grid {
    grid-template-columns: 1fr;
  }

  .type-switch {
    width: 100%;
    flex-wrap: wrap;
    border-radius: 18px;
  }

  .type-switch__item {
    flex: 1 1 calc(50% - 6px);
  }

  .project-section__header,
  .account-card__title,
  .account-row {
    flex-wrap: wrap;
  }

  .account-card {
    width: 100%;
  }
}
</style>
