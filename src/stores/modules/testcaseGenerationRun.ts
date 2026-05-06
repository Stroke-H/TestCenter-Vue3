import { defineStore } from 'pinia'
import { computed, shallowRef } from 'vue'
import axios from 'axios'

export interface TestcaseRunRequirementPoint {
  module: string
  feature: string
  description: string
  rules: string[]
  is_new?: boolean
}

export interface TestcaseRunCase {
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

export interface TestcaseGenerationPointStatus {
  key: string
  module: string
  feature: string
  status: 'pending' | 'running' | 'done' | 'error'
  caseCount: number
  error?: string
}

const API_BASE = '/api/testcase-gen'

const reindexCases = (cases: TestcaseRunCase[]) => {
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

export const useTestcaseGenerationRunStore = defineStore('testcaseGenerationRun', () => {
  const taskPhase = shallowRef<'idle' | 'generating' | 'reviewing' | 'optimizing' | 'result'>('idle')
  const requirementText = shallowRef('')
  const generatedRequirementPoints = shallowRef<TestcaseRunRequirementPoint[]>([])
  const analyzerUsed = shallowRef(false)
  const generatedCases = shallowRef<TestcaseRunCase[]>([])
  const generationStatuses = shallowRef<TestcaseGenerationPointStatus[]>([])
  const currentBatch = shallowRef(0)
  const totalBatches = shallowRef(0)
  const generationInProgress = shallowRef(false)
  const generationError = shallowRef('')
  const currentGeneratingPointTitle = shallowRef('')
  const reviewInProgress = shallowRef(false)
  const reviewingRoleCount = shallowRef(0)
  const reviewedRoleDoneCount = shallowRef(0)
  const optimizeInProgress = shallowRef(false)

  const progressPercent = computed(() => {
    if (generationStatuses.value.length === 0) return 0
    const doneCount = generationStatuses.value.filter(item => item.status === 'done').length
    const runningWeight = generationStatuses.value.some(item => item.status === 'running') ? 0.5 : 0
    return Math.min(99, Math.round(((doneCount + runningWeight) / generationStatuses.value.length) * 100))
  })

  const assistantLabel = computed(() => {
    if (generationInProgress.value) return `生成 ${progressPercent.value}%`
    if (reviewInProgress.value) {
      const total = reviewingRoleCount.value || 0
      const done = reviewedRoleDoneCount.value || 0
      return total > 0 ? `评审 ${done}/${total}` : '评审中'
    }
    if (optimizeInProgress.value) return '优化中'
    if (hasRecoverableRun.value) return '用例结果'
    return '智能助手'
  })

  const isAssistantActive = computed(() => hasRecoverableRun.value)
  const hasRecoverableRun = computed(() => generationInProgress.value || reviewInProgress.value || optimizeInProgress.value || generatedCases.value.length > 0 || generationStatuses.value.length > 0)

  const reset = () => {
    taskPhase.value = 'idle'
    requirementText.value = ''
    generatedRequirementPoints.value = []
    analyzerUsed.value = false
    generatedCases.value = []
    generationStatuses.value = []
    currentBatch.value = 0
    totalBatches.value = 0
    generationInProgress.value = false
    generationError.value = ''
    currentGeneratingPointTitle.value = ''
    reviewInProgress.value = false
    reviewingRoleCount.value = 0
    reviewedRoleDoneCount.value = 0
    optimizeInProgress.value = false
  }

  const startGeneration = async (payload: {
    points: TestcaseRunRequirementPoint[]
    text: string
    analyzerUsed: boolean
  }) => {
    const pointsToGenerate = [...payload.points]
    if (pointsToGenerate.length === 0) return

    requirementText.value = payload.text
    generatedRequirementPoints.value = pointsToGenerate
    analyzerUsed.value = payload.analyzerUsed
    generatedCases.value = []
    generationError.value = ''
    generationInProgress.value = true
    taskPhase.value = 'generating'
    generationStatuses.value = pointsToGenerate.map((point, index) => ({
      key: `${point.module}-${point.feature}-${index}`,
      module: point.module,
      feature: point.feature,
      status: 'pending',
      caseCount: 0
    }))

    const batchSize = 1
    totalBatches.value = Math.ceil(pointsToGenerate.length / batchSize)
    currentBatch.value = 0

    try {
      for (let i = 0; i < pointsToGenerate.length; i += batchSize) {
        currentBatch.value++
        const batchIndex = Math.floor(i / batchSize)
        const batch = pointsToGenerate.slice(i, i + batchSize)
        const point = batch[0]
        const currentStatus = generationStatuses.value[batchIndex]
        if (!point || !currentStatus) continue

        currentGeneratingPointTitle.value = `${point.module} / ${point.feature}`
        generationStatuses.value = generationStatuses.value.map((item, index) => {
          if (index !== batchIndex) return item
          return { ...item, status: 'running' }
        })

        try {
          const res = await axios.post(`${API_BASE}/generate`, {
            points: batch,
            text: requirementText.value,
            analyzer_used: analyzerUsed.value
          })

          const incomingCases = Array.isArray(res.data?.cases) ? res.data.cases : []
          const mappedCases = incomingCases.map((item: TestcaseRunCase) => ({
            ...item,
            source_module: point.module,
            source_feature: point.feature
          }))

          if (mappedCases.length > 0) {
            generatedCases.value = reindexCases([...generatedCases.value, ...mappedCases])
          }

          if (res.data?.analyzer_used) {
            analyzerUsed.value = true
          }

          generationStatuses.value = generationStatuses.value.map((item, index) => {
            if (index !== batchIndex) return item
            return {
              ...item,
              status: res.data.partial && res.data.error ? 'error' : 'done',
              caseCount: mappedCases.length,
              error: res.data.partial ? res.data.error || '' : ''
            }
          })

          if (res.data.partial && res.data.error) {
            generationError.value = res.data.error
          }
        } catch (error: any) {
          const errorMsg = error.response?.data?.error || '用例生成失败'
          generationError.value = errorMsg
          generationStatuses.value = generationStatuses.value.map((item, index) => {
            if (index !== batchIndex) return item
            return { ...item, status: 'error', error: errorMsg }
          })
        }
      }
    } finally {
      generationInProgress.value = false
      currentGeneratingPointTitle.value = ''
      currentBatch.value = 0
      totalBatches.value = 0
      taskPhase.value = generatedCases.value.length > 0 ? 'result' : 'idle'
    }
  }

  const startReviewSession = (roleCount: number) => {
    reviewInProgress.value = true
    reviewingRoleCount.value = roleCount
    reviewedRoleDoneCount.value = 0
    taskPhase.value = 'reviewing'
  }

  const markReviewProgress = (doneCount: number) => {
    reviewedRoleDoneCount.value = doneCount
  }

  const finishReviewSession = () => {
    reviewInProgress.value = false
    reviewedRoleDoneCount.value = reviewingRoleCount.value
    taskPhase.value = generatedCases.value.length > 0 ? 'result' : 'idle'
  }

  const startOptimizeSession = () => {
    optimizeInProgress.value = true
    taskPhase.value = 'optimizing'
  }

  const finishOptimizeSession = () => {
    optimizeInProgress.value = false
    taskPhase.value = generatedCases.value.length > 0 ? 'result' : 'idle'
  }

  return {
    taskPhase,
    requirementText,
    generatedRequirementPoints,
    analyzerUsed,
    generatedCases,
    generationStatuses,
    currentBatch,
    totalBatches,
    generationInProgress,
    generationError,
    currentGeneratingPointTitle,
    reviewInProgress,
    reviewingRoleCount,
    reviewedRoleDoneCount,
    optimizeInProgress,
    progressPercent,
    assistantLabel,
    isAssistantActive,
    hasRecoverableRun,
    startGeneration,
    startReviewSession,
    markReviewProgress,
    finishReviewSession,
    startOptimizeSession,
    finishOptimizeSession,
    reset
  }
})
