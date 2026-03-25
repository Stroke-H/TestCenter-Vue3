<script setup lang="ts">
import { ref, onUnmounted, nextTick, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useReportStore } from '@/stores'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Warning,
  CopyDocument,
  Download,
  Close,
  VideoPlay,
  VideoPause,
  Delete
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const reportStore = useReportStore()

// 从路由参数获取工具信息
const toolName = ref((route.query.name as string) || '测试剧集是否重复')
const toolDesc = ref((route.query.desc as string) || '检测剧集数据中是否存在重复的drama_intid')
const projectName = ref('ShortsWave')

// 协助定位后端地址
const getBackendHost = () => {
  return `${window.location.protocol}//${window.location.hostname}:8080`
}

const getWsBase = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${protocol}://${window.location.hostname}:8080`
}

// 是否是剧集播放自检工具
const isDramaCheck = toolName.value.includes('播放')
// 是否是删除账号工具
const isDeleteAccount = toolName.value === '删除账号'
const deleteAccountParam = ref('')

// 自动解析参数并匹配项目
watch(deleteAccountParam, (newVal) => {
  if (!isDeleteAccount || !newVal) return
  
  // 正则匹配 app 参数的值，支持多种空格情况
  const appMatch = newVal.match(/"app":\s*"([^"]+)"/)
  if (appMatch && appMatch[1]) {
    const appId = appMatch[1]
    if (appId === 'com.novelnova.readstory') {
      projectName.value = 'NovelNova'
    } else if (appId === 'com.company.shortsdrama.wave') {
      projectName.value = 'ShortsWave'
    }
  }
})

// 服务器配置档
const serverOptions = [
  { label: '测试服', value: 'test' },
  { label: '正式服', value: 'prod' }
]
const testServer = ref('test')

const serverProfiles = {
  test: {
    email: "test_super_001@shortswave.com",
    password: "test123456",
    loginUrl: "http://35.225.224.94:8080/api/pwd_login",
    dramaListUrl: "http://35.225.224.94:8080/api/management/drama/all_online_ids"
  },
  prod: {
    email: "test001@wedrama.com",
    password: "fb3b2e9961b58",
    loginUrl: "https://admin.shortswave.com/api/pwd_login", // 假设路径对标
    dramaListUrl: "https://admin.shortswave.com/api/management/drama/all_online_ids"
  }
}

// 域名映射配置
const domainMappings = {
  ShortsWave: {
    prod: 'https://api.shortswave.com',
    test: 'http://35.225.224.94'
  },
  NovelNova: {
    prod: 'https://api.novelnovastory.com',
    test: 'http://34.10.7.187'
  }
}

// 辅助函数：解析参数对
const parseParams = (str: string) => {
  const params: Record<string, string> = {}
  const regex = /"([^"]+)":\s*"([^"]*)"/g
  let match
  while ((match = regex.exec(str)) !== null) {
    if (match[1]) {
      params[match[1]] = match[2] || ''
    }
  }
  return params
}

// 辅助函数：执行具体的获取账号删除操作
const handleAccountDelete = async (token: string, originalHeaders: Record<string, string>) => {
  const baseDomain = domainMappings[projectName.value as keyof typeof domainMappings][testServer.value as 'prod' | 'test']
  const deleteUrl = `${baseDomain}/user/delete`
  
  // 准备删除请求的 Headers，替换 X-SESSION-TOKEN
  const deleteHeaders = { 
    ...originalHeaders, 
    'X-SESSION-TOKEN': token 
  }

  logs.value.push(`[${new Date().toLocaleTimeString()}] 正在发起账号注销请求 (id: ${originalHeaders.user_id || '未知'})...`)
  scrollToBottom()

  try {
    const response = await fetch(`${getBackendHost()}/api/proxy`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        method: 'GET',
        url: deleteUrl,
        data: deleteHeaders
      })
    })

    const resData = await response.json()
    logs.value.push(`[${new Date().toLocaleTimeString()}] 注销请求完成。服务器响应: ${JSON.stringify(resData)}`)
    scrollToBottom()

    if (resData.code === 0 || resData.msg === 'success') {
      ElMessage.success('账号注销指令已下发成功')
    } else {
      ElMessage.error('注销失败: ' + (resData.msg || '未知错误'))
    }
  } catch (error: any) {
    ElMessage.error('注销请求异常: ' + error.message)
    logs.value.push(`[ERROR] 注销请求失败: ${error.message}`)
    scrollToBottom()
  }
}

