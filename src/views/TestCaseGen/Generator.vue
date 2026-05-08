<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { Edit, DocumentChecked, Download, ArrowLeft, ArrowRight, MagicStick, Refresh, Collection, Loading } from '@element-plus/icons-vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'
import { useRouter } from 'vue-router'
import request from '@/api/request'
import { useTestcaseGenerationRunStore } from '@/stores'

const props = defineProps<{
  id?: string
}>()

const router = useRouter()
const testcaseGenerationRunStore = useTestcaseGenerationRunStore()
const isViewMode = ref(false)
const projects = ref<any[]>([])
const selectedProjectCode = ref('')
const selectedModule = ref('')
const existingModules = ref<string[]>([])

const originalProjectCode = ref('')
const originalModule = ref('')
const recordTitle = ref('')

const activeStep = ref(0)
const requirementLink = ref('')
const requirementDescription = ref('')
const loading = ref(false)

interface TestcaseAIModelMeta {
  provider?: string
  model?: string
}

interface TestcaseAIModelMap {
  decompose?: TestcaseAIModelMeta
  smart_decompose?: TestcaseAIModelMeta
  generate?: TestcaseAIModelMeta
  review?: TestcaseAIModelMeta
}

interface ReviewRole {
  key: string
  name: string
  description: string
  identity_md: string
}

interface ReviewResult {
  role_key: string
  role_name: string
  identity_md: string
  overall_conclusion: string
  highlights: string[]
  missing_coverage: string[]
  suggested_new_cases: string[]
  suggested_merge_or_drop: string[]
  risk_level: string
  review_focus: string[]
  reviewed_at: string
  ai_provider?: string
  ai_model?: string
}

interface ReviewInsightItem {
  text: string
  type: string
  roles: string[]
}

interface GenerationUndoSnapshot {
  requirementPoints: RequirementPoint[]
  generatedRequirementPoints: RequirementPoint[]
  generatedCases: TestCase[]
  generationStatuses: GenerationPointStatus[]
  currentBatch: number
  totalBatches: number
  generationInProgress: boolean
  generationError: string
  currentGeneratingPointTitle: string
  baselineCases: TestCase[]
  optimizedRemovedCases: TestCase[]
  optimizedPreviewActive: boolean
  reviewResults: Record<string, ReviewResult>
  analyzerUsed: boolean
  activeStep: number
}

interface ReviewUndoSnapshot {
  reviewResults: Record<string, ReviewResult>
  currentReviewModel: string
  currentReviewProvider: string
}

interface OptimizeUndoSnapshot {
  generatedCases: TestCase[]
  baselineCases: TestCase[]
  optimizedRemovedCases: TestCase[]
  optimizedPreviewActive: boolean
  reviewResults: Record<string, ReviewResult>
  activeStep: number
  currentReviewModel: string
  currentReviewProvider: string
  analyzerUsed: boolean
}

const testcaseAIModels = ref<TestcaseAIModelMap>({})
const currentDecomposeModel = ref('')
const currentDecomposeProvider = ref('')
const analyzerUsed = ref(false)
const reviewRoles = ref<ReviewRole[]>([])
const reviewResults = ref<Record<string, ReviewResult>>({})
const selectedReviewRoleKey = ref('product_manager')
const participatingReviewRoleKeys = ref<string[]>([])
const reviewLoading = ref(false)
const optimizeLoading = ref(false)
const currentReviewModel = ref('')
const currentReviewProvider = ref('')
const decomposeLoadingText = computed(() => {
  const model = testcaseAIModels.value.decompose?.model || currentDecomposeModel.value
  const provider = testcaseAIModels.value.decompose?.provider || currentDecomposeProvider.value
  if (model && provider) {
    return `正在使用 ${provider} / ${model} 进行需求拆解...`
  }
  if (model) {
    return `正在使用 ${model} 进行需求拆解...`
  }
  return '正在使用当前配置的需求拆解模型进行需求拆解...'
})
const decomposeResultModelText = computed(() => {
  const model = currentDecomposeModel.value || testcaseAIModels.value.decompose?.model
  const provider = currentDecomposeProvider.value || testcaseAIModels.value.decompose?.provider
  if (model && provider) {
    return `${provider} / ${model}`
  }
  return model || provider || '当前配置模型'
})
const currentReviewRole = computed(() => {
  return reviewRoles.value.find(role => role.key === selectedReviewRoleKey.value) || reviewRoles.value[0] || null
})
const currentReviewResult = computed(() => {
  if (!currentReviewRole.value) return null
  return reviewResults.value[currentReviewRole.value.key] || null
})
const reviewModelText = computed(() => {
  const model = currentReviewModel.value || testcaseAIModels.value.review?.model || testcaseAIModels.value.generate?.model
  const provider = currentReviewProvider.value || testcaseAIModels.value.review?.provider || testcaseAIModels.value.generate?.provider
  if (model && provider) {
    return `${provider} / ${model}`
  }
  return model || provider || '当前配置模型'
})
const selectedReviewRoles = computed(() => {
  return reviewRoles.value.filter(role => participatingReviewRoleKeys.value.includes(role.key))
})
const reviewReadyRoleResults = computed(() => {
  return selectedReviewRoles.value
    .map(role => reviewResults.value[role.key])
    .filter(Boolean) as ReviewResult[]
})
const generationUndoSnapshot = ref<GenerationUndoSnapshot | null>(null)
const reviewUndoSnapshot = ref<ReviewUndoSnapshot | null>(null)
const optimizeUndoSnapshot = ref<OptimizeUndoSnapshot | null>(null)

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

const fetchTestcaseAIModels = async () => {
  try {
    const res = await axios.get(`${API_BASE}/models`)
    testcaseAIModels.value = res.data || {}
  } catch (err) {
    console.error('Failed to fetch testcase AI models:', err)
  }
}

const fetchReviewRoles = async () => {
  try {
    const res = await axios.get(`${API_BASE}/review/roles`)
    reviewRoles.value = Array.isArray(res.data?.roles) ? res.data.roles : []
    if (participatingReviewRoleKeys.value.length === 0) {
      participatingReviewRoleKeys.value = reviewRoles.value.map(role => role.key)
    }
    if (!reviewRoles.value.some(role => role.key === selectedReviewRoleKey.value) && reviewRoles.value[0]) {
      selectedReviewRoleKey.value = reviewRoles.value[0].key
    }
  } catch (err) {
    console.error('Failed to fetch review roles:', err)
  }
}

const cloneRequirementPoints = (points: RequirementPoint[]) => points.map(point => ({
  ...point,
  rules: [...point.rules]
}))

const cloneTestCases = (cases: TestCase[]) => cases.map(item => ({
  ...item,
  steps: [...item.steps],
  test_data: typeof item.test_data === 'object' && item.test_data !== null
    ? JSON.parse(JSON.stringify(item.test_data))
    : item.test_data
}))

const cloneReviewResults = (results: Record<string, ReviewResult>) => JSON.parse(JSON.stringify(results || {})) as Record<string, ReviewResult>

const normalizeCaseText = (value: any) => {
  if (value === null || value === undefined) return ''
  return String(value).trim().replace(/\s+/g, ' ')
}

const buildExactCaseSignature = (item: TestCase) => {
  return [
    normalizeCaseText(item.source_module),
    normalizeCaseText(item.source_feature),
    normalizeCaseText(item.category),
    normalizeCaseText(item.type),
    normalizeCaseText(item.title),
    normalizeCaseText(item.precondition),
    Array.isArray(item.steps) ? item.steps.map(step => normalizeCaseText(step)).join('|') : '',
    normalizeCaseText(typeof item.test_data === 'string' ? item.test_data : JSON.stringify(item.test_data ?? '')),
    normalizeCaseText(item.expected_result),
    normalizeCaseText(item.priority),
    normalizeCaseText(item.remark)
  ].join('::')
}

const buildLooseCaseSignature = (item: TestCase) => {
  return [
    normalizeCaseText(item.source_module),
    normalizeCaseText(item.source_feature),
    normalizeCaseText(item.category),
    normalizeCaseText(item.type),
    normalizeCaseText(item.title)
  ].join('::')
}

const applyOptimizedComparison = (previousCases: TestCase[], nextCases: TestCase[]) => {
  const previousExact = new Map(previousCases.map(item => [buildExactCaseSignature(item), item]))
  const previousLoose = new Map(previousCases.map(item => [buildLooseCaseSignature(item), item]))
  const nextExactSet = new Set(nextCases.map(item => buildExactCaseSignature(item)))

  const comparedCases = nextCases.map(item => {
    const exactKey = buildExactCaseSignature(item)
    const looseKey = buildLooseCaseSignature(item)
    if (previousExact.has(exactKey)) {
      return {
        ...item,
        comparison_status: 'initial_kept' as const,
        comparison_note: '初始用例保留'
      }
    }
    if (previousLoose.has(looseKey)) {
      return {
        ...item,
        comparison_status: 'optimized_adjusted' as const,
        comparison_note: '评审后优化调整'
      }
    }
    return {
      ...item,
      comparison_status: 'optimized_new' as const,
      comparison_note: '评审后新增'
    }
  })

  optimizedRemovedCases.value = previousCases.filter(item => !nextExactSet.has(buildExactCaseSignature(item)))
  return comparedCases
}

