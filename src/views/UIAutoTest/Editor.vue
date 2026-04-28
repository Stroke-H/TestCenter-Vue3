<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import { usePlaywrightStore, type TestStep } from '@/stores/modules/playwright'
import { ArrowLeft, VideoPlay, Files, Aim } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import KeywordPanel from './components/KeywordPanel.vue'
import StepFlow from './components/StepFlow.vue'
import PropertyPanel from './components/PropertyPanel.vue'
import { v4 as uuidv4 } from 'uuid'

const route = useRoute()
const router = useRouter()
const pwStore = usePlaywrightStore()

const caseId = route.params.id as string
const suiteId = route.query.suite_id as string

const currentCase = ref<any>({
  id: '',
  suite_id: suiteId || '',
  name: '未命名用例',
  description: '',
  tags: [],
  variables: {},
  steps: [] as TestStep[]
})

const currentSuite = ref<any>(null)

const fetchSuite = async (id: string) => {
  if (!id) return
  try {
    const res = await axios.get(`/api/playwright/suites/${id}`)
    currentSuite.value = res.data
  } catch (err) {
    console.error('Failed to fetch suite', err)
  }
}

const selectedStepIndex = ref<number>(-1)
const isRecording = ref(false)
const recordingSocket = ref<WebSocket | null>(null)

onMounted(async () => {
  await pwStore.fetchBuiltinKeywords()
  
  if (caseId) {
    await pwStore.fetchCases() 
    const found = pwStore.cases.find(c => c.id === caseId)
    if (found) {
      currentCase.value = JSON.parse(JSON.stringify(found))
      if (currentCase.value.suite_id) {
        await fetchSuite(currentCase.value.suite_id)
      }
    }
  } else if (suiteId) {
    await fetchSuite(suiteId)
  }
})

const handleAddStep = (keyword: any) => {
  const newStep: TestStep = {
    id: uuidv4().substring(0, 8),
    keyword: keyword.keyword,
    args: {},
    return_var: '',
    description: keyword.name,
    disabled: false
  }
  
  // Default args based on keyword
  keyword.args.forEach((arg: string) => {
    newStep.args[arg] = ''
  })

  currentCase.value.steps.push(newStep)
  selectedStepIndex.value = currentCase.value.steps.length - 1
  ElMessage.success(`已添加步骤: ${keyword.name}`)
}

const appendRecordedStep = (step: TestStep) => {
  currentCase.value.steps.push(step)
  selectedStepIndex.value = currentCase.value.steps.length - 1
}

const ensureRecordingBootstrapSteps = (url: string) => {
  const normalizedUrl = url.startsWith('http') ? url : `https://${url}`
  const launchStep: TestStep = {
    id: uuidv4().substring(0, 8),
    keyword: 'Launch',
    args: { headless: 'false' },
    return_var: '',
    description: '打开浏览器',
    disabled: false
  }
  const gotoStep: TestStep = {
    id: uuidv4().substring(0, 8),
    keyword: 'Goto',
    args: { url: normalizedUrl, waitUntil: 'load' },
    return_var: '',
    description: '导航到录制页面',
    disabled: false
  }

  appendRecordedStep(launchStep)
  appendRecordedStep(gotoStep)
}

const handleToggleRecording = async () => {
  if (isRecording.value) {
    // Stop
    recordingSocket.value?.close()
    isRecording.value = false
    ElMessage.info('已停止录制')
    return
  }

  // Start - Prompt for URL
  try {
    const { value: url } = await ElMessageBox.prompt('请输入初始录制地址', '开启探测器', {
      confirmButtonText: '开启',
      cancelButtonText: '取消',
      inputPlaceholder: 'https://example.com',
      inputValue: 'https://',
    })

    if (!url) return

    isRecording.value = true
    const wsUrl = `ws://${window.location.hostname}:8080/api/ws/inspector?url=${encodeURIComponent(url)}`
    const ws = new WebSocket(wsUrl)
    recordingSocket.value = ws

    ensureRecordingBootstrapSteps(url)

    ws.onmessage = (event) => {
      console.log('WS Message received:', event.data)
      const data = JSON.parse(event.data)
      if (data.type === 'element_picked') {
        handleElementPicked(data.selector)
      } else if (data.type === 'element_filled') {
        handleElementFilled(data.selector, data.value || '')
      } else if (data.type === 'element_keydown') {
        handleElementKeydown(data.selector, data.value || '')
      } else if (data.type === 'status') {
        console.log('Inspector Status:', data.message)
      } else if (data.type === 'error') {
        ElMessage.error(`探测器错误: ${data.message}`)
        isRecording.value = false
      }
    }

    ws.onclose = () => {
      isRecording.value = false
    }

    ws.onerror = () => {
      ElMessage.error('探测器连接失败')
      isRecording.value = false
    }

  } catch {
    // User cancelled
  }
}

const handleElementPicked = (selector: string) => {
  console.log('[DEBUG] handleElementPicked called with selector:', selector)
  // Recording should always append a new step card instead of silently
  // mutating the currently selected step, otherwise the user sees no new card.
  const clickKeyword = pwStore.builtinKeywords.find(k => k.keyword === 'Click')
  if (clickKeyword) {
    const newStep: TestStep = {
      id: uuidv4().substring(0, 8),
      keyword: 'Click',
      args: { selector },
      return_var: '',
      description: '点击元素',
      disabled: false
    }
    appendRecordedStep(newStep)
    console.log('[DEBUG] Added new recorded click step, steps.length now:', currentCase.value.steps.length)
    ElMessage.success('已通过录制添加点击步骤')
  } else {
    console.warn('Click keyword not found in store, adding step fallback')
    const newStep: TestStep = {
      id: uuidv4().substring(0, 8),
      keyword: 'Click',
      args: { selector },
      return_var: '',
      description: '点击元素 (Fallback)',
      disabled: false
    }
    appendRecordedStep(newStep)
    console.log('[DEBUG] Added fallback recorded click step, steps.length now:', currentCase.value.steps.length)
    ElMessage.info('已录制点击 (关键字库未加载)')
  }
}

