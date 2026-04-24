<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { Notebook, Edit, DocumentChecked, Download, ArrowLeft, ArrowRight, MagicStick, Refresh, Collection } from '@element-plus/icons-vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import request from '@/api/request'

const props = defineProps<{
  id?: string
}>()

const router = useRouter()
const isViewMode = ref(false)
const projects = ref<any[]>([])
const selectedProjectCode = ref('')
const selectedModule = ref('')
const existingModules = ref<string[]>([])

const originalProjectCode = ref('')
const originalModule = ref('')
const recordTitle = ref('')

const isModified = computed(() => {
  return selectedProjectCode.value !== originalProjectCode.value || 
         selectedModule.value !== originalModule.value
})

const activeStep = ref(0)
const requirementDescription = ref('')
const loading = ref(false)

// --- 项目获取 ---
const fetchProjects = async () => {
  try {
    const res = await request.get('/config/projects')
    projects.value = Array.isArray(res) ? res : []
  } catch (err) {
    console.error('Failed to fetch projects:', err)
  }
}

const fetchModules = async () => {
  try {
    const res = await axios.get(`${API_BASE}/records`)
    const modules = new Set<string>()
    res.data.forEach((r: any) => {
      if (r.module) modules.add(r.module)
    })
    existingModules.value = Array.from(modules)
  } catch (err) {
    console.error('Failed to fetch modules:', err)
  }
}

// Step 1: 需求点数据
interface RequirementPoint {
  module: string
  feature: string
  description: string
  rules: string[]
  is_new?: boolean // 用于标注融合后的新增项
}
const requirementPoints = ref<RequirementPoint[]>([])

// Step 2: 测试用例数据
interface TestCase {
  id: string
  category?: string
  type: string
  title: string
  precondition: string
  steps: string[]
  test_data: any
  expected_result: string
  priority: string
  remark: string
}
const generatedCases = ref<TestCase[]>([])
const rawAIResponse = ref('')
const showRawDialog = ref(false)
const currentBatch = ref(0)
const totalBatches = ref(0)
const categoryOrder = ['常规功能测试', '边界极限测试', '异常容错测试', '稳定性并发测试']
const categoryStats = computed(() => {
  const counters: Record<string, number> = {}
  for (const category of categoryOrder) counters[category] = 0
  for (const tc of generatedCases.value) {
    const key = tc.category || '常规功能测试'
    counters[key] = (counters[key] || 0) + 1
  }
  return categoryOrder
    .filter(category => (counters[category] || 0) > 0)
    .map(category => ({ category, count: counters[category] || 0 }))
})

// Smart Decompose 状态
const smartLoading = ref(false)
const enhancedPoints = ref<RequirementPoint[]>([])
const mergedPoints = ref<RequirementPoint[]>([])
const similarity = ref('')
const smartStats = ref<{ original_count: number; enhanced_count: number; merged_count: number; new_points: number } | null>(null)
const showSmartResult = ref(false)

// 基础 API 地址
const API_BASE = '/api/testcase-gen'

// 拆解需求
const handleDecompose = async () => {
  if (!requirementDescription.value.trim()) {
    ElMessage.warning('请输入需求描述')
    return
  }
  loading.value = true
  try {
    const res = await axios.post(`${API_BASE}/decompose`, { text: requirementDescription.value })
    requirementPoints.value = res.data.points || []
    activeStep.value = 1
    ElMessage.success('需求拆解完成')
  } catch (err: any) {
    console.error(err)
    ElMessage.error(err.response?.data?.error || '需求拆解失败')
  } finally {
    loading.value = false
  }
}