const normalizeInsightText = (text: string) => {
  return text.trim().replace(/\s+/g, ' ').replace(/[，。；：、,.!?！？]/g, '')
}

const collectReviewInsights = (result: ReviewResult, roleName: string) => {
  return [
    ...(result.highlights || []).map(text => ({ text, type: '已覆盖较好的点', roleName })),
    ...(result.missing_coverage || []).map(text => ({ text, type: '当前遗漏点', roleName })),
    ...(result.suggested_new_cases || []).map(text => ({ text, type: '建议新增用例方向', roleName })),
    ...(result.suggested_merge_or_drop || []).map(text => ({ text, type: '建议合并/删除项', roleName }))
  ]
}

const commonReviewInsights = computed<ReviewInsightItem[]>(() => {
  const insightMap = new Map<string, ReviewInsightItem>()
  for (const role of selectedReviewRoles.value) {
    const result = reviewResults.value[role.key]
    if (!result) continue
    for (const item of collectReviewInsights(result, role.name)) {
      const normalized = normalizeInsightText(item.text)
      if (!normalized) continue
      const key = `${item.type}::${normalized}`
      const existing = insightMap.get(key)
      if (existing) {
        if (!existing.roles.includes(role.name)) {
          existing.roles.push(role.name)
        }
      } else {
        insightMap.set(key, {
          text: item.text,
          type: item.type,
          roles: [role.name]
        })
      }
    }
  }
  return Array.from(insightMap.values()).filter(item => item.roles.length >= 2)
})

const differentiatedReviewInsights = computed<Record<string, ReviewInsightItem[]>>(() => {
  const insightMap = new Map<string, ReviewInsightItem>()
  for (const role of selectedReviewRoles.value) {
    const result = reviewResults.value[role.key]
    if (!result) continue
    for (const item of collectReviewInsights(result, role.name)) {
      const normalized = normalizeInsightText(item.text)
      if (!normalized) continue
      const key = `${item.type}::${normalized}`
      const existing = insightMap.get(key)
      if (existing) {
        if (!existing.roles.includes(role.name)) {
          existing.roles.push(role.name)
        }
      } else {
        insightMap.set(key, {
          text: item.text,
          type: item.type,
          roles: [role.name]
        })
      }
    }
  }

  const grouped: Record<string, ReviewInsightItem[]> = {}
  for (const role of selectedReviewRoles.value) {
    grouped[role.key] = []
  }
  for (const item of insightMap.values()) {
    if (item.roles.length !== 1) continue
    const owner = selectedReviewRoles.value.find(role => role.name === item.roles[0])
    if (!owner) continue
    grouped[owner.key] = [...(grouped[owner.key] || []), item]
  }
  return grouped
})

// Step 1: 需求点数据
interface RequirementPoint {
  module: string
  feature: string
  description: string
  rules: string[]
  is_new?: boolean // 用于标注融合后的新增项
}
const requirementPoints = ref<RequirementPoint[]>([])
const localGeneratedRequirementPoints = ref<RequirementPoint[]>([])
const generatedRequirementPoints = computed<RequirementPoint[]>({
  get: () => (isViewMode.value ? localGeneratedRequirementPoints.value : testcaseGenerationRunStore.generatedRequirementPoints as RequirementPoint[]),
  set: (value) => {
    if (isViewMode.value) {
      localGeneratedRequirementPoints.value = value
      return
    }
    testcaseGenerationRunStore.generatedRequirementPoints = value
  }
})

// Step 2: 测试用例数据
interface TestCase {
  id: string
  source_module?: string
  source_feature?: string
  category?: string
  type: string
  title: string
  precondition: string
  steps: string[]
  test_data: any
  expected_result: string
  priority: string
  remark: string
  comparison_status?: 'initial_kept' | 'optimized_new' | 'optimized_adjusted'
  comparison_note?: string
}
const generatedCases = computed<TestCase[]>({
  get: () => (isViewMode.value ? localGeneratedCases.value : testcaseGenerationRunStore.generatedCases as TestCase[]),
  set: (value) => {
    if (isViewMode.value) {
      localGeneratedCases.value = value
      return
    }
    testcaseGenerationRunStore.generatedCases = value
  }
})
const localGeneratedCases = ref<TestCase[]>([])
const baselineCases = ref<TestCase[]>([])
const optimizedRemovedCases = ref<TestCase[]>([])
const optimizedPreviewActive = ref(false)
const rawAIResponse = ref('')
const showRawDialog = ref(false)
const currentBatch = computed<number>({
  get: () => testcaseGenerationRunStore.currentBatch,
  set: (value) => {
    testcaseGenerationRunStore.currentBatch = value
  }
})
const totalBatches = computed<number>({
  get: () => testcaseGenerationRunStore.totalBatches,
  set: (value) => {
    testcaseGenerationRunStore.totalBatches = value
  }
})
const generationInProgress = computed<boolean>({
  get: () => testcaseGenerationRunStore.generationInProgress,
  set: (value) => {
    testcaseGenerationRunStore.generationInProgress = value
  }
})
const generationError = computed<string>({
  get: () => testcaseGenerationRunStore.generationError,
  set: (value) => {
    testcaseGenerationRunStore.generationError = value
  }
})
const currentGeneratingPointTitle = computed<string>({
  get: () => testcaseGenerationRunStore.currentGeneratingPointTitle,
  set: (value) => {
    testcaseGenerationRunStore.currentGeneratingPointTitle = value
  }
})

interface GenerationPointStatus {
  key: string
  module: string
  feature: string
  status: 'pending' | 'running' | 'done' | 'error'
  caseCount: number
  error?: string
}

const generationStatuses = computed<GenerationPointStatus[]>({
  get: () => testcaseGenerationRunStore.generationStatuses as GenerationPointStatus[],
  set: (value) => {
    testcaseGenerationRunStore.generationStatuses = value
  }
})
const categoryOrder = ['常规功能测试', '边界极限测试', '异常容错测试', '稳定性并发测试']
const workflowSteps = [
  {
    title: '需求输入',
    description: '录入原始需求内容'
  },
  {
    title: '需求拆解',
    description: '确认 AI 拆解结果'
  },
  {
    title: '用例生成',
    description: '逐批生成并预览用例'
  },
  {
    title: '用例评审',
    description: '按角色视角完成评审'
  }
] as const
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
const persistedRequirementPoints = computed(() => {
  return generatedRequirementPoints.value.length > 0 ? generatedRequirementPoints.value : requirementPoints.value
})
const saveButtonText = computed(() => {
  if (optimizedPreviewActive.value) return '保存优化结果'
  if (activeStep.value >= 3) return '保存评审进度'
  if (activeStep.value >= 1) return '保存当前进度'
  return '保存到历史'
})
const optimizationSummary = computed(() => {
  const nextCases = generatedCases.value.filter(item => !!item.comparison_status)
  return {
    kept: nextCases.filter(item => item.comparison_status === 'initial_kept').length,
    added: nextCases.filter(item => item.comparison_status === 'optimized_new').length,
    adjusted: nextCases.filter(item => item.comparison_status === 'optimized_adjusted').length,
    removed: optimizedRemovedCases.value.length
  }
})

// Smart Decompose 状态
const smartLoading = ref(false)
const enhancedPoints = ref<RequirementPoint[]>([])
const mergedPoints = ref<RequirementPoint[]>([])
const selectedSmartPointKeys = ref<string[]>([])
const similarity = ref('')
const smartStats = ref<{ original_count: number; enhanced_count: number; merged_count: number; new_points: number } | null>(null)
const showSmartResult = ref(false)
const displayedRequirementPoints = computed(() => {
  return showSmartResult.value ? mergedPoints.value : requirementPoints.value
})
const selectedMergedPoints = computed(() => {
  if (!showSmartResult.value) return requirementPoints.value
  return mergedPoints.value.filter(point => !point.is_new || selectedSmartPointKeys.value.includes(getRequirementPointKey(point)))
})
const completedGenerationCount = computed(() => generationStatuses.value.filter(item => item.status === 'done').length)
const canUndoGeneration = computed(() => generationUndoSnapshot.value !== null)
const canUndoReview = computed(() => reviewUndoSnapshot.value !== null)
const canUndoOptimize = computed(() => optimizeUndoSnapshot.value !== null)

// 基础 API 地址
const API_BASE = '/api/testcase-gen'
const getRequirementPointKey = (point: RequirementPoint) => `${point.module}::${point.feature}`
const isSmartPointSelected = (point: RequirementPoint) => !point.is_new || selectedSmartPointKeys.value.includes(getRequirementPointKey(point))
const toggleSmartPointSelection = (point: RequirementPoint) => {
  if (!showSmartResult.value || !point.is_new) return
  const key = getRequirementPointKey(point)
  if (selectedSmartPointKeys.value.includes(key)) {
    selectedSmartPointKeys.value = selectedSmartPointKeys.value.filter(item => item !== key)
    return
  }
  selectedSmartPointKeys.value = [...selectedSmartPointKeys.value, key]
}

