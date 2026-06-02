import { shallowRef } from 'vue'
import { buildBackendUrl } from '@/utils/runtimeUrl'

interface MonkeyStreamMessage<T = unknown> {
  id: number
  type: string
  data: T
}

interface UseMonkeyRunStreamOptions {
  getToken: () => string
  onMessage: (message: MonkeyStreamMessage) => void
  onConnected?: () => void
  onDisconnected?: () => void
  onError?: (error: Error) => void
}

export function useMonkeyRunStream(options: UseMonkeyRunStreamOptions) {
  const connected = shallowRef(false)
  let activeRunId = ''
  let abortController: AbortController | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectAttempt = 0
  let lastEventId = 0

  const disconnect = () => {
    activeRunId = ''
    connected.value = false
    abortController?.abort()
    abortController = null
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = null
  }

  const scheduleReconnect = (runId: string) => {
    if (runId !== activeRunId) return
    const delay = Math.min(10_000, 1_000 * 2 ** Math.min(reconnectAttempt, 4))
    reconnectAttempt += 1
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      void openStream(runId)
    }, delay)
  }

  const dispatchFrame = (frame: string) => {
    let type = 'message'
    let id = 0
    const dataLines: string[] = []
    for (const rawLine of frame.split('\n')) {
      const line = rawLine.replace(/\r$/, '')
      if (!line || line.startsWith(':')) continue
      if (line.startsWith('id:')) id = Number(line.slice(3).trim()) || 0
      if (line.startsWith('event:')) type = line.slice(6).trim()
      if (line.startsWith('data:')) dataLines.push(line.slice(5).trimStart())
    }
    if (!dataLines.length) return
    if (id > 0) lastEventId = Math.max(lastEventId, id)
    try {
      options.onMessage({ id, type, data: JSON.parse(dataLines.join('\n')) })
    } catch (error) {
      options.onError?.(error instanceof Error ? error : new Error(String(error)))
    }
  }

  const consumeStream = async (response: Response, signal: AbortSignal) => {
    const reader = response.body?.getReader()
    if (!reader) throw new Error('Monkey SSE 响应不支持流式读取')
    const decoder = new TextDecoder()
    let buffer = ''
    while (!signal.aborted) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n')
      let boundary = buffer.indexOf('\n\n')
      while (boundary >= 0) {
        dispatchFrame(buffer.slice(0, boundary))
        buffer = buffer.slice(boundary + 2)
        boundary = buffer.indexOf('\n\n')
      }
    }
  }

  async function openStream(runId: string) {
    if (!runId || runId !== activeRunId) return
    abortController?.abort()
    abortController = new AbortController()
    try {
      const response = await fetch(buildBackendUrl(`/api/monkey/runs/${runId}/stream`), {
        headers: {
          Accept: 'text/event-stream',
          Authorization: options.getToken(),
          ...(lastEventId > 0 ? { 'Last-Event-ID': String(lastEventId) } : {})
        },
        signal: abortController.signal
      })
      if (!response.ok) throw new Error(`Monkey SSE 连接失败: HTTP ${response.status}`)
      connected.value = true
      reconnectAttempt = 0
      options.onConnected?.()
      await consumeStream(response, abortController.signal)
      if (runId !== activeRunId || abortController.signal.aborted) return
      connected.value = false
      options.onDisconnected?.()
      scheduleReconnect(runId)
    } catch (error) {
      if (runId !== activeRunId || abortController.signal.aborted) return
      connected.value = false
      const streamError = error instanceof Error ? error : new Error(String(error))
      options.onError?.(streamError)
      scheduleReconnect(runId)
    }
  }

  const connect = (runId: string) => {
    disconnect()
    activeRunId = runId
    reconnectAttempt = 0
    lastEventId = 0
    void openStream(runId)
  }

  return {
    connected,
    connect,
    disconnect
  }
}
