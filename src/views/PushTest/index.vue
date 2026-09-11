<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowDown,
  Check,
  Close,
  Connection,
  Cpu,
  Document as DocumentIcon,
  Edit,
  Promotion,
  RefreshRight,
  VideoPause,
  VideoPlay,
  Warning
} from '@element-plus/icons-vue'
import request from '@/api/request'

type RunStatus = 'ready' | 'running' | 'success' | 'failed' | 'skipped' | 'stopped'
type StepStatus = 'pending' | 'running' | 'success' | 'failed' | 'skipped'
type ActiveTab = 'pipeline' | 'payload' | 'logs'

interface PushTestResult {
  ok: boolean
  status: number
  duration_ms: number
  body: any
}

interface PushTestStep {
  id: 'health' | 'push' | 'stats'
  name: string
  method: 'GET' | 'POST'
  path: string
  status: StepStatus
  code?: number
  latency?: number
  error?: string
  requestData?: unknown
  responseData?: unknown
}

interface SavedPushTestConfig {
  baseURL?: string
  healthPath?: string
  pushPath?: string
  statsPath?: string
  statsDelaySeconds?: number
  payload?: string
}

const STORAGE_KEY = 'testcenter.pushTest.config.v1'
const defaultPayload = {
  app: 'com.cocoshort.dramareels',
  user_ids: ['97bff12567d334cb'],
  contents: '这是用户ID的立即推送消息内容，测试完整参数功能',
  headings: '用户ID立即推送测试',
  app_url: 'launch://com.cocoshort.dramareels/Detail?bookId=29597&chapterId=1',
  data: {
    contentType: '充值',
    type: '0'
  },
  big_picture: 'https://s.shortswave.com/image/cover/5125.webp',
  large_icon: 'https://s.shortswave.com/image/cover/5125.webp',
  ios_badgeCount: 1,
  ios_interruption_level: 'time_sensitive',
  ios_attachments: {
    id: 'https://s.shortswave.com/image/202507/04/175160345737.png'
  },
  priority: 1,
  delivery_type: 'immediate'
}

const loadSavedConfig = (): SavedPushTestConfig => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

const savedConfig = loadSavedConfig()
const baseURL = ref(savedConfig.baseURL || 'http://34.61.243.173:8080')
const healthPath = ref(savedConfig.healthPath || '/health')
const pushPath = ref(savedConfig.pushPath || '/api/v1/push')
const statsPath = ref(savedConfig.statsPath || '/api/v1/push/stats')
const statsDelaySeconds = ref(Math.min(30, Math.max(0, Number(savedConfig.statsDelaySeconds ?? 3))))
const payloadText = ref(savedConfig.payload || JSON.stringify(defaultPayload, null, 2))
const isPayloadEditing = ref(false)
const activeTab = ref<ActiveTab>('pipeline')
const selectedStepID = ref<PushTestStep['id']>('health')
const runStatus = ref<RunStatus>('ready')
const logs = ref<string[]>([])
const elapsedSeconds = ref(0)
const statsCountdown = ref(0)
const steps = ref<PushTestStep[]>([
  { id: 'health', name: '服务健康检查', method: 'GET', path: healthPath.value, status: 'pending' },
  { id: 'push', name: '用户 ID 立即推送', method: 'POST', path: pushPath.value, status: 'pending' },
  { id: 'stats', name: '推送结果统计', method: 'GET', path: statsPath.value, status: 'pending' }
])

let abortController: AbortController | null = null
let elapsedTimer: number | null = null

const selectedStep = computed(() => steps.value.find(item => item.id === selectedStepID.value) || steps.value[0]!)
const isRunning = computed(() => runStatus.value === 'running')
const formattedElapsed = computed(() => {
  const minutes = Math.floor(elapsedSeconds.value / 60).toString().padStart(2, '0')
  const seconds = (elapsedSeconds.value % 60).toString().padStart(2, '0')
  return `${minutes}:${seconds}`
})
const runStatusText = computed(() => ({
  ready: '待执行',
  running: '执行中',
  success: '已完成',
  failed: '执行失败',
  skipped: '部分跳过',
  stopped: '已停止'
})[runStatus.value])

interface PayloadPreviewLine {
  prefix: string
  highlight?: string
  suffix?: string
}