// 根据卡片名称决定执行的 K6 脚本名
const scriptName = isDramaCheck ? 'drama_check_flow.js' : 'episode.js'

// 执行状态
type ExecStatus = 'Ready' | 'Executing' | 'Stopped' | 'Finished'
const currentStatus = ref<ExecStatus>('Ready')

// 运行时间控制
const uptime = ref(0)
const duration = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

// 日志及报告
const logs = ref<string[]>(['准备就绪，点击 Execute 开始执行'])
const logContainer = ref<HTMLElement | null>(null)
let ws: WebSocket | null = null
const reportUrl = ref<string | null>(null)

const formatTime = (seconds: number) => {
  const h = Math.floor(seconds / 3600).toString().padStart(2, '0')
  const m = Math.floor((seconds % 3600) / 60).toString().padStart(2, '0')
  const s = (seconds % 60).toString().padStart(2, '0')
  return `${h}:${m}:${s}`
}

const scrollToBottom = () => {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}

const startExecution = async () => {
  if (currentStatus.value === 'Executing') return

  // 特殊处理：删除账号工具的匿名登录逻辑
  if (isDeleteAccount) {
    if (!deleteAccountParam.value) {
      ElMessage.warning('请先填入参数')
      return
    }

    const params = parseParams(deleteAccountParam.value)
    const baseDomain = domainMappings[projectName.value as keyof typeof domainMappings][testServer.value as 'prod' | 'test']
    const loginUrl = `${baseDomain}/login/anonymous`

    currentStatus.value = 'Executing'
    logs.value = [
      `[${new Date().toLocaleTimeString()}] 准备发起匿名登录请求...`,
      `[DEBUG] 目标 URL: ${loginUrl}`,
      `[DEBUG] 解析后的请求头 (Headers): ${JSON.stringify(params, null, 2)}`
    ]
    scrollToBottom()
    
    try {
      const response = await fetch(`${getBackendHost()}/api/proxy`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          url: loginUrl,
          data: params
        })
      })

      const resData = await response.json()
      currentStatus.value = 'Finished'
      logs.value.push(`[${new Date().toLocaleTimeString()}] 登录请求成功，正在解析用户信息...`)
      scrollToBottom()
      
      const userData = resData.data || {}
      const userId = userData.user_id || '未知'
      const userName = userData.user_name || '未知'
      const sessionToken = userData.session_token

      if (!sessionToken) {
        ElMessage.error('登录响应中未找到有效 Token')
        logs.value.push(`[ERROR] 登陆失败: ${JSON.stringify(resData)}`)
        return
      }

      // 弹出美化后的确认窗口
      ElMessageBox.confirm(
        `<div class="confirm-content-wrapper">
          <p class="confirm-tip">已成功获取临时登录凭证，请核对并确认是否执行注销操作：</p>
          <div class="user-card">
            <div class="user-info-item">
              <span class="info-label">用户 ID</span>
              <span class="info-value-id">${userId}</span>
            </div>
            <div class="user-info-item">
              <span class="info-label">用户名</span>
              <span class="info-value-name">${userName}</span>
            </div>
          </div>
          <div class="warning-footer">
            <span style="font-size: 14px;">⚠️</span>
            <span>注意：此操作将永久抹除该账号所有数据，不可撤销。</span>
          </div>
        </div>`,
        '账号注销确认',
        {
          confirmButtonText: '确认',
          cancelButtonText: '取消',
          confirmButtonClass: 'el-button--danger is-plain custom-confirm-btn',
          cancelButtonClass: 'custom-cancel-btn',
          dangerouslyUseHTMLString: true,
          center: false,
          icon: Warning,
          customClass: 'delete-account-confirm-box'
        }
      ).then(() => {
        handleAccountDelete(sessionToken, params)
      }).catch(() => {
        logs.value.push(`[${new Date().toLocaleTimeString()}] 用户取消了注销操作。`)
        scrollToBottom()
      })
    } catch (error: any) {
      currentStatus.value = 'Ready'
      ElMessage.error('登录请求失败: ' + error.message)
      logs.value.push(`[ERROR] ${error.message}`)
      scrollToBottom()
    }
    return
  }

  reportUrl.value = null // 清除上一次的报告
  logs.value = []
  logs.value.push(`[${new Date().toLocaleTimeString()}] 准备连接调度引擎...`)

  currentStatus.value = 'Executing'
  
  // 获取当前配置
  const profile = serverProfiles[testServer.value as keyof typeof serverProfiles]
  
  // 组装 WebSocket 链接地址，注入动态参数
  const query = new URLSearchParams({
    script: scriptName,
    email: profile.email,
    password: profile.password,
    loginUrl: profile.loginUrl,
    dramaListUrl: profile.dramaListUrl
  }).toString()

  ws = new WebSocket(`${getWsBase()}/api/ws/k6?${query}`)

  ws.onopen = () => {
    logs.value.push(`[${new Date().toLocaleTimeString()}] WebSocket 连接已建立`)
    // 启动计时器
    uptime.value = 0
    duration.value = 0
    timer = setInterval(() => {
      uptime.value++
      duration.value++
    }, 1000)
  }

  ws.onmessage = (event) => {
    logs.value.push(event.data)
    scrollToBottom()
  }

  ws.onerror = () => {
    logs.value.push(`[WARN/ERR] WebSocket 连接错误`)
  }

  ws.onclose = () => {
    if (timer) clearInterval(timer)
    if (currentStatus.value === 'Executing') {
      currentStatus.value = 'Finished'
      
      // 指向特定的剧集检测报告或通用报告
      const reportFile = isDramaCheck ? 'drama_check_report.html' : 'summary.html'
      const finalReportUrl = `${getBackendHost()}/reports/${reportFile}?t=${Date.now()}`
      
      logs.value.push(`[${new Date().toLocaleTimeString()}] 任务执行完成。`)
      reportUrl.value = finalReportUrl

      // 追加到存储仓库
      reportStore.addReport({
        name: toolName.value,
        type: isDramaCheck ? '业务自动化' : 'K6 压测',
        status: 'Passed',
        duration: formatTime(duration.value),
        author: 'Current User',
        reportUrl: finalReportUrl
      })
    }
  }
}

