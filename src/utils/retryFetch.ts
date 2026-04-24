import { ElMessage } from 'element-plus'

const retryDelayMs = 800

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function shouldRetryResponse(response: Response) {
  return [500, 502, 503, 504].includes(response.status)
}

function notifyRetrying() {
  ElMessage({
    message: '网络波动，正在重新尝试...',
    type: 'warning',
    duration: 1800,
    showClose: true
  })
}

export async function retryFetch(input: RequestInfo | URL, init?: RequestInit) {
  try {
    const firstResponse = await fetch(input, init)
    if (!shouldRetryResponse(firstResponse)) {
      return firstResponse
    }

    notifyRetrying()
    await sleep(retryDelayMs)
    return fetch(input, init)
  } catch (error) {
    notifyRetrying()
    await sleep(retryDelayMs)
    return fetch(input, init)
  }
}
