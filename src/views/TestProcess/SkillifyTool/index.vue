<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { MagicStick, Search, Connection, List, CopyDocument, Picture, Check, FolderOpened, Link, CollectionTag, Edit, Delete } from '@element-plus/icons-vue'
import { ElMessage, ElNotification } from 'element-plus'
import axios from 'axios'

interface ElementData {
  selector: string
  text: string
  tag_name: string
  role: string
  aria_label: string
  placeholder: string
  parent_context: string
}

interface UserKeyword {
  id: string
  name: string
  description: string
  suite_name: string
  source_url: string
  created_at: string
  steps: any[]
}

interface SkillNode {
  name: string
  description: string
  keyword: string
  args: Record<string, string>
  category: string
}

const targetUrl = ref('')
const loading = ref(false)
const scanning = ref(false)
const generating = ref(false)
const elements = ref<ElementData[]>([])
const skills = ref<SkillNode[]>([])
const activeTab = ref('library')

// Library Data
const librarySkills = ref<UserKeyword[]>([])
const savedSuites = ref<string[]>([])
const libraryLoading = ref(false)

const fetchLibrary = async () => {
  libraryLoading.value = true
  try {
    const res = await axios.get('/api/playwright/keywords')
    librarySkills.value = res.data || []
    // Extract unique suite names
    const suites = new Set<string>()
    librarySkills.value.forEach(kw => {
      if (kw.suite_name) suites.add(kw.suite_name)
    })
    savedSuites.value = Array.from(suites)
  } catch (err: any) {
    ElMessage.error('获取技能库失败')
  } finally {
    libraryLoading.value = false
  }
}

const fetchSuites = async () => {
  try {
    const res = await axios.get('/api/playwright/suites')
    const existing = res.data.map((s: any) => s.name)
    // Merge with suites found in keywords
    const combined = new Set([...existing, ...savedSuites.value])
    savedSuites.value = Array.from(combined)
  } catch (err) {}
}

onMounted(() => {
  fetchLibrary()
  fetchSuites()
})

const handleScan = async () => {
  if (!targetUrl.value) {
    ElMessage.warning('请输入目标网址')
    return
  }

  scanning.value = true
  loading.value = true
  elements.value = []
  skills.value = []

  try {
    const res = await axios.post('/api/skillify/scan', {
      url: targetUrl.value
    })
    elements.value = res.data.elements
    ElNotification({
      title: '扫描完成',
      message: `成功识别 ${elements.value.length} 个可点击项`,
      type: 'success'
    })
    activeTab.value = 'elements'
  } catch (err: any) {
    ElMessage.error('扫描失败: ' + (err.response?.data?.error || err.message))
  } finally {
    scanning.value = false
    loading.value = false
  }
}

const handleSkillify = async () => {
  if (elements.value.length === 0) {
    ElMessage.warning('请先启动扫描以获取页面元素')
    return
  }

  generating.value = true
  try {
    const res = await axios.post('/api/skillify/generate', {
      elements: elements.value
    })
    skills.value = res.data.skills
    ElNotification({
      title: 'Skill化完成',
      message: 'AI 已成功将原始节点转化为标准 Skill',
      type: 'success'
    })
    activeTab.value = 'skills'
  } catch (err: any) {
    ElMessage.error('AI 处理失败: ' + (err.response?.data?.error || err.message))
  } finally {
    generating.value = false
  }
}

// Advanced Save Dialog
const saveDialogVisible = ref(false)
const saving = ref(false)
const saveForm = ref({
  suiteName: '',
  sourceUrl: '',
  addToExisting: ''
})

const openSaveDialog = () => {
  saveForm.value.sourceUrl = targetUrl.value
  saveForm.value.suiteName = ''
  saveForm.value.addToExisting = ''
  saveDialogVisible.value = true
}

const handleSave = async () => {
  const finalSuiteName = saveForm.value.suiteName || saveForm.value.addToExisting
  if (!finalSuiteName) {
    ElMessage.warning('请提供套件名称')
    return
  }

  saving.value = true
  try {
    await axios.post('/api/skillify/save', {
      skills: skills.value,
      suite_name: finalSuiteName,
      source_url: saveForm.value.sourceUrl
    })
    ElNotification({
      title: '保存成功',
      message: `Skill 节点已归档至套件: ${finalSuiteName}`,
      type: 'success'
    })
    saveDialogVisible.value = false
    fetchLibrary() // Refresh library
  } catch (err: any) {
    ElMessage.error('保存失败: ' + (err.response?.data?.error || err.message))
  } finally {
    saving.value = false
  }
}

