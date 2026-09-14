<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { usePlaywrightStore } from '@/stores/modules/playwright'
import { Plus, Delete, Edit, Monitor, Collection, CollectionTag } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'

const router = useRouter()
const pwStore = usePlaywrightStore()

const activeSuiteId = ref<string>('')
const showSuiteDialog = ref(false)
const suiteForm = ref({
  id: '',
  name: '',
  description: '',
  skill_suites: [] as string[]
})

const allSkillSuites = ref<string[]>([])

const fetchSkillSuites = async () => {
  try {
    const res = await axios.get('/api/playwright/keywords')
    const keywords = res.data || []
    const suites = new Set<string>()
    keywords.forEach((kw: any) => {
      if (kw.suite_name) suites.add(kw.suite_name)
    })
    allSkillSuites.value = Array.from(suites)
  } catch (err) {
    console.error('获取技能套件失败', err)
  }
}

onMounted(async () => {
  await pwStore.fetchSuites()
  await fetchSkillSuites()
  const firstSuite = pwStore.suites[0]
  if (firstSuite?.id) {
    selectSuite(firstSuite.id)
  }
})

const selectSuite = async (id: string) => {
  activeSuiteId.value = id
  await pwStore.fetchCases(id)
}

const handleAddSuite = () => {
  suiteForm.value = { id: '', name: '', description: '', skill_suites: [] }
  showSuiteDialog.value = true
}

const handleEditSuite = (suite: any) => {
  suiteForm.value = { 
    ...suite, 
    skill_suites: suite.skill_suites || [] 
  }
  showSuiteDialog.value = true
}

const submitSuite = async () => {
  if (!suiteForm.value.name) return
  await pwStore.saveSuite(suiteForm.value)
  showSuiteDialog.value = false
  ElMessage.success('测试套件已保存')
}

const confirmDeleteSuite = (id: string) => {
  ElMessageBox.confirm('确定要删除该测试套件吗？其下的所有用例也将失去关联。', '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    await pwStore.deleteSuite(id)
    if (activeSuiteId.value === id) {
      activeSuiteId.value = ''
      pwStore.cases = []
    }
    ElMessage.success('测试套件已删除')
  })
}

const handleCreateCase = () => {
  if (!activeSuiteId.value) {
    ElMessage.warning('请先选择或创建一个测试套件')
    return
  }
  router.push({
    name: 'UIAutoEditor',
    query: { suite_id: activeSuiteId.value }
  })
}

const handleEditCase = (caseId: string) => {
  router.push(`/ui_auto/edit/${caseId}`)
}

const handleRunCase = (caseId: string) => {
  router.push(`/ui_auto/run/${caseId}`)
}

const confirmDeleteCase = (id: string) => {
  ElMessageBox.confirm('确定要删除该测试用例吗？', '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    await pwStore.deleteCase(id)
    ElMessage.success('测试用例已删除')
  })
}

</script>