const payloadPreviewLines = computed<PayloadPreviewLine[]>(() => {
  const lines = (payloadText.value.trim() || '未配置请求体').split('\n')
  let insideUserIDs = false

  return lines.map(line => {
    const appMatch = line.match(/^(\s*"app"\s*:\s*)(.*)$/)
    if (appMatch) {
      return { prefix: appMatch[1]!, highlight: appMatch[2]! }
    }

    const userIDsMatch = line.match(/^(\s*"user_ids"\s*:\s*)(.*)$/)
    if (userIDsMatch) {
      const value = userIDsMatch[2]!
      insideUserIDs = !value.includes(']')
      return { prefix: userIDsMatch[1]!, highlight: value }
    }

    if (insideUserIDs) {
      const closingIndex = line.indexOf(']')
      if (closingIndex >= 0) {
        insideUserIDs = false
        return {
          prefix: '',
          highlight: line.slice(0, closingIndex),
          suffix: line.slice(closingIndex)
        }
      }
      return { prefix: '', highlight: line }
    }

    return { prefix: line }
  })
})

const normalizePath = (value: string, fallback: string) => {
  const trimmed = value.trim()
  if (!trimmed) return fallback
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}

const persistConfig = () => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify({
    baseURL: baseURL.value.trim(),
    healthPath: healthPath.value,
    pushPath: pushPath.value,
    statsPath: statsPath.value,
    statsDelaySeconds: statsDelaySeconds.value,
    payload: payloadText.value
  }))
}

watch([baseURL, healthPath, pushPath, statsPath, statsDelaySeconds, payloadText], persistConfig, { flush: 'post' })

const startPayloadEditing = () => {
  isPayloadEditing.value = true
}

const finishPayloadEditing = () => {
  try {
    const parsed = JSON.parse(payloadText.value)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('请求体必须是 JSON 对象')
    }
    payloadText.value = JSON.stringify(parsed, null, 2)
    isPayloadEditing.value = false
    persistConfig()
  } catch (error: any) {
    ElMessage.warning(`推送请求体格式错误：${error.message}`)
  }
}

const syncStepPaths = () => {
  healthPath.value = normalizePath(healthPath.value, '/health')
  pushPath.value = normalizePath(pushPath.value, '/api/v1/push')
  statsPath.value = normalizePath(statsPath.value, '/api/v1/push/stats')
  steps.value[0]!.path = healthPath.value
  steps.value[1]!.path = pushPath.value
  steps.value[2]!.path = statsPath.value
  persistConfig()
}

const resetRuntime = () => {
  steps.value.forEach(item => {
    item.status = 'pending'
    item.code = undefined
    item.latency = undefined
    item.error = undefined
    item.requestData = undefined
    item.responseData = undefined
  })
  logs.value = []
  elapsedSeconds.value = 0
  statsCountdown.value = 0
  selectedStepID.value = 'health'
  activeTab.value = 'pipeline'
}

const appendLog = (message: string) => {
  logs.value.push(`[${new Date().toLocaleTimeString()}] ${message}`)
}

const extractError = (error: any) => (
  error?.response?.data?.error || error?.customMessage || error?.message || '未知错误'
)

const assertUpstreamOK = (result: PushTestResult, label: string) => {
  if (!result?.ok) {
    throw new Error(`${label}返回 HTTP ${result?.status || '未知'}`)
  }
}

const waitOneSecond = (signal: AbortSignal) => new Promise<void>((resolve, reject) => {
  const timerID = window.setTimeout(resolve, 1000)
  signal.addEventListener('abort', () => {
    window.clearTimeout(timerID)
    reject(new DOMException('执行已停止', 'AbortError'))
  }, { once: true })
})

