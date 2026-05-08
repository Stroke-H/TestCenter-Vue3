<template>
  <div class="runner-container" v-loading="loading">
    <!-- Top Header -->
    <header class="runner-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" circle @click="goBack" />
        <div class="case-info">
          <h1>{{ testCase?.name || 'Loading...' }}</h1>
          <el-tag :type="statusType" effect="dark" size="small">{{ statusText }}</el-tag>
        </div>
      </div>
      <div class="header-right">
        <div class="timer">
          <el-icon><Timer /></el-icon>
          <span>{{ executionTime }}</span>
        </div>
        <el-button type="danger" :disabled="status !== 'running'" @click="stopExecution">
          停止执行
        </el-button>
        <el-button type="primary" :disabled="status === 'running'" @click="restartExecution">
          重新开始
        </el-button>
      </div>
    </header>

    <div class="runner-content">
      <!-- Left: Steps Timeline -->
      <aside class="steps-panel">
        <div class="panel-header">
          <h3>执行步骤</h3>
          <span class="progress-text">{{ completedSteps }}/{{ totalSteps }}</span>
        </div>
        <el-scrollbar>
          <div class="steps-list">
            <div 
              v-for="(step, index) in executionSteps" 
              :key="step.id" 
              class="step-item"
              :class="{ 
                'is-active': index === currentStepIndex,
                'is-success': step.status === 'pass',
                'is-error': step.status === 'fail',
                'is-running': step.status === 'running'
              }"
            >
              <div class="step-status-icon">
                <el-icon v-if="step.status === 'pass'" color="#67C23A"><CircleCheckFilled /></el-icon>
                <el-icon v-else-if="step.status === 'fail'" color="#F56C6C"><CircleCloseFilled /></el-icon>
                <el-icon v-else-if="step.status === 'running'" class="is-loading"><Loading /></el-icon>
                <div v-else class="status-dot"></div>
              </div>
              <div class="step-details">
                <div class="step-name">{{ step.keyword }}</div>
                <div class="step-meta" v-if="step.status !== 'pending'">
                  <span class="duration" v-if="step.duration">{{ step.duration }}s</span>
                  <span class="error-msg" v-if="step.error">{{ step.error }}</span>
                </div>
              </div>
            </div>
          </div>
        </el-scrollbar>
      </aside>

      <!-- Right: Main View Area -->
      <main class="main-view">
        <div class="view-header">
          <el-tabs v-model="activeTab" class="view-tabs">
            <el-tab-pane label="实时截图" name="screenshot">
              <div class="screenshot-container">
                <div v-if="currentScreenshot" class="screenshot-wrapper">
                  <el-image 
                    :src="'data:image/png;base64,' + currentScreenshot" 
                    fit="contain"
                    :preview-src-list="['data:image/png;base64,' + currentScreenshot]"
                  />
                </div>
                <div v-else class="empty-view">
                  <el-empty description="等待执行开始..." />
                </div>
              </div>
            </el-tab-pane>
            <el-tab-pane label="变量列表" name="variables">
              <el-table :data="variableList" stripe style="width: 100%">
                <el-table-column prop="key" label="变量名" />
                <el-table-column prop="value" label="当前值" />
              </el-table>
            </el-tab-pane>
          </el-tabs>
        </div>

        <!-- Log Console -->
        <div class="console-panel">
          <div class="console-header">
            <span>实时日志</span>
            <el-button link :icon="Delete" @click="logs = []">清空</el-button>
          </div>
          <div class="console-body" ref="logContainer">
            <div 
              v-for="(log, index) in logs" 
              :key="index" 
              class="log-line"
              :class="log.type"
            >
              <span class="log-time">[{{ formatTime(log.timestamp) }}]</span>
              <span class="log-msg">{{ log.message }}</span>
            </div>
          </div>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { usePlaywrightStore } from '@/stores/modules/playwright'
import { Timer, ArrowLeft, CircleCheckFilled, CircleCloseFilled, Loading, Delete } from '@element-plus/icons-vue'
import { buildBackendWsUrl } from '@/utils/runtimeUrl'
import dayjs from 'dayjs'

const route = useRoute()
const router = useRouter()
const playwrightStore = usePlaywrightStore()