// AI 智能增强拆解
const handleSmartDecompose = async () => {
  if (requirementPoints.value.length === 0) return
  smartLoading.value = true
  showSmartResult.value = false
  try {
    const res = await axios.post(`${API_BASE}/smart-decompose`, {
      text: requirementDescription.value,
      existing_points: requirementPoints.value
    })
    enhancedPoints.value = res.data.enhanced_points || []
    mergedPoints.value = res.data.merged_points || []
    similarity.value = res.data.similarity || '0'
    smartStats.value = res.data.stats || null
    showSmartResult.value = true
    ElMessage.success('智能增强拆解完成')
  } catch (err: any) {
    console.error(err)
    ElMessage.error(err.response?.data?.error || '智能拆解失败')
  } finally {
    smartLoading.value = false
  }
}

const applyMerged = () => {
  requirementPoints.value = [...mergedPoints.value]
  showSmartResult.value = false
  ElMessage.success(`已应用合并结果，当前共 ${mergedPoints.value.length} 个需求点`)
}

// 生成用例
const handleGenerate = async () => {
  if (requirementPoints.value.length === 0) return
  
  loading.value = true
  rawAIResponse.value = ''
  generatedCases.value = []
  
  const points = requirementPoints.value
  const batchSize = 1 // 与后端保持一致，每批 1 个需求点
  totalBatches.value = Math.ceil(points.length / batchSize)
  currentBatch.value = 0
  
  try {
    const allGenerated: TestCase[] = []
    
    for (let i = 0; i < points.length; i += batchSize) {
      currentBatch.value++
      const batch = points.slice(i, i + batchSize)
      
      const res = await axios.post(`${API_BASE}/generate`, { points: batch })
      
      if (res.data.cases) {
        allGenerated.push(...res.data.cases)
      }
      
      // If the backend returned an error but also partial results (though unlikely with this frontend loop)
      if (res.data.partial && res.data.error) {
        ElMessage.warning(`第 ${currentBatch.value} 批次生成不完整: ${res.data.error}`)
      }
    }
    
    // 合并后统一重新编号
    generatedCases.value = reindexCases(allGenerated)
    activeStep.value = 2
    ElMessage.success('用例生成完成')
  } catch (err: any) {
    console.error(err)
    const errorMsg = err.response?.data?.error || '用例生成失败'
    const rawData = err.response?.data?.raw
    
    if (rawData) {
      rawAIResponse.value = rawData
      ElMessage.error({
        message: `第 ${currentBatch.value} 批次解析失败，可能由于生成的用例过多。你可以查看原始数据。`,
        duration: 5000
      })
    } else {
      ElMessage.error(errorMsg)
    }
  } finally {
    loading.value = false
    currentBatch.value = 0
    totalBatches.value = 0
  }
}

// 导出 Excel
const handleExport = async () => {
  if (generatedCases.value.length === 0) return
  loading.value = true
  try {
    const res = await axios.post(`${API_BASE}/export`, { cases: generatedCases.value }, { responseType: 'blob' })
    const url = window.URL.createObjectURL(new Blob([res.data]))
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', `Generated_Test_Cases_${Date.now()}.xlsx`)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    ElMessage.success('导出 Excel 成功')
  } catch (err: any) {
    console.error(err)
    ElMessage.error('导出失败')
  } finally {
    loading.value = false
  }
}

const prevStep = () => {
  if (activeStep.value > 0) activeStep.value--
}

const resetAll = () => {
  requirementDescription.value = ''
  requirementPoints.value = []
  generatedCases.value = []
  activeStep.value = 0
  isViewMode.value = false
  recordTitle.value = ''
  selectedProjectCode.value = ''
  selectedModule.value = ''
  originalProjectCode.value = ''
  originalModule.value = ''
}

// 保存到历史记录
const handleSave = async () => {
  if (generatedCases.value.length === 0) return
  loading.value = true
  try {
    const title = requirementPoints.value[0]?.feature || '未命名用例集'
    await axios.post(`${API_BASE}/records`, {
      title: title,
      project_code: selectedProjectCode.value,
      module: selectedModule.value,
      requirement_text: requirementDescription.value,
      points: requirementPoints.value,
      cases: generatedCases.value
    })
    ElMessage.success('已保存到历史记录')
    router.push('/testcase_gen/list')
  } catch (err: any) {
    console.error(err)
    ElMessage.error('保存失败')
  } finally {
    loading.value = false
  }
}