const captureGenerationUndoSnapshot = () => {
  generationUndoSnapshot.value = {
    requirementPoints: cloneRequirementPoints(requirementPoints.value),
    generatedRequirementPoints: cloneRequirementPoints(generatedRequirementPoints.value),
    generatedCases: cloneTestCases(generatedCases.value),
    generationStatuses: JSON.parse(JSON.stringify(generationStatuses.value)) as GenerationPointStatus[],
    currentBatch: currentBatch.value,
    totalBatches: totalBatches.value,
    generationInProgress: generationInProgress.value,
    generationError: generationError.value,
    currentGeneratingPointTitle: currentGeneratingPointTitle.value,
    baselineCases: cloneTestCases(baselineCases.value),
    optimizedRemovedCases: cloneTestCases(optimizedRemovedCases.value),
    optimizedPreviewActive: optimizedPreviewActive.value,
    reviewResults: cloneReviewResults(reviewResults.value),
    analyzerUsed: analyzerUsed.value,
    activeStep: activeStep.value
  }
}

const undoLastGeneration = () => {
  if (!generationUndoSnapshot.value) return
  const snapshot = generationUndoSnapshot.value
  requirementPoints.value = cloneRequirementPoints(snapshot.requirementPoints)
  generatedRequirementPoints.value = cloneRequirementPoints(snapshot.generatedRequirementPoints)
  generatedCases.value = cloneTestCases(snapshot.generatedCases)
  generationStatuses.value = JSON.parse(JSON.stringify(snapshot.generationStatuses)) as GenerationPointStatus[]
  currentBatch.value = snapshot.currentBatch
  totalBatches.value = snapshot.totalBatches
  generationInProgress.value = snapshot.generationInProgress
  generationError.value = snapshot.generationError
  currentGeneratingPointTitle.value = snapshot.currentGeneratingPointTitle
  baselineCases.value = cloneTestCases(snapshot.baselineCases)
  optimizedRemovedCases.value = cloneTestCases(snapshot.optimizedRemovedCases)
  optimizedPreviewActive.value = snapshot.optimizedPreviewActive
  reviewResults.value = cloneReviewResults(snapshot.reviewResults)
  analyzerUsed.value = snapshot.analyzerUsed
  activeStep.value = snapshot.activeStep
  generationUndoSnapshot.value = null
  ElMessage.success('已回退到本轮生成前的状态')
}

const captureReviewUndoSnapshot = () => {
  reviewUndoSnapshot.value = {
    reviewResults: cloneReviewResults(reviewResults.value),
    currentReviewModel: currentReviewModel.value,
    currentReviewProvider: currentReviewProvider.value
  }
}

const undoLastReview = () => {
  if (!reviewUndoSnapshot.value) return
  const snapshot = reviewUndoSnapshot.value
  reviewResults.value = cloneReviewResults(snapshot.reviewResults)
  currentReviewModel.value = snapshot.currentReviewModel
  currentReviewProvider.value = snapshot.currentReviewProvider
  reviewUndoSnapshot.value = null
  ElMessage.success('已恢复到上一轮评审前的状态')
}

const captureOptimizeUndoSnapshot = () => {
  optimizeUndoSnapshot.value = {
    generatedCases: cloneTestCases(generatedCases.value),
    baselineCases: cloneTestCases(baselineCases.value),
    optimizedRemovedCases: cloneTestCases(optimizedRemovedCases.value),
    optimizedPreviewActive: optimizedPreviewActive.value,
    reviewResults: cloneReviewResults(reviewResults.value),
    activeStep: activeStep.value,
    currentReviewModel: currentReviewModel.value,
    currentReviewProvider: currentReviewProvider.value,
    analyzerUsed: analyzerUsed.value
  }
}

const undoLastOptimize = () => {
  if (!optimizeUndoSnapshot.value) return
  const snapshot = optimizeUndoSnapshot.value
  generatedCases.value = cloneTestCases(snapshot.generatedCases)
  baselineCases.value = cloneTestCases(snapshot.baselineCases)
  optimizedRemovedCases.value = cloneTestCases(snapshot.optimizedRemovedCases)
  optimizedPreviewActive.value = snapshot.optimizedPreviewActive
  reviewResults.value = cloneReviewResults(snapshot.reviewResults)
  activeStep.value = snapshot.activeStep
  currentReviewModel.value = snapshot.currentReviewModel
  currentReviewProvider.value = snapshot.currentReviewProvider
  analyzerUsed.value = snapshot.analyzerUsed
  optimizeUndoSnapshot.value = null
  ElMessage.success('已撤销评审优化，恢复到优化前状态')
}

// 拆解需求
const handleDecompose = async () => {
  const hasDescription = requirementDescription.value.trim().length > 0
  const hasRequirementLink = requirementLink.value.trim().length > 0
  if (!hasDescription && !hasRequirementLink) {
    ElMessage.warning('请输入需求描述或需求链接')
    return
  }
  loading.value = true
  try {
    const res = await axios.post(`${API_BASE}/decompose`, {
      text: requirementDescription.value,
      wiki_url: requirementLink.value
    })
    if (res.data?.source_text) {
      requirementDescription.value = res.data.source_text
    }
    requirementPoints.value = res.data.points || []
    currentDecomposeModel.value = res.data?.ai_model || ''
    currentDecomposeProvider.value = res.data?.ai_provider || ''
    analyzerUsed.value = !!res.data?.analyzer_used
    activeStep.value = 1
    ElMessage.success(requirementLink.value ? '已读取飞书需求并完成拆解' : '需求拆解完成')
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
      existing_points: requirementPoints.value,
      previous_analyzer_used: analyzerUsed.value
    })
    enhancedPoints.value = res.data.enhanced_points || []
    mergedPoints.value = res.data.merged_points || []
    selectedSmartPointKeys.value = mergedPoints.value
      .filter(point => point.is_new)
      .map(point => getRequirementPointKey(point))
    similarity.value = res.data.similarity || '0'
    smartStats.value = res.data.stats || null
    analyzerUsed.value = !!res.data?.analyzer_used
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
  requirementPoints.value = [...selectedMergedPoints.value]
  showSmartResult.value = false
  selectedSmartPointKeys.value = []
  ElMessage.success(`已应用合并结果，当前共 ${requirementPoints.value.length} 个需求点`)
}

const buildRecordPayload = () => {
  const pointsForRecord = cloneRequirementPoints(requirementPoints.value)
  const generatedPointsForRecord = cloneRequirementPoints(persistedRequirementPoints.value)
  const titleSource = generatedPointsForRecord[0]?.feature || pointsForRecord[0]?.feature || recordTitle.value || '未命名用例集'

  return {
    id: props.id,
    title: titleSource,
    project_code: selectedProjectCode.value,
    module: selectedModule.value,
    requirement_link: requirementLink.value,
    requirement_text: requirementDescription.value,
    current_step: activeStep.value,
    analyzer_used: analyzerUsed.value,
    points: pointsForRecord,
    generated_requirement_points: generatedPointsForRecord,
    cases: cloneTestCases(generatedCases.value),
    baseline_cases: cloneTestCases(baselineCases.value),
    optimized_removed_cases: cloneTestCases(optimizedRemovedCases.value),
    optimized_preview_active: optimizedPreviewActive.value,
    review_results: cloneReviewResults(reviewResults.value)
  }
}

// 生成用例
const handleGenerate = async () => {
  const pointsToGenerate = showSmartResult.value && mergedPoints.value.length > 0
    ? selectedMergedPoints.value
    : requirementPoints.value
  if (pointsToGenerate.length === 0) return

  if (generatedCases.value.length > 0 || Object.keys(reviewResults.value).length > 0 || optimizedPreviewActive.value) {
    captureGenerationUndoSnapshot()
  }

  rawAIResponse.value = ''
  generatedRequirementPoints.value = [...pointsToGenerate]
  baselineCases.value = []
  optimizedRemovedCases.value = []
  optimizedPreviewActive.value = false
  reviewResults.value = {}
  currentReviewModel.value = ''
  currentReviewProvider.value = ''
  activeStep.value = 2
  await testcaseGenerationRunStore.startGeneration({
    points: pointsToGenerate,
    text: requirementDescription.value,
    analyzerUsed: analyzerUsed.value
  })

  if (generatedCases.value.length > 0) {
    analyzerUsed.value = testcaseGenerationRunStore.analyzerUsed
    requirementDescription.value = testcaseGenerationRunStore.requirementText || requirementDescription.value
    ElMessage.success('用例生成完成')
  } else if (generationError.value) {
    ElMessage.error(generationError.value)
  }
}

const goToReview = () => {
  if (generatedCases.value.length === 0) {
    ElMessage.warning('请先生成测试用例')
    return
  }
  activeStep.value = 3
}

const toggleReviewRoleParticipation = (roleKey: string) => {
  if (participatingReviewRoleKeys.value.includes(roleKey)) {
    if (participatingReviewRoleKeys.value.length === 1) {
      ElMessage.warning('至少保留一个参与评审的角色')
      return
    }
    participatingReviewRoleKeys.value = participatingReviewRoleKeys.value.filter(key => key !== roleKey)
    return
  }
  participatingReviewRoleKeys.value = [...participatingReviewRoleKeys.value, roleKey]
}

