import { defineStore } from 'pinia'
import { computed, shallowRef } from 'vue'
import { buildBackendUrl, buildBackendWsUrl, normalizeBackendUrl } from '@/utils/runtimeUrl'

type DramaRunStatus = 'idle' | 'running' | 'done' | 'failed' | 'stopped'

interface StartDramaRunOptions {
  email: string
  password: string
  loginUrl: string
  dramaListUrl: string
  toolName: string
  author: string
  environment: 'test' | 'prod' | string
}

interface DramaRunSnapshot {
  runId?: string
  status: DramaRunStatus
  progress?: number
  logs?: string[]
  reportUrl?: string
  duration?: number
  error?: string
}

export const useDramaRunStore = defineStore('dramaRun', () => {
  const runId = shallowRef('')
  const status = shallowRef<DramaRunStatus>('idle')
  const progress = shallowRef(0)
  const logs = shallowRef<string[]>(['准备就绪，点击 Execute 开始执行'])
  const reportUrl = shallowRef('')
  const duration = shallowRef(0)
  const uptime = shallowRef(0)
  const isRunningInBackground = shallowRef(false)
  const bubbleText = shallowRef('')
  const bubbleVisible = shallowRef(false)

  const ws = shallowRef<WebSocket | null>(null)
  let timer: ReturnType<typeof setInterval> | null = null
  let bubbleTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let intentionalClose = false

  const assistantLabel = computed(() => {
    if (status.value === 'running') return `${progress.value}%`
    if (status.value === 'done') return 'Done'
    return '智能助手'
  })

  const isAssistantActive = computed(() => status.value === 'running' || status.value === 'done')

  const showBubble = (text: string) => {
    bubbleText.value = text
    bubbleVisible.value = true
    if (bubbleTimer) clearTimeout(bubbleTimer)
    bubbleTimer = window.setTimeout(() => {
      bubbleVisible.value = false
      if (status.value === 'done') {
        resetAssistantState()
      }
    }, 3000)
  }

  const pushLog = (line: string) => {
    logs.value = [...logs.value, line]
  }

  const startTimer = (initialDuration = 0) => {
    if (timer) clearInterval(timer)
    uptime.value = initialDuration
    duration.value = initialDuration
    timer = window.setInterval(() => {
      uptime.value++
      duration.value++
    }, 1000)
  }

  const stopTimer = () => {
    if (timer) {
      clearInterval(timer)
      timer = null
    }
  }

  const applySnapshot = (snapshot: DramaRunSnapshot) => {
    runId.value = snapshot.runId || runId.value
    status.value = snapshot.status || 'idle'
    progress.value = snapshot.progress ?? progress.value
    logs.value = snapshot.logs?.length ? snapshot.logs : logs.value
    reportUrl.value = snapshot.reportUrl ? normalizeBackendUrl(snapshot.reportUrl) : reportUrl.value
    duration.value = snapshot.duration ?? duration.value
    uptime.value = snapshot.duration ?? uptime.value

    if (status.value === 'running') {
      startTimer(snapshot.duration || 0)
    } else {
      stopTimer()
    }
  }

  const start = async (options: StartDramaRunOptions) => {
    if (status.value === 'running') return

    closeSubscriber()
    status.value = 'running'
    progress.value = 0
    reportUrl.value = ''
    logs.value = [`[${new Date().toLocaleTimeString()}] 正在创建后端剧集测试任务...`]
    isRunningInBackground.value = false

    const response = await fetch(buildBackendUrl('/api/drama-runs/start'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(options)
    })
    if (!response.ok) {
      status.value = 'failed'
      pushLog(`[ERROR] 创建后端任务失败: ${response.status}`)
      return
    }
    const snapshot = await response.json() as DramaRunSnapshot
    applySnapshot(snapshot)
    connectSubscriber()
  }

  const recoverCurrentRun = async () => {
    if (status.value === 'running') return
    try {
      const response = await fetch(buildBackendUrl('/api/drama-runs/current'))
      if (!response.ok) return
      const snapshot = await response.json() as DramaRunSnapshot
      if (snapshot.status !== 'running') return
      applySnapshot(snapshot)
      isRunningInBackground.value = true
      connectSubscriber()
      showBubble('当前剧集接口测试正在后台运行')
    } catch (error) {
      console.warn('Failed to recover drama run', error)
    }
  }

  const connectSubscriber = () => {
    closeSubscriber()
    intentionalClose = false
    const socket = new WebSocket(buildBackendWsUrl('/api/ws/drama-run'))
    ws.value = socket

    socket.onmessage = (event) => {
      const data = event.data
      if (typeof data !== 'string') return
      if (data.startsWith('DRAMA_SNAPSHOT:')) {
        try {
          applySnapshot(JSON.parse(data.slice('DRAMA_SNAPSHOT:'.length)))
        } catch (error) {
          console.warn('Failed to parse drama snapshot', error)
        }
        return
      }
      if (data.startsWith('DRAMA_PROGRESS:')) {
        const nextProgress = Number(data.split(':')[1] || 0)
        if (Number.isFinite(nextProgress)) {
          progress.value = Math.max(progress.value, Math.min(100, nextProgress))
        }
        return
      }
      if (data.startsWith('REPORT_READY_URL:')) {
        const nextReportUrl = data.slice('REPORT_READY_URL:'.length)
        reportUrl.value = nextReportUrl ? normalizeBackendUrl(nextReportUrl) : ''
        return
      }
      if (data.startsWith('REPORT_READY:')) {
        pushLog(`[WARN] 收到旧版报告文件事件，已忽略以避免读取可覆盖报告。`)
        return
      }
      if (data.startsWith('EXECUTION_STATUS:')) {
        handleExecutionStatus(data)
        return
      }
      pushLog(data)
    }

    socket.onclose = () => {
      ws.value = null
      if (intentionalClose || status.value !== 'running') return
      pushLog('[INFO] 进度订阅已断开，后端任务仍在运行，正在自动重连...')
      scheduleReconnect()
    }

    socket.onerror = () => {
      pushLog('[WARN/ERR] 剧集测试进度订阅连接异常')
    }
  }

  const handleExecutionStatus = (data: string) => {
    const [, nextStatus, reason] = data.split(':')
    if (nextStatus === 'success') {
      progress.value = 100
      stopTimer()
      if (!reportUrl.value) {
        status.value = 'failed'
        pushLog(`[${new Date().toLocaleTimeString()}] 任务完成但未收到唯一报告快照，已阻止使用默认覆盖报告。`)
        return
      }
      status.value = 'done'
      pushLog(`[${new Date().toLocaleTimeString()}] 任务执行完成。`)
      if (isRunningInBackground.value) {
        showBubble('当前接口测试完成啦！')
      }
      return
    }
    if (nextStatus === 'stopped') {
      status.value = 'stopped'
      stopTimer()
      isRunningInBackground.value = false
      return
    }
    if (nextStatus === 'failed') {
      status.value = 'failed'
      stopTimer()
      pushLog(`[${new Date().toLocaleTimeString()}] 任务执行失败${reason ? `: ${reason}` : ''}`)
    }
  }

  const scheduleReconnect = () => {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = window.setTimeout(() => {
      if (status.value === 'running') {
        connectSubscriber()
      }
    }, 2000)
  }

  const closeSubscriber = () => {
    intentionalClose = true
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    ws.value?.close()
    ws.value = null
  }

  const markBackground = () => {
    if (status.value !== 'running' || isRunningInBackground.value) return
    isRunningInBackground.value = true
    showBubble('当前剧集接口测试正在后台运行')
  }

  const stop = async () => {
    if (status.value !== 'running') return
    await fetch(buildBackendUrl('/api/drama-runs/stop'), { method: 'POST' })
    status.value = 'stopped'
    stopTimer()
    closeSubscriber()
    isRunningInBackground.value = false
    pushLog(`[${new Date().toLocaleTimeString()}] 手动终止执行。`)
  }

  const resetAssistantState = () => {
    if (status.value === 'done') {
      status.value = 'idle'
      progress.value = 0
      isRunningInBackground.value = false
      bubbleText.value = ''
      closeSubscriber()
    }
  }

  return {
    runId,
    status,
    progress,
    logs,
    reportUrl,
    duration,
    uptime,
    isRunningInBackground,
    bubbleText,
    bubbleVisible,
    assistantLabel,
    isAssistantActive,
    start,
    recoverCurrentRun,
    markBackground,
    stop,
    resetAssistantState
  }
})