const handleUpdate = async () => {
  if (!props.id) return
  loading.value = true
  try {
    const title = requirementPoints.value[0]?.feature || '未命名用例集'
    await axios.put(`${API_BASE}/records/${props.id}`, {
      title: title,
      project_code: selectedProjectCode.value,
      module: selectedModule.value,
      requirement_text: requirementDescription.value,
      points: requirementPoints.value,
      cases: generatedCases.value
    })
    ElMessage.success('历史记录已更新')
    originalProjectCode.value = selectedProjectCode.value
    originalModule.value = selectedModule.value
    router.push('/testcase_gen/list')
  } catch (err: any) {
    console.error(err)
    ElMessage.error('更新失败')
  } finally {
    loading.value = false
  }
}

const goBack = () => {
  router.push('/testcase_gen/list')
}

onMounted(async () => {
  fetchProjects()
  fetchModules()
  if (props.id) {
    loading.value = true
    isViewMode.value = true
    try {
      const res = await axios.get(`${API_BASE}/records/${props.id}`)
      requirementDescription.value = res.data.requirement_text
      requirementPoints.value = res.data.points
      generatedCases.value = res.data.cases
      
      const pCode = res.data.project_code || ''
      const mod = res.data.module || ''
      
      recordTitle.value = res.data.title || ''
      selectedProjectCode.value = pCode
      selectedModule.value = mod
      originalProjectCode.value = pCode
      originalModule.value = mod
      
      activeStep.value = 2 // 直接跳到预览
    } catch (err) {
      ElMessage.error('获取记录详请失败')
      router.push('/testcase_gen/list')
    } finally {
      loading.value = false
    }
  }
})

const getPriorityTag = (p: string) => {
  switch (p) {
    case 'P0': return 'danger'
    case 'P1': return 'warning'
    case 'P2': return 'primary'
    default: return 'info'
  }
}

const getTypeColor = (type: string) => {
  switch (type) {
    case 'POSITIVE': return '#10b981'
    case 'NEGATIVE': return '#ef4444'
    case 'EXCEPTION': return '#f59e0b'
    case 'CONCURRENCY': return '#8b5cf6'
    default: return '#64748b'
  }
}

const getCategoryTagType = (category?: string) => {
  switch (category) {
    case '常规功能测试': return 'success'
    case '边界极限测试': return 'warning'
    case '异常容错测试': return 'danger'
    case '稳定性并发测试': return 'primary'
    default: return 'info'
  }
}

// 重新编号
const reindexCases = (cases: TestCase[]) => {
  const counters: Record<string, number> = {}
  return cases.map(tc => {
    const parts = tc.id.split('-')
    if (parts.length >= 3) {
      const module = parts[1]
      const typeChar = parts[2]
      const key = `${module}-${typeChar}`
      counters[key] = (counters[key] || 0) + 1
      const seq = String(counters[key]).padStart(3, '0')
      return { ...tc, id: `TC-${module}-${typeChar}-${seq}` }
    }
    return tc
  })
}

// 将 TestData 转化为可展示的字符串
const formatTestData = (data: any) => {
  if (data === null || data === undefined) return '-'
  if (typeof data === 'string') return data
  return JSON.stringify(data, null, 2)
}

// Excel 列头
const excelColumns = [
  { label: 'A', prop: 'id', title: '用例ID', width: '180' },
  { label: 'B', prop: 'category', title: '测试分类', width: '150' },
  { label: 'C', prop: 'type', title: '场景类型', width: '120' },
  { label: 'D', prop: 'title', title: '用例标题', width: '220' },
  { label: 'E', prop: 'precondition', title: '前置条件', width: '220' },
  { label: 'F', prop: 'steps', title: '测试步骤', width: '300' },
  { label: 'G', prop: 'test_data', title: '测试数据', width: '200' },
  { label: 'H', prop: 'expected_result', title: '预期结果', width: '300' },
  { label: 'I', prop: 'priority', title: '优先级', width: '100' },
  { label: 'J', prop: 'remark', title: '备注', width: '150' },
]
</script>