const id = route.params.id as string
const loading = ref(false)
const testCase = ref<any>(null)
const executionSteps = ref<any[]>([])
const currentStepIndex = ref(-1)
const logs = ref<any[]>([])
const status = ref<'ready' | 'running' | 'completed' | 'error'>('ready')
const currentScreenshot = ref('')
const activeTab = ref('screenshot')
const variables = ref<Record<string, any>>({})
const logContainer = ref<HTMLElement | null>(null)

// Timer
const startTime = ref<number | null>(null)
const duration = ref(0)
let timerInterval: any = null

const executionTime = computed(() => {
  const mins = Math.floor(duration.value / 60)
  const secs = duration.value % 60
  return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
})

const statusText = computed(() => {
  switch (status.value) {
    case 'running': return '执行中'
    case 'completed': return '已完成'
    case 'error': return '执行失败'
    default: return '就绪'
  }
})

const statusType = computed(() => {
  switch (status.value) {
    case 'running': return 'primary'
    case 'completed': return 'success'
    case 'error': return 'danger'
    default: return 'info'
  }
})

const totalSteps = computed(() => executionSteps.value.length)
const completedSteps = computed(() => executionSteps.value.filter(s => s.status === 'pass' || s.status === 'fail').length)

const variableList = computed(() => {
  return Object.entries(variables.value).map(([key, value]) => ({ key, value }))
})

let socket: WebSocket | null = null

const initData = async () => {
  loading.value = true
  try {
    const data = await playwrightStore.fetchCaseById(id)
    testCase.value = data
    executionSteps.value = data.steps.map((s: any) => ({
      ...s,
      status: 'pending',
      duration: null,
      error: null
    }))
  } catch (err) {
    console.error(err)
  } finally {
    loading.value = false
  }
}

const startExecution = () => {
  if (socket) socket.close()
  
  logs.value = []
  status.value = 'running'
  duration.value = 0
  startTime.value = Date.now()
  currentStepIndex.value = -1
  currentScreenshot.value = ''
  
  executionSteps.value.forEach(s => {
    s.status = 'pending'
    s.duration = null
    s.error = null
  })

  timerInterval = setInterval(() => {
    duration.value = Math.floor((Date.now() - (startTime.value || 0)) / 1000)
  }, 1000)

  socket = new WebSocket(buildBackendWsUrl(`/api/ws/playwright?caseId=${id}`))

  socket.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    handleIncomingMessage(msg)
  }

  socket.onclose = () => {
    if (status.value === 'running') {
      status.value = 'completed'
    }
    clearInterval(timerInterval)
  }

  socket.onerror = () => {
    status.value = 'error'
    addLog('WebSocket connection error', 'error')
    clearInterval(timerInterval)
  }
}

const handleIncomingMessage = (msg: any) => {
  switch (msg.type) {
    case 'log':
      addLog(msg.message, msg.error ? 'error' : 'info')
      break
    case 'step_start':
      currentStepIndex.value = msg.step_index
      if (executionSteps.value[msg.step_index]) {
        const step = executionSteps.value[msg.step_index]
        step.status = 'running'
        step.startTime = Date.now()
        addLog(`Step ${msg.step_index + 1} started: ${step.keyword}`, 'info')
      }
      break
    case 'step_pass':
      if (executionSteps.value[msg.step_index]) {
        const step = executionSteps.value[msg.step_index]
        step.status = 'pass'
        step.duration = ((Date.now() - (step.startTime || 0)) / 1000).toFixed(1)
        if (msg.screenshot) {
          currentScreenshot.value = msg.screenshot
          step.screenshot = msg.screenshot
        }
        addLog(`Step ${msg.step_index + 1} passed in ${step.duration}s`, 'info')
      }
      break
    case 'step_fail':
      if (executionSteps.value[msg.step_index]) {
        const step = executionSteps.value[msg.step_index]
        step.status = 'fail'
        step.error = msg.error
        step.duration = ((Date.now() - (step.startTime || 0)) / 1000).toFixed(1)
        addLog(`Step ${msg.step_index + 1} failed: ${msg.error}`, 'error')
      }
      status.value = 'error'
      break
    case 'screenshot':
      currentScreenshot.value = msg.screenshot
      break
    case 'suite_done':
      status.value = 'completed'
      clearInterval(timerInterval)
      break
  }
}

const addLog = (message: string, type: 'info' | 'error' = 'info') => {
  logs.value.push({
    message,
    type,
    timestamp: new Date().toISOString()
  })
  scrollToBottom()
}