const copyToClipboard = (text: string) => {
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}

const groupedSkills = () => {
  const groups: Record<string, SkillNode[]> = {}
  skills.value.forEach(skill => {
    const cat = skill.category || '未分类'
    if (!groups[cat]) {
      groups[cat] = []
    }
    groups[cat].push(skill)
  })
  return groups
}

const groupedLibrary = computed(() => {
  const groups: Record<string, UserKeyword[]> = {}
  librarySkills.value.forEach(skill => {
    const suite = skill.suite_name || '未归类套件'
    if (!groups[suite]) groups[suite] = []
    groups[suite].push(skill)
  })
  return groups
})

// Edit/Delete Skills
const editSkillDialogVisible = ref(false)
const editSkillForm = ref({
  id: '',
  name: '',
  description: '',
  suite_name: '',
  source_url: '',
  steps: [] as any[]
})

const openEditSkillDialog = (skill: UserKeyword) => {
  editSkillForm.value = { ...skill }
  editSkillDialogVisible.value = true
}

const handleUpdateSkill = async () => {
  try {
    await axios.post('/api/playwright/keywords', editSkillForm.value)
    ElMessage.success('技能已更新')
    editSkillDialogVisible.value = false
    fetchLibrary()
  } catch (err) {
    ElMessage.error('更新失败')
  }
}

const handleDeleteSkill = (id: string) => {
  ElMessageBox.confirm('确定删除该技能吗？', '提示', { type: 'warning' }).then(async () => {
    try {
      await axios.delete(`/api/playwright/keywords/${id}`)
      ElMessage.success('技能已删除')
      fetchLibrary()
    } catch (err) {
      ElMessage.error('删除失败')
    }
  })
}

// Rename/Delete Suites
const renameSuiteDialogVisible = ref(false)
const renameSuiteForm = ref({ oldName: '', newName: '' })

const openRenameSuiteDialog = (name: string) => {
  renameSuiteForm.value = { oldName: name, newName: name }
  renameSuiteDialogVisible.value = true
}

const handleRenameSuite = async () => {
  try {
    await axios.put(`/api/playwright/keywords/suite/${renameSuiteForm.value.oldName}`, {
      new_name: renameSuiteForm.value.newName
    })
    ElMessage.success('套件已重命名')
    renameSuiteDialogVisible.value = false
    fetchLibrary()
  } catch (err) {
    ElMessage.error('重命名失败')
  }
}

const handleDeleteSuite = (name: string) => {
  ElMessageBox.confirm(`确定删除套件「${name}」及其下所有技能吗？`, '警告', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'error'
  }).then(async () => {
    try {
      await axios.delete(`/api/playwright/keywords/suite/${name}`)
      ElMessage.success('套件已删除')
      fetchLibrary()
    } catch (err) {
      ElMessage.error('删除失败')
    }
  })
}

import { ElMessageBox } from 'element-plus'
</script>