<template>
  <div class="testcase-gen">
    <div class="header-section">
      <div class="title-row">
        <el-button :icon="ArrowLeft" circle @click="goBack" class="back-btn" />
        <el-icon :size="24" color="#8b5cf6"><Notebook /></el-icon>
        <h2 class="page-title">{{ isViewMode ? recordTitle : '智能测试用例生成' }}</h2>
        <div class="header-actions">
           <el-button v-if="activeStep > 0 && !isViewMode" link @click="resetAll" :icon="Refresh">重新开始</el-button>
        </div>
      </div>
      <p class="page-desc" v-if="!isViewMode">基于 AI 自动完成需求拆解，并生成更聚焦于 App 功能测试的高价值用例。</p>
    </div>

    <!-- 步骤条 -->
    <div class="steps-wrapper" v-if="!isViewMode">
      <el-steps :active="activeStep" finish-status="success" align-center>
        <el-step title="需求输入" :icon="Edit" />
        <el-step title="需求拆解" :icon="MagicStick" />
        <el-step title="生成完成" :icon="DocumentChecked" />
      </el-steps>
    </div>

    <div class="content-body">
      <transition name="fade" mode="out-in">
        <!-- Step 0: 需求输入 -->
        <div v-if="activeStep === 0" key="step0">
          <el-card class="glass-card" shadow="never">
            <template #header>
              <div class="card-header">
                <span class="header-text">1. 原始需求描述</span>
                <span class="header-tip">支持 PRD 文本、接口定义、或非结构化功能描述</span>
              </div>
            </template>
            <el-input
              v-model="requirementDescription"
              type="textarea"
              :rows="12"
              placeholder="例如：系统需要支持用户登录功能。