const runPushTest = async () => {
  if (isRunning.value) return
  syncStepPaths()

  let payload: Record<string, unknown>
  try {
    payload = JSON.parse(payloadText.value)
    if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
      throw new Error('请求体必须是 JSON 对象')
    }
    const app = typeof payload.app === 'string' ? payload.app.trim() : ''
    const userIDs = Array.isArray(payload.user_ids)
      ? payload.user_ids.map(value => String(value).trim()).filter(Boolean)
      : []
    if (!app) throw new Error('app 不能为空')
    if (userIDs.length === 0) throw new Error('至少填写一个 user_id')
    payload.app = app
    payload.user_ids = userIDs
    payloadText.value = JSON.stringify(payload, null, 2)
  } catch (error: any) {
    ElMessage.warning(`推送请求体格式错误：${error.message}`)
    return
  }
  if (!baseURL.value.trim()) {
    ElMessage.warning('请填写 GoFCM API 服务地址')
    return
  }

  const targetUsers = Array.isArray(payload.user_ids) ? payload.user_ids.join('、') : '未配置'
  try {
    await ElMessageBox.confirm(
      `即将向 ${targetUsers || '未配置用户'} 发送真实推送，是否继续？`,
      '推送测试确认',
      {
        type: 'warning',
        confirmButtonText: '确认发送',
        cancelButtonText: '取消',
        confirmButtonClass: 'el-button--warning'
      }
    )
  } catch {
    return
  }

  resetRuntime()
  abortController = new AbortController()
  runStatus.value = 'running'
  elapsedTimer = window.setInterval(() => elapsedSeconds.value++, 1000)
  appendLog('开始执行 GoFCM 推送测试链路。')

  const healthStep = steps.value[0]!
  const pushStep = steps.value[1]!
  const statsStep = steps.value[2]!
  let activeStep = healthStep

  try {
    healthStep.status = 'running'
    healthStep.requestData = { method: 'GET', url: `${baseURL.value.replace(/\/$/, '')}${healthPath.value}` }
    appendLog(`[1/3] 检查服务 ${healthPath.value}`)
    const healthResult = await request.post('/push-test/health', {
      base_url: baseURL.value,
      path: healthPath.value
    }, { signal: abortController.signal }) as unknown as PushTestResult
    healthStep.code = healthResult.status
    healthStep.latency = healthResult.duration_ms
    healthStep.responseData = healthResult.body
    assertUpstreamOK(healthResult, '健康检查')
    healthStep.status = 'success'
    appendLog(`[1/3] 服务健康检查通过，HTTP ${healthResult.status}。`)

    activeStep = pushStep
    selectedStepID.value = 'push'
    pushStep.status = 'running'
    pushStep.requestData = {
      method: 'POST',
      url: `${baseURL.value.replace(/\/$/, '')}${pushPath.value}`,
      body: payload
    }
    appendLog(`[2/3] 正在发送用户 ID 立即推送。`)
    const pushResult = await request.post('/push-test/execute', {
      base_url: baseURL.value,
      path: pushPath.value,
      payload
    }, { signal: abortController.signal }) as unknown as PushTestResult
    pushStep.code = pushResult.status
    pushStep.latency = pushResult.duration_ms
    pushStep.responseData = pushResult.body
    assertUpstreamOK(pushResult, '推送接口')
    const taskID = String(pushResult.body?.data?.task_id || pushResult.body?.task_id || '').trim()
    if (!taskID) throw new Error('推送响应中没有返回 task_id，无法查询统计')
    pushStep.status = 'success'
    appendLog(`[2/3] 推送任务创建成功，task_id=${taskID}。`)

    activeStep = statsStep
    selectedStepID.value = 'stats'
    statsStep.requestData = {
      method: 'GET',
      url: `${baseURL.value.replace(/\/$/, '')}${statsPath.value}/${taskID}`,
      task_id: taskID
    }
    for (let remaining = statsDelaySeconds.value; remaining > 0; remaining--) {
      statsCountdown.value = remaining
      statsStep.status = 'running'
      await waitOneSecond(abortController.signal)
    }
    statsCountdown.value = 0
    appendLog(`[3/3] 正在查询推送统计。`)
    const statsResult = await request.post('/push-test/stats', {
      base_url: baseURL.value,
      path: statsPath.value,
      task_id: taskID
    }, { signal: abortController.signal }) as unknown as PushTestResult
    statsStep.code = statsResult.status
    statsStep.latency = statsResult.duration_ms
    statsStep.responseData = statsResult.body
    assertUpstreamOK(statsResult, '统计接口')
    statsStep.status = 'success'
    runStatus.value = 'success'
    appendLog('[3/3] 推送统计查询完成，测试链路执行成功。')
    ElMessage.success('推送测试执行完成')
  } catch (error: any) {
    if (abortController?.signal.aborted || error?.name === 'AbortError' || error?.code === 'ERR_CANCELED') {
      steps.value.forEach(item => {
        if (item.status === 'running' || item.status === 'pending') item.status = 'skipped'
      })
      runStatus.value = 'stopped'
      appendLog('推送测试已手动停止。')
    } else {
      activeStep.status = 'failed'
      activeStep.error = extractError(error)
      const activeIndex = steps.value.findIndex(item => item.id === activeStep.id)
      steps.value.slice(activeIndex + 1).forEach(item => {
        item.status = 'skipped'
        item.error = '前置步骤失败'
      })
      runStatus.value = 'failed'
      appendLog(`[ERROR] ${activeStep.name}：${activeStep.error}`)
      ElMessage.error(activeStep.error)
    }
  } finally {
    if (elapsedTimer !== null) window.clearInterval(elapsedTimer)
    elapsedTimer = null
    abortController = null
    statsCountdown.value = 0
  }
}

const stopPushTest = () => {
  abortController?.abort()
}

