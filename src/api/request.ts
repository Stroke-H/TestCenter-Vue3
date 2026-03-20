// Axios 请求封装 — 统一拦截器、错误处理、基础配置
import axios from 'axios'
import type { AxiosInstance, AxiosResponse, InternalAxiosRequestConfig } from 'axios'

// 创建 Axios 实例，配置基础 URL 与超时时间
const service: AxiosInstance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// 请求拦截器 — 可在此统一注入 token 等
service.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // 如有 token，在此注入
    // const token = localStorage.getItem('token')
    // if (token) config.headers.Authorization = `Bearer ${token}`
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
  (error) => {
    const { response } = error
    if (response) {
      // HTTP 状态码错误处理
      switch (response.status) {
        case 401:
          console.error('[Auth] 未授权，请重新登录')
          break
        case 403:
          console.error('[Auth] 权限不足')
          break
        case 500:
          console.error('[Server] 服务器内部错误')
          break
        default:
          console.error(`[HTTP ${response.status}]`, response.data?.message || '请求失败')
      }
    } else {
      console.error('[Network] 网络异常，请检查连接')
    }
    return Promise.reject(error)
  }
)

export default service