<template>
  <div class="skillify-container">
    <!-- Header Section -->
    <div class="header-card glass-morphism">
      <div class="title-row">
        <div class="icon-box">
          <el-icon><MagicStick /></el-icon>
        </div>
        <div>
          <h1>节点 Skill 化工具</h1>
          <p>自动化识别网页元素并转化为 AI 可识别的原子技能</p>
        </div>
      </div>

      <div class="search-box">
        <el-input
          v-model="targetUrl"
          placeholder="输入目标网站 URL (如 https://www.google.com)"
          class="url-input"
          :prefix-icon="Search"
          @keyup.enter="handleScan"
        />
        <el-button 
          type="primary" 
          :loading="scanning" 
          class="scan-btn"
          @click="handleScan"
        >
          开始扫描
        </el-button>
      </div>
    </div>

    <!-- Main Content -->
    <div class="main-content" v-loading="loading">
      <el-tabs v-model="activeTab" class="custom-tabs">
        <!-- Library Tab (Default) -->
        <el-tab-pane name="library">
          <template #label>
            <div class="tab-label">
              <el-icon><FolderOpened /></el-icon>
              <span>技能库 ({{ librarySkills.length }})</span>
            </div>
          </template>

          <div v-if="libraryLoading" class="generating-state">
            <el-skeleton :rows="5" animated />
          </div>

          <div v-else-if="librarySkills.length === 0" class="empty-state">
            <el-empty description="暂无已保存的技能" />
          </div>

          <div v-else class="skill-groups">
            <div 
              v-for="(group, suite) in groupedLibrary" 
              :key="suite" 
              class="category-section"
            >
              <div class="category-header">
                <h3 class="category-title">
                  <el-icon><CollectionTag /></el-icon>
                  {{ suite }}
                </h3>
                <div class="category-actions">
                  <el-button link type="primary" :icon="Edit" @click="openRenameSuiteDialog(suite)">重命名</el-button>
                  <el-button link type="danger" :icon="Delete" @click="handleDeleteSuite(suite)">删除套件</el-button>
                </div>
              </div>
              <div class="skill-list">
                <div 
                  v-for="skill in group" 
                  :key="skill.id" 
                  class="skill-item card-shine"
                >
                  <div class="skill-header">
                    <span class="skill-name">{{ skill.name }}</span>
                    <div class="skill-actions-small">
                      <el-button link :icon="Edit" @click="openEditSkillDialog(skill)" />
                      <el-button link type="danger" :icon="Delete" @click="handleDeleteSkill(skill.id)" />
                    </div>
                  </div>
                  <p class="skill-desc">{{ skill.description }}</p>
                  <div class="skill-meta" v-if="skill.source_url">
                    <el-icon><Link /></el-icon>
                    <span class="source-link">{{ skill.source_url }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <!-- Elements Tab (Conditional) -->
        <el-tab-pane v-if="elements.length > 0" name="elements">
          <template #label>
            <div class="tab-label">
              <el-icon><List /></el-icon>
              <span>扫描结果 ({{ elements.length }})</span>
            </div>
          </template>

          <div class="element-grid">
            <div 
              v-for="(el, index) in elements" 
              :key="index" 
              class="element-card"
            >
              <div class="card-tag">
                <el-icon v-if="el.tag_name === 'img'"><Picture /></el-icon>
                <span v-else>{{ el.tag_name }}</span>
              </div>
              <div class="card-content">
                <div class="element-text" :title="el.text || el.placeholder">
                  {{ el.text || el.placeholder || '无文本' }}
                </div>
                <div class="element-selector">
                  <code>{{ el.selector }}</code>
                  <el-icon @click="copyToClipboard(el.selector)" class="copy-icon"><CopyDocument /></el-icon>
                </div>
              </div>
              <div class="card-footer">
                <span class="context">{{ el.parent_context }}</span>
                <span class="role">{{ el.role || 'normal' }}</span>
              </div>
            </div>
          </div>
        </el-tab-pane>

        <!-- Skills Tab (Conditional) -->
        <el-tab-pane v-if="skills.length > 0 || generating" name="skills">
          <template #label>
            <div class="tab-label">
              <el-icon><Connection /></el-icon>
              <span>AI Skill 节点 ({{ skills.length }})</span>
            </div>
          </template>

          <div v-if="skills.length === 0 && !generating" class="skillify-action-full">
            <div class="action-card">
              <el-icon class="action-icon"><MagicStick /></el-icon>
              <h2>点击“一键 Skill 化”</h2>
              <p>利用 DeepSeek AI 进行智能命名、分类和描述归纳</p>
              <el-button 
                type="success" 
                size="large" 
                :loading="generating"
                @click="handleSkillify"
              >
                立即开始转换
              </el-button>
            </div>
          </div>

          <div v-else-if="generating" class="generating-state">
            <div class="loader-wave">
              <span></span><span></span><span></span><span></span><span></span>
            </div>
            <p>DeepSeek 正在解析元素结构并生成技能节点...</p>
          </div>

          <div v-else class="skill-groups">
            <div class="skills-header-actions">
              <el-button 
                type="success" 
                @click="openSaveDialog"
                class="save-btn"
              >
                <el-icon><Check /></el-icon>
                保存到技能库
              </el-button>
            </div>
            <div 
              v-for="(group, category) in groupedSkills()" 
              :key="category" 
              class="category-section"
            >
              <h3 class="category-title">{{ category }}</h3>
              <div class="skill-list">
                <div 
                  v-for="(skill, sIdx) in group" 
                  :key="sIdx" 
                  class="skill-item card-shine"
                >
                  <div class="skill-header">
                    <span class="skill-name">{{ skill.name }}</span>
                    <el-tag size="small" effect="plain">{{ skill.keyword }}</el-tag>
                  </div>
                  <p class="skill-desc">{{ skill.description }}</p>
                  <div class="skill-meta" v-if="skill.args">
                    <code>{{ skill.args.selector }}</code>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <!-- Save Dialog -->
      <el-dialog
        v-model="saveDialogVisible"
        title="保存 Skills 到库"
        width="500px"
        class="custom-dialog"
      >
        <el-form :model="saveForm" label-position="top">
          <el-form-item label="套件库名 (新建)">
            <el-input v-model="saveForm.suiteName" placeholder="输入新套件名称" />
          </el-form-item>
          
          <el-form-item label="是否加入已有套件">
            <el-select v-model="saveForm.addToExisting" placeholder="选择已有套件" clearable style="width: 100%">
              <el-option v-for="s in savedSuites" :key="s" :label="s" :value="s" />
            </el-select>
          </el-form-item>

          <el-form-item label="套件来源地址">
            <el-input v-model="saveForm.sourceUrl" disabled />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="saveDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="handleSave">确认保存</el-button>
        </template>
      </el-dialog>

      <!-- Edit Skill Dialog -->
      <el-dialog v-model="editSkillDialogVisible" title="编辑技能" width="450px" class="custom-dialog">
        <el-form :model="editSkillForm" label-position="top">
          <el-form-item label="名称" required>
            <el-input v-model="editSkillForm.name" />
          </el-form-item>
          <el-form-item label="描述">
            <el-input v-model="editSkillForm.description" type="textarea" :rows="3" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="editSkillDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleUpdateSkill">确认更新</el-button>
        </template>
      </el-dialog>

      <!-- Rename Suite Dialog -->
      <el-dialog v-model="renameSuiteDialogVisible" title="重命名套件" width="400px" class="custom-dialog">
        <el-form :model="renameSuiteForm" label-position="top">
          <el-form-item label="新名称" required>
            <el-input v-model="renameSuiteForm.newName" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="renameSuiteDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleRenameSuite">确认重命名</el-button>
        </template>
      </el-dialog>

      <!-- Float Action Button -->
      <transition name="fade">
        <div v-if="elements.length > 0 && activeTab === 'elements'" class="float-action">
          <el-button 
            type="success" 
            round 
            size="large" 
            :loading="generating"
            @click="handleSkillify"
            class="ai-btn"
          >
            <el-icon><MagicStick /></el-icon>
            AI 一键 Skill 化
          </el-button>
        </div>
      </transition>
    </div>
  </div>
</template>

<style scoped>
.skillify-container {
  padding: 20px;
  max-width: 1400px;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Header Styling */
.header-card {
  padding: 30px;
  border-radius: 20px;
  background: white;
  border: 1px solid rgba(226, 232, 240, 0.8);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.05);
}

.glass-morphism {
  background: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.title-row {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}

.icon-box {
  width: 50px;
  height: 50px;
  background: linear-gradient(135deg, #6366f1 0%, #a855f7 100%);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 24px;
}

.title-row h1 {
  font-size: 24px;
  margin: 0;
  color: #1e293b;
}

.title-row p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 14px;
}

.search-box {
  display: flex;
  gap: 12px;
}

.url-input :deep(.el-input__wrapper) {
  border-radius: 12px;
  padding: 8px 16px;
  background: #f8fafc;
}

.scan-btn {
  border-radius: 12px;
  padding: 0 28px;
  font-weight: 600;
}

/* Tabs & Content */
.custom-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.tab-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  padding: 4px 12px;
}

/* Element Grid */
.element-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
  padding: 10px 0;
}