const handleReview = async () => {
  if (selectedReviewRoles.value.length === 0) {
    ElMessage.warning('请至少选择一个参与评审的角色')
    return
  }
  if (generatedCases.value.length === 0) {
    ElMessage.warning('请先生成测试用例')
    return
  }

  captureReviewUndoSnapshot()
  reviewLoading.value = true
  testcaseGenerationRunStore.startReviewSession(selectedReviewRoles.value.length)
  try {
    const reviewRequests = selectedReviewRoles.value.map(role => axios.post(`${API_BASE}/review`, {
      role_key: role.key,
      text: requirementDescription.value,
      points: persistedRequirementPoints.value,
      cases: generatedCases.value,
      analyzer_used: analyzerUsed.value
    }))
    const settled = await Promise.allSettled(reviewRequests)
    const nextResults = { ...reviewResults.value }
    const successRoles: string[] = []
    const failedRoles: string[] = []

    settled.forEach((item, index) => {
      const role = selectedReviewRoles.value[index]
      if (!role) return
      if (item.status === 'fulfilled' && item.value.data?.result) {
        nextResults[role.key] = item.value.data.result
        currentReviewModel.value = item.value.data.result.ai_model || currentReviewModel.value
        currentReviewProvider.value = item.value.data.result.ai_provider || currentReviewProvider.value
        if (item.value.data?.analyzer_used) {
          analyzerUsed.value = true
        }
        successRoles.push(role.name)
        testcaseGenerationRunStore.markReviewProgress(successRoles.length)
      } else {
        failedRoles.push(role.name)
      }
    })

    reviewResults.value = nextResults

    if (successRoles.length > 0 && failedRoles.length === 0) {
      ElMessage.success(`已完成 ${successRoles.join('、')} 的评审`)
    } else if (successRoles.length > 0) {
      ElMessage.warning(`已完成 ${successRoles.join('、')} 的评审，${failedRoles.join('、')} 评审失败`)
    } else {
      ElMessage.error('所有角色评审都失败了')
    }
  } catch (err: any) {
    console.error(err)
    ElMessage.error(err.response?.data?.error || '用例评审失败')
  } finally {
    reviewLoading.value = false
    testcaseGenerationRunStore.finishReviewSession()
  }
}

