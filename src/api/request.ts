// Axios 请求封装 — 统一拦截器、错误处理、基础配置
import axios from 'axios'
import type { AxiosInstance, AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'

const retryDelayMs = 800

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function isRetryableRequest(error: any) {
  const method = String(error?.config?.method || 'get').toLowerCase()
  if (!['get', 'head', 'options'].includes(method)) return false

  const status = error?.response?.status
  const message = String(error?.message || '').toLowerCase()
  const code = error?.code

  return code === 'ECONNABORTED'
    || message.includes('timeout')
    || message.includes('network error')
    || [500, 502, 503, 504].includes(status)
}

function notifyRetrying() {
  ElMessage({
    message: '网络波动，正在重新尝试...',
    type: 'warning',
    duration: 1800,
    showClose: true
  })
}

// 创建 Axios 实例，配置基础 URL 与超时时间
const service: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 15000,
  withCredentials: true,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器 — 可在此统一注入 token 等
service.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    return config
  },
  (error) => {
    console.error('[Request Error]', error)
    return Promise.reject(error)
  }
)

// 响应拦截器 — 统一错误处理
service.interceptors.response.use(
  (response: AxiosResponse) => {
    // 直接返回 data 层，减少调用方解包
    return response.data
  },
  async (error) => {
    const config = error.config as any
    if (config && isRetryableRequest(error) && !config.__retried) {
      config.__retried = true
      notifyRetrying()
      await sleep(retryDelayMs)
      return service.request(config)
    }

    const { response, code, message } = error
    let errorMsg = '网络或服务器异常'

    if (response) {
      // HTTP 状态码错误处理
      switch (response.status) {
        case 401:
          errorMsg = '未授权，请检查登录凭证或重新登录'
          break
        case 403:
          errorMsg = '拒绝访问：权限不足'
          break
        case 404:
          errorMsg = '请求地址错误 (404)'
          break
        case 500:
          errorMsg = response.data?.error || response.data?.message || '后端服务器运行报错 (500)'
          break
        case 502:
        case 503:
          errorMsg = '服务器正在启动中或网关连接失败 (502/503)'
          break
        default:
          errorMsg = response.data?.error || response.data?.message || `请求失败 (${response.status})`
      }
    } else if (code === 'ECONNABORTED' || message.includes('timeout')) {
      errorMsg = '服务器响应超时，请确认后端 8080 端口是否正常开启'
    } else if (message.includes('Network Error')) {
      errorMsg = '无法连接到服务器，可能原因：\n1. 后端服务尚未启动\n2. 网络连接不通\n3. 域名解析错误'
    } else {
      errorMsg = message || '发生了未知的网络错误'
    }

    // 将具体的错误信息挂载到 error 对象上，方便 UI 侧精准显示
    error.customMessage = errorMsg
    console.error('[Request Error]', { code, message, response, errorMsg })
    
    return Promise.reject(error)
  }
)

export default service