const stopExecution = () => {
  if (currentStatus.value !== 'Executing') return

  if (ws) {
    ws.close()
    ws = null
  }
  if (timer) clearInterval(timer)
  
  if (currentStatus.value === 'Executing') {
    reportStore.addReport({
      name: toolName.value,
      type: isDramaCheck ? '业务自动化' : 'K6 压测',
      status: 'Failed',
      duration: formatTime(duration.value),
      author: 'Current User',
    })
  }

  currentStatus.value = 'Stopped'
  logs.value.push(`[${new Date().toLocaleTimeString()}] 手动终止执行。`)
}

const clearLogs = () => {
  logs.value = []
}

const closePage = () => {
  if (currentStatus.value === 'Executing') {
    stopExecution()
  }
  router.push('/')
}

onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (ws) ws.close()
})
</script>

<template>
  <div class="com-api-container">
    <div class="main-content">
      
      <!-- 左侧信息区 -->
      <div class="sidebar-panel">
        <div class="status-badge-row">
          <span class="badge-ready">READY</span>
          <el-icon class="info-icon" color="#9ca3af"><Warning /></el-icon>
        </div>
        
        <div class="tool-title-section">
          <h1 class="tool-title">{{ toolName }}</h1>
          <p class="tool-desc">{{ toolDesc }}</p>
        </div>

        <div class="params-section">
          <!-- 针对删除账号工具，新增填入参数输入框 -->
          <div v-if="isDeleteAccount" class="param-group">
            <label class="param-label">填入参数</label>
            <el-input v-model="deleteAccountParam" placeholder="请输入参数" />
          </div>

          <!-- 针对业务自检工具，隐藏原本的链接输入框 -->
          <div v-if="!isDramaCheck" class="param-group">
            <label class="param-label">
              <span class="link-icon">🔗</span> 测试链接
            </label>
            <el-input v-model="projectName" disabled class="param-input-disabled" />
          </div>

          <div class="param-row">
            <div class="param-group half">
              <label class="param-label">测试服务器</label>
              <!-- 改为下拉框切换 -->
              <el-select v-model="testServer" placeholder="选择服务器" class="param-select">
                <el-option
                  v-for="item in serverOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </div>
            <div class="param-group half">
              <label class="param-label">项目</label>
              <el-input v-model="projectName" readonly class="param-input-readonly" />
            </div>
          </div>
        </div>
      </div>

      <!-- 右侧日志区 / 报告区 -->
      <div class="log-panel" style="position: relative;">
        <!-- 日志顶栏 -->
        <div class="log-header">
          <div class="log-status-info">
            <span class="dot" :class="{ 'dot-active': currentStatus === 'Executing', 'dot-ready': currentStatus !== 'Executing' }"></span>
            <span class="status-text">Status: {{ currentStatus }}</span>
            <span class="divider">|</span>
            <span class="uptime-text">UPTIME: <span class="time-val">{{ formatTime(uptime) }}</span></span>
          </div>
          <div class="log-actions">
            <el-icon class="action-btn" title="Copy"><CopyDocument /></el-icon>
            <el-icon class="action-btn" title="Download"><Download /></el-icon>
            <el-icon class="action-btn" title="Clear Logs" @click="clearLogs"><Delete /></el-icon>
            <span class="divider"></span>
            <el-icon class="action-btn" title="Close" @click="closePage"><Close /></el-icon>
          </div>
        </div>

        <!-- 两种视图状态：日志 / HTML报告 -->
        <div v-if="reportUrl" class="report-container">
          <iframe :src="reportUrl" class="report-iframe" frameborder="0"></iframe>
        </div>
        <div v-else class="log-content" ref="logContainer">
          <div v-for="(log, idx) in logs" :key="idx" class="log-line">
            {{ log }}
          </div>
        </div>
      </div>

    </div>

    <!-- 底部吸底操作栏 -->
    <div class="bottom-bar">
      <div class="bottom-left">
        <div class="stat-item">
          <span class="stat-label">DURATION</span>
          <span class="stat-value">{{ formatTime(duration) }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">STATUS</span>
          <span class="stat-value capitalize">{{ currentStatus.toLowerCase() === 'ready' ? 'Idle' : currentStatus }}</span>
        </div>
      </div>
      <div class="bottom-right">
        <button 
          class="btn-stop" 
          :disabled="currentStatus !== 'Executing'"
          @click="stopExecution"
        >
          <el-icon><VideoPause /></el-icon>
          Stop
        </button>
        <button 
          class="btn-execute" 
          :disabled="currentStatus === 'Executing'"
          @click="startExecution"
        >
          <el-icon><VideoPlay /></el-icon>
          Execute
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* ==================== 页面容器 ==================== */
.com-api-container {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 60px); /* 减去顶部 header 高度 */
  background: #f5f6fa;
  margin: -24px; /* 抵消 layout-content 的 padding，实现全屏和底部吸底 */
  position: relative;
}