.element-card {
  background: white;
  border: 1px solid #eef2ff;
  border-radius: 16px;
  padding: 16px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.element-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 10px 25px rgba(99, 102, 241, 0.08);
  border-color: #c7d2fe;
}

.card-tag {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  color: #6366f1;
  background: #eef2ff;
  padding: 2px 8px;
  border-radius: 6px;
  align-self: flex-start;
}

.element-text {
  font-size: 15px;
  font-weight: 600;
  color: #1e293b;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.element-selector {
  font-family: 'Fira Code', monospace;
  font-size: 11px;
  color: #64748b;
  background: #f1f5f9;
  padding: 8px;
  border-radius: 8px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.copy-icon {
  cursor: pointer;
  transition: color 0.2s;
}

.copy-icon:hover {
  color: #6366f1;
}

.card-footer {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #94a3b8;
  border-top: 1px solid #f8fafc;
  padding-top: 10px;
}

.context { text-transform: uppercase; }

.source-link {
  font-size: 11px;
  color: #94a3b8;
  margin-left: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
}

.category-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding: 0 4px;
}

.category-title {
  font-size: 18px;
  color: #1e293b;
  margin: 0;
  padding-left: 12px;
  border-left: 4px solid #6366f1;
  display: flex;
  align-items: center;
  gap: 8px;
}