<template>
  <div class="ui-auto-container">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">UI 自动化测试</h1>
          <span class="page-badge">Playwright Automation</span>
        </div>
        <p class="page-desc">基于 Playwright 的端到端自动化测试套件编排、技能库关联与用例调度中心</p>
      </div>
    </div>

    <!-- Main Workspace Split Container -->
    <div class="workspace-card">
      <div class="suite-sidebar">
        <div class="sidebar-header">
          <div class="sidebar-title">
            <span class="title-text">测试套件</span>
            <span class="count-pill">{{ pwStore.suites.length }}</span>
          </div>
          <el-button type="primary" :icon="Plus" size="small" class="add-suite-btn" @click="handleAddSuite">
            新建
          </el-button>
        </div>
        
        <div v-loading="pwStore.loading" class="suite-list">
          <div 
            v-for="suite in pwStore.suites" 
            :key="suite.id"
            class="suite-item"
            :class="{ active: activeSuiteId === suite.id }"
            @click="selectSuite(suite.id)"
          >
            <div class="suite-info">
              <div class="suite-icon-box">
                <el-icon><Collection /></el-icon>
              </div>
              <span class="suite-name">{{ suite.name }}</span>
            </div>
            <div class="suite-actions" v-if="activeSuiteId === suite.id">
              <el-button :icon="Edit" link type="primary" @click.stop="handleEditSuite(suite)" />
              <el-button :icon="Delete" link type="danger" @click.stop="confirmDeleteSuite(suite.id)" />
            </div>
          </div>
          
          <el-empty v-if="pwStore.suites.length === 0" description="暂无测试套件" :image-size="60" />
        </div>
      </div>

      <div class="case-content">
        <div v-if="activeSuiteId" class="content-body">
          <div class="content-header">
            <div class="header-left">
              <h2>{{ pwStore.suites.find(s => s.id === activeSuiteId)?.name }}</h2>
              <p class="suite-desc">{{ pwStore.suites.find(s => s.id === activeSuiteId)?.description || '暂无描述信息' }}</p>
            </div>
            <div class="header-actions">
              <el-button type="primary" :icon="Plus" class="add-case-btn" @click="handleCreateCase">新建用例</el-button>
            </div>
          </div>

          <div class="table-wrapper">
            <el-table :data="pwStore.cases" style="width: 100%" v-loading="pwStore.loading" stripe>
              <el-table-column prop="name" label="用例名称" min-width="200">
                <template #default="{ row }">
                  <div class="case-name-cell">
                    <strong>{{ row.name }}</strong>
                    <div class="case-tags" v-if="row.tags && row.tags.length">
                      <el-tag v-for="tag in row.tags" :key="tag" size="small" effect="plain" class="tag-item">{{ tag }}</el-tag>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="description" label="详细描述" min-width="240" show-overflow-tooltip>
                <template #default="{ row }">
                  <span class="desc-text">{{ row.description || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="updated_at" label="最后更新" width="180">
                <template #default="{ row }">
                  <span class="date-text">{{ row.updated_at || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="220" fixed="right">
                <template #default="{ row }">
                  <div class="action-buttons">
                    <el-button type="success" size="small" :icon="Monitor" @click="handleRunCase(row.id)">执行</el-button>
                    <el-button size="small" :icon="Edit" @click="handleEditCase(row.id)">编辑</el-button>
                    <el-button type="danger" size="small" :icon="Delete" @click="confirmDeleteCase(row.id)" />
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>
          
          <el-empty v-if="pwStore.cases.length === 0 && !pwStore.loading" description="该套件下暂无用例，点击右上角新建" />
        </div>
        
        <div v-else class="empty-state">
          <div class="empty-icon-box">
            <el-icon><Monitor /></el-icon>
          </div>
          <p class="empty-title">选择一个测试套件以开始</p>
          <p class="empty-sub">或在左侧点击新建按钮创建新的自动化测试套件</p>
        </div>
      </div>
    </div>

    <!-- Suite Dialog -->
    <el-dialog 
      v-model="showSuiteDialog" 
      :title="suiteForm.id ? '编辑套件' : '新建套件'" 
      width="480px"
      destroy-on-close
    >
      <el-form :model="suiteForm" label-position="top" class="suite-dialog-form">
        <el-form-item label="套件名称" required>
          <el-input v-model="suiteForm.name" placeholder="请输入测试套件名称" />
        </el-form-item>
        <el-form-item label="详细描述">
          <el-input v-model="suiteForm.description" type="textarea" :rows="3" placeholder="简要说明此套件涵盖的业务范围..." />
        </el-form-item>
        <el-form-item label="关联技能套件库 (Skills)">
          <el-select
            v-model="suiteForm.skill_suites"
            multiple
            placeholder="选择关联的技能套件"
            style="width: 100%"
          >
            <el-option
              v-for="s in allSkillSuites"
              :key="s"
              :label="s"
              :value="s"
            >
              <div class="suite-option">
                <el-icon><CollectionTag /></el-icon>
                <span>{{ s }}</span>
              </div>
            </el-option>
          </el-select>
          <div class="form-tip">关联后可在测试用例编辑器中优先联想并调用这些套件内的关键词技能</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showSuiteDialog = false">取消</el-button>
          <el-button type="primary" @click="submitSuite">保存</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ui-auto-container {
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

/* Workspace Card */
.workspace-card {
  display: flex;
  min-height: calc(100vh - 200px);
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
}

.suite-sidebar {
  width: 290px;
  border-right: 1px solid #e2e8f0;
  display: flex;
  flex-direction: column;
  background: #fcfdfe;
}

.sidebar-header {
  padding: 18px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #e2e8f0;
  background: #ffffff;
}

.sidebar-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.title-text {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.count-pill {
  font-size: 11px;
  font-weight: 700;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  padding: 1px 7px;
  border-radius: 9999px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.add-suite-btn {
  border-radius: 6px;
}

.suite-list {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.suite-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-radius: 10px;
  cursor: pointer;
  border: 1px solid transparent;
  transition: all 0.2s ease;
  color: #475569;
}

.suite-item:hover {
  background: #f1f5f9;
  color: #0f172a;
}

.suite-item.active {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #2563eb;
  font-weight: 600;
}

.suite-icon-box {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: #f1f5f9;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  transition: all 0.2s ease;
}

.suite-item.active .suite-icon-box {
  background: #2563eb;
  color: #ffffff;
}

.suite-info {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.suite-name {
  font-size: 13px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 150px;
}

.suite-actions {
  display: flex;
  align-items: center;
  gap: 2px;
}

.case-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #ffffff;
}

.content-body {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.content-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  padding-bottom: 16px;
  border-bottom: 1px solid #f1f5f9;
}

.header-left h2 {
  margin: 0 0 6px 0;
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

.suite-desc {
  margin: 0;
  color: #64748b;
  font-size: 13px;
}

.add-case-btn {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  border: none;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.25);
}

.table-wrapper {
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  overflow: hidden;
}

.case-name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.case-name-cell strong {
  font-size: 14px;
  color: #0f172a;
}

.action-buttons {
  display: flex;
  gap: 8px;
  align-items: center;
  white-space: nowrap;
}

.case-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.tag-item {
  font-size: 11px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.desc-text {
  color: #475569;
  font-size: 13px;
}

.date-text {
  font-size: 13px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  padding: 60px 24px;
}

.empty-icon-box {
  width: 68px;
  height: 68px;
  border-radius: 20px;
  background: #f1f5f9;
  color: #94a3b8;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 32px;
  margin-bottom: 16px;
}

.empty-title {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 6px;
}

.empty-sub {
  font-size: 13px;
  color: #64748b;
  margin: 0;
}

.suite-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.form-tip {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 6px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
</style>
