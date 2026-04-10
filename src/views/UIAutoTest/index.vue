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
    <div class="suite-sidebar">
      <div class="sidebar-header">
        <h3>测试套件</h3>
        <el-button type="primary" :icon="Plus" circle size="small" @click="handleAddSuite" />
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
            <el-icon><Collection /></el-icon>
            <span class="suite-name">{{ suite.name }}</span>
          </div>
          <div class="suite-actions" v-if="activeSuiteId === suite.id">
            <el-button :icon="Edit" link @click.stop="handleEditSuite(suite)" />
            <el-button :icon="Delete" link type="danger" @click.stop="confirmDeleteSuite(suite.id)" />
          </div>
        </div>
        
        <el-empty v-if="pwStore.suites.length === 0" description="暂无套件" :image-size="60" />
      </div>
    </div>

    <div class="case-content">
      <div v-if="activeSuiteId" class="content-body">
        <div class="content-header">
          <div class="header-left">
            <h2>{{ pwStore.suites.find(s => s.id === activeSuiteId)?.name }}</h2>
            <p class="suite-desc">{{ pwStore.suites.find(s => s.id === activeSuiteId)?.description }}</p>
          </div>
          <div class="header-actions">
            <el-button type="primary" :icon="Plus" @click="handleCreateCase">新建用例</el-button>
          </div>
        </div>

        <el-table :data="pwStore.cases" style="width: 100%" v-loading="pwStore.loading">
          <el-table-column prop="name" label="用例名称" min-width="180">
            <template #default="{ row }">
              <div class="case-name-cell">
                <strong>{{ row.name }}</strong>
                <div class="case-tags">
                  <el-tag v-for="tag in row.tags" :key="tag" size="small" class="tag-item">{{ tag }}</el-tag>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="详细描述" show-overflow-tooltip />
          <el-table-column prop="updated_at" label="最后更新" width="180" />
          <el-table-column label="操作" width="260" fixed="right">
            <template #default="{ row }">
              <div class="action-buttons">
                <el-button type="success" size="small" :icon="Monitor" @click="handleRunCase(row.id)">执行</el-button>
                <el-button size="small" :icon="Edit" @click="handleEditCase(row.id)">编辑</el-button>
                <el-button type="danger" size="small" :icon="Delete" @click="confirmDeleteCase(row.id)" />
              </div>
            </template>
          </el-table-column>
        </el-table>
        
        <el-empty v-if="pwStore.cases.length === 0 && !pwStore.loading" description="该套件下暂无用例" />
      </div>
      
      <div v-else class="empty-state">
        <el-icon :size="64" color="#dcdfe6"><Monitor /></el-icon>
        <p>选择一个测试套件以开始</p>
      </div>
    </div>

    <!-- Suite Dialog -->
    <el-dialog v-model="showSuiteDialog" :title="suiteForm.id ? '编辑套件' : '新建套件'" width="450px">
      <el-form :model="suiteForm" label-position="top">
        <el-form-item label="名称" required>
          <el-input v-model="suiteForm.name" placeholder="输入套件名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="suiteForm.description" type="textarea" placeholder="简介..." />
        </el-form-item>
        <el-form-item label="关联套件库 (Skills)">
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
          <div class="form-tip">关联后可在编辑器中优先使用这些套件内的技能</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showSuiteDialog = false">取消</el-button>
        <el-button type="primary" @click="submitSuite">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.ui-auto-container {
  display: flex;
  height: calc(100vh - 120px);
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0,0,0,0.05);
}

.suite-sidebar {
  width: 280px;
  border-right: 1px solid #f0f0f0;
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #f8f9fa;
}

.sidebar-header h3 {
  margin: 0;
  font-size: 16px;
  color: #1a1a1a;
}

.suite-list {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
}

.suite-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 15px;
  border-radius: 8px;
  cursor: pointer;
  margin-bottom: 4px;
  transition: all 0.2s;
  color: #606266;
}

.suite-item:hover {
  background: #f5f7fa;
}

.suite-item.active {
  background: #ecf5ff;
  color: #409eff;
  font-weight: 600;
}

.suite-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.suite-name {
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 160px;
}

.case-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: #fcfcfc;
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
}

.header-left h2 {
  margin: 0 0 8px 0;
  font-size: 22px;
}

.suite-desc {
  margin: 0;
  color: #909399;
  font-size: 14px;
}

.case-name-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
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
  font-size: 10px;
}

.empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  color: #909399;
}

.empty-state p {
  margin-top: 20px;
}

.suite-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
</style>
