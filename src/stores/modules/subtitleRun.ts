import { defineStore } from 'pinia'
import { shallowRef } from 'vue'
import { buildBackendUrl, buildBackendWsUrl, normalizeBackendUrl } from '@/utils/runtimeUrl'

type SubtitleRunStatus = 'idle' | 'running' | 'done' | 'failed' | 'stopped'

interface StartSubtitleRunOptions {
  email: string
  password: string
  loginUrl: string
  dramaListUrl: string
  toolName: string
  author: string
  environment: 'test' | 'prod' | string
}

interface SubtitleRunSnapshot {
  runId?: string
  status: SubtitleRunStatus
  progress?: number
  logs?: string[]
  reportUrl?: string
  duration?: number
  error?: string
}

export const useSubtitleRunStore = defineStore('subtitleRun', () => {
  const runId = shallowRef('')
  const status = shallowRef<SubtitleRunStatus>('idle')
  const progress = shallowRef(0)
  const logs = shallowRef<string[]>(['准备就绪，点击 Execute 开始执行'])
  const reportUrl = shallowRef('')
  const duration = shallowRef(0)
  const uptime = shallowRef(0)

  const ws = shallowRef<WebSocket | null>(null)
  let timer: ReturnType<typeof setInterval> | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let intentionalClose = false

  const pushLog = (line: string) => {
    if (logs.value[logs.value.length - 1] === line) return
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

  const applySnapshot = (snapshot: SubtitleRunSnapshot) => {
    runId.value = snapshot.runId || runId.value
    status.value = snapshot.status || 'idle'
    progress.value = snapshot.progress ?? progress.value
    logs.value = snapshot.logs?.length ? dedupeAdjacentLogs(snapshot.logs) : logs.value
    reportUrl.value = snapshot.reportUrl ? normalizeBackendUrl(snapshot.reportUrl) : reportUrl.value
    duration.value = snapshot.duration ?? duration.value
    uptime.value = snapshot.duration ?? uptime.value

    if (status.value === 'running') {
      startTimer(snapshot.duration || 0)
    } else {
      stopTimer()
    }
  }

  const start = async (options: StartSubtitleRunOptions) => {
    if (status.value === 'running') return

    closeSubscriber()
    status.value = 'running'
    progress.value = 0
    reportUrl.value = ''
    logs.value = [`[${new Date().toLocaleTimeString()}] 正在创建后端字幕检查任务...`]

    const response = await fetch(buildBackendUrl('/api/subtitle-runs/start'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(options)
    })
    if (!response.ok) {
      status.value = 'failed'
      pushLog(`[ERROR] 创建后端任务失败: ${response.status}`)
      return
    }
    const snapshot = await response.json() as SubtitleRunSnapshot
    applySnapshot(snapshot)
    connectSubscriber()
  }

  const recoverCurrentRun = async () => {
    if (status.value === 'running') return
    try {
      const response = await fetch(buildBackendUrl('/api/subtitle-runs/current'))
      if (!response.ok) return
      const snapshot = await response.json() as SubtitleRunSnapshot
      if (snapshot.status !== 'running') return
      applySnapshot(snapshot)
      connectSubscriber()
    } catch (error) {
      console.warn('Failed to recover subtitle run', error)
    }
  }

  const connectSubscriber = () => {
    closeSubscriber()
    intentionalClose = false
    const socket = new WebSocket(buildBackendWsUrl('/api/ws/subtitle-run'))
    ws.value = socket

    socket.onmessage = (event) => {
      const data = event.data
      if (typeof data !== 'string') return
      if (data.startsWith('SUBTITLE_SNAPSHOT:')) {
        try {
          applySnapshot(JSON.parse(data.slice('SUBTITLE_SNAPSHOT:'.length)))
        } catch (error) {
          console.warn('Failed to parse subtitle snapshot', error)
        }
        return
      }
      if (data.startsWith('SUBTITLE_PROGRESS:') || data.startsWith('DRAMA_PROGRESS:')) {
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
      if (data.startsWith('EXECUTION_STATUS:')) {
        handleExecutionStatus(data)
        return
      }
      pushLog(data)
    }

    socket.onclose = () => {
      ws.value = null
      if (intentionalClose || status.value !== 'running') return
      pushLog('[INFO] 字幕检查进度订阅已断开，后端任务仍在运行，正在自动重连...')
      scheduleReconnect()
    }

    socket.onerror = () => {
      pushLog('[WARN/ERR] 字幕检查进度订阅连接异常')
    }
  }

  const handleExecutionStatus = (data: string) => {
    const [, nextStatus, reason] = data.split(':')
    if (nextStatus === 'success') {
      progress.value = 100
      stopTimer()
      if (!reportUrl.value) {
        status.value = 'failed'
        pushLog(`[${new Date().toLocaleTimeString()}] 任务完成但未收到报告地址。`)
        return
      }
      status.value = 'done'
      pushLog(`[${new Date().toLocaleTimeString()}] 任务执行完成。`)
      return
    }
    if (nextStatus === 'stopped') {
      status.value = 'stopped'
      stopTimer()
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

  const stop = async () => {
    if (status.value !== 'running') return
    await fetch(buildBackendUrl('/api/subtitle-runs/stop'), { method: 'POST' })
    status.value = 'stopped'
    stopTimer()
    closeSubscriber()
    pushLog(`[${new Date().toLocaleTimeString()}] 手动终止执行。`)
  }

  const dedupeAdjacentLogs = (items: string[]) => {
    return items.filter((item, index) => index === 0 || item !== items[index - 1])
  }

  return {
    runId,
    status,
    progress,
    logs,
    reportUrl,
    duration,
    uptime,
    start,
    recoverCurrentRun,
    stop
  }
})