1. 账号为 5-12 位字母数字。
2. 密码必须包含特殊字符。
3. 连续失败 5 次触发 30 分钟锁定。
4. 后端需要处理高并发下的登录事务一致性..."
              resize="none"
            />
            <div class="action-row">
              <el-button 
                type="primary" 
                size="large"
                class="gradient-btn"
                :loading="loading" 
                @click="handleDecompose"
              >
                开始需求拆解
                <el-icon class="el-icon--right"><ArrowRight /></el-icon>
              </el-button>
            </div>
          </el-card>
        </div>

        <!-- Step 1: 需求点确认 -->
        <div v-else-if="activeStep === 1" key="step1">
          <el-card class="glass-card" shadow="never">
            <template #header>
              <div class="card-header">
                <span class="header-text">2. AI 需求分析报告</span>
                <div class="header-actions">
                  <el-button @click="prevStep" :icon="ArrowLeft">上一步</el-button>
                </div>
              </div>
            </template>
            
            <div class="requirement-grid">
              <div v-for="(point, idx) in requirementPoints" :key="idx" class="point-item">
                <div class="point-header">
                  <el-tag size="small" type="info" effect="plain">{{ point.module }}</el-tag>
                  <span class="point-title">{{ point.feature }}</span>
                </div>
                <p class="point-desc">{{ point.description }}</p>
                <div class="point-rules">
                  <div v-for="(rule, rIdx) in point.rules" :key="rIdx" class="rule-tag">
                    <el-icon size="10" style="margin-right: 4px"><Edit /></el-icon>
                    {{ rule }}
                  </div>
                </div>
              </div>
            </div>

            <!-- 智能增强拆解结果展示 -->
            <div v-if="showSmartResult" class="smart-result-panel">
              <el-divider content-position="left">
                <el-icon><MagicStick /></el-icon>
                AI 增强拆解结果
              </el-divider>

              <div class="stats-row">
                <el-tag type="primary">初始因子: {{ smartStats?.original_count }}</el-tag>
                <el-tag type="success">增强因子: {{ smartStats?.enhanced_count }}</el-tag>
                <el-tag type="warning">融合结果: {{ smartStats?.merged_count }}</el-tag>
                <el-tag v-if="(smartStats?.new_points ?? 0) > 0" type="danger" effect="dark">
                  +{{ smartStats?.new_points }} 差异项
                </el-tag>
                <div class="similarity-meter">
                  <span class="label">需求重合度:</span>
                  <el-progress 
                    :percentage="parseFloat(similarity)" 
                    :stroke-width="12" 
                    :format="(p: number) => p + '%'"
                    class="sim-bar"
                    :status="parseFloat(similarity) > 80 ? 'success' : 'warning'"
                  />
                </div>
              </div>

              <div class="enhanced-grid">
                <div v-for="(point, idx) in mergedPoints" :key="'m'+idx" :class="['point-item', point.is_new ? 'is-new' : '']">
                  <div class="point-header">
                    <el-tag size="small" :type="point.is_new ? 'danger' : 'info'" effect="plain">{{ point.module }}</el-tag>
                    <span class="point-title">{{ point.feature }}</span>
                    <el-tag v-if="point.is_new" size="small" type="danger" effect="dark" style="margin-left: auto">新增</el-tag>
                  </div>
                  <p class="point-desc">{{ point.description }}</p>
                </div>
              </div>

              <div class="merge-action">
                <el-button type="success" @click="applyMerged" :icon="MagicStick">
                  应用合并结果 ({{ mergedPoints.length }} 个需求点)
                </el-button>
              </div>
            </div>

            <div class="action-row">
              <el-button
                v-if="!isViewMode"
                type="info"
                :loading="smartLoading"
                @click="handleSmartDecompose"
                :icon="MagicStick"
              >
                AI 智能增强拆解
              </el-button>
              <el-button 
                type="primary" 
                size="large"
                class="gradient-btn"
                :loading="loading" 
                @click="handleGenerate"
              >
                确认并生成聚焦用例
                <el-icon class="el-icon--right"><MagicStick /></el-icon>
              </el-button>
              <el-button v-if="rawAIResponse" type="warning" @click="showRawDialog = true" style="margin-left: 12px">
                查看解析失败数据
              </el-button>
            </div>
          </el-card>
        </div>

        <!-- Step 2: Excel 风格预览 -->
        <div v-else-if="activeStep === 2" key="step2">
          <div class="spreadsheet-container">
            <div class="spreadsheet-toolbar">
              <div class="toolbar-left">
                <el-tag type="success" effect="dark" class="res-tag">生成成功: {{ generatedCases.length }} 条用例</el-tag>
                <el-tag
                  v-for="item in categoryStats"
                  :key="item.category"
                  :type="getCategoryTagType(item.category)"
                  effect="plain"
                  class="res-tag category-tag"
                >
                  {{ item.category }}: {{ item.count }}
                </el-tag>
              </div>
              <div class="toolbar-right">
                <el-button v-if="!isViewMode" @click="prevStep" :icon="ArrowLeft">回退修改</el-button>
                <el-button v-if="!isViewMode" type="primary" @click="handleSave" :icon="Collection" :loading="loading">保存到历史</el-button>
                <el-button v-if="isViewMode && isModified" type="primary" @click="handleUpdate" :icon="DocumentChecked" :loading="loading">更新记录</el-button>
                
                <el-select
                  v-model="selectedModule"
                  placeholder="功能模块"
                  style="width: 140px; margin-right: 12px;"
                  filterable
                  allow-create
                  default-first-option
                  clearable
                >
                  <el-option
                    v-for="m in existingModules"
                    :key="m"
                    :label="m"
                    :value="m"
                  />
                </el-select>

                <el-select
                  v-model="selectedProjectCode"
                  placeholder="所属项目"
                  style="width: 140px; margin-right: 12px;"
                  clearable
                >
                  <el-option
                    v-for="p in projects"
                    :key="p.project_code"
                    :label="p.project_name"
                    :value="p.project_code"
                  />
                </el-select>

                <el-button type="success" @click="handleExport" :icon="Download">下载 Excel 文件 (.xlsx)</el-button>
              </div>
            </div>

            <!-- Virtual Excel Table -->
            <div class="excel-frame">
              <el-table 
                :data="generatedCases" 
                border 
                stripe 
                row-key="id"
                class="excel-table"
                max-height="650"
                style="width: 100%"
                header-cell-class-name="excel-header-cell"
                cell-class-name="excel-cell"
              >
                <!-- Row Index Column -->
                <el-table-column type="index" label=" " width="60" align="center" fixed />

                <!-- Excel Columns A-I -->
                <el-table-column
                  v-for="col in excelColumns"
                  :key="col.prop"
                  :prop="col.prop"
                  :min-width="col.width"
                >
                  <template #header>
                    <div class="excel-col-header">
                       <span class="col-abc">{{ col.label }}</span>
                       <span class="col-title">{{ col.title }}</span>
                    </div>
                  </template>
                  <template #default="{ row }">
                    <!-- Type Column -->
                    <div v-if="col.prop === 'category'" class="prio-cell">
                      <el-tag :type="getCategoryTagType(row.category)" size="small">{{ row.category || '常规功能测试' }}</el-tag>
                    </div>

                    <!-- Type Column -->
                    <div v-else-if="col.prop === 'type'" class="type-cell">
                      <span :style="{ color: getTypeColor(row.type) }">{{ row.type }}</span>
                    </div>
                    
                    <!-- Steps Column -->
                    <div v-else-if="col.prop === 'steps'" class="steps-cell">
                      <div v-for="(step, sIdx) in row.steps" :key="sIdx" class="step-item">
                        {{ sIdx + 1 }}. {{ step }}
                      </div>
                    </div>
                    
                    <!-- Test Data Column -->
                    <div v-else-if="col.prop === 'test_data'" class="data-cell">
                      <pre>{{ formatTestData(row.test_data) }}</pre>
                    </div>
                    
                    <!-- Priority Column -->
                    <div v-else-if="col.prop === 'priority'" class="prio-cell">
                      <el-tag :type="getPriorityTag(row.priority)" size="small">{{ row.priority }}</el-tag>
                    </div>
                    
                    <!-- Default Text Column -->
                    <div v-else class="text-cell">
                      {{ row[col.prop as keyof TestCase] || '-' }}
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </div>
        </div>
      </transition>

      <!-- 错误原始数据对话框 -->
      <el-dialog v-model="showRawDialog" title="AI 原始响应数据 (调试用)" width="60%">
        <div class="raw-alert">
          <el-alert title="提示" type="warning" description="生成的 JSON 可能由于长度限制被截断，你可以手动复制完整部分或减少需求点后再重试。" show-icon :closable="false" />
        </div>
        <pre class="raw-viewer">{{ rawAIResponse }}</pre>
        <template #footer>
          <el-button @click="showRawDialog = false">关闭</el-button>
        </template>
      </el-dialog>
    </div>

    <!-- 全局加载遮罩 (增强版) -->
    <div v-if="loading && activeStep === 1" class="loading-overlay">
       <el-icon class="is-loading" :size="40"><Refresh /></el-icon>
       <p class="loading-text">
         {{ totalBatches > 1 ? `正在生成第 ${currentBatch}/${totalBatches} 批次用例...` : 'AI 正在深度思考并生成聚焦功能测试用例...' }}
       </p>
       <p class="loading-sub">系统会优先收敛主流程与高风险场景，减少低价值重复用例</p>
    </div>
  </div>