const restoreDefaults = async () => {
  if (isRunning.value) return
  try {
    await ElMessageBox.confirm('确定恢复脚本中的默认地址和请求体吗？', '恢复默认配置', {
      confirmButtonText: '恢复',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  baseURL.value = 'http://34.61.243.173:8080'
  healthPath.value = '/health'
  pushPath.value = '/api/v1/push'
  statsPath.value = '/api/v1/push/stats'
  statsDelaySeconds.value = 3
  payloadText.value = JSON.stringify(defaultPayload, null, 2)
  isPayloadEditing.value = false
  resetRuntime()
  syncStepPaths()
}

const selectStep = (step: PushTestStep) => {
  selectedStepID.value = step.id
  activeTab.value = 'payload'
}

const formatJSON = (value: unknown) => value === undefined ? '暂无数据' : JSON.stringify(value, null, 2)
const stepStatusText = (status: StepStatus) => ({
  pending: '待执行', running: '执行中', success: '通过', failed: '失败', skipped: '已跳过'
})[status]

onBeforeUnmount(() => {
  abortController?.abort()
  if (elapsedTimer !== null) window.clearInterval(elapsedTimer)
})
</script>

<template>
  <div class="push-test-page">
    <div class="push-test-workspace">
      <aside class="push-config-panel">
        <div class="panel-badge-row">
          <span class="push-badge">GOFCM PUSH</span>
          <el-icon class="panel-info-icon"><Promotion /></el-icon>
        </div>

        <div class="panel-title-block">
          <h1>推送测试</h1>
          <p>用户 ID 立即推送与任务统计验证</p>
        </div>

        <div class="real-push-warning">
          <el-icon><Warning /></el-icon>
          <span>执行后会向请求体中的 user_ids 发送真实推送，请先核对应用、用户和跳转地址。</span>
        </div>

        <section class="config-card connection-card">
          <div class="config-card__header">
            <div class="config-card__identity">
              <span class="case-order">01</span>
              <div>
                <strong>GoFCM 服务配置</strong>
                <small>独立后端代理，不受删除账号页面影响</small>
              </div>
            </div>
            <el-icon><Connection /></el-icon>
          </div>
          <div class="config-card__body">
            <label class="field-block">
              <span>API 服务地址</span>
              <el-input v-model="baseURL" placeholder="http://34.61.243.173:8080" @blur="baseURL = baseURL.trim(); persistConfig()" />
            </label>
            <div class="field-grid">
              <label class="field-block">
                <span>健康检查路径</span>
                <el-input v-model="healthPath" @blur="syncStepPaths" />
              </label>
              <label class="field-block">
                <span>统计等待时间</span>
                <el-input-number v-model="statsDelaySeconds" :min="0" :max="30" :step="1" controls-position="right" />
              </label>
            </div>
          </div>
        </section>

        <div class="nested-arrow">
          <span>健康检查通过后执行</span>
          <el-icon><ArrowDown /></el-icon>
        </div>

        <section class="config-card push-case-card">
          <div class="config-card__header">
            <div class="config-card__identity">
              <span class="case-order case-order--orange">02</span>
              <div>
                <strong>用户 ID 立即推送</strong>
                <small>POST 请求体可完整编辑</small>
              </div>
            </div>
            <span class="method-chip">POST</span>
          </div>
          <div class="config-card__body">
            <div class="field-grid">
              <label class="field-block">
                <span>推送接口路径</span>
                <el-input v-model="pushPath" @blur="syncStepPaths" />
              </label>
              <label class="field-block">
                <span>统计接口路径</span>
                <el-input v-model="statsPath" @blur="syncStepPaths" />
              </label>
            </div>
            <div class="payload-editor-heading">
              <div>
                <strong>完整推送请求体</strong>
                <small>所有配置项均可修改，双击预览进入编辑</small>
              </div>
              <button
                type="button"
                class="inline-edit-button"
                @mousedown.prevent
                @click.stop="isPayloadEditing ? finishPayloadEditing() : startPayloadEditing()"
              >
                <el-icon><Edit /></el-icon>
                {{ isPayloadEditing ? '完成编辑' : '编辑请求体' }}
              </button>
            </div>
            <div
              class="json-request-card"
              :class="{ 'json-request-card--editing': isPayloadEditing }"
              title="双击编辑完整请求体"
              @dblclick="startPayloadEditing"
            >
              <div class="json-request-toolbar">
                <div>
                  <span class="editor-state-dot"></span>
                  <span>{{ isPayloadEditing ? '请求体编辑' : '请求体预览' }}</span>
                </div>
                <em>JSON</em>
              </div>
              <el-input
                v-if="isPayloadEditing"
                v-model="payloadText"
                type="textarea"
                :rows="18"
                resize="none"
                spellcheck="false"
                class="payload-editor"
                autofocus
                @blur="finishPayloadEditing"
              />
              <pre v-else class="json-preview-lines"><span
                v-for="(line, index) in payloadPreviewLines"
                :key="index"
                class="json-preview-line"
              ><span>{{ line.prefix }}</span><span v-if="line.highlight !== undefined" class="json-highlight-value">{{ line.highlight }}</span><span v-if="line.suffix !== undefined">{{ line.suffix }}</span></span></pre>
            </div>
          </div>
        </section>
      </aside>

      <main class="push-result-panel">
        <header class="result-header">
          <div class="result-status">
            <span class="status-dot" :class="`status-dot--${runStatus}`"></span>
            <strong>{{ runStatusText }}</strong>
            <span>·</span>
            <span>{{ formattedElapsed }}</span>
          </div>
          <span v-if="statsCountdown > 0" class="countdown-chip">{{ statsCountdown }}s 后查询统计</span>
        </header>

        <nav class="result-tabs">
          <button :class="{ active: activeTab === 'pipeline' }" @click="activeTab = 'pipeline'">
            <Promotion /> 执行链路
          </button>
          <button :class="{ active: activeTab === 'payload' }" @click="activeTab = 'payload'">
            <DocumentIcon /> 报文详情
          </button>
          <button :class="{ active: activeTab === 'logs' }" @click="activeTab = 'logs'">
            <Cpu /> 原始日志
          </button>
        </nav>

        <div class="result-body">
          <section v-if="activeTab === 'pipeline'" class="pipeline-view">
            <div class="pipeline-intro">
              <span>ISOLATED PUSH WORKFLOW</span>
              <h2>GoFCM 推送验证链路</h2>
              <p>完整复现脚本的健康检查、立即推送和任务统计查询。</p>
            </div>

            <div class="pipeline-list">
              <template v-for="(step, index) in steps" :key="step.id">
                <button class="pipeline-step" :class="[`pipeline-step--${step.status}`]" @click="selectStep(step)">
                  <span class="step-state">
                    <span v-if="step.status === 'pending'" class="step-pending"></span>
                    <span v-else-if="step.status === 'running'" class="step-spinner"></span>
                    <el-icon v-else-if="step.status === 'success'"><Check /></el-icon>
                    <el-icon v-else><Close /></el-icon>
                  </span>
                  <span class="step-copy">
                    <span class="step-title-row">
                      <strong>{{ index + 1 }}. {{ step.name }}</strong>
                      <small v-if="step.latency !== undefined">{{ step.latency }}ms</small>
                    </span>
                    <span class="step-meta">
                      <em :class="`method-${step.method.toLowerCase()}`">{{ step.method }}</em>
                      <code>{{ step.path }}</code>
                      <b v-if="step.code">HTTP {{ step.code }}</b>
                    </span>
                    <small v-if="step.error" class="step-error">{{ step.error }}</small>
                  </span>
                  <span class="step-status-text">{{ stepStatusText(step.status) }}</span>
                </button>
                <div v-if="index < steps.length - 1" class="pipeline-connector">
                  <el-icon><ArrowDown /></el-icon>
                </div>
              </template>
            </div>
          </section>

          <section v-else-if="activeTab === 'payload'" class="payload-view">
            <div class="payload-titlebar">
              <div>
                <span>SELECTED STEP</span>
                <strong>{{ selectedStep.name }}</strong>
              </div>
              <em>{{ selectedStep.method }} {{ selectedStep.path }}</em>
            </div>
            <div class="payload-columns">
              <div class="payload-box">
                <h3>Request</h3>
                <pre>{{ formatJSON(selectedStep.requestData) }}</pre>
              </div>
              <div class="payload-box">
                <h3>Response</h3>
                <pre>{{ formatJSON(selectedStep.responseData) }}</pre>
              </div>
            </div>
          </section>

          <section v-else class="logs-view">
            <div v-if="logs.length === 0" class="empty-logs">等待执行推送测试...</div>
            <div v-for="(log, index) in logs" :key="index" class="log-line">{{ log }}</div>
          </section>
        </div>
      </main>
    </div>

    <footer class="push-action-bar">
      <div class="action-summary">
        <div><span>STATUS</span><strong>{{ runStatusText }}</strong></div>
        <div><span>DURATION</span><strong>{{ formattedElapsed }}</strong></div>
        <div><span>MODE</span><strong>Immediate</strong></div>
      </div>
      <div class="action-buttons">
        <button class="action-button action-button--secondary" :disabled="isRunning" @click="restoreDefaults">
          <el-icon><RefreshRight /></el-icon>
          恢复默认
        </button>
        <button v-if="isRunning" class="action-button action-button--stop" @click="stopPushTest">
          <el-icon><VideoPause /></el-icon>
          停止执行
        </button>
        <button v-else class="action-button action-button--execute" @click="runPushTest">
          <el-icon><VideoPlay /></el-icon>
          执行推送测试
        </button>
      </div>
    </footer>
  </div>
</template>

<style scoped>
.push-test-page {
  position: relative;
  display: flex;
  flex-direction: column;
  height: calc(100vh - 60px);
  margin: -24px;
  overflow: hidden;
  background: #f5f6fa;
  color: #1e293b;
}

.push-test-workspace {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: 16px;
  padding: 16px 16px 80px;
}

.push-config-panel,
.push-result-panel {
  border: 1px solid #edf0f4;
  border-radius: 14px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.035);
}

.push-config-panel {
  width: clamp(460px, 32vw, 590px);
  flex-shrink: 0;
  padding: 22px;
  overflow-y: auto;
}

.panel-badge-row,
.config-card__header,
.result-header,
.payload-titlebar,
.step-title-row,
.push-action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.push-badge {
  padding: 6px 12px;
  border-radius: 999px;
  background: rgba(245, 158, 11, 0.12);
  color: #b45309;
  font-size: 11px;
  font-weight: 900;
  letter-spacing: .08em;
}

.panel-info-icon { color: #f59e0b; font-size: 20px; }
.panel-title-block { margin: 18px 0 16px; padding-bottom: 16px; border-bottom: 1px solid #eef2f7; }
.panel-title-block h1 { margin: 0; font-size: 22px; }
.panel-title-block p { margin: 6px 0 0; color: #64748b; font-size: 13px; }

.real-push-warning {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 11px 12px;
  border: 1px solid #fde68a;
  border-radius: 10px;
  background: #fffbeb;
  color: #92400e;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.55;
}

.real-push-warning .el-icon { margin-top: 2px; flex-shrink: 0; }
.config-card { margin-top: 14px; overflow: hidden; border: 1px solid #e2e8f0; border-radius: 12px; }
.connection-card { border-color: #bfdbfe; }
.push-case-card { border-color: #dbe5f1; box-shadow: 0 10px 28px rgba(15, 23, 42, .045); }
.config-card__header { padding: 13px 14px; border-bottom: 1px solid #eef2f7; background: linear-gradient(135deg, #f8fafc, #fff); }
.push-case-card .config-card__header { background: linear-gradient(135deg, #f8fbff, #fff 72%); }
.config-card__identity { display: flex; align-items: center; gap: 10px; min-width: 0; }
.config-card__identity > div { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.config-card__identity strong { font-size: 13px; }
.config-card__identity small { color: #94a3b8; font-size: 10px; }
.case-order { display: grid; place-items: center; width: 26px; height: 26px; border-radius: 8px; background: #dbeafe; color: #1d4ed8; font-size: 10px; font-weight: 900; }
.case-order--orange { background: #dbeafe; color: #1d4ed8; }
.method-chip { padding: 4px 8px; border-radius: 6px; background: #dbeafe; color: #1d4ed8; font: 800 10px/1 ui-monospace, monospace; }
.config-card__body { display: flex; flex-direction: column; gap: 13px; padding: 14px; }
.field-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 10px; }
.field-block { display: flex; flex-direction: column; gap: 6px; min-width: 0; color: #475569; font-size: 11px; font-weight: 700; }
.field-block :deep(.el-input-number) { width: 100%; }
.field-block :deep(.el-input__wrapper), .field-block :deep(.el-input-number .el-input__wrapper) { border-radius: 7px; background: #f8fafc; box-shadow: 0 0 0 1px #e2e8f0 inset; }
.payload-editor-heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.payload-editor-heading > div { display: flex; flex-direction: column; gap: 3px; }
.payload-editor-heading strong { color: #334155; font-size: 11px; }
.payload-editor-heading small { color: #94a3b8; font-size: 9px; font-weight: 500; }
.inline-edit-button { display: inline-flex; height: 28px; align-items: center; gap: 5px; padding: 0 10px; border: 1px solid #dbe5f1; border-radius: 7px; background: #fff; color: #475569; cursor: pointer; font-size: 10px; font-weight: 700; transition: .18s ease; }
.inline-edit-button:hover { border-color: #93c5fd; background: #eff6ff; color: #2563eb; }
.json-request-card { overflow: hidden; border: 1px solid #263449; border-radius: 10px; background: #0f172a; cursor: text; transition: border-color .18s ease, box-shadow .18s ease; }
.json-request-card:hover { border-color: #475569; }
.json-request-card--editing { border-color: #60a5fa; box-shadow: 0 0 0 3px rgba(59, 130, 246, .12); }
.json-request-toolbar { display: flex; height: 34px; align-items: center; justify-content: space-between; padding: 0 11px; border-bottom: 1px solid #263449; background: #111c2e; }
.json-request-toolbar > div { display: flex; align-items: center; gap: 6px; color: #94a3b8; font-size: 9px; font-weight: 700; }
.json-request-toolbar em { padding: 2px 6px; border-radius: 999px; background: rgba(59, 130, 246, .13); color: #93c5fd; font-size: 9px; font-style: normal; font-weight: 800; }
.editor-state-dot { width: 6px; height: 6px; border-radius: 50%; background: #4ade80; box-shadow: 0 0 0 3px rgba(74, 222, 128, .1); }
.json-preview-lines { min-height: 250px; max-height: 390px; margin: 0; padding: 12px; overflow: auto; color: #dbeafe; font: 11px/1.55 ui-monospace, SFMono-Regular, Menlo, monospace; scrollbar-width: none; -ms-overflow-style: none; }
.json-preview-lines::-webkit-scrollbar { display: none; width: 0; height: 0; }
.json-preview-line { display: block; min-height: 17px; white-space: pre-wrap; word-break: break-word; }
.json-highlight-value { color: #4ade80; font-weight: 700; }
.payload-editor :deep(.el-textarea__inner) { min-height: 250px !important; padding: 12px; border: 0; border-radius: 0; background: #0f172a; color: #dbeafe; caret-color: #93c5fd; box-shadow: none; font: 11px/1.55 ui-monospace, SFMono-Regular, Menlo, monospace; scrollbar-width: none; -ms-overflow-style: none; }
.payload-editor :deep(.el-textarea__inner::-webkit-scrollbar) { display: none; width: 0; height: 0; }
.nested-arrow { display: flex; flex-direction: column; align-items: center; gap: 2px; padding-top: 9px; color: #64748b; font-size: 9px; font-weight: 800; }

.push-result-panel { display: flex; flex: 1; min-width: 0; flex-direction: column; overflow: hidden; }
.result-header { height: 50px; flex-shrink: 0; padding: 0 20px; border-bottom: 1px solid #eef2f7; }
.result-status { display: flex; align-items: center; gap: 8px; color: #64748b; font-size: 12px; }
.result-status strong { color: #1e293b; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; background: #94a3b8; }
.status-dot--running { background: #3b82f6; animation: pulse 1.2s infinite; }
.status-dot--success { background: #10b981; }
.status-dot--failed { background: #ef4444; }
.status-dot--stopped, .status-dot--skipped { background: #f59e0b; }
.countdown-chip { padding: 4px 9px; border-radius: 999px; background: #fff7ed; color: #c2410c; font-size: 10px; font-weight: 800; }
.result-tabs { display: flex; gap: 5px; padding: 8px 12px; border-bottom: 1px solid #eef2f7; background: #f8fafc; }
.result-tabs button { display: flex; align-items: center; gap: 6px; padding: 7px 12px; border: 0; border-radius: 7px; background: transparent; color: #64748b; cursor: pointer; font-size: 11px; font-weight: 700; }
.result-tabs button svg { width: 14px; }
.result-tabs button.active { background: #3b82f6; color: #fff; box-shadow: 0 3px 8px rgba(59, 130, 246, .22); }
.result-body { flex: 1; min-height: 0; overflow: auto; }
.pipeline-view, .payload-view { padding: 24px; }
.pipeline-intro span, .payload-titlebar span { color: #f59e0b; font-size: 9px; font-weight: 900; letter-spacing: .1em; }
.pipeline-intro h2 { margin: 5px 0 6px; font-size: 20px; }
.pipeline-intro p { margin: 0; color: #64748b; font-size: 12px; }
.pipeline-list { max-width: 760px; margin: 28px auto 0; }
.pipeline-step { display: flex; width: 100%; align-items: center; gap: 13px; padding: 15px; border: 1px solid #e2e8f0; border-radius: 12px; background: #fff; color: inherit; text-align: left; cursor: pointer; transition: .2s; }
.pipeline-step:hover { transform: translateY(-1px); border-color: #93c5fd; box-shadow: 0 8px 22px rgba(15, 23, 42, .06); }
.pipeline-step--running { border-color: #60a5fa; box-shadow: 0 0 0 3px rgba(59, 130, 246, .08); }
.pipeline-step--success { border-color: #86efac; }
.pipeline-step--failed { border-color: #fca5a5; background: #fffafa; }
.step-state { display: grid; place-items: center; width: 30px; height: 30px; flex-shrink: 0; border-radius: 9px; background: #f1f5f9; color: #10b981; }
.step-pending { width: 8px; height: 8px; border-radius: 50%; background: #cbd5e1; }
.step-spinner { width: 13px; height: 13px; border: 2px solid #bfdbfe; border-top-color: #2563eb; border-radius: 50%; animation: spin .8s linear infinite; }
.step-copy { display: flex; flex: 1; min-width: 0; flex-direction: column; gap: 6px; }
.step-title-row strong { font-size: 13px; }
.step-title-row small { color: #64748b; font-size: 10px; }
.step-meta { display: flex; align-items: center; gap: 8px; min-width: 0; }
.step-meta em { padding: 3px 6px; border-radius: 5px; font: 800 9px/1 ui-monospace, monospace; font-style: normal; }
.method-get { background: #dcfce7; color: #15803d; }
.method-post { background: #dbeafe; color: #1d4ed8; }
.step-meta code { overflow: hidden; color: #64748b; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.step-meta b { margin-left: auto; color: #475569; font-size: 9px; white-space: nowrap; }
.step-error { color: #dc2626; font-size: 10px; }
.step-status-text { color: #94a3b8; font-size: 10px; font-weight: 700; }
.pipeline-connector { display: grid; height: 32px; place-items: center; color: #cbd5e1; }
.payload-titlebar { padding-bottom: 14px; border-bottom: 1px solid #eef2f7; }
.payload-titlebar > div { display: flex; flex-direction: column; gap: 4px; }
.payload-titlebar strong { font-size: 15px; }
.payload-titlebar em { padding: 5px 8px; border-radius: 6px; background: #eff6ff; color: #1d4ed8; font: 800 9px/1 ui-monospace, monospace; font-style: normal; }
.payload-columns { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 14px; margin-top: 16px; }
.payload-box { min-width: 0; overflow: hidden; border: 1px solid #e2e8f0; border-radius: 10px; }
.payload-box h3 { margin: 0; padding: 9px 12px; border-bottom: 1px solid #e2e8f0; background: #f8fafc; font-size: 10px; text-transform: uppercase; }
.payload-box pre { min-height: 320px; max-height: calc(100vh - 300px); margin: 0; padding: 14px; overflow: auto; background: #0f172a; color: #dbeafe; font: 11px/1.6 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre-wrap; word-break: break-word; }
.logs-view { min-height: 100%; padding: 18px 20px; background: #0f172a; color: #cbd5e1; font: 11px/1.65 ui-monospace, SFMono-Regular, Menlo, monospace; }
.log-line { margin-bottom: 5px; white-space: pre-wrap; word-break: break-word; }
.empty-logs { color: #64748b; }

.push-action-bar { position: absolute; right: 0; bottom: 0; left: 0; height: 64px; padding: 0 24px; border-top: 1px solid #e2e8f0; background: #fff; box-shadow: 0 -4px 12px rgba(0, 0, 0, .025); }
.action-summary { display: flex; gap: 34px; }
.action-summary > div { display: flex; flex-direction: column; gap: 2px; }
.action-summary span { color: #94a3b8; font-size: 9px; font-weight: 800; letter-spacing: .06em; }
.action-summary strong { font-size: 13px; }
.action-buttons { display: flex; gap: 10px; }
.action-button { display: inline-flex; height: 40px; align-items: center; justify-content: center; gap: 7px; padding: 0 20px; border: 1px solid transparent; border-radius: 8px; cursor: pointer; font-size: 14px; font-weight: 600; transition: .18s ease; }
.action-button:disabled { cursor: not-allowed; opacity: .5; }
.action-button--secondary, .action-button--stop { border-color: #e2e8f0; background: #fff; color: #64748b; }
.action-button--secondary:not(:disabled):hover { border-color: #bfdbfe; background: #f8fbff; color: #2563eb; }
.action-button--stop:hover { border-color: #fecaca; background: #fff7f7; color: #dc2626; }
.action-button--execute { background: #3b82f6; color: #fff; box-shadow: 0 5px 13px rgba(59, 130, 246, .22); }
.action-button--execute:hover { background: #2563eb; box-shadow: 0 7px 16px rgba(37, 99, 235, .27); transform: translateY(-1px); }

@keyframes spin { to { transform: rotate(360deg); } }
@keyframes pulse { 50% { opacity: .35; } }

@media (max-width: 1100px) {
  .push-config-panel { width: 440px; }
  .payload-columns { grid-template-columns: 1fr; }
}
</style>