/* ==================== 主体区域 ==================== */
.main-content {
  display: flex;
  flex: 1;
  overflow: hidden;
  padding: 16px;
  gap: 16px;
  /* 为底部操作栏留出空间 */
  padding-bottom: 80px; 
}

/* ==================== 左侧信息面 ==================== */
.sidebar-panel {
  width: 320px;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #f0f0f0;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  flex-shrink: 0;
  overflow-y: auto;
}

.status-badge-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.badge-ready {
  background: rgba(16, 185, 129, 0.1);
  color: #10b981;
  font-size: 11px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 12px;
  letter-spacing: 0.5px;
}

.info-icon {
  cursor: pointer;
}

.tool-title-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f0f0f0;
}

.tool-title {
  font-size: 20px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
}

.tool-desc {
  font-size: 13px;
  color: #64748b;
  margin: 0;
  line-height: 1.5;
}

.params-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.param-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.param-row {
  display: flex;
  gap: 12px;
}

.half {
  flex: 1;
}

.param-label {
  font-size: 12.5px;
  font-weight: 600;
  color: #1e293b;
  display: flex;
  align-items: center;
  gap: 4px;
}

.link-icon {
  font-size: 14px;
  color: #3b82f6;
}

/* 覆盖 el-input 与 el-select 样式，以匹配白底灰框设计 */
:deep(.el-input__wrapper),
:deep(.el-select__wrapper) {
  background-color: #f8fafc;
  box-shadow: 0 0 0 1px #e2e8f0 inset;
  border-radius: 6px;
}

:deep(.el-input.is-disabled .el-input__wrapper) {
  background-color: #f1f5f9;
}

.param-select {
  width: 100%;
}

/* ==================== 右侧日志面 ==================== */
.log-panel {
  flex: 1;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #f0f0f0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.log-header {
  height: 50px;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: #ffffff;
}