</template>

<style scoped>
.testcase-gen {
  padding: 24px;
  max-width: 1300px;
  margin: 0 auto;
}

.header-section {
  margin-bottom: 24px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  color: #0f172a;
  margin: 0;
  letter-spacing: -0.8px;
}

.page-desc {
  font-size: 15px;
  color: #64748b;
}

.steps-wrapper {
  margin-bottom: 32px;
}

.glass-card {
  border-radius: 16px;
  border: 1px solid #e2e8f0;
  background: #fff;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-text {
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.header-tip {
  font-size: 12px;
  color: #94a3b8;
  margin-left: 12px;
  font-weight: 400;
}

.action-row {
  margin-top: 24px;
  display: flex;
  justify-content: flex-end;
}

.gradient-btn {
  background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
  border: none;
  font-weight: 600;
}

/* 需求拆解样式 */
.requirement-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.point-item {
  padding: 18px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
}

.point-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.point-title {
  font-weight: 700;
  color: #1e293b;
}

.point-desc {
  font-size: 13.5px;
  color: #475569;
  line-height: 1.6;
  margin-bottom: 14px;
}

.rule-tag {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 6px;
  display: flex;
  align-items: center;
}

/* 智能增强拆解面板 */
.smart-result-panel {
  margin-top: 24px;
  padding: 20px;
  background: linear-gradient(135deg, #f0fdf4 0%, #ecfdf5 100%);
  border: 1px solid #bbf7d0;
  border-radius: 12px;
}

.stats-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.enhanced-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.point-item.is-new {
  border: 1.5px dashed #f87171;
  background: #fef2f2;
}

.similarity-meter {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  min-width: 250px;
  margin-left: 20px;
}

.similarity-meter .label {
  font-size: 13px;
  font-weight: 600;
  color: #475569;
  white-space: nowrap;
}

.sim-bar {
  flex: 1;
}

.back-btn {
  margin-right: 4px;
}

/* Excel 风格展示 */
.spreadsheet-container {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.spreadsheet-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 4px;
}

.excel-frame {
  border: 1px solid #cbd5e1;
  border-radius: 4px;
  overflow: hidden;
  box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1);
}

:deep(.excel-table) {
  --el-table-border-color: #cbd5e1;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
}

:deep(.excel-header-cell) {
  background-color: #f1f5f9 !important;
  color: #475569;
  height: 60px !important;
  border-bottom: 2px solid #cbd5e1 !important;
}

:deep(.excel-cell) {
  font-size: 13px;
  padding: 8px 0 !important;
}

.excel-col-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  line-height: 1.2;
}