.skill-actions-small {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s;
}

.skill-item:hover .skill-actions-small {
  opacity: 1;
}

/* Skill Groups */
.skills-header-actions {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 20px;
  padding: 0 12px;
}

.save-btn {
  box-shadow: 0 4px 12px rgba(16, 185, 129, 0.2);
  border-radius: 10px;
  font-weight: 600;
}

.skillify-action-full {
  height: 400px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.action-card {
  text-align: center;
  padding: 40px;
  background: #f8fafc;
  border: 2px dashed #e2e8f0;
  border-radius: 24px;
}

.action-icon {
  font-size: 48px;
  color: #6366f1;
  margin-bottom: 16px;
}

.category-section {
  margin-bottom: 32px;
}

.skill-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.skill-item {
  background: linear-gradient(135deg, #ffffff 0%, #f9fafb 100%);
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 20px;
  position: relative;
  overflow: hidden;
}

.skill-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.skill-name {
  font-weight: 700;
  color: #1e293b;
  font-size: 16px;
}

.skill-desc {
  font-size: 13px;
  color: #64748b;
  line-height: 1.6;
  margin-bottom: 12px;
}

.skill-meta {
  display: flex;
  align-items: center;
  gap: 4px;
}

.skill-meta code {
  font-size: 11px;
  color: #10b981;
}

.custom-dialog :deep(.el-dialog) {
  border-radius: 20px;
  overflow: hidden;
}

.custom-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 24px;
  background: #f8fafc;
  border-bottom: 1px solid #f1f5f9;
}

.custom-dialog :deep(.el-dialog__title) {
  font-weight: 700;
  color: #1e293b;
}

.custom-dialog :deep(.el-dialog__body) {
  padding: 24px;
}

.custom-dialog :deep(.el-dialog__footer) {
  padding: 16px 24px;
  background: #f8fafc;
  border-top: 1px solid #f1f5f9;
}

/* Animations */
.card-shine::after {
  content: '';
  position: absolute;
  top: 0; left: -100%;
  width: 50%; height: 100%;
  background: linear-gradient(to right, transparent, rgba(255,255,255,0.3), transparent);
  transform: skewX(-25deg);
  transition: 0.75s;
}

.card-shine:hover::after {
  left: 150%;
}

.float-action {
  position: fixed;
  bottom: 40px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 100;
}

.ai-btn {
  box-shadow: 0 10px 30px rgba(16, 185, 129, 0.3);
  padding: 12px 30px;
  font-weight: 700;
}

.loader-wave {
  display: flex;
  justify-content: center;
  gap: 6px;
  margin-bottom: 16px;
}

.loader-wave span {
  width: 8px;
  height: 8px;
  background: #6366f1;
  border-radius: 50%;
  animation: wave 1s infinite alternate;
}

@keyframes wave {
  to { transform: translateY(-10px); opacity: 0.3; }
}

.loader-wave span:nth-child(2) { animation-delay: 0.1s; }
.loader-wave span:nth-child(3) { animation-delay: 0.2s; }
.loader-wave span:nth-child(4) { animation-delay: 0.3s; }
.loader-wave span:nth-child(5) { animation-delay: 0.4s; }

.generating-state {
  text-align: center;
  padding: 60px 0;
  color: #64748b;
}

.fade-enter-active, .fade-leave-active { transition: opacity 0.3s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