.log-status-info {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: #1e293b;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.dot-ready { background: #10b981; }
.dot-active { background: #3b82f6; animation: blink 1.5s infinite; }

@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.divider {
  color: #cbd5e1;
  margin: 0 4px;
  font-weight: 300;
}

.uptime-text {
  color: #94a3b8;
  font-weight: 500;
}

.time-val {
  color: #1e293b;
  font-weight: 700;
  margin-left: 2px;
}

.log-actions {
  display: flex;
  gap: 16px;
}

.action-btn {
  color: #94a3b8;
  font-size: 16px;
  cursor: pointer;
  transition: color 0.2s;
}

.action-btn:hover {
  color: #1e293b;
}

.log-content {
  flex: 1;
  background: #fafafa;
  padding: 16px 20px;
  overflow-y: auto;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  color: #475569;
  line-height: 1.6;
}

.log-line {
  margin-bottom: 4px;
}

/* ==================== 报告展现区 ==================== */
.report-container {
  flex: 1;
  width: 100%;
  height: 100%;
  background: #fff;
  overflow: hidden;
}

.report-iframe {
  width: 100%;
  height: 100%;
  border: none;
}

/* ==================== 底部吸底操作栏 ==================== */
.bottom-bar {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 64px;
  background: #ffffff;
  border-top: 1px solid #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  box-shadow: 0 -4px 12px rgba(0, 0, 0, 0.02);
}

.bottom-left {
  display: flex;
  gap: 32px;
}

.stat-item {
  display: flex;
  flex-direction: column;
}

.stat-label {
  font-size: 11px;
  font-weight: 700;
  color: #94a3b8;
  letter-spacing: 0.5px;
}

.stat-value {
  font-size: 15px;
  font-weight: 700;
  color: #1e293b;
}

.capitalize {
  text-transform: capitalize;
}

.bottom-right {
  display: flex;
  gap: 12px;
}

button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 40px;
  padding: 0 20px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  border: none;
}

button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-stop {
  background: #ffffff;
  color: #64748b;
  border: 1px solid #e2e8f0;
}

.btn-stop:not(:disabled):hover {
  background: #f8fafc;
  color: #ef4444;
  border-color: #fca5a5;
}

.btn-execute {
  background: #3b82f6;
  color: #ffffff;
  box-shadow: 0 2px 6px rgba(59, 130, 246, 0.3);
}

.btn-execute:not(:disabled):hover {
  background: #2563eb;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.4);
}
</style>

<style>
/* ==================== 账号删除弹窗全局样式 (非 Scoped) ==================== */
.delete-account-confirm-box {
  width: 420px !important;
  border-radius: 16px !important;
  padding: 12px 12px 20px !important;
  border: 1px solid rgba(255, 77, 79, 0.2) !important;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.1) !important;
}

.delete-account-confirm-box .el-message-box__header {
  padding-bottom: 12px;
}

.delete-account-confirm-box .el-message-box__title {
  font-size: 18px;
  font-weight: 800;
  color: #1e293b;
}

.delete-account-confirm-box .el-message-box__status.el-icon {
  font-size: 24px;
}

.confirm-content-wrapper {
  color: #475569;
}

.confirm-tip {
  font-size: 13.5px;
  line-height: 1.6;
  margin-bottom: 16px;
  color: #64748b;
}

.user-card {
  background: linear-gradient(135deg, #fffafa 0%, #fff 100%);
  border: 1px solid #fee2e2;
  padding: 16px;
  border-radius: 12px;
  margin-bottom: 20px;
  box-shadow: 0 4px 10px rgba(239, 68, 68, 0.03);
}

.user-info-item {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.user-info-item:last-child {
  margin-bottom: 0;
}

.info-label {
  font-size: 11px;
  font-weight: 700;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  min-width: 60px;
}

.info-value-id {
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  color: #dc2626;
  background: #fef2f2;
  padding: 4px 10px;
  border-radius: 6px;
  font-weight: 700;
  font-size: 14px;
}

.info-value-name {
  color: #0f172a;
  font-weight: 700;
  font-size: 15px;
}

.warning-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  background: #f8fafc;
  border-radius: 8px;
  color: #64748b;
  font-size: 12px;
}

.custom-confirm-btn {
  border-radius: 8px !important;
  font-weight: 700 !important;
  padding: 8px 20px !important;
}

.custom-cancel-btn {
  border-radius: 8px !important;
  font-weight: 600 !important;
  color: #64748b !important;
}
</style>