const scrollToBottom = () => {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

const formatTime = (ts: string) => {
  return dayjs(ts).format('HH:mm:ss')
}

const stopExecution = () => {
  if (socket) {
    socket.close()
  }
  status.value = 'ready'
  clearInterval(timerInterval)
}

const restartExecution = () => {
  stopExecution()
  startExecution()
}

const goBack = () => {
  router.push('/ui_auto')
}

onMounted(() => {
  initData().then(() => {
    startExecution()
  })
})

onUnmounted(() => {
  if (socket) socket.close()
  if (timerInterval) clearInterval(timerInterval)
})
</script>

<style scoped>
.runner-container {
  height: calc(100vh - 100px);
  display: flex;
  flex-direction: column;
  background-color: #f5f7fa;
  padding: 20px;
  gap: 20px;
}

.runner-header {
  background: #fff;
  padding: 15px 25px;
  border-radius: 12px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: 0 4px 12px rgba(0,0,0,0.05);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 20px;
}

.case-info h1 {
  margin: 0;
  font-size: 1.25rem;
  color: #2c3e50;
  font-weight: 600;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 20px;
}

.timer {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: monospace;
  font-size: 1.1rem;
  background: #f0f2f5;
  padding: 4px 12px;
  border-radius: 6px;
  color: #606266;
}

.runner-content {
  flex: 1;
  display: flex;
  gap: 20px;
  min-height: 0;
}

.steps-panel {
  width: 320px;
  background: #fff;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 4px 12px rgba(0,0,0,0.05);
  overflow: hidden;
}

.panel-header {
  padding: 15px 20px;
  border-bottom: 1px solid #ebeef5;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.panel-header h3 {
  margin: 0;
  font-size: 1rem;
  color: #303133;
}

.progress-text {
  font-size: 0.85rem;
  color: #909399;
}

.steps-list {
  padding: 10px 0;
}

.step-item {
  padding: 12px 20px;
  display: flex;
  gap: 12px;
  transition: all 0.3s;
  border-left: 3px solid transparent;
}

.step-item.is-active {
  background-color: #ecf5ff;
  border-left-color: #409eff;
}

.step-item.is-success {
  background-color: #f0f9eb;
}

.step-item.is-error {
  background-color: #fef0f0;
}

.step-status-icon {
  width: 20px;
  display: flex;
  justify-content: center;
  align-items: center;
}

.status-dot {
  width: 8px;
  height: 8px;
  background: #dcdfe6;
  border-radius: 50%;
}

.step-details {
  flex: 1;
}

.step-name {
  font-size: 0.9rem;
  color: #303133;
  margin-bottom: 4px;
}

.step-meta {
  font-size: 0.75rem;
  display: flex;
  justify-content: space-between;
}

.duration {
  color: #909399;
}

.error-msg {
  color: #f56c6c;
}

.main-view {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

.view-header {
  background: #fff;
  border-radius: 12px;
  padding: 10px 20px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.05);
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.view-tabs {
  height: 100%;
}

:deep(.el-tabs__content) {
  flex: 1;
  overflow: auto;
}

.screenshot-container {
  height: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  background: #f8fafc;
  border: 1px dashed #dcdfe6;
  border-radius: 8px;
  overflow: hidden;
}

.screenshot-wrapper {
  max-width: 100%;
  max-height: 100%;
}

.console-panel {
  height: 250px;
  background: #1e1e1e;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
}

.console-header {
  padding: 8px 15px;
  background: #2d2d2d;
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: #ccc;
  font-size: 0.8rem;
}

.console-body {
  flex: 1;
  overflow-y: auto;
  padding: 10px 15px;
  font-family: 'Fira Code', 'Roboto Mono', monospace;
  font-size: 0.85rem;
  line-height: 1.5;
}

.log-line {
  margin-bottom: 4px;
  word-break: break-all;
}

.log-time {
  color: #888;
  margin-right: 10px;
}

.log-msg {
  color: #eee;
}

.log-line.error .log-msg {
  color: #f56c6c;
}

.log-line.error .log-time {
  color: #f56c6c80;
}

/* Scrollbar styling for console */
.console-body::-webkit-scrollbar {
  width: 6px;
}
.console-body::-webkit-scrollbar-thumb {
  background: #444;
  border-radius: 3px;
}
.console-body::-webkit-scrollbar-track {
  background: #222;
}

.is-loading {
  animation: rotate 2s linear infinite;
}

@keyframes rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