const handleOptimizeCases = async () => {
  const selectedResults = selectedReviewRoles.value.reduce<Record<string, ReviewResult>>((acc, role) => {
    const result = reviewResults.value[role.key]
    if (result) {
      acc[role.key] = result
    }
    return acc
  }, {})

  if (Object.keys(selectedResults).length === 0) {
    ElMessage.warning('请先完成至少一个参与角色的评审')
    return
  }

  captureOptimizeUndoSnapshot()
  optimizeLoading.value = true
  testcaseGenerationRunStore.startOptimizeSession()
  try {
    const previousCases = generatedCases.value.map(item => ({ ...item }))
    const res = await axios.post(`${API_BASE}/review/optimize`, {
      text: requirementDescription.value,
      points: persistedRequirementPoints.value,
      cases: generatedCases.value,
      review_results: selectedResults,
      analyzer_used: analyzerUsed.value
    })
    const optimizedCases = Array.isArray(res.data?.cases) ? res.data.cases : []
    baselineCases.value = previousCases
    optimizedPreviewActive.value = true
    generatedCases.value = reindexCases(applyOptimizedComparison(previousCases, optimizedCases))
    currentReviewModel.value = res.data?.ai_model || currentReviewModel.value
    currentReviewProvider.value = res.data?.ai_provider || currentReviewProvider.value
    if (res.data?.analyzer_used) {
      analyzerUsed.value = true
    }
    activeStep.value = 2
    ElMessage.success('已根据多角色评审意见优化用例，当前预览页展示的是优化后的结果，你可以直接保存优化结果，或继续重新评审确认最新质量')
  } catch (err: any) {
    console.error(err)
    ElMessage.error(err.response?.data?.error || '智能优化用例失败')
  } finally {
    optimizeLoading.value = false
    testcaseGenerationRunStore.finishOptimizeSession()
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
  testcaseGenerationRunStore.reset()
  requirementLink.value = ''
  requirementDescription.value = ''
  requirementPoints.value = []
  localGeneratedRequirementPoints.value = []
  generatedRequirementPoints.value = []
  currentDecomposeModel.value = ''
  currentDecomposeProvider.value = ''
  analyzerUsed.value = false
  localGeneratedCases.value = []
  generatedCases.value = []
  baselineCases.value = []
  optimizedRemovedCases.value = []
  optimizedPreviewActive.value = false
  reviewResults.value = {}
  generationUndoSnapshot.value = null
  reviewUndoSnapshot.value = null
  optimizeUndoSnapshot.value = null
  reviewLoading.value = false
  optimizeLoading.value = false
  currentReviewModel.value = ''
  currentReviewProvider.value = ''
  selectedReviewRoleKey.value = reviewRoles.value[0]?.key || 'product_manager'
  participatingReviewRoleKeys.value = reviewRoles.value.map(role => role.key)
  selectedSmartPointKeys.value = []
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
  if (requirementPoints.value.length === 0 && generatedCases.value.length === 0) {
    ElMessage.warning('当前还没有可保存的阶段数据')
    return
  }
  loading.value = true
  try {
    const res = await axios.post(`${API_BASE}/records`, buildRecordPayload())
    const savedId = res.data?.id
    ElMessage.success('当前阶段进度已保存')
    if (savedId && !props.id) {
      router.push(`/testcase_gen/view/${savedId}`)
      return
    }
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
    await axios.put(`${API_BASE}/records/${props.id}`, buildRecordPayload())
    ElMessage.success('当前阶段进度已更新')
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
  fetchTestcaseAIModels()
  fetchReviewRoles()
  if (!props.id && testcaseGenerationRunStore.hasRecoverableRun) {
    requirementDescription.value = testcaseGenerationRunStore.requirementText
    analyzerUsed.value = testcaseGenerationRunStore.analyzerUsed
    generatedRequirementPoints.value = [...(testcaseGenerationRunStore.generatedRequirementPoints as RequirementPoint[])]
    activeStep.value = 2
  }
  if (props.id) {
    loading.value = true
    isViewMode.value = true
    try {
      const res = await axios.get(`${API_BASE}/records/${props.id}`)
      requirementLink.value = res.data.requirement_link || ''
      requirementDescription.value = res.data.requirement_text
      requirementPoints.value = res.data.points || []
      generatedRequirementPoints.value = res.data.generated_requirement_points || res.data.points || []
      generatedCases.value = res.data.cases || []
      baselineCases.value = res.data.baseline_cases || []
      optimizedRemovedCases.value = res.data.optimized_removed_cases || []
      optimizedPreviewActive.value = !!res.data.optimized_preview_active
      reviewResults.value = res.data.review_results || {}
      analyzerUsed.value = !!res.data.analyzer_used
      generationUndoSnapshot.value = null
      reviewUndoSnapshot.value = null
      optimizeUndoSnapshot.value = null
      
      const pCode = res.data.project_code || ''
      const mod = res.data.module || ''
      
      recordTitle.value = res.data.title || ''
      selectedProjectCode.value = pCode
      selectedModule.value = mod
      originalProjectCode.value = pCode
      originalModule.value = mod
      
      if (typeof res.data.current_step === 'number' && res.data.current_step >= 0) {
        activeStep.value = res.data.current_step
      } else {
        activeStep.value = Object.keys(reviewResults.value).length > 0 ? 3 : 2
      }
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
  { label: 'B', prop: 'source_module', title: '来源模块', width: '150' },
  { label: 'C', prop: 'source_feature', title: '来源需求点', width: '240' },
  { label: 'D', prop: 'category', title: '测试分类', width: '150' },
  { label: 'E', prop: 'type', title: '场景类型', width: '120' },
  { label: 'F', prop: 'title', title: '用例标题', width: '220' },
  { label: 'G', prop: 'precondition', title: '前置条件', width: '220' },
  { label: 'H', prop: 'steps', title: '测试步骤', width: '300' },
  { label: 'I', prop: 'test_data', title: '测试数据', width: '200' },
  { label: 'J', prop: 'expected_result', title: '预期结果', width: '300' },
  { label: 'K', prop: 'priority', title: '优先级', width: '100' },
  { label: 'L', prop: 'remark', title: '备注', width: '150' },
]
</script>

<template>
  <div class="testcase-gen">
    <div class="header-section">
      <div class="page-header">
        <div class="page-header__main">
          <div class="page-header__copy">
            <div class="breadcrumb">测试用例生成 <span class="divider">/</span> {{ isViewMode ? '记录详情' : '智能测试用例生成' }}</div>
            <div class="page-header__title-row">
              <h2 class="page-title">{{ isViewMode ? recordTitle : '智能测试用例生成' }}</h2>
              <span class="page-header__tag">{{ isViewMode ? '历史记录' : 'AI 工作台' }}</span>
            </div>
            <p class="page-desc">
              {{ isViewMode ? '查看并维护已保存的测试用例生成记录。' : '从需求描述到拆解确认，再到用例预览与沉淀，保持一条清晰、连续的生成链路。' }}
            </p>
          </div>
        </div>
        <div class="page-header__actions">
          <el-button text @click="goBack" class="back-text-btn">返回</el-button>
          <el-button v-if="activeStep > 0 && !isViewMode" @click="resetAll" :icon="Refresh">重新开始</el-button>
        </div>
      </div>
    </div>

    <!-- 步骤条 -->
    <div class="steps-wrapper" v-if="!isViewMode">
      <div class="workflow-steps">
        <div
          v-for="(step, index) in workflowSteps"
          :key="step.title"
          :class="[
            'workflow-step-card',
            {
              'is-active': activeStep === index,
              'is-done': activeStep > index,
              'is-pending': activeStep < index
            }
          ]"
        >
          <div class="workflow-step-card__head">
            <div class="workflow-step-card__title-row">
              <div class="workflow-step-card__title">{{ step.title }}</div>
              <span class="workflow-step-card__badge">
                {{ activeStep > index ? '已完成' : activeStep === index ? '进行中' : '待开始' }}
              </span>
            </div>
          </div>
          <div class="workflow-step-card__body">
            <div class="workflow-step-card__desc">{{ step.description }}</div>
            <div class="workflow-step-card__index">
              <el-icon v-if="activeStep > index"><DocumentChecked /></el-icon>
              <span v-else>Part {{ index + 1 }}</span>
            </div>
          </div>
          <div v-if="index < workflowSteps.length - 1" class="workflow-step-card__connector" />
        </div>
      </div>
    </div>

    <div class="content-body">
      <transition name="fade" mode="out-in">
        <!-- Step 0: 需求输入 -->
        <div v-if="activeStep === 0" key="step0">
          <el-card class="panel-card" shadow="never">
            <template #header>
              <div class="card-header">
                <div>
                  <div class="header-text">1. 原始需求描述</div>
                  <div class="header-tip">支持 PRD 文本、接口定义、用户故事或非结构化功能说明，也可以补充对应需求链接。</div>
                </div>
              </div>
            </template>
            <el-input
              v-model="requirementLink"
              class="requirement-link-input"
              placeholder="请输入需求链接，例如飞书文档、Jira、项目需求地址；填入飞书链接后可直接读取正文并拆解"
              clearable
            />
            <el-input
              v-model="requirementDescription"
              type="textarea"
              :rows="14"
              class="requirement-input"
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
                class="primary-btn"
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
          <el-card class="panel-card" shadow="never">
            <template #header>
              <div class="card-header">
                <div>
                  <div class="header-text">2. AI 需求分析结果</div>
                  <div class="header-tip">当前需求拆解模型：{{ decomposeResultModelText }}。先确认拆解结果，再决定是否进行增强拆解和用例生成。</div>
                </div>
                <div class="header-actions">
                  <el-button @click="prevStep" :icon="ArrowLeft">上一步</el-button>
                </div>
              </div>
            </template>

            <div class="analysis-split-layout">
              <div class="analysis-panel source-panel">
                <div class="analysis-panel__header">
                  <div>
                    <div class="analysis-panel__title">需求原文</div>
                    <div class="analysis-panel__tip">展示本次用于需求拆解的原始文本内容。</div>
                  </div>
                  <div class="analysis-panel__badges">
                    <el-tag v-if="analyzerUsed" size="small" type="danger" effect="dark">短剧需求</el-tag>
                    <el-tag size="small" type="primary" effect="plain">原文输入</el-tag>
                  </div>
                </div>
                <pre class="source-text-viewer">{{ requirementDescription || '暂无需求原文' }}</pre>
              </div>

              <div class="analysis-panel result-panel">
                <div class="analysis-panel__header">
                  <div>
                    <div class="analysis-panel__title">拆分需求点</div>
                    <div class="analysis-panel__tip">
                      {{ showSmartResult ? `增强后共整理出 ${displayedRequirementPoints.length} 个需求点卡片，其中新增 ${smartStats?.new_points ?? 0} 个。` : `当前共拆分出 ${requirementPoints.length} 个需求点卡片。` }}
                    </div>
                  </div>
                  <el-tag size="small" :type="showSmartResult ? 'danger' : 'success'" effect="plain">{{ showSmartResult ? '增强结果' : 'AI 结果' }}</el-tag>
                </div>

                <div v-if="showSmartResult" class="inline-smart-summary">
                  <div class="stats-row stats-row--inline">
                    <el-tag type="primary">初始因子: {{ smartStats?.original_count }}</el-tag>
                    <el-tag type="success">增强因子: {{ smartStats?.enhanced_count }}</el-tag>
                    <el-tag type="warning">融合结果: {{ smartStats?.merged_count }}</el-tag>
                    <el-tag v-if="(smartStats?.new_points ?? 0) > 0" type="danger" effect="dark">
                      +{{ smartStats?.new_points }} 差异项
                    </el-tag>
                  </div>
                  <div class="inline-smart-actions">
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
                    <el-button type="success" @click="applyMerged" :icon="MagicStick">
                      应用合并结果 ({{ selectedMergedPoints.length }} 个需求点)
                    </el-button>
                  </div>
                </div>

                <div class="requirement-grid requirement-grid--split">
                  <div
                    v-for="(point, idx) in displayedRequirementPoints"
                    :key="`${point.module}-${point.feature}-${idx}`"
                    :class="[
                      'point-item',
                      point.is_new ? 'is-new' : '',
                      point.is_new && isSmartPointSelected(point) ? 'is-selected-new' : '',
                      point.is_new && !isSmartPointSelected(point) ? 'is-deselected-new' : '',
                      point.is_new ? 'is-toggleable' : ''
                    ]"
                    @click="toggleSmartPointSelection(point)"
                  >
                    <div class="point-header">
                      <el-tag size="small" :type="point.is_new && isSmartPointSelected(point) ? 'danger' : 'info'" effect="plain">{{ point.module }}</el-tag>
                      <span class="point-title">{{ point.feature }}</span>
                      <el-tag
                        v-if="point.is_new"
                        size="small"
                        :type="isSmartPointSelected(point) ? 'danger' : 'info'"
                        :effect="isSmartPointSelected(point) ? 'dark' : 'plain'"
                        style="margin-left: auto"
                      >
                        {{ isSmartPointSelected(point) ? '新增已选' : '新增未选' }}
                      </el-tag>
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
              </div>
            </div>

            <div class="action-row">
              <el-button
                v-if="!isViewMode"
                plain
                size="large"
                class="action-btn"
                :icon="Collection"
                @click="handleSave"
              >
                保存当前进度
              </el-button>
              <el-button
                v-else
                type="primary"
                plain
                size="large"
                class="action-btn"
                :icon="DocumentChecked"
                :loading="loading"
                @click="handleUpdate"
              >
                更新当前进度
              </el-button>
              <el-button
                v-if="!isViewMode"
                type="default"
                size="large"
                class="action-btn"
                :loading="smartLoading"
                @click="handleSmartDecompose"
                :icon="MagicStick"
              >
                AI 智能增强拆解
              </el-button>
              <el-button 
                type="primary" 
                size="large"
                class="action-btn primary-btn"
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

        <!-- Step 2: 用例生成预览 -->
        <div v-else-if="activeStep === 2" key="step2">
          <div class="spreadsheet-container">
            <div v-if="generationStatuses.length > 0" class="generation-progress-card">
              <div class="generation-progress-card__head">
                <div>
                  <div class="generation-progress-card__title">生成进度预览</div>
                  <div class="generation-progress-card__desc">
                    {{ generationInProgress ? `正在生成第 ${completedGenerationCount + 1}/${generationStatuses.length} 个需求点：${currentGeneratingPointTitle || '准备中'}` : `已完成 ${completedGenerationCount}/${generationStatuses.length} 个需求点的用例生成` }}
                  </div>
                </div>
                <div class="generation-progress-card__meta">
                  <el-tag v-if="generationInProgress" type="warning" effect="dark" class="generation-status-tag">
                    <el-icon class="is-loading generation-status-tag__icon"><Loading /></el-icon>
                    持续生成
                  </el-tag>
                  <el-tag v-else type="success" effect="dark">生成完成</el-tag>
                  <el-tag v-if="analyzerUsed" type="danger" effect="dark">短剧需求</el-tag>
                </div>
              </div>
              <div class="generation-point-list">
                <div
                  v-for="item in generationStatuses"
                  :key="item.key"
                  :class="['generation-point-item', `is-${item.status}`]"
                >
                  <div class="generation-point-item__main">
                    <div class="generation-point-item__title">{{ item.module }} / {{ item.feature }}</div>
                    <div class="generation-point-item__meta">
                      <span>{{ item.caseCount }} 条用例</span>
                      <span v-if="item.error" class="generation-point-item__error">{{ item.error }}</span>
                    </div>
                  </div>
                  <el-tag
                    size="small"
                    :type="item.status === 'done' ? 'success' : item.status === 'running' ? 'warning' : item.status === 'error' ? 'danger' : 'info'"
                    effect="plain"
                    class="generation-point-item__tag"
                  >
                    <template v-if="item.status === 'running'">
                      <el-icon class="is-loading generation-point-item__spinner"><Loading /></el-icon>
                    </template>
                    {{ item.status === 'done' ? '已完成' : item.status === 'running' ? '生成中' : item.status === 'error' ? '失败' : '待生成' }}
                  </el-tag>
                </div>
              </div>
              <el-alert
                v-if="generationError"
                class="generation-progress-alert"
                type="warning"
                :closable="false"
                :title="generationError"
                show-icon
              />
            </div>

            <div class="result-summary-card">
              <div class="result-summary-card__main">
                <div class="result-summary-card__title">生成结果预览</div>
                <div class="result-summary-card__desc">
                  {{ generationInProgress ? '用例会按需求点逐批生成并持续补充到下方 Excel 预览中，你可以边查看边滚动表格。' : optimizedPreviewActive ? '当前显示的是基于评审意见优化后的用例结果，你可以先对比初始版本差异，再决定是否重新评审或保存。' : '可在保存前继续调整项目和模块归属，并导出 Excel 文件留档。' }}
                </div>
              </div>
              <div class="toolbar-left">
                <el-tag type="success" effect="dark" class="res-tag">已生成 {{ generatedCases.length }} 条用例</el-tag>
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
            </div>

            <div v-if="optimizedPreviewActive" class="optimization-preview-card">
              <div class="optimization-preview-card__main">
                <div class="optimization-preview-card__title">评审优化预览</div>
                <div class="optimization-preview-card__desc">下方表格已切换为优化后的最新用例，并会标识“初始保留 / 优化调整 / 优化新增”。</div>
              </div>
              <div class="optimization-preview-card__stats">
                <el-tag type="success" effect="dark">保留 {{ optimizationSummary.kept }}</el-tag>
                <el-tag type="warning" effect="dark">调整 {{ optimizationSummary.adjusted }}</el-tag>
                <el-tag type="danger" effect="dark">新增 {{ optimizationSummary.added }}</el-tag>
                <el-tag type="info" effect="plain">移除 {{ optimizationSummary.removed }}</el-tag>
                <el-button v-if="!isViewMode" type="primary" size="small" :icon="Collection" @click="handleSave">
                  保存优化结果
                </el-button>
                <el-button v-if="canUndoOptimize" plain type="warning" size="small" :icon="Refresh" @click="undoLastOptimize">
                  撤销本轮优化
                </el-button>
              </div>
            </div>

            <el-alert
              v-if="optimizedPreviewActive && optimizedRemovedCases.length > 0"
              class="optimization-removed-alert"
              type="warning"
              :closable="false"
              show-icon
            >
              <template #title>
                已从本轮优化结果中移除 {{ optimizedRemovedCases.length }} 条初始用例：
                {{ optimizedRemovedCases.slice(0, 4).map(item => item.title).join('；') }}<span v-if="optimizedRemovedCases.length > 4"> 等</span>
              </template>
            </el-alert>

            <div class="result-config-card">
              <div class="result-config-card__fields">
                <el-select
                  v-model="selectedProjectCode"
                  placeholder="所属项目"
                  class="toolbar-select"
                  clearable
                >
                  <el-option
                    v-for="p in projects"
                    :key="p.project_code"
                    :label="p.project_name"
                    :value="p.project_code"
                  />
                </el-select>

                <el-select
                  v-model="selectedModule"
                  placeholder="功能模块"
                  class="toolbar-select"
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
              </div>

              <div class="toolbar-right">
                <el-button v-if="!isViewMode" @click="prevStep" :icon="ArrowLeft">回退修改</el-button>
                <el-button v-if="!isViewMode && canUndoGeneration" plain type="warning" :icon="Refresh" @click="undoLastGeneration">
                  撤销本轮生成
                </el-button>
                <el-button type="warning" plain @click="goToReview" :icon="DocumentChecked" :disabled="generatedCases.length === 0 || generationInProgress">进入用例评审</el-button>
                <el-button v-if="!isViewMode" type="primary" @click="handleSave" :icon="Collection" :loading="loading" :disabled="generationInProgress || (requirementPoints.length === 0 && generatedCases.length === 0)">{{ saveButtonText }}</el-button>
                <el-button v-if="isViewMode" type="primary" @click="handleUpdate" :icon="DocumentChecked" :loading="loading" :disabled="generationInProgress">更新当前进度</el-button>
                <el-button type="success" @click="handleExport" :icon="Download" :disabled="generatedCases.length === 0">下载 Excel 文件 (.xlsx)</el-button>
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

                <el-table-column v-if="optimizedPreviewActive" prop="comparison_status" label="状态" min-width="140" fixed="left">
                  <template #default="{ row }">
                    <el-tag
                      v-if="row.comparison_status === 'initial_kept'"
                      type="success"
                      size="small"
                    >
                      初始保留
                    </el-tag>
                    <el-tag
                      v-else-if="row.comparison_status === 'optimized_adjusted'"
                      type="warning"
                      size="small"
                    >
                      优化调整
                    </el-tag>
                    <el-tag
                      v-else-if="row.comparison_status === 'optimized_new'"
                      type="danger"
                      size="small"
                    >
                      优化新增
                    </el-tag>
                    <span v-else>-</span>
                  </template>
                </el-table-column>

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

        <!-- Step 3: 用例评审 -->
        <div v-else-if="activeStep === 3" key="step3">
          <div class="review-workbench">
            <el-card class="panel-card review-role-panel" shadow="never">
              <template #header>
                <div class="card-header">
                  <div>
                    <div class="header-text">4. 用例评审</div>
                    <div class="header-tip">切换角色视角后发起评审，AI 会结合角色身份说明、需求原文、需求点和当前用例集给出专业意见。</div>
                  </div>
                  <div class="header-actions">
                    <el-button @click="activeStep = 2" :icon="ArrowLeft">返回生成页</el-button>
                  </div>
                </div>
              </template>

              <div class="review-role-grid">
                <button
                  v-for="role in reviewRoles"
                  :key="role.key"
                  type="button"
                  :class="['review-role-card', { 'is-active': selectedReviewRoleKey === role.key, 'has-result': !!reviewResults[role.key], 'is-participating': participatingReviewRoleKeys.includes(role.key) }]"
                  @click="selectedReviewRoleKey = role.key"
                >
                  <div class="review-role-card__head">
                    <span class="review-role-card__name">{{ role.name }}</span>
                    <div class="review-role-card__tags">
                      <el-tag v-if="reviewResults[role.key]" size="small" type="success" effect="dark">已评审</el-tag>
                      <el-tag
                        size="small"
                        :type="participatingReviewRoleKeys.includes(role.key) ? 'primary' : 'info'"
                        :effect="participatingReviewRoleKeys.includes(role.key) ? 'dark' : 'plain'"
                        @click.stop="toggleReviewRoleParticipation(role.key)"
                      >
                        {{ participatingReviewRoleKeys.includes(role.key) ? '参与评审' : '不参与' }}
                      </el-tag>
                    </div>
                  </div>
                  <div class="review-role-card__desc">{{ role.description }}</div>
                </button>
              </div>

              <div class="review-toolbar">
                <div class="review-toolbar__meta">
                  <el-tag type="primary" effect="plain">评审模型：{{ reviewModelText }}</el-tag>
                  <el-tag v-if="analyzerUsed" type="danger" effect="dark">短剧需求</el-tag>
                  <el-tag type="info" effect="plain">参与角色：{{ selectedReviewRoles.length }} 个</el-tag>
                </div>
                <div class="review-toolbar__actions">
                  <el-button type="primary" :loading="reviewLoading" :icon="MagicStick" @click="handleReview">
                    开始评审
                  </el-button>
                  <el-button v-if="canUndoReview" plain type="warning" :icon="Refresh" @click="undoLastReview" :disabled="reviewLoading || optimizeLoading">
                    撤销本轮评审
                  </el-button>
                  <el-button type="warning" plain :loading="optimizeLoading" :icon="Refresh" @click="handleOptimizeCases" :disabled="!reviewReadyRoleResults.length">
                    根据评审意见智能优化用例
                  </el-button>
                  <el-button v-if="!isViewMode" type="success" @click="handleSave" :icon="Collection" :loading="loading" :disabled="generationInProgress || (requirementPoints.length === 0 && generatedCases.length === 0)">{{ saveButtonText }}</el-button>
                  <el-button v-if="isViewMode" type="success" @click="handleUpdate" :icon="DocumentChecked" :loading="loading" :disabled="generationInProgress">更新当前进度</el-button>
                </div>
              </div>

              <div v-if="reviewReadyRoleResults.length > 0" class="review-consensus-card">
                <div class="review-block-title">多角色评审汇总</div>
                <div class="review-consensus-grid">
                  <div class="review-section">
                    <div class="review-section__title">重合点</div>
                    <ul class="review-list">
                      <li v-for="(item, index) in commonReviewInsights" :key="`common-${index}`">
                        <span class="review-insight-type">{{ item.type }}</span>
                        <span>{{ item.text }}</span>
                        <span class="review-insight-roles">({{ item.roles.join(' / ') }})</span>
                      </li>
                      <li v-if="commonReviewInsights.length === 0" class="is-empty">当前还没有明显重合的评审意见</li>
                    </ul>
                  </div>

                  <div class="review-section">
                    <div class="review-section__title">差异点</div>
                    <div class="review-diff-groups">
                      <div v-for="role in selectedReviewRoles" :key="`diff-${role.key}`" class="review-diff-group">
                        <div class="review-diff-group__title">{{ role.name }}</div>
                        <ul class="review-list">
                          <li v-for="(item, index) in differentiatedReviewInsights[role.key] || []" :key="`${role.key}-diff-${index}`">
                            <span class="review-insight-type">{{ item.type }}</span>
                            <span>{{ item.text }}</span>
                          </li>
                          <li v-if="!(differentiatedReviewInsights[role.key] || []).length" class="is-empty">暂无独有差异点</li>
                        </ul>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div class="review-body">
                <div class="review-identity-card">
                  <div class="review-block-title">角色身份 MD</div>
                  <pre class="review-identity-md">{{ currentReviewRole?.identity_md || '暂无角色身份说明' }}</pre>
                </div>

                <div class="review-result-card">
                  <div class="review-block-title">评审结论</div>
                  <div v-if="currentReviewResult" class="review-result-content">
                    <div class="review-result-summary">
                      <el-tag type="danger" effect="dark">{{ currentReviewResult.risk_level || '中' }}</el-tag>
                      <span class="review-result-time">评审时间：{{ currentReviewResult.reviewed_at }}</span>
                    </div>

                    <div class="review-section">
                      <div class="review-section__title">总体结论</div>
                      <p class="review-section__text">{{ currentReviewResult.overall_conclusion || '暂无总结' }}</p>
                    </div>

                    <div class="review-section-grid">
                      <div class="review-section">
                        <div class="review-section__title">已覆盖较好的点</div>
                        <ul class="review-list">
                          <li v-for="(item, index) in currentReviewResult.highlights" :key="`highlight-${index}`">{{ item }}</li>
                          <li v-if="!currentReviewResult.highlights?.length" class="is-empty">暂无</li>
                        </ul>
                      </div>

                      <div class="review-section">
                        <div class="review-section__title">当前遗漏点</div>
                        <ul class="review-list">
                          <li v-for="(item, index) in currentReviewResult.missing_coverage" :key="`missing-${index}`">{{ item }}</li>
                          <li v-if="!currentReviewResult.missing_coverage?.length" class="is-empty">暂无</li>
                        </ul>
                      </div>
                    </div>

                    <div class="review-section-grid">
                      <div class="review-section">
                        <div class="review-section__title">建议新增用例方向</div>
                        <ul class="review-list">
                          <li v-for="(item, index) in currentReviewResult.suggested_new_cases" :key="`add-${index}`">{{ item }}</li>
                          <li v-if="!currentReviewResult.suggested_new_cases?.length" class="is-empty">暂无</li>
                        </ul>
                      </div>

                      <div class="review-section">
                        <div class="review-section__title">建议合并 / 删除项</div>
                        <ul class="review-list">
                          <li v-for="(item, index) in currentReviewResult.suggested_merge_or_drop" :key="`drop-${index}`">{{ item }}</li>
                          <li v-if="!currentReviewResult.suggested_merge_or_drop?.length" class="is-empty">暂无</li>
                        </ul>
                      </div>
                    </div>

                    <div class="review-section">
                      <div class="review-section__title">本轮评审关注点</div>
                      <div class="review-focus-tags">
                        <el-tag v-for="(item, index) in currentReviewResult.review_focus" :key="`focus-${index}`" type="info" effect="plain">
                          {{ item }}
                        </el-tag>
                        <span v-if="!currentReviewResult.review_focus?.length" class="is-empty">暂无</span>
                      </div>
                    </div>
                  </div>
                  <div v-else class="review-empty-state">
                    <div class="review-empty-state__title">当前角色还没有评审结果</div>
                    <div class="review-empty-state__desc">点击上方按钮后，系统会按当前勾选的参与角色批量完成评审；这里会展示 {{ currentReviewRole?.name || '当前角色' }} 的详细结论。</div>
                  </div>
                </div>
              </div>
            </el-card>
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
         {{ totalBatches > 1 ? `正在生成第 ${currentBatch}/${totalBatches} 批次用例...` : decomposeLoadingText }}
       </p>
       <p class="loading-sub">{{ totalBatches > 1 ? '系统会优先收敛主流程与高风险场景，减少低价值重复用例' : '系统会先读取需求原文，再将其拆分为可落地的功能测试需求点' }}</p>
    </div>
  </div>
</template>

<style scoped>
.testcase-gen {
  padding: 12px 8px 24px;
  max-width: 1360px;
  margin: 0 auto;
}

.header-section {
  margin-bottom: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  background: #fff;
  border: 1px solid #e5edf7;
  border-radius: 16px;
}

.page-header__main {
  display: flex;
  align-items: center;
  gap: 0;
  min-width: 0;
  flex: 1;
}

.page-header__copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.page-header__title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.page-header__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
  align-self: flex-start;
}

.breadcrumb {
  font-size: 13px;
  color: #64748b;
}

.divider {
  margin: 0 4px;
  color: #cbd5e1;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  line-height: 1.2;
}

.page-header__tag {
  display: inline-flex;
  align-items: center;
  height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 12px;
  font-weight: 600;
}

.back-text-btn {
  height: 28px;
  padding: 0 10px;
  border-radius: 8px;
  background: #f4f7fb;
  border: 1px solid #e2e8f0;
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.page-desc {
  margin: 0;
  font-size: 14px;
  color: #64748b;
  line-height: 1.6;
}

.steps-wrapper {
  margin: 20px 0 24px;
}

.workflow-steps {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  padding: 18px;
  background: #fff;
  border: 1px solid #e5edf7;
  border-radius: 16px;
}

.workflow-step-card {
  position: relative;
  padding: 8px 12px;
  border-radius: 12px;
  border: 1px solid #dbe5f1;
  background: #f8fafc;
  min-height: 0;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease, transform 0.2s ease;
}

.workflow-step-card.is-active {
  background: linear-gradient(180deg, #eff6ff 0%, #f8fbff 100%);
  border-color: #60a5fa;
  box-shadow: 0 8px 18px rgba(59, 130, 246, 0.12);
  transform: translateY(-1px);
}

.workflow-step-card.is-done {
  background: linear-gradient(180deg, #f0fdf4 0%, #f8fffb 100%);
  border-color: #86efac;
}

.workflow-step-card.is-pending {
  background: #f8fafc;
  border-color: #e2e8f0;
}

.workflow-step-card__head {
  display: flex;
  align-items: center;
  margin-bottom: 4px;
}

.workflow-step-card__title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.workflow-step-card__body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.workflow-step-card__index {
  min-width: 20px;
  height: 20px;
  padding: 0 8px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  font-size: 11px;
  font-weight: 700;
  color: #475569;
  background: #e2e8f0;
}

.workflow-step-card.is-active .workflow-step-card__index {
  color: #1d4ed8;
  background: #dbeafe;
}

.workflow-step-card.is-done .workflow-step-card__index {
  color: #15803d;
  background: #dcfce7;
}

.workflow-step-card__badge {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  background: #eef2f7;
}

.workflow-step-card.is-active .workflow-step-card__badge {
  color: #1d4ed8;
  background: #dbeafe;
}

.workflow-step-card.is-done .workflow-step-card__badge {
  color: #15803d;
  background: #dcfce7;
}

.workflow-step-card__title {
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.2;
}

.workflow-step-card__desc {
  font-size: 11px;
  line-height: 1.2;
  color: #64748b;
  max-width: 220px;
}

.workflow-step-card__connector {
  position: absolute;
  top: 50%;
  right: -15px;
  width: 16px;
  height: 2px;
  background: linear-gradient(90deg, #cbd5e1 0%, #e2e8f0 100%);
  transform: translateY(-50%);
}

.panel-card {
  border-radius: 16px;
  border: 1px solid #e5edf7;
  background: #fff;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.header-text {
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.header-tip {
  font-size: 12px;
  color: #94a3b8;
  margin-top: 6px;
  font-weight: 400;
}

.requirement-link-input {
  margin-bottom: 14px;
}

.requirement-link-input :deep(.el-input__wrapper) {
  border-radius: 12px;
  background: #f8fafc;
}

.requirement-input :deep(.el-textarea__inner) {
  border-radius: 14px;
  background: #f8fafc;
}

.action-row {
  margin-top: 24px;
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 12px;
}

.primary-btn {
  font-weight: 600;
}

.action-btn {
  min-height: 40px;
  font-weight: 600;
}

/* 需求拆解样式 */
.analysis-split-layout {
  display: grid;
  grid-template-columns: minmax(0, 0.92fr) minmax(0, 1.08fr);
  gap: 18px;
  align-items: start;
}

.analysis-panel {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 16px;
}

.analysis-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.analysis-panel__badges {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.analysis-panel__title {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin-bottom: 4px;
}

.analysis-panel__tip {
  font-size: 12px;
  line-height: 1.5;
  color: #64748b;
}

.source-text-viewer {
  margin: 0;
  min-height: 480px;
  max-height: 760px;
  overflow: auto;
  padding: 16px;
  border-radius: 14px;
  background: #ffffff;
  border: 1px solid #e5edf7;
  color: #334155;
  font-size: 13px;
  line-height: 1.75;
  font-family: inherit;
  white-space: pre-wrap;
  word-break: break-word;
}

.requirement-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 20px;
}

.requirement-grid--split {
  gap: 12px;
}

.point-item {
  padding: 14px 14px 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.requirement-grid--split .point-item:hover {
  transform: translateY(-3px);
  border-color: #cbdcf8;
  background: #fbfdff;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
}

.requirement-grid--split .point-item.is-toggleable {
  cursor: pointer;
}

.requirement-grid--split .point-item.is-deselected-new {
  border: 1px dashed #cbd5e1;
  background: #f1f5f9;
  box-shadow: none;
}

.requirement-grid--split .point-item.is-deselected-new:hover {
  transform: none;
  border-color: #cbd5e1;
  background: #f1f5f9;
  box-shadow: none;
}

.requirement-grid--split .point-item.is-deselected-new .point-title,
.requirement-grid--split .point-item.is-deselected-new .point-desc,
.requirement-grid--split .point-item.is-deselected-new .rule-tag {
  color: #94a3b8;
}

.point-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}

.point-title {
  font-weight: 700;
  color: #1e293b;
  font-size: 13px;
}

.point-desc {
  font-size: 13px;
  color: #475569;
  line-height: 1.55;
  margin-bottom: 10px;
}

.rule-tag {
  font-size: 11.5px;
  color: #64748b;
  margin-bottom: 4px;
  display: flex;
  align-items: center;
}

.stats-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.point-item.is-new {
  border: 1.5px dashed #f87171;
  background: #fef2f2;
}

.point-item.is-selected-new {
  border: 1.5px dashed #f87171;
  background: #fef2f2;
}

.inline-smart-summary {
  margin-bottom: 14px;
  padding: 14px;
  border-radius: 14px;
  background: #fff7f7;
  border: 1px solid #fecaca;
}

.stats-row--inline {
  margin-bottom: 12px;
}

.inline-smart-actions {
  display: flex;
  align-items: center;
  gap: 14px;
  justify-content: space-between;
  flex-wrap: wrap;
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

.generation-progress-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px 20px;
  background: #fff;
  border: 1px solid #e5edf7;
  border-radius: 16px;
}

.generation-progress-card__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.generation-progress-card__title {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.generation-progress-card__desc {
  margin-top: 6px;
  font-size: 13px;
  color: #64748b;
  line-height: 1.6;
}

.generation-progress-card__meta {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
  margin-left: auto;
  max-width: 100%;
}

.generation-status-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
  flex: 0 0 auto;
  white-space: nowrap;
  word-break: keep-all;
  padding: 0 10px;
  font-size: 12px;
  line-height: 1;
  max-width: max-content;
}

:deep(.generation-status-tag .el-tag__content) {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: nowrap;
  flex: 0 0 auto;
  white-space: nowrap;
  word-break: keep-all;
  line-height: 1;
}

.generation-status-tag__icon {
  font-size: 11px;
}

.generation-point-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
}

.generation-point-item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
}

.generation-point-item.is-running {
  border-color: #fbbf24;
  background: #fffaf0;
}

.generation-point-item.is-done {
  border-color: #86efac;
  background: #f0fdf4;
}

.generation-point-item.is-error {
  border-color: #fca5a5;
  background: #fff5f5;
}

.generation-point-item__main {
  min-width: 0;
}

.generation-point-item__title {
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.5;
}

.generation-point-item__meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 6px;
  font-size: 12px;
  color: #64748b;
  line-height: 1.5;
}

.generation-point-item__error {
  color: #dc2626;
}

.generation-point-item__tag {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
  white-space: nowrap;
}

:deep(.generation-point-item__tag .el-tag__content) {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
}

.generation-point-item__spinner {
  font-size: 12px;
}

.generation-progress-alert {
  margin-top: 4px;
}

.optimization-preview-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  border-radius: 16px;
  border: 1px solid #fed7aa;
  background: #fff7ed;
}

.optimization-preview-card__main {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.optimization-preview-card__title {
  font-size: 15px;
  font-weight: 700;
  color: #9a3412;
}

.optimization-preview-card__desc {
  font-size: 13px;
  line-height: 1.6;
  color: #9a3412;
}

.optimization-preview-card__stats {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.optimization-removed-alert {
  margin-top: -4px;
}

.review-workbench {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.review-role-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 18px;
}

.review-role-card {
  width: 100%;
  padding: 14px;
  border: 1px solid #dbe5f1;
  border-radius: 14px;
  background: #f8fafc;
  text-align: left;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.review-role-card:hover {
  transform: translateY(-2px);
  border-color: #cbdcf8;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.08);
}

.review-role-card.is-active {
  border-color: #60a5fa;
  background: #eff6ff;
  box-shadow: 0 10px 24px rgba(59, 130, 246, 0.12);
}

.review-role-card.is-participating {
  border-color: #93c5fd;
}

.review-role-card.has-result {
  background: #f8fffb;
}

.review-role-card__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.review-role-card__tags {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.review-role-card__name {
  font-size: 14px;
  font-weight: 700;
  color: #0f172a;
}

.review-role-card__desc {
  font-size: 12px;
  line-height: 1.6;
  color: #64748b;
}

.review-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}

.review-toolbar__meta,
.review-toolbar__actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.review-consensus-card {
  margin-bottom: 18px;
  padding: 16px;
  border-radius: 16px;
  border: 1px solid #e5edf7;
  background: #f8fafc;
}

.review-consensus-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.review-body {
  display: grid;
  grid-template-columns: minmax(300px, 0.95fr) minmax(0, 1.45fr);
  gap: 18px;
  align-items: start;
}

.review-identity-card,
.review-result-card {
  min-height: 100%;
  padding: 16px;
  border-radius: 16px;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
}

.review-block-title {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  margin-bottom: 14px;
}

.review-identity-md {
  margin: 0;
  min-height: 540px;
  max-height: 760px;
  overflow: auto;
  padding: 16px;
  border-radius: 14px;
  background: #ffffff;
  border: 1px solid #e5edf7;
  color: #334155;
  font-size: 13px;
  line-height: 1.75;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: inherit;
}

.review-result-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.review-result-summary {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.review-result-time {
  font-size: 12px;
  color: #64748b;
}

.review-section-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.review-section {
  padding: 14px;
  border-radius: 14px;
  border: 1px solid #e5edf7;
  background: #ffffff;
}

.review-section__title {
  margin-bottom: 10px;
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
}

.review-section__text {
  margin: 0;
  font-size: 13px;
  line-height: 1.7;
  color: #475569;
}

.review-list {
  margin: 0;
  padding-left: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  line-height: 1.65;
  color: #475569;
}

.review-insight-type {
  display: inline-block;
  margin-right: 6px;
  font-size: 12px;
  font-weight: 700;
  color: #2563eb;
}

.review-insight-roles {
  margin-left: 6px;
  font-size: 12px;
  color: #94a3b8;
}

.review-diff-groups {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.review-diff-group {
  padding: 12px;
  border-radius: 12px;
  border: 1px dashed #dbe5f1;
  background: #ffffff;
}

.review-diff-group__title {
  margin-bottom: 8px;
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
}

.review-focus-tags {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.review-empty-state {
  min-height: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border-radius: 14px;
  border: 1px dashed #cbd5e1;
  background: #ffffff;
  color: #64748b;
  text-align: center;
}

.review-empty-state__title {
  font-size: 15px;
  font-weight: 700;
  color: #334155;
}

.review-empty-state__desc,
.is-empty {
  font-size: 12px;
  color: #94a3b8;
}

.result-summary-card,
.result-config-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  background: #fff;
  border: 1px solid #e5edf7;
  border-radius: 16px;
}

.result-summary-card__main {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.result-summary-card__title {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.result-summary-card__desc {
  font-size: 13px;
  color: #64748b;
}

.result-config-card__fields {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-select {
  width: 180px;
}

.excel-frame {
  border: 1px solid #cbd5e1;
  border-radius: 14px;
  overflow: hidden;
  background: #fff;
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

.toolbar-left,
.toolbar-right {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
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

@media (max-width: 960px) {
  .page-header,
  .optimization-preview-card,
  .result-summary-card,
  .result-config-card,
  .card-header,
  .review-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .page-header__main {
    align-items: flex-start;
  }

  .page-header__title-row {
    flex-wrap: wrap;
  }

  .analysis-split-layout {
    grid-template-columns: 1fr;
  }

  .review-role-grid,
  .review-consensus-grid,
  .review-section-grid,
  .review-body {
    grid-template-columns: 1fr;
  }

  .source-text-viewer {
    min-height: 280px;
    max-height: 420px;
  }

  .review-identity-md {
    min-height: 280px;
    max-height: 420px;
  }

  .workflow-steps {
    grid-template-columns: 1fr;
  }

  .workflow-step-card__connector {
    top: auto;
    bottom: -8px;
    left: 22px;
    right: auto;
    width: 2px;
    height: 10px;
    transform: none;
    background: linear-gradient(180deg, #cbd5e1 0%, #e2e8f0 100%);
  }

  .requirement-grid,
  .enhanced-grid {
    grid-template-columns: 1fr;
  }

  .toolbar-select {
    width: 100%;
  }

  .generation-progress-card__meta,
  .review-toolbar__meta,
  .review-toolbar__actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>