.col-abc {
  font-size: 11px;
  font-weight: 800;
  color: #94a3b8;
}

.col-title {
  font-size: 13px;
  font-weight: 700;
  color: #334155;
}

.type-cell {
  font-weight: 800;
  font-size: 12px;
  text-align: center;
}

.steps-cell {
  white-space: pre-wrap;
  line-height: 1.6;
}

.step-item {
  margin-bottom: 4px;
}

.data-cell pre {
  margin: 0;
  background: #f8fafc;
  padding: 8px;
  border-radius: 4px;
  font-size: 11px;
  max-width: 100%;
  overflow-x: auto;
  border: 1px dashed #e2e8f0;
}

.text-cell {
  white-space: pre-wrap;
  line-height: 1.5;
}

.res-tag {
  font-weight: 700;
}

/* 动画 */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.4s ease, transform 0.4s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

.loading-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(4px);
  z-index: 2001;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.loading-text {
  margin-top: 20px;
  font-size: 18px;
  font-weight: 700;
  color: #8b5cf6;
}

.loading-sub {
  margin-top: 8px;
  font-size: 13px;
  color: #94a3b8;
}

.raw-viewer {
  margin-top: 16px;
  background: #0f172a;
  color: #f8fafc;
  padding: 16px;
  border-radius: 8px;
  font-family: monospace;
  font-size: 12px;
  max-height: 400px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.raw-alert {
  margin-bottom: 12px;
}
</style>