const handleElementFilled = (selector: string, value: string) => {
  console.log('[DEBUG] handleElementFilled called with selector/value:', selector, value)
  const newStep: TestStep = {
    id: uuidv4().substring(0, 8),
    keyword: 'Fill',
    args: { selector, value },
    return_var: '',
    description: '输入文本',
    disabled: false
  }
  appendRecordedStep(newStep)
  ElMessage.success('已通过录制添加输入步骤')
}

const handleElementKeydown = (selector: string, key: string) => {
  if (!key) return
  console.log('[DEBUG] handleElementKeydown called with selector/key:', selector, key)
  const newStep: TestStep = {
    id: uuidv4().substring(0, 8),
    keyword: 'Press',
    args: { selector, key },
    return_var: '',
    description: `按下按键 ${key}`,
    disabled: false
  }
  appendRecordedStep(newStep)
  ElMessage.success(`已通过录制添加按键步骤: ${key}`)
}

const handleSave = async () => {
  if (!currentCase.value.name) {
    ElMessage.error('请输入用例名称')
    return
  }
  await pwStore.saveCase(currentCase.value)
  ElMessage.success('保存成功')
  if (!caseId) {
    const lastCase = pwStore.cases[pwStore.cases.length - 1]
    if (lastCase) {
      router.replace(`/ui_auto/edit/${lastCase.id}`)
    }
  }
}

const handleRun = () => {
  if (!caseId) {
    ElMessage.warning('请先保存用例后再执行')
    return
  }
  router.push(`/ui_auto/run/${caseId}`)
}

const selectedStep = computed(() => {
  if (selectedStepIndex.value >= 0 && selectedStepIndex.value < currentCase.value.steps.length) {
    return currentCase.value.steps[selectedStepIndex.value]
  }
  return null
})

</script>

<template>
  <div class="editor-container">
    <!-- Header -->
    <div class="editor-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" circle @click="router.back()" />
        <el-divider direction="vertical" />
        <el-input v-model="currentCase.name" class="case-name-input" placeholder="用例名称" />
      </div>
      <div class="header-actions">
        <el-button 
          :type="isRecording ? 'danger' : 'default'" 
          :icon="Aim" 
          @click="handleToggleRecording"
          :class="{ 'pulse-recording': isRecording }"
        >
          {{ isRecording ? '停止录制' : '可视化录制' }}
        </el-button>
        <el-button :icon="Files" @click="handleSave">保存</el-button>
        <el-button type="primary" :icon="VideoPlay" @click="handleRun">调试运行</el-button>
      </div>
    </div>

    <!-- Main Content -->
    <div class="editor-main">
      <!-- Left: Keywords -->
      <div class="panel keywords-panel">
        <KeywordPanel 
          :associated-suites="currentSuite?.skill_suites || []" 
          @add-step="handleAddStep" 
        />
      </div>

      <!-- Center: Steps -->
      <div class="panel steps-panel">
        <div class="panel-header">
          <span>步骤编排 ({{ currentCase.steps.length }})</span>
        </div>
        <div class="steps-content">
          <StepFlow 
            v-model:steps="currentCase.steps" 
            v-model:selectedIndex="selectedStepIndex" 
          />
          
          <div v-if="currentCase.steps.length === 0" class="empty-steps">
            <p>从左侧面板添加关键字开始编排</p>
          </div>
        </div>
      </div>

      <!-- Right: Properties -->
      <div class="panel properties-panel">
        <PropertyPanel 
          v-if="selectedStep" 
          :step="selectedStep" 
          :variables="currentCase.variables" 
        />
        <div v-else class="no-selection">
          <p>选中一个步骤以编辑属性</p>
          <el-divider>或描述用例</el-divider>
          <el-form label-position="top" style="padding: 0 20px">
             <el-form-item label="用例描述">
               <el-input v-model="currentCase.description" type="textarea" :rows="3" />
             </el-form-item>
          </el-form>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.editor-container {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 100px);
  background: #f5f7fa;
  margin: -20px;
}

.editor-header {
  height: 60px;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.case-name-input :deep(.el-input__wrapper) {
  box-shadow: none;
  font-size: 18px;
  font-weight: 600;
}

.editor-main {
  flex: 1;
  display: flex;
  overflow: hidden;
  gap: 1px;
}

.panel {
  background: #fff;
  display: flex;
  flex-direction: column;
}

.keywords-panel {
  width: 260px;
  border-right: 1px solid #e4e7ed;
}

.steps-panel {
  flex: 1;
  background: #fcfcfc;
}

.properties-panel {
  width: 320px;
  border-left: 1px solid #e4e7ed;
}

.panel-header {
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid #f0f0f0;
  font-weight: 600;
  font-size: 14px;
  color: #303133;
}

.steps-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

.empty-steps {
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  color: #909399;
  border: 2px dashed #ebeef5;
  border-radius: 8px;
}

.no-selection {
  padding-top: 40px;
  text-align: center;
  color: #909399;
}

.pulse-recording {
  animation: pulse-red 2s infinite;
}

@keyframes pulse-red {
  0% {
    box-shadow: 0 0 0 0 rgba(245, 108, 108, 0.7);
  }
  70% {
    box-shadow: 0 0 0 10px rgba(245, 108, 108, 0);
  }
  100% {
    box-shadow: 0 0 0 0 rgba(245, 108, 108, 0);
  }
}
</style>
