<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useDramaRunStore, usePermissionStore, useReportStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import { buildBackendUrl, buildBackendWsUrl, normalizeBackendUrl } from '@/utils/runtimeUrl'
import MonkeyHologram3D from './components/MonkeyHologram3D.vue'
import MonkeyNodeSummary from './components/MonkeyNodeSummary.vue'
import { useAuthenticatedImage } from './composables/useAuthenticatedImage'
import { useMonkeyRunStream } from './composables/useMonkeyRunStream'
import {
  Warning,
  CopyDocument,
  Download,
  Close,
  VideoPlay,
  VideoPause,
  Delete,
  Tickets,
  Document as DocumentIcon,
  Odometer,
  Cpu,
  Check,
  Edit,
  ArrowRight
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const reportStore = useReportStore()
const authStore = useAuthStore()
const dramaRunStore = useDramaRunStore()
const permissionStore = usePermissionStore()

// 从路由参数获取工具信息
const toolTypeName = (route.query.name as string) || '测试剧集是否重复'
const toolName = ref(toolTypeName)
const toolDesc = ref((route.query.desc as string) || '检测剧集数据中是否存在重复的drama_intid')
const projectName = ref('ShortsWave')

// 对于 Web前端压测，重置 projectName 为空，方便输入 URL
if (toolTypeName === 'Web前端压测') {
  projectName.value = ''
}

// 是否是剧集播放自检工具
const isDramaCheck = toolTypeName.includes('播放')
// 是否是Web前端压测
const isWebFrontendStressTest = toolTypeName === 'WebFrontend性能' || toolTypeName === 'Web前端压测'
// 是否是 Monkey 稳定性测试 demo
const isMonkeyTest = toolTypeName === 'Monkey测试'
// 是否是删除账号工具
const isDeleteAccount = toolTypeName === '删除账号'
const deleteAccountParam = ref('')

// ===== 短剧类接口测试专属状态 =====
const isShortDramaApiTest = computed(() => toolTypeName === '短剧类接口测试')
const shortDramaActiveTab = ref('pipeline')
// ===== 通用接口测试：类型定义与状态管理 =====
// ApiInterface 描述一个可测试的 HTTP 接口条目
interface ApiInterface {
  id: number          // 唯一标识
  name: string        // 接口名称（用户自定义）
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'  // HTTP 方法
  url: string         // 请求接口路径
  headers: string     // JSON 格式的请求头
  body: string        // JSON 格式的请求体
  status: 'pending' | 'running' | 'success' | 'failed'  // 执行状态
  code?: number       // HTTP 响应状态码
  latency?: number    // 响应耗时（毫秒）
  errorMsg?: string   // 错误信息
  requestData?: any   // 发送的请求数据（用于 Payload 展示）
  responseData?: any  // 接收的响应数据（用于 Payload 展示）
}

interface ApiTestSuite {
  id: number
  name: string
  interfaces: ApiInterface[]
  isExpanded: boolean
  isNameEditing: boolean
}

type ApiEnvironment = 'test' | 'prod' | 'gray'
type ApiJsonField = 'headers' | 'body'

const SHORT_DRAMA_API_CONFIG_STORAGE_KEY = 'testcenter.shortDramaApi.interfaces.v1'

// 自增 ID 计数器，确保每个接口条目有唯一标识
let apiInterfaceIdCounter = 1
let apiTestSuiteIdCounter = 1

const shortDramaAnonymousLoginHeaders = {
  contype: '2',
  afid: '1732255304101-8624506',
  'advertising-id': '1337CC3C-3A4F-455B-A138-87B234B2D9F4',
  'User-Agent': 'ShortsWave/2.48.0.2507 (iOS; iPhone16,1; Version 26.2 (Build 23C55)) CFNetwork/1.0 Darwin/25.2.0',
  attribution_type: 'af',
  os: '2',
  os_ver: '26.0',
  build: '10',
  lang: 'us',
  'mobile-brand': 'apple',
  model: 'iPhone16,1',
  idfv: 'BF14B9E4-D63E-4882-A5A5-57B710A63D64',
  attribution_id: '',
  'device-uuid': '127ACC18-6EB4-4101-928E-43FD1D05617D+3996E782AA3D2A1D5477EA8ABA7539F5',
  carrier: '',
  app: 'com.company.shortsdrama.wave',
  'Content-Type': 'application/json',
  width: '390',
  timezone: 'America/Los_Angeles',
  height: '844',
  region: 'us',
  'X-SESSION-TOKEN': ''
}

// 创建一个空白接口条目的工厂函数
const createEmptyInterface = (): ApiInterface => ({
  id: apiInterfaceIdCounter++,
  name: `接口 ${apiInterfaceIdCounter - 1}`,
  method: 'GET',
  url: '',
  headers: '',
  body: '',
  status: 'pending'
})

// 创建短剧匿名登录接口，用于获取 session_token
const createShortDramaAnonymousLoginInterface = (): ApiInterface => ({
  id: apiInterfaceIdCounter++,
  name: '匿名登录获取 session_token',
  method: 'POST',
  url: '/login/anonymous',
  headers: JSON.stringify(shortDramaAnonymousLoginHeaders, null, 2),
  body: '{}',
  status: 'pending'
})

const createApiTestSuite = (name: string, interfaces: ApiInterface[], isNameEditing = false, isExpanded = true): ApiTestSuite => ({
  id: apiTestSuiteIdCounter++,
  name,
  interfaces,
  isExpanded,
  isNameEditing
})

const normalizeInterfacePath = (value: string) => {
  const rawValue = value.trim()
  if (!rawValue) return ''

  try {
    const parsed = new URL(rawValue)
    return `${parsed.pathname}${parsed.search}${parsed.hash}` || '/'
  } catch {
    const withoutDomain = rawValue.replace(/^https?:\/\/[^/]+/i, '')
    return withoutDomain.startsWith('/') ? withoutDomain : `/${withoutDomain}`
  }
}

const resetApiRuntimeState = (item: ApiInterface): ApiInterface => ({
  ...item,
  url: normalizeInterfacePath(item.url),
  status: 'pending',
  code: undefined,
  latency: undefined,
  errorMsg: undefined,
  requestData: undefined,
  responseData: undefined
})

const loadSavedShortDramaEnvironment = (): ApiEnvironment | null => {
  if (typeof window === 'undefined') return null
  try {
    const rawData = window.localStorage.getItem(SHORT_DRAMA_API_CONFIG_STORAGE_KEY)
    if (!rawData) return null
    const parsed = JSON.parse(rawData)
    return ['test', 'prod', 'gray'].includes(parsed?.environment) ? parsed.environment : null
  } catch {
    return null
  }
}

const loadSavedShortDramaInterfaces = (): ApiInterface[] => {
  if (typeof window === 'undefined') return []
  try {
    const rawData = window.localStorage.getItem(SHORT_DRAMA_API_CONFIG_STORAGE_KEY)
    if (!rawData) return []
    const parsed = JSON.parse(rawData)
    if (!Array.isArray(parsed?.interfaces)) return []

    const savedItems = parsed.interfaces
      .filter((item: Partial<ApiInterface>) => item?.name && item?.method)
      .map((item: Partial<ApiInterface>, index: number) => resetApiRuntimeState({
        id: typeof item.id === 'number' ? item.id : apiInterfaceIdCounter + index,
        name: item.name || `接口 ${index + 1}`,
        method: item.method || 'GET',
        url: item.url || '',
        headers: item.headers || '',
        body: item.body || '',
        status: 'pending'
      }))

    const maxId = savedItems.reduce((max: number, item: ApiInterface) => Math.max(max, item.id), 0)
    apiInterfaceIdCounter = Math.max(apiInterfaceIdCounter, maxId + 1)
    return savedItems
  } catch {
    return []
  }
}

const loadSavedShortDramaTestName = () => {
  if (typeof window === 'undefined') return ''
  try {
    const rawData = window.localStorage.getItem(SHORT_DRAMA_API_CONFIG_STORAGE_KEY)
    if (!rawData) return ''
    const parsed = JSON.parse(rawData)
    return typeof parsed?.testName === 'string' ? parsed.testName.trim() : ''
  } catch {
    return ''
  }
}

const loadSavedShortDramaTestSuites = (): ApiTestSuite[] => {
  if (typeof window === 'undefined') return []
  try {
    const rawData = window.localStorage.getItem(SHORT_DRAMA_API_CONFIG_STORAGE_KEY)
    if (!rawData) return []
    const parsed = JSON.parse(rawData)
    if (!Array.isArray(parsed?.testSuites)) return []

    const savedSuites: ApiTestSuite[] = parsed.testSuites
      .filter((suite: Partial<ApiTestSuite>) => typeof suite?.name === 'string')
      .map((suite: Partial<ApiTestSuite>, suiteIndex: number) => {
        const interfaces = Array.isArray(suite.interfaces)
          ? suite.interfaces
            .filter((item: Partial<ApiInterface>) => item?.name && item?.method)
            .map((item: Partial<ApiInterface>, index: number) => resetApiRuntimeState({
              id: typeof item.id === 'number' ? item.id : apiInterfaceIdCounter + index,
              name: item.name || `接口 ${index + 1}`,
              method: item.method || 'GET',
              url: item.url || '',
              headers: item.headers || '',
              body: item.body || '',
              status: 'pending'
            }))
          : []

        return createApiTestSuite(
          suite.name?.trim() || `接口测试 ${suiteIndex + 1}`,
          interfaces,
          false,
          typeof suite.isExpanded === 'boolean' ? suite.isExpanded : true
        )
      })

    const maxInterfaceId = savedSuites
      .flatMap(suite => suite.interfaces)
      .reduce((max: number, item: ApiInterface) => Math.max(max, item.id), 0)
    apiInterfaceIdCounter = Math.max(apiInterfaceIdCounter, maxInterfaceId + 1)
    return savedSuites
  } catch {
    return []
  }
}

// 接口列表（替代原先固定的 shortDramaPipeline）
const savedShortDramaTestSuites = loadSavedShortDramaTestSuites()
const savedShortDramaInterfaces = savedShortDramaTestSuites.length > 0 ? [] : loadSavedShortDramaInterfaces()
const savedShortDramaTestName = loadSavedShortDramaTestName()
if (savedShortDramaTestName) {
  toolName.value = savedShortDramaTestName
}
const apiInterfaces = ref<ApiInterface[]>(savedShortDramaInterfaces.length > 0 ? savedShortDramaInterfaces : [createShortDramaAnonymousLoginInterface()])
const apiTestSuites = ref<ApiTestSuite[]>(
  savedShortDramaTestSuites.length > 0
    ? savedShortDramaTestSuites
    : [createApiTestSuite(toolName.value, apiInterfaces.value, !savedShortDramaTestName)]
)
// 当前展开编辑的接口 ID
const expandedInterfaceId = ref<number | null>(apiTestSuites.value[0]?.interfaces[0]?.id ?? null)
const editingJsonBlock = ref<string | null>(null)
const editingInterfaceNameId = ref<number | null>(null)
const selectedApiTestSuiteId = ref<number | null>(apiTestSuites.value[0]?.id ?? null)
const selectedApiInterfaceId = ref<number | null>(apiTestSuites.value[0]?.interfaces[0]?.id ?? null)
let lastSavedShortDramaConfigSnapshot = ''
// 新增一个空白接口条目到列表末尾
const addApiInterface = (suite: ApiTestSuite = apiTestSuites.value[0]!) => {
  const newItem = createEmptyInterface()
  suite.interfaces.push(newItem)
  selectedApiTestSuiteId.value = suite.id
  selectedApiInterfaceId.value = newItem.id
  expandedInterfaceId.value = newItem.id
  editingInterfaceNameId.value = newItem.id
}

const addApiTestSuite = () => {
  const defaultInterface = createShortDramaAnonymousLoginInterface()
  const newSuite = createApiTestSuite(`接口测试 ${apiTestSuiteIdCounter}`, [defaultInterface], true, true)
  apiTestSuites.value.push(newSuite)
  selectedApiTestSuiteId.value = newSuite.id
  selectedApiInterfaceId.value = defaultInterface.id
  expandedInterfaceId.value = defaultInterface.id
}

// 删除指定接口条目（至少保留一个）
const removeApiInterface = (id: number) => {
  const targetSuite = apiTestSuites.value.find(suite => suite.interfaces.some(item => item.id === id))
  if (!targetSuite) return
  if (targetSuite.interfaces.length <= 1) {
    ElMessage.warning('至少保留一个接口')
    return
  }
  const targetIndex = targetSuite.interfaces.findIndex(item => item.id === id)
  if (targetIndex >= 0) {
    targetSuite.interfaces.splice(targetIndex, 1)
  }
  if (selectedApiInterfaceId.value === id) {
    selectedApiInterfaceId.value = targetSuite.interfaces[0]?.id ?? null
  }
  if (expandedInterfaceId.value === id) {
    expandedInterfaceId.value = targetSuite.interfaces[0]?.id ?? null
  }
}

const buildShortDramaApiConfigSnapshot = () => JSON.stringify({
  environment: testServer.value,
  project: projectName.value,
  testName: apiTestSuites.value[0]?.name.trim() || toolName.value.trim() || toolTypeName,
  interfaces: apiTestSuites.value[0]?.interfaces.map(({ id, name, method, url, headers, body }) => ({
    id,
    name,
    method,
    url: normalizeInterfacePath(url),
    headers,
    body
  })) || [],
  testSuites: apiTestSuites.value.map(suite => ({
    id: suite.id,
    name: suite.name.trim() || `接口测试 ${suite.id}`,
    isExpanded: suite.isExpanded,
    interfaces: suite.interfaces.map(({ id, name, method, url, headers, body }) => ({
      id,
      name,
      method,
      url: normalizeInterfacePath(url),
      headers,
      body
    }))
  }))
})

const persistShortDramaApiConfig = () => {
  if (!isShortDramaApiTest.value) return
  if (typeof window !== 'undefined') {
    const snapshot = buildShortDramaApiConfigSnapshot()
    if (snapshot === lastSavedShortDramaConfigSnapshot) return
    window.localStorage.setItem(SHORT_DRAMA_API_CONFIG_STORAGE_KEY, snapshot)
    lastSavedShortDramaConfigSnapshot = snapshot
  }
}

const startApiTestSuiteNameEditing = (suite: ApiTestSuite) => {
  suite.isNameEditing = true
}

const finishApiTestSuiteNameEditing = (suite: ApiTestSuite) => {
  suite.name = suite.name.trim() || `接口测试 ${suite.id}`
  suite.isNameEditing = false
  if (suite.id === apiTestSuites.value[0]?.id) {
    toolName.value = suite.name
  }
  persistShortDramaApiConfig()
}

const toggleApiTestSuite = (suite: ApiTestSuite) => {
  suite.isExpanded = !suite.isExpanded
  if (suite.isExpanded) {
    expandedInterfaceId.value = suite.interfaces[0]?.id ?? null
  }
  persistShortDramaApiConfig()
}

const selectApiTestSuite = (suite: ApiTestSuite) => {
  selectedApiTestSuiteId.value = suite.id
  if (!suite.interfaces.some(item => item.id === selectedApiInterfaceId.value)) {
    selectedApiInterfaceId.value = suite.interfaces[0]?.id ?? null
  }
  const selectedIndex = suite.interfaces.findIndex(item => item.id === selectedApiInterfaceId.value)
  selectedPipelineStepIndex.value = selectedIndex >= 0 ? selectedIndex : null
}

const selectApiInterface = (suite: ApiTestSuite, item: ApiInterface) => {
  selectedApiTestSuiteId.value = suite.id
  selectedApiInterfaceId.value = item.id
  selectedPipelineStepIndex.value = suite.interfaces.findIndex(step => step.id === item.id)
}

const startInterfaceNameEditing = (id: number) => {
  editingInterfaceNameId.value = id
}

const finishInterfaceNameEditing = (item: ApiInterface) => {
  item.name = item.name.trim() || `接口 ${item.id}`
  editingInterfaceNameId.value = null
  persistShortDramaApiConfig()
}

// 切换接口卡片的展开/折叠状态
const toggleInterfaceExpand = (id: number) => {
  expandedInterfaceId.value = expandedInterfaceId.value === id ? null : id
}

const selectedPipelineStepIndex = ref<number | null>(null)

const selectedApiTestSuite = computed(() => {
  return apiTestSuites.value.find(suite => suite.id === selectedApiTestSuiteId.value) || apiTestSuites.value[0] || null
})

// 右侧运行状态详情只展示当前选中的接口测试
const activePipelineSteps = computed<ApiInterface[]>(() => selectedApiTestSuite.value?.interfaces || [])

const selectedStepForPayload = computed(() => {
  return activePipelineSteps.value.find(step => step.id === selectedApiInterfaceId.value) || activePipelineSteps.value[0] || null
})

const selectPipelineStep = (index: number) => {
  selectedPipelineStepIndex.value = index
  selectedApiInterfaceId.value = activePipelineSteps.value[index]?.id ?? null
  shortDramaActiveTab.value = 'payload'
}

const formatJSON = (val: any) => {
  if (!val) return '暂无数据'
  return JSON.stringify(val, null, 2)
}

const getJsonBlockKey = (id: number, field: ApiJsonField) => `${id}:${field}`

const isJsonBlockEditing = (id: number, field: ApiJsonField) => editingJsonBlock.value === getJsonBlockKey(id, field)

const startJsonBlockEditing = (id: number, field: ApiJsonField) => {
  editingJsonBlock.value = getJsonBlockKey(id, field)
}

const stopJsonBlockEditing = () => {
  editingJsonBlock.value = null
  persistShortDramaApiConfig()
}

const getJsonPreviewText = (value: string, emptyLabel: string) => {
  const trimmedValue = value.trim()
  return trimmedValue || emptyLabel
}

const getJsonPreviewLines = (value: string, emptyLabel: string) => {
  return getJsonPreviewText(value, emptyLabel).split('\n')
}

const getJsonEditorRows = (value: string, minRows: number) => {
  return Math.max(minRows, getJsonPreviewText(value, '').split('\n').length)
}

const getInterfaceDisplayUrl = (item?: Pick<ApiInterface, 'url'> | null) => {
  const path = normalizeInterfacePath(item?.url || '')
  return path || '未填写接口'
}

const getCurrentApiBaseDomain = () => {
  const projectDomains = domainMappings[projectName.value as keyof typeof domainMappings]
  return projectDomains?.[testServer.value] || domainMappings.ShortsWave[testServer.value]
}

const getFullInterfaceUrl = (item?: Pick<ApiInterface, 'url'> | null) => {
  const path = normalizeInterfacePath(item?.url || '')
  if (!path) return ''
  return `${getCurrentApiBaseDomain().replace(/\/$/, '')}${path}`
}

const getInterfaceStatusText = (status: ApiInterface['status']) => {
  const statusMap: Record<ApiInterface['status'], string> = {
    pending: '待执行',
    running: '执行中',
    success: '通过',
    failed: '失败'
  }
  return statusMap[status]
}

const getLatencyClass = (latency?: number) => {
  if (!latency) return 'latency-ok'
  if (latency < 200) return 'latency-ok'
  if (latency < 500) return 'latency-warn'
  return 'latency-error'
}

const runShortDramaApiTest = async () => {
  currentStatus.value = 'Executing'
  logs.value = [`[${new Date().toLocaleTimeString()}] 开始接口链路测试...`]
  uptime.value = 0
  duration.value = 0
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    uptime.value++
    duration.value++
  }, 1000)

  // 重置所有接口的状态
  activePipelineSteps.value.forEach(s => {
    s.status = 'pending'
    s.code = undefined
    s.latency = undefined
    s.errorMsg = undefined
    s.requestData = undefined
    s.responseData = undefined
  })

  logs.value.push(`[INFO] 真实请求模式，接口数: ${activePipelineSteps.value.length}`)
  scrollToBottom()

  let hasFailed = false

  for (let index = 0; index < activePipelineSteps.value.length; index++) {
    const step = activePipelineSteps.value[index]
    if (!step) continue
    step.url = normalizeInterfacePath(step.url)
    const fullInterfaceUrl = getFullInterfaceUrl(step)
    step.status = 'running'
    shortDramaActiveTab.value = 'pipeline'
    selectedPipelineStepIndex.value = index
    selectedApiInterfaceId.value = step.id
    logs.value.push(`[${new Date().toLocaleTimeString()}] 正在执行: ${step.name} (${step.method} ${step.url || '未填写接口'})...`)
    scrollToBottom()

    const startTime = Date.now()

    if (!fullInterfaceUrl) {
      step.status = 'failed'
      step.code = 400
      step.errorMsg = '请求接口不能为空'
      logs.value.push(`[ERROR] 接口 [${step.name}] 执行失败: 请求接口不能为空`)
      hasFailed = true
      scrollToBottom()
      continue
    }

    let parsedHeaders = {}
    try {
      if (step.headers) parsedHeaders = JSON.parse(step.headers)
    } catch (e: any) {
      step.status = 'failed'
      step.errorMsg = `请求头 JSON 格式错误: ${e.message}`
      logs.value.push(`[ERROR] 接口 [${step.name}] 执行失败: 请求头 JSON 格式错误`)
      hasFailed = true
      scrollToBottom()
      continue
    }

    let parsedBody = {}
    try {
      if (step.body && step.method !== 'GET') parsedBody = JSON.parse(step.body)
    } catch (e: any) {
      step.status = 'failed'
      step.errorMsg = `请求体 JSON 格式错误: ${e.message}`
      logs.value.push(`[ERROR] 接口 [${step.name}] 执行失败: 请求体 JSON 格式错误`)
      hasFailed = true
      scrollToBottom()
      continue
    }

    // 合并 headers 和 body 作为 proxy data
    const mergedData = { ...parsedHeaders, ...parsedBody }

    step.requestData = {
      url: fullInterfaceUrl,
      method: step.method,
      headers: parsedHeaders,
      body: parsedBody
    }

    try {
      const response = await fetch(buildBackendUrl('/api/proxy'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          method: step.method,
          url: fullInterfaceUrl,
          data: mergedData
        })
      })

      const endTime = Date.now()
      step.latency = endTime - startTime
      step.code = response.status

      const contentType = response.headers.get('content-type') || ''
      let resData: any
      if (contentType.includes('application/json')) {
        resData = await response.json()
      } else {
        resData = await response.text()
        try {
          resData = JSON.parse(resData)
        } catch (e) {
          resData = { rawResponse: resData }
        }
      }
      step.responseData = resData

      if (response.ok) {
        step.status = 'success'
        logs.value.push(`[SUCCESS] 接口 [${step.name}] 执行成功，HTTP ${response.status}`)
      } else {
        step.status = 'failed'
        step.errorMsg = resData?.msg || resData?.error || '接口返回异常状态'
        logs.value.push(`[ERROR] 接口 [${step.name}] 执行失败: ${step.errorMsg}`)
        hasFailed = true
      }
    } catch (err: any) {
      const endTime = Date.now()
      step.latency = endTime - startTime
      step.code = 500
      step.status = 'failed'
      step.errorMsg = err.message || '网络连接异常'
      step.responseData = { error: step.errorMsg }
      logs.value.push(`[ERROR] 接口 [${step.name}] 网络请求异常: ${step.errorMsg}`)
      hasFailed = true
    }

    scrollToBottom()
    // 每个步骤之间稍微有一点停顿，体验更好
    await new Promise(r => setTimeout(r, 400))
  }

  if (timer) clearInterval(timer)
  currentStatus.value = hasFailed ? 'Failed' : 'Finished'
  logs.value.push(`\n[${new Date().toLocaleTimeString()}] 测试完成。`)
  scrollToBottom()

  await reportStore.addReport({
    name: toolName.value,
    type: '短剧类接口测试',
    status: hasFailed ? 'Failed' : 'Passed',
    duration: formatTime(duration.value),
    author: authStore.user?.username || 'tester',
    reportUrl: '',
    analysisResult: hasFailed ? '测试未通过：存在失败的接口' : '接口测试完成，时延和状态符合预期。',
    environment: testServer.value
  })
}

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
  { label: '正式服', value: 'prod' },
  { label: '灰度服', value: 'gray' }
]
const testServer = ref<ApiEnvironment>(loadSavedShortDramaEnvironment() || 'test')
const getViteEnv = (key: string) => String((import.meta.env as Record<string, string | undefined>)[key] || '')

const serverProfiles = {
  test: {
    email: getViteEnv('VITE_DRAMA_TEST_EMAIL'),
    password: getViteEnv('VITE_DRAMA_TEST_PASSWORD'),
    loginUrl: "http://35.225.224.94:8080/api/pwd_login",
    dramaListUrl: "http://35.225.224.94:8080/api/management/drama/all_online_ids"
  },
  prod: {
    email: getViteEnv('VITE_DRAMA_PROD_EMAIL'),
    password: getViteEnv('VITE_DRAMA_PROD_PASSWORD'),
    loginUrl: "https://admin.shortswave.com/api/pwd_login", // 假设路径对标
    dramaListUrl: "https://admin.shortswave.com/api/management/drama/all_online_ids"
  },
  gray: {
    email: getViteEnv('VITE_DRAMA_GRAY_EMAIL'),
    password: getViteEnv('VITE_DRAMA_GRAY_PASSWORD'),
    loginUrl: "http://35.193.183.77:8080/api/pwd_login",
    dramaListUrl: "http://35.193.183.77:8080/api/management/drama/all_online_ids"
  }
}

// 域名映射配置
const domainMappings = {
  ShortsWave: {
    prod: 'https://api.shortswave.com',
    test: 'http://35.225.224.94',
    gray: 'http://35.193.183.77'
  },
  NovelNova: {
    prod: 'https://api.novelnovastory.com',
    test: 'http://34.10.7.187',
    gray: 'http://34.10.7.187'
  }
}

watch([toolName, projectName, testServer], () => {
  persistShortDramaApiConfig()
}, { flush: 'post' })

watch(apiTestSuites, () => {
  persistShortDramaApiConfig()
}, { deep: true, flush: 'post' })

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
  const baseDomain = domainMappings[projectName.value as keyof typeof domainMappings][testServer.value]
  const deleteUrl = `${baseDomain}/user/delete`
  
  // 准备删除请求的 Headers，替换 X-SESSION-TOKEN
  const deleteHeaders = { 
    ...originalHeaders, 
    'X-SESSION-TOKEN': token 
  }

  logs.value.push(`[${new Date().toLocaleTimeString()}] 正在发起账号注销请求 (id: ${originalHeaders.user_id || '未知'})...`)
  scrollToBottom()

  try {
    const response = await fetch(buildBackendUrl('/api/proxy'), {
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
type ExecStatus = 'Ready' | 'Executing' | 'Stopped' | 'Finished' | 'Failed'
interface MonkeyGraphNode {
  id: string
  title: string
  event: string
  activity: string
  risk: 'normal' | 'warning' | 'critical' | 'unknown'
  imageUrl?: string
  summary?: string
  evidence?: MonkeyRiskEvidence[]
  x?: number
  y?: number
}

interface MonkeyGraphEdge {
  source: string
  target: string
  event: string
}

interface MonkeyRiskEvidence {
  level: 'warning' | 'critical'
  source: string
  message: string
  timestamp?: string
}

interface MonkeyDevice {
  id: string
  status: string
  model?: string
  product?: string
  source?: 'usb' | 'wifi'
}

interface MonkeyPackage {
  name: string
}

interface MonkeyRunSummary {
  runId: string
  status: 'running' | 'passed' | 'warning' | 'failed' | 'stopped' | 'incomplete'
  duration: string
  screenshotCount: number
  batchCount: number
  normalCount: number
  warningCount: number
  criticalCount: number
  unknownCount: number
  appCrashDetected: boolean
  terminationReason?: string
  ignoredSystemLogCount: number
  foregroundGuardEnabled: boolean
  foregroundRestartCount: number
  systemOverlayDismissCount: number
  adDismissCount: number
  adRestartCount: number
  lastForeignPackage?: string
  error?: string
}

interface MonkeyForegroundGuardEvent {
  action: 'restored' | 'restore-failed' | 'system-overlay-dismissed' | 'system-overlay-dismiss-failed' | 'ad-waiting' | 'ad-dismissed' | 'ad-restarted' | 'ad-restart-failed'
  foregroundPackage?: string
  targetPackage: string
  restartCount: number
  message: string
  timestamp: string
}

const currentStatus = ref<ExecStatus>('Ready')

// 运行时间控制
const reportUrl = ref('')
const analysisResult = ref('')
const tempLighthouseFile = ref('')
const uptime = ref(0)
const duration = ref(0)
let timer: ReturnType<typeof setInterval> | null = null
let monkeyTimer: ReturnType<typeof setInterval> | null = null
let monkeyReconcileTimer: ReturnType<typeof setInterval> | null = null

// 日志及报告
const logs = ref<string[]>(['准备就绪，点击 Execute 开始执行'])
const logContainer = ref<HTMLElement | null>(null)
let ws: WebSocket | null = null
const executionSucceeded = ref(false)
const executionFailed = ref(false)
const visibleLogs = computed(() => isDramaCheck ? dramaRunStore.logs : logs.value)
const visibleReportUrl = computed(() => isDramaCheck ? dramaRunStore.reportUrl : reportUrl.value)
const visibleDuration = computed(() => isDramaCheck ? dramaRunStore.duration : duration.value)
const visibleUptime = computed(() => isDramaCheck ? dramaRunStore.uptime : uptime.value)
const visibleStatus = computed<ExecStatus>(() => {
  if (!isDramaCheck) return currentStatus.value
  if (dramaRunStore.status === 'running') return 'Executing'
  if (dramaRunStore.status === 'done') return 'Finished'
  if (dramaRunStore.status === 'failed') return 'Failed'
  if (dramaRunStore.status === 'stopped') return 'Stopped'
  return currentStatus.value
})

const monkeyTarget = ref('com.company.shortsdrama.wave')
const monkeyDevice = ref('')
const monkeyDevices = ref<MonkeyDevice[]>([])
const monkeyPackages = ref<MonkeyPackage[]>([])
const monkeyPackagesLoading = ref(false)
const monkeyAdbAvailable = ref(false)
const monkeyAdbPath = ref('')
const monkeyWirelessDialogVisible = ref(false)
const monkeyWirelessPairAddress = ref('')
const monkeyWirelessPairingCode = ref('')
const monkeyWirelessConnectAddress = ref('')
const monkeyWirelessSubmitting = ref(false)
const monkeyWirelessPaired = ref(false)
const monkeyDurationSec = ref(3600)
const monkeyEventTotal = ref(1200)
const monkeySeed = ref('20260526')
const monkeyStrategy = ref('balanced')
const monkeyThrottleMs = ref(300)
const monkeyPrecisionMode = ref<'high' | 'low'>('low')
const monkeyDenseSamplingEnabled = ref(true)
const monkeyContinueAfterCrash = ref(false)
const monkeyRunId = ref('')
const monkeyGraphNodes = ref<MonkeyGraphNode[]>([])
const monkeyGraphEdges = ref<MonkeyGraphEdge[]>([])
const selectedMonkeyNode = ref<MonkeyGraphNode | null>(null)

const monkeyStrategyOptions = [
  { label: '均衡探索', value: 'balanced' },
  { label: '高频点击', value: 'tap-heavy' },
  { label: '滑动优先', value: 'scroll-heavy' },
  { label: '导航压力', value: 'navigation-heavy' }
]

const monkeyRiskLevel = computed(() => {
  if (monkeyEventTotal.value >= 5000 || monkeyDurationSec.value >= 30) return '高压'
  if (monkeyEventTotal.value >= 2000 || monkeyDurationSec.value >= 10) return '中压'
  return '真机'
})

const monkeyGraphReady = computed(() => isMonkeyTest && monkeyGraphNodes.value.length > 0 && visibleStatus.value !== 'Executing')
const selectedMonkeyImageSource = computed(() => normalizeBackendUrl(selectedMonkeyNode.value?.imageUrl || ''))
const { imageUrl: selectedMonkeyImageUrl, loading: selectedMonkeyImageLoading } = useAuthenticatedImage({
  sourceUrl: selectedMonkeyImageSource,
  getToken: () => authStore.token || ''
})
const monkeyScreenshotIntervalSec = computed(() => monkeyPrecisionMode.value === 'high' ? 10 : 30)
const monkeyScreenshotEvery = computed(() => Math.max(1, Math.floor((monkeyScreenshotIntervalSec.value * 1000) / Math.max(1, monkeyThrottleMs.value))))
const monkeyPrecisionLabel = computed(() => monkeyPrecisionMode.value === 'high' ? '高精度 · 10s/张' : '低精度 · 30s/张')

const toggleMonkeyPrecisionMode = () => {
  monkeyPrecisionMode.value = monkeyPrecisionMode.value === 'high' ? 'low' : 'high'
}

const monkeyCommandPreview = computed(() => {
  const baseArgs = [
    `adb -s ${monkeyDevice.value || '<deviceId>'} shell monkey`,
    `  -p ${monkeyTarget.value || '<packageName>'}`,
    `  --pct-touch ${monkeyStrategy.value === 'tap-heavy' ? 70 : 45}`,
    `  --pct-motion ${monkeyStrategy.value === 'scroll-heavy' ? 40 : 20}`,
    `  --pct-nav ${monkeyStrategy.value === 'navigation-heavy' ? 25 : 10}`,
    `  --pct-majornav ${monkeyStrategy.value === 'navigation-heavy' ? 20 : 10}`,
    '  --pct-appswitch 0',
    `  --throttle ${monkeyThrottleMs.value}`,
    ...(monkeyContinueAfterCrash.value ? ['  --ignore-crashes'] : []),
    `  -s ${monkeySeed.value}`,
    `  -v -v -v ${monkeyEventTotal.value}`
  ]
  return baseArgs.join('\n')
})

const monkeyScreenshotCommand = computed(() => {
  return `adb -s ${monkeyDevice.value || '<deviceId>'} exec-out screencap -p > screenshots/screen-0001.png`
})

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

watch(visibleLogs, () => {
  scrollToBottom()
})

// 新增：URL 校验函数
const isValidUrl = (url: string) => {
  if (!url) return false
  // 更加宽松的判断逻辑：支持 http/https 协议头，支持 IP 地址、端口号及 localhost
  // 符合用户描述：理论上带有 http:// 的就是一个正常链接
  const pattern = /^(https?:\/\/)?(([a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|localhost|(?:\d{1,3}\.){3}\d{1,3})(:\d+)?(\/.*)?$/
  return pattern.test(url) || url.startsWith('http://') || url.startsWith('https://')
}

const createSeededRandom = (seed: string) => {
  let hash = 2166136261
  for (let i = 0; i < seed.length; i++) {
    hash ^= seed.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return () => {
    hash += 0x6D2B79F5
    let t = hash
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

const createDemoScreenshot = (title: string, subtitle: string, risk: MonkeyGraphNode['risk']) => {
  const accent = risk === 'critical' ? '#ef4444' : risk === 'warning' ? '#f59e0b' : '#10b981'
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="360" height="720" viewBox="0 0 360 720">
      <rect width="360" height="720" rx="34" fill="#0f172a"/>
      <rect x="18" y="28" width="324" height="664" rx="28" fill="#f8fafc"/>
      <rect x="38" y="58" width="284" height="56" rx="16" fill="${accent}"/>
      <text x="58" y="93" font-family="Arial" font-size="22" font-weight="700" fill="#ffffff">${title}</text>
      <rect x="38" y="138" width="284" height="96" rx="18" fill="#e2e8f0"/>
      <rect x="58" y="160" width="170" height="14" rx="7" fill="#94a3b8"/>
      <rect x="58" y="188" width="224" height="12" rx="6" fill="#cbd5e1"/>
      <rect x="58" y="210" width="130" height="12" rx="6" fill="#cbd5e1"/>
      <rect x="38" y="258" width="132" height="132" rx="20" fill="#ffffff"/>
      <rect x="190" y="258" width="132" height="132" rx="20" fill="#ffffff"/>
      <rect x="38" y="414" width="284" height="82" rx="18" fill="#ffffff"/>
      <rect x="38" y="520" width="284" height="82" rx="18" fill="#ffffff"/>
      <circle cx="104" cy="324" r="30" fill="${accent}" opacity="0.78"/>
      <circle cx="256" cy="324" r="30" fill="#3b82f6" opacity="0.72"/>
      <text x="58" y="462" font-family="Arial" font-size="18" font-weight="700" fill="#0f172a">${subtitle}</text>
      <text x="58" y="568" font-family="Arial" font-size="15" fill="#64748b">screencap sample</text>
    </svg>
  `
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}

const buildMonkeyGraph = (random: () => number, warningCount: number, crashCount: number, anrCount: number) => {
  const nodeCount = 300
  const activities = [
    'SplashActivity', 'HomeActivity', 'FeedActivity', 'PlayerActivity', 
    'DetailActivity', 'LoginDialog', 'PurchaseSheet', 'SettingsActivity', 
    'WebViewActivity', 'ErrorBoundary', 'ProfileActivity', 'SearchActivity', 
    'CommentSheet', 'ShareDialog', 'ThemeSelector', 'CacheManager'
  ]
  const events = ['tap', 'swipe', 'back', 'fling', 'input', 'appswitch', 'majornav', 'screencap', 'keyevent', 'pinch']
  
  const nodes: MonkeyGraphNode[] = []
  for (let i = 0; i < nodeCount; i++) {
    const x = 10 + random() * 80
    const y = 10 + random() * 80
    
    let risk: MonkeyGraphNode['risk'] = 'normal'
    if (i === nodeCount - 1 && (crashCount > 0 || anrCount > 0)) {
      risk = 'critical'
    } else if (random() > 0.97) {
      risk = 'critical'
    } else if (random() > (warningCount > 2 ? 0.88 : 0.93)) {
      risk = 'warning'
    }
    
    const activity = activities[Math.floor(random() * activities.length)] || 'UnknownActivity'
    const eventName = events[Math.floor(random() * events.length)] || 'sample'
    
    nodes.push({
      id: `screen-${String(i + 1).padStart(3, '0')}`,
      title: `采样截图 ${i + 1}`,
      event: `${eventName} #${Math.floor((i + 1) * monkeyScreenshotEvery.value + random() * 30)}`,
      activity,
      risk,
      imageUrl: createDemoScreenshot(`Screen ${i + 1}`, activity, risk),
      x,
      y
    })
  }

  const edges: MonkeyGraphEdge[] = []
  for (let i = 1; i < nodeCount; i++) {
    const parentIndex = Math.floor(random() * i)
    const sourceNode = nodes[parentIndex]
    const targetNode = nodes[i]
    if (sourceNode && targetNode) {
      edges.push({
        source: sourceNode.id,
        target: targetNode.id,
        event: events[Math.floor(random() * events.length)] || 'tap'
      })
    }
    
    if (random() > 0.85 && i > 2) {
      const randomTargetIndex = Math.floor(random() * i)
      const targetCrossNode = nodes[randomTargetIndex]
      if (randomTargetIndex !== parentIndex && targetCrossNode && targetNode) {
        edges.push({
          source: targetNode.id,
          target: targetCrossNode.id,
          event: 'crosspath'
        })
      }
    }
  }

  monkeyGraphNodes.value = nodes
  monkeyGraphEdges.value = edges
  selectedMonkeyNode.value = nodes.find((node) => node.risk === 'critical') || nodes.find((node) => node.risk === 'warning') || nodes[0] || null
}

const startMonkeyDemo = async () => {
  if (!monkeyTarget.value.trim()) {
    ElMessage.warning('请先填写 Monkey 测试目标')
    return
  }

  currentStatus.value = 'Executing'
  executionSucceeded.value = false
  executionFailed.value = false
  reportUrl.value = ''
  analysisResult.value = ''
  monkeyGraphNodes.value = []
  monkeyGraphEdges.value = []
  selectedMonkeyNode.value = null
  uptime.value = 0
  duration.value = 0
  logs.value = [
    `[${new Date().toLocaleTimeString()}] Android Monkey 图谱 Demo 调度启动`,
    `[ADB] ${monkeyCommandPreview.value.replace(/\n/g, ' ')}`,
    `[ADB] ${monkeyScreenshotCommand.value}`,
    `[CONFIG] 每 ${monkeyScreenshotEvery.value} 个事件采样一张截图，并写入本次 run 的截图节点。`,
    `[INFO] 正在启动 monkey、logcat 与 screencap 采集管线...`
  ]
  scrollToBottom()

  const random = createSeededRandom(monkeySeed.value)
  const actionPools: Record<string, string[]> = {
    balanced: ['tap', 'swipe', 'back', 'input', 'rotate', 'wait'],
    'tap-heavy': ['tap', 'tap', 'tap', 'long_press', 'input', 'wait'],
    'scroll-heavy': ['swipe', 'swipe', 'fling', 'tap', 'back', 'wait'],
    'navigation-heavy': ['back', 'tap', 'home_resume', 'deep_link', 'swipe', 'wait']
  }
  const actions = actionPools[monkeyStrategy.value] ?? actionPools.balanced ?? ['tap', 'swipe', 'back', 'wait']
  const checkpoints = [
    'adb logcat -c && 开始采集 logcat',
    'screencap 首页状态并生成 screen-01 节点',
    '随机点击后采样详情页截图',
    '滑动列表后采样滚动状态',
    '返回键触发导航分支并采样',
    '扫描 FATAL EXCEPTION / ANR / 白屏信号'
  ]

  timer = setInterval(() => {
    uptime.value++
    duration.value++
  }, 1000)

  let step = 0
  let executedEvents = 0
  let warningCount = 0
  const totalSteps = 12
  const batchSize = Math.max(20, Math.floor(monkeyEventTotal.value / totalSteps))
  const demoTickMs = Math.max(180, Math.floor((monkeyDurationSec.value * 1000) / totalSteps))

  monkeyTimer = setInterval(async () => {
    step += 1
    executedEvents = Math.min(monkeyEventTotal.value, executedEvents + batchSize)
    const action = actions[Math.floor(random() * actions.length)]
    const x = Math.floor(random() * 1080)
    const y = Math.floor(random() * 2400)
    const checkpoint = checkpoints[step % checkpoints.length]
    const hasWarning = random() > 0.84

    const shouldCapture = executedEvents % monkeyScreenshotEvery.value < batchSize
    logs.value.push(`[MONKEY] event #${String(executedEvents).padStart(4, '0')} ${action} x=${x} y=${y}`)
    logs.value.push(`[PIPELINE] ${checkpoint}${shouldCapture ? ' -> screencap 写入截图节点' : ''}`)
    if (hasWarning) {
      warningCount += 1
      logs.value.push(`[LOGCAT][WARN] Choreographer skipped frames, 页面恢复时间 ${Math.floor(450 + random() * 900)}ms`)
    }
    scrollToBottom()

    if (step >= totalSteps || executedEvents >= monkeyEventTotal.value) {
      if (monkeyTimer) clearInterval(monkeyTimer)
      monkeyTimer = null
      if (timer) clearInterval(timer)
      timer = null

      const crashCount = random() > 0.92 ? 1 : 0
      const anrCount = random() > 0.9 ? 1 : 0
      const coverage = Math.min(96, Math.floor(62 + random() * 28 + warningCount))
      const status = crashCount || anrCount ? 'Failed' : 'Passed'
      buildMonkeyGraph(random, warningCount, crashCount, anrCount)

      currentStatus.value = status === 'Passed' ? 'Finished' : 'Failed'
      executionSucceeded.value = status === 'Passed'
      executionFailed.value = status !== 'Passed'
      analysisResult.value = [
        `Monkey Hologram Demo Summary`,
        `包名: ${monkeyTarget.value}`,
        `设备: ${monkeyDevice.value}`,
        `执行命令: ${monkeyCommandPreview.value.replace(/\n/g, ' ')}`,
        `截图节点: ${monkeyGraphNodes.value.length}`,
        `执行事件: ${executedEvents}/${monkeyEventTotal.value}`,
        `覆盖估算: ${coverage}%`,
        `慢响应告警: ${warningCount}`,
        `Crash: ${crashCount}, ANR: ${anrCount}`,
        `建议: ${warningCount > 2 ? '优先点击黄色/红色节点回看截图与 logcat 片段。' : '当前图谱未发现明显稳定性风险。'}`
      ].join('\n')

      logs.value.push(`[GRAPH] 已生成 ${monkeyGraphNodes.value.length} 个截图节点、${monkeyGraphEdges.value.length} 条事件路径连线。`)
      logs.value.push(`[SUMMARY] events=${executedEvents}, coverage=${coverage}%, screenshots=${monkeyGraphNodes.value.length}, warnings=${warningCount}, crash=${crashCount}, anr=${anrCount}`)
      logs.value.push(`[${new Date().toLocaleTimeString()}] Monkey 图谱 Demo 执行${status === 'Passed' ? '完成' : '失败'}。`)
      scrollToBottom()

      await reportStore.addReport({
        name: toolName.value,
        type: 'UI 自动化',
        status,
        duration: formatTime(duration.value),
        author: authStore.user?.username || 'tester',
        reportUrl: '',
        analysisResult: analysisResult.value,
        environment: testServer.value
      })
    }
  }, demoTickMs)
}
void startMonkeyDemo

const monkeyAuthHeaders = () => ({
  'Content-Type': 'application/json',
  Authorization: authStore.token || ''
})

const fetchMonkeyDevices = async () => {
  if (!isMonkeyTest) return
  try {
    const response = await fetch(buildBackendUrl('/api/monkey/devices'), {
      headers: { Authorization: authStore.token || '' }
    })
    const data = await response.json()
    if (!response.ok) throw new Error(data.error || '设备读取失败')
    monkeyAdbAvailable.value = Boolean(data.adbAvailable)
    monkeyAdbPath.value = data.adbPath || ''
    monkeyDevices.value = Array.isArray(data.devices) ? data.devices : []
    const previousDevice = monkeyDevice.value
    if (!monkeyDevice.value) {
      const online = monkeyDevices.value.find(device => device.status === 'device')
      monkeyDevice.value = online?.id || monkeyDevices.value[0]?.id || ''
    }
    if (monkeyDevice.value && monkeyDevice.value === previousDevice) {
      await fetchMonkeyPackages()
    }
    if (!data.adbAvailable) {
      logs.value = [`[ADB] ${data.message || '未找到 adb'}`]
    }
  } catch (error: any) {
    ElMessage.error(error.message || '读取 Android 设备失败')
  }
}

const postMonkeyWirelessADB = async (path: string, body: Record<string, string>) => {
  monkeyWirelessSubmitting.value = true
  try {
    const response = await fetch(buildBackendUrl(path), {
      method: 'POST',
      headers: monkeyAuthHeaders(),
      body: JSON.stringify(body)
    })
    const data = await response.json()
    if (!response.ok) throw new Error(data.error || '无线 ADB 操作失败')
    ElMessage.success(data.message || '无线 ADB 操作成功')
    await fetchMonkeyDevices()
    return true
  } catch (error: any) {
    ElMessage.error(error.message || '无线 ADB 操作失败')
    return false
  } finally {
    monkeyWirelessSubmitting.value = false
  }
}

const pairMonkeyWirelessADB = async () => {
  const paired = await postMonkeyWirelessADB('/api/monkey/wireless/pair', {
    address: monkeyWirelessPairAddress.value,
    pairingCode: monkeyWirelessPairingCode.value
  })
  if (paired) {
    monkeyWirelessPairingCode.value = ''
    monkeyWirelessPaired.value = true
  }
}

const connectMonkeyWirelessADB = async () => {
  const connected = await postMonkeyWirelessADB('/api/monkey/wireless/connect', {
    address: monkeyWirelessConnectAddress.value
  })
  if (connected) {
    monkeyDevice.value = monkeyWirelessConnectAddress.value.trim()
    monkeyWirelessDialogVisible.value = false
  }
}

const disconnectSelectedMonkeyWirelessADB = async () => {
  if (!monkeyDevice.value || !monkeyDevices.value.find(device => device.id === monkeyDevice.value && device.source === 'wifi')) {
    ElMessage.warning('请先选择已连接的 Wi-Fi 设备')
    return
  }
  const disconnected = await postMonkeyWirelessADB('/api/monkey/wireless/disconnect', {
    address: monkeyDevice.value
  })
  if (disconnected) {
    monkeyDevice.value = ''
  }
}

const fetchMonkeyPackages = async () => {
  if (!isMonkeyTest || !monkeyDevice.value) {
    monkeyPackages.value = []
    return
  }
  monkeyPackagesLoading.value = true
  try {
    const response = await fetch(buildBackendUrl(`/api/monkey/devices/${encodeURIComponent(monkeyDevice.value)}/packages`), {
      headers: { Authorization: authStore.token || '' }
    })
    const data = await response.json()
    if (!response.ok) throw new Error(data.error || 'App 包名读取失败')
    monkeyPackages.value = Array.isArray(data.packages) ? data.packages : []
    if (!monkeyPackages.value.some(item => item.name === monkeyTarget.value)) {
      monkeyTarget.value = monkeyPackages.value[0]?.name || ''
    }
  } catch (error: any) {
    monkeyPackages.value = []
    ElMessage.error(error.message || '读取三方 App 包名失败')
  } finally {
    monkeyPackagesLoading.value = false
  }
}

watch(monkeyDevice, (deviceId, previousDeviceId) => {
  if (!isMonkeyTest || deviceId === previousDeviceId) return
  fetchMonkeyPackages()
})

const mapMonkeyStatus = (status: MonkeyRunSummary['status']): ExecStatus => {
  if (status === 'running') return 'Executing'
  if (status === 'failed' || status === 'incomplete') return 'Failed'
  if (status === 'stopped') return 'Stopped'
  return 'Finished'
}

const loadMonkeyEvents = async (runId: string) => {
  const response = await fetch(buildBackendUrl(`/api/monkey/runs/${runId}/events`), {
    headers: { Authorization: authStore.token || '' }
  })
  if (!response.ok) return false
  const events = await response.json() as MonkeyGraphNode[]
  monkeyGraphNodes.value = events.map((event, index) => ({
    ...event,
    imageUrl: event.imageUrl ? normalizeBackendUrl(event.imageUrl) : '',
    x: 12 + (index % 12) * 7,
    y: 12 + Math.floor(index / 12) * 8
  }))
  monkeyGraphEdges.value = monkeyGraphNodes.value.slice(1).map((node, index) => ({
    source: monkeyGraphNodes.value[index]?.id || node.id,
    target: node.id,
    event: node.event || 'screencap'
  }))
  selectedMonkeyNode.value =
    monkeyGraphNodes.value.find(node => node.risk === 'critical') ||
    monkeyGraphNodes.value.find(node => node.risk === 'warning') ||
    monkeyGraphNodes.value[0] ||
    null
  return true
}

const appendMonkeyLiveLog = (line: string) => {
  logs.value = [...logs.value, line].slice(-240)
  scrollToBottom()
}

const upsertMonkeyGraphNode = (event: MonkeyGraphNode) => {
  const existingIndex = monkeyGraphNodes.value.findIndex(node => node.id === event.id)
  const nextNode = {
    ...event,
    imageUrl: event.imageUrl ? normalizeBackendUrl(event.imageUrl) : '',
    x: 12 + ((existingIndex >= 0 ? existingIndex : monkeyGraphNodes.value.length) % 12) * 7,
    y: 12 + Math.floor((existingIndex >= 0 ? existingIndex : monkeyGraphNodes.value.length) / 12) * 8
  }
  if (existingIndex >= 0) {
    monkeyGraphNodes.value = monkeyGraphNodes.value.map((node, index) => index === existingIndex ? nextNode : node)
  } else {
    monkeyGraphNodes.value = [...monkeyGraphNodes.value, nextNode]
    const previousNode = monkeyGraphNodes.value[monkeyGraphNodes.value.length - 2]
    if (previousNode) {
      monkeyGraphEdges.value = [...monkeyGraphEdges.value, {
        source: previousNode.id,
        target: nextNode.id,
        event: nextNode.event || 'screencap'
      }]
    }
  }
  if (!selectedMonkeyNode.value || event.risk === 'critical') selectedMonkeyNode.value = nextNode
}

let finalizedMonkeyRunId = ''

const finalizeMonkeyRun = async (summary: MonkeyRunSummary) => {
  if (finalizedMonkeyRunId === summary.runId) return
  executionSucceeded.value = summary.status === 'passed'
  executionFailed.value = ['failed', 'incomplete', 'stopped'].includes(summary.status)
  const eventsLoaded = await loadMonkeyEvents(summary.runId)
  if (!eventsLoaded) {
    appendMonkeyLiveLog('[REPORT][WARN] 截图节点尚未准备完成，保留状态校准以便自动重试')
    return
  }
  finalizedMonkeyRunId = summary.runId
  monkeyRunStream.disconnect()
  if (monkeyReconcileTimer) clearInterval(monkeyReconcileTimer)
  monkeyReconcileTimer = null
  if (timer) clearInterval(timer)
  timer = null
  const runtimeLogs = await fetchMonkeyRuntimeLogs(summary.runId)
  logs.value = [...logs.value, ...runtimeLogs].slice(-240)
  reportStore.fetchReports()
}

const applyMonkeySummary = (summary: MonkeyRunSummary) => {
  currentStatus.value = mapMonkeyStatus(summary.status)
  duration.value = summary.duration ? summary.duration.split(':').reduce((acc, part) => acc * 60 + Number(part || 0), 0) : duration.value
  if (summary.status === 'running' && summary.screenshotCount > monkeyGraphNodes.value.length) {
    void loadMonkeyEvents(summary.runId)
  }
  const summaryLines = [
    `[SSE][RUN] ${summary.runId} ${summary.status}`,
    `[SCREENSHOT] ${summary.screenshotCount} 张，normal=${summary.normalCount}, warning=${summary.warningCount}, critical=${summary.criticalCount}, unknown=${summary.unknownCount}`,
    `[BATCH] 已执行 ${summary.batchCount || 1} 个 Monkey 批次，平台将持续续跑至设定时长`,
    `[EVIDENCE] 真实 App Crash=${summary.appCrashDetected ? '是' : '否'}，结束原因=${summary.terminationReason || '运行中'}，已过滤系统噪声=${summary.ignoredSystemLogCount || 0}`,
    `[GUARD] 前台守护=${summary.foregroundGuardEnabled ? '开启' : '关闭'}，自动重启=${summary.foregroundRestartCount || 0} 次，系统菜单收起=${summary.systemOverlayDismissCount || 0} 次，广告关闭=${summary.adDismissCount || 0} 次，广告重启=${summary.adRestartCount || 0} 次${summary.lastForeignPackage ? `，最近偏离=${summary.lastForeignPackage}` : ''}`,
    summary.error ? `[ERROR] ${summary.error}` : '[INFO] Monkey 真机测试采集中...'
  ]
  logs.value = [
    ...summaryLines,
    ...logs.value.filter(line => !line.startsWith('[SSE][RUN]') && !line.startsWith('[SCREENSHOT]') && !line.startsWith('[BATCH]') && !line.startsWith('[EVIDENCE]') && !line.startsWith('[GUARD]') && line !== '[INFO] Monkey 真机测试采集中...')
  ].slice(-240)
  if (summary.status !== 'running') {
    void finalizeMonkeyRun(summary)
  }
}

const refreshMonkeyRun = async (runId: string) => {
  const response = await fetch(buildBackendUrl(`/api/monkey/runs/${runId}`), {
    headers: { Authorization: authStore.token || '' }
  })
  const summary = await response.json() as MonkeyRunSummary
  if (!response.ok) throw new Error((summary as any).error || '读取 Monkey 状态失败')
  applyMonkeySummary(summary)
}

const fetchMonkeyRuntimeLogs = async (runId: string) => {
  const readLog = async (kind: 'monkey' | 'logcat') => {
    try {
      const response = await fetch(buildBackendUrl(`/api/monkey/runs/${runId}/logs?kind=${kind}`), {
        headers: { Authorization: authStore.token || '' }
      })
      if (!response.ok) return []
      const text = await response.text()
      return text
        .split('\n')
        .map(line => line.trim())
        .filter(Boolean)
        .slice(-40)
        .map(line => `[${kind.toUpperCase()}] ${line}`)
    } catch {
      return []
    }
  }
  const [monkeyLines, logcatLines] = await Promise.all([readLog('monkey'), readLog('logcat')])
  return [...monkeyLines, ...logcatLines].slice(-80)
}

const monkeyRunStream = useMonkeyRunStream({
  getToken: () => authStore.token || '',
  onConnected: () => appendMonkeyLiveLog('[SSE] Monkey 实时通道已连接'),
  onDisconnected: () => appendMonkeyLiveLog('[SSE] Monkey 实时通道断开，正在自动重连'),
  onError: error => appendMonkeyLiveLog(`[SSE][ERROR] ${error.message}`),
  onMessage: message => {
    if (message.type === 'summary' || message.type === 'complete') {
      applyMonkeySummary(message.data as MonkeyRunSummary)
      return
    }
    if (message.type === 'graph-event') {
      upsertMonkeyGraphNode(message.data as MonkeyGraphNode)
      return
    }
    if (message.type === 'risk-evidence') {
      const evidence = message.data as MonkeyRiskEvidence
      appendMonkeyLiveLog(`[SSE][${evidence.source.toUpperCase()}][${evidence.level.toUpperCase()}] ${evidence.message}`)
      return
    }
    if (message.type === 'foreground-guard') {
      const event = message.data as MonkeyForegroundGuardEvent
      appendMonkeyLiveLog(`[SSE][GUARD][${event.action.toUpperCase()}] ${event.message}`)
    }
  }
})

const startMonkeyRun = async () => {
  if (!monkeyTarget.value.trim()) {
    ElMessage.warning('请先填写 Android App 包名')
    return
  }
  if (!monkeyDevice.value) {
    ElMessage.warning('请先选择已授权的 Android 设备')
    await fetchMonkeyDevices()
    return
  }

  currentStatus.value = 'Executing'
  executionSucceeded.value = false
  executionFailed.value = false
  monkeyGraphNodes.value = []
  monkeyGraphEdges.value = []
  selectedMonkeyNode.value = null
  logs.value = [
    `[${new Date().toLocaleTimeString()}] Android Monkey 真机测试启动`,
    `[ADB] ${monkeyCommandPreview.value.replace(/\n/g, ' ')}`,
    `[SCREENSHOT] ${monkeyPrecisionMode.value === 'high' ? '高精度' : '低精度'}模式，每 ${monkeyScreenshotIntervalSec.value}s 截图一次`,
    `[DENSE] 异常加密采样${monkeyDenseSamplingEnabled.value ? '开启：warning 5s/张 60s，critical 2s/张 120s' : '关闭'}`,
    `[CRASH] ${monkeyContinueAfterCrash.value ? '发现 Crash 后继续执行并持续留证' : '发现 Crash 后立即停止并留证'}`,
    '[GUARD] 边界保护已开启：自动收起系统菜单；检测到广告页后等待 5 秒，优先尝试关闭，无法关闭时重启目标 App'
  ]
  uptime.value = 0
  duration.value = 0
  timer = setInterval(() => {
    uptime.value++
    duration.value++
  }, 1000)

  try {
    const response = await fetch(buildBackendUrl('/api/monkey/runs'), {
      method: 'POST',
      headers: monkeyAuthHeaders(),
      body: JSON.stringify({
        packageName: monkeyTarget.value,
        deviceId: monkeyDevice.value,
        precisionMode: monkeyPrecisionMode.value,
        durationSec: monkeyDurationSec.value,
        eventTotal: monkeyEventTotal.value,
        throttleMs: monkeyThrottleMs.value,
        seed: monkeySeed.value,
        strategy: monkeyStrategy.value,
        denseSamplingEnabled: monkeyDenseSamplingEnabled.value,
        continueAfterCrash: monkeyContinueAfterCrash.value,
        author: authStore.user?.username || 'tester',
        environment: testServer.value
      })
    })
    const summary = await response.json() as MonkeyRunSummary
    if (!response.ok) throw new Error((summary as any).error || 'Monkey 启动失败')
    monkeyRunId.value = summary.runId
    finalizedMonkeyRunId = ''
    logs.value.push(`[RUN] ${summary.runId} 已创建，开始采集 monkey/logcat/screencap`)
    monkeyRunStream.connect(summary.runId)
    await loadMonkeyEvents(summary.runId)
    monkeyReconcileTimer = setInterval(() => {
      refreshMonkeyRun(summary.runId).catch(error => appendMonkeyLiveLog(`[RECONCILE][ERROR] ${error.message}`))
    }, 30000)
  } catch (error: any) {
    currentStatus.value = 'Failed'
    executionFailed.value = true
    if (timer) clearInterval(timer)
    timer = null
    logs.value.push(`[ERROR] ${error.message}`)
    ElMessage.error(error.message || 'Monkey 启动失败')
  }
}

const startExecution = async () => {
  if (currentStatus.value === 'Executing') return

  if (isShortDramaApiTest.value) {
    await runShortDramaApiTest()
    return
  }

  if (isMonkeyTest) {
    await startMonkeyRun()
    return
  }

  // 针对 Web前端压测的链接校验
  if (isWebFrontendStressTest) {
    if (!projectName.value || !isValidUrl(projectName.value)) {
      ElMessage.warning('请输入有效的测试链接 (如: http://example.com)')
      return
    }
  }

  // 特殊处理：删除账号工具的匿名登录逻辑
  if (isDeleteAccount) {
    if (!deleteAccountParam.value) {
      ElMessage.warning('请先填入参数')
      return
    }

    const params = parseParams(deleteAccountParam.value)
    const baseDomain = domainMappings[projectName.value as keyof typeof domainMappings][testServer.value]
    const loginUrl = `${baseDomain}/login/anonymous`

    currentStatus.value = 'Executing'
    logs.value = [
      `[${new Date().toLocaleTimeString()}] 准备发起匿名登录请求...`,
      `[DEBUG] 目标 URL: ${loginUrl}`,
      `[DEBUG] 解析后的请求头 (Headers): ${JSON.stringify(params, null, 2)}`
    ]
    scrollToBottom()
    
    try {
      const response = await fetch(buildBackendUrl('/api/proxy'), {
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

  reportUrl.value = '' // 清除上一次的报告
  executionSucceeded.value = false
  executionFailed.value = false
  logs.value = []
  logs.value.push(`[${new Date().toLocaleTimeString()}] 准备连接调度引擎...`)

  currentStatus.value = 'Executing'
  
  // 获取当前配置
  const profile = serverProfiles[testServer.value as keyof typeof serverProfiles]
  
    // 组装 WebSocket 链接地址，注入动态参数
    let wsUrl = buildBackendWsUrl('/api/ws/k6')
    const params: Record<string, string> = {
      script: scriptName,
      email: profile.email,
      password: profile.password,
      loginUrl: profile.loginUrl,
      dramaListUrl: profile.dramaListUrl
    }

    if (isWebFrontendStressTest) {
      wsUrl = buildBackendWsUrl('/api/ws/lighthouse')
      // 确保有协议头
      const finalUrl = projectName.value.startsWith('http') ? projectName.value : `http://${projectName.value}`
      delete params.script // lighthouse 不需要 script 参数
      params.url = finalUrl
    }

    const query = new URLSearchParams(params).toString()
    if (isDramaCheck) {
      await dramaRunStore.start({
        email: profile.email,
        password: profile.password,
        loginUrl: profile.loginUrl,
        dramaListUrl: profile.dramaListUrl,
        toolName: toolName.value,
        author: authStore.user?.username || 'tester',
        environment: testServer.value
      })
      return
    }

    ws = new WebSocket(`${wsUrl}?${query}`)

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
      const data = event.data
      if (typeof data === 'string' && data.startsWith('EXECUTION_STATUS:')) {
        const [, status, reason] = data.split(':')
        if (status === 'success') {
          executionSucceeded.value = true
        } else {
          executionFailed.value = true
          logs.value.push(`[ERROR] 执行失败${reason ? `: ${reason}` : ''}`)
          scrollToBottom()
        }
        return
      }
      // 识别后端发送的报告就绪信号
      if (typeof data === 'string' && data.startsWith('REPORT_READY_URL:')) {
        const nextReportUrl = data.slice('REPORT_READY_URL:'.length)
        reportUrl.value = nextReportUrl ? normalizeBackendUrl(nextReportUrl) : ''
        return
      }
      if (typeof data === 'string' && data.startsWith('REPORT_READY:')) {
        const reportFile = data.split(':')[1] || ''
        if (isWebFrontendStressTest) {
          // 暂存文件名，等分析完再显示
          tempLighthouseFile.value = reportFile
        } else if (isDramaCheck) {
          logs.value.push(`[WARN] 收到旧版报告文件事件，已忽略以避免读取可覆盖报告。`)
        } else {
          const staticPath = 'reports'
          reportUrl.value = buildBackendUrl(`/${staticPath}/${reportFile}?t=${Date.now()}`)
        }
        return
      }
      logs.value.push(data)
      scrollToBottom()
    }

    ws.onerror = () => {
      logs.value.push(`[WARN/ERR] WebSocket 连接错误`)
    }

    ws.onclose = async () => {
      if (timer) clearInterval(timer)
      if (currentStatus.value === 'Executing') {
        // K6 类任务必须等后端明确返回成功信号，避免异常关闭时展示旧报告。
        // Web 性能分析收到后端失败事件时也直接进入失败态，避免空报告被误展示为完成。
        const reportType = isWebFrontendStressTest ? 'Web 性能分析' : 'K6 压测'
        const hasExecutionFailure = executionFailed.value || (!isWebFrontendStressTest && !executionSucceeded.value)
        if (hasExecutionFailure) {
          currentStatus.value = 'Failed'
          logs.value.push(`[${new Date().toLocaleTimeString()}] 任务执行失败，未生成新报告。`)
          await reportStore.addReport({
            name: toolName.value,
            type: reportType,
            status: 'Failed',
            duration: formatTime(duration.value),
            author: authStore.user?.username || 'tester',
            reportUrl: '',
            analysisResult: '',
            environment: isWebFrontendStressTest ? 'prod' : testServer.value
          })
          return
        }

        currentStatus.value = 'Finished'

        // 如果是 K6 类任务，手动设置报告路径（Lighthouse 类任务由 ws 消息驱动）
        if (!isWebFrontendStressTest && !reportUrl.value) {
          if (isDramaCheck) {
            currentStatus.value = 'Failed'
            logs.value.push(`[${new Date().toLocaleTimeString()}] 剧集测试未收到唯一报告快照，已阻止写入默认覆盖报告。`)
            await reportStore.addReport({
              name: toolName.value,
              type: reportType,
              status: 'Failed',
              duration: formatTime(duration.value),
              author: authStore.user?.username || 'tester',
              reportUrl: '',
              analysisResult: '',
              environment: testServer.value
            })
            return
          }
          const reportFile = 'summary.html'
          const finalReportUrl = buildBackendUrl(`/reports/${reportFile}?t=${Date.now()}`)
          reportUrl.value = finalReportUrl
        }
        
        logs.value.push(`[${new Date().toLocaleTimeString()}] 任务执行完成。`)

        // 如果是 Web 性能分析，补充分析逻辑
        if (isWebFrontendStressTest && tempLighthouseFile.value) {
          logs.value.push(`\n[AI] 正在分析报告结果，请稍候。。。`)
          scrollToBottom()
          
          try {
            const jsonFile = tempLighthouseFile.value.replace('.html', '.json')
            const response = await fetch(buildBackendUrl('/api/performance/analyze'), {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify({ filename: jsonFile })
            })
            const result = await response.json()
            analysisResult.value = result.analysis
            
            // 将 AI 分析结果输出到日志区
            logs.value.push(`\n${analysisResult.value}\n`)
            
            // 分析完成后，正式显示 HTML 报告
            reportUrl.value = buildBackendUrl(`/performance-reports/${tempLighthouseFile.value}?t=${Date.now()}`)
          } catch (e) {
            logs.value.push(`[ERROR] AI 分析失败: ${e}`)
          }
          scrollToBottom()
        } else if (isWebFrontendStressTest) {
          currentStatus.value = 'Failed'
          logs.value.push(`[${new Date().toLocaleTimeString()}] 未收到 Lighthouse 报告文件，任务标记为失败。`)
        }

        // 同步到后端执行记录
        await reportStore.addReport({
          name: toolName.value,
          type: reportType,
          status: 'Passed',
          duration: formatTime(duration.value),
          author: authStore.user?.username || 'tester',
          reportUrl: reportUrl.value || '',
          analysisResult: analysisResult.value || '',
          environment: isWebFrontendStressTest ? 'prod' : testServer.value
        })
      }
    }
}

const stopExecution = async () => {
  if (visibleStatus.value !== 'Executing') return

  if (isShortDramaApiTest.value) {
    if (timer) clearInterval(timer)
    currentStatus.value = 'Stopped'
    logs.value.push(`[${new Date().toLocaleTimeString()}] 测试被手动终止。`)
    return
  }

  if (isDramaCheck) {
    await dramaRunStore.stop()
    currentStatus.value = 'Stopped'
    return
  }

  if (isMonkeyTest) {
    if (!monkeyRunId.value) return
    appendMonkeyLiveLog(`[${new Date().toLocaleTimeString()}] 正在停止设备端 Monkey 进程并生成报告...`)
    try {
      const response = await fetch(buildBackendUrl(`/api/monkey/runs/${monkeyRunId.value}/stop`), {
        method: 'POST',
        headers: { Authorization: authStore.token || '' }
      })
      const result = await response.json() as MonkeyRunSummary | { message?: string; error?: string }
      if (!response.ok && response.status !== 202) {
        throw new Error('error' in result ? result.error : '停止 Monkey 失败')
      }
      if (response.status === 202) {
        appendMonkeyLiveLog(`[STOP] ${'message' in result ? result.message : '报告仍在生成，等待实时通道返回最终状态'}`)
        return
      }
      applyMonkeySummary(result as MonkeyRunSummary)
      appendMonkeyLiveLog(`[${new Date().toLocaleTimeString()}] Monkey 已停止，当前测试数据已生成报告。`)
    } catch (error: any) {
      appendMonkeyLiveLog(`[STOP][ERROR] ${error.message}`)
      ElMessage.error(error.message || '停止 Monkey 失败')
    }
    return
  }

  currentStatus.value = 'Stopped'

  if (ws) {
    ws.close()
    ws = null
  }
  if (timer) clearInterval(timer)
  if (monkeyTimer) {
    clearInterval(monkeyTimer)
    monkeyTimer = null
  }
  if (monkeyReconcileTimer) {
    clearInterval(monkeyReconcileTimer)
    monkeyReconcileTimer = null
  }
  monkeyRunStream.disconnect()

  if (!isDramaCheck) {
    await reportStore.addReport({
      name: toolName.value,
      type: 'K6 压测',
      status: 'Failed',
      duration: formatTime(duration.value),
      author: authStore.user?.username || 'tester',
      environment: testServer.value,
    })
  }

  logs.value.push(`[${new Date().toLocaleTimeString()}] 手动终止执行。`)
}

const clearLogs = () => {
  logs.value = []
}

const closePage = () => {
  persistShortDramaApiConfig()
  if (isDramaCheck && visibleStatus.value === 'Executing') {
    dramaRunStore.markBackground()
  } else if (currentStatus.value === 'Executing') {
    stopExecution()
  }
  router.push('/')
}

const persistShortDramaApiConfigBeforeUnload = () => {
  persistShortDramaApiConfig()
}

onMounted(() => {
  reportStore.fetchReports()
  fetchMonkeyDevices()
  window.addEventListener('beforeunload', persistShortDramaApiConfigBeforeUnload)
})

onUnmounted(() => {
  persistShortDramaApiConfig()
  window.removeEventListener('beforeunload', persistShortDramaApiConfigBeforeUnload)
  if (timer) clearInterval(timer)
  if (monkeyTimer) clearInterval(monkeyTimer)
  if (monkeyReconcileTimer) clearInterval(monkeyReconcileTimer)
  monkeyRunStream.disconnect()
  if (isDramaCheck && visibleStatus.value === 'Executing') {
    dramaRunStore.markBackground()
    return
  }
  if (ws) ws.close()
})

onBeforeRouteLeave(() => {
  persistShortDramaApiConfig()
})
</script>

<template>
  <div class="com-api-container">
    <div class="main-content">
      
      <!-- 左侧信息区 -->
      <div class="sidebar-panel" :class="{ 'sidebar-panel--short-drama': isShortDramaApiTest }">
        <div class="status-badge-row">
          <span class="badge-ready" :class="{ 'badge-ready--short-drama': isShortDramaApiTest }">
            {{ isShortDramaApiTest ? '接口配置列表' : 'READY' }}
          </span>
            <el-button
              v-if="isShortDramaApiTest"
              type="primary"
              size="small"
              class="add-api-test-btn"
              @click="addApiTestSuite"
            >
              新增接口测试
            </el-button>
          <el-icon v-else class="info-icon"><Warning /></el-icon>
        </div>

        <div v-if="!isShortDramaApiTest" class="tool-title-section">
          <h1 class="tool-title">{{ toolName }}</h1>
          <p class="tool-desc">{{ toolDesc }}</p>
        </div>

        <div class="params-section">
          <!-- 针对删除账号工具，新增填入参数输入框 -->
          <div v-if="isDeleteAccount" class="param-group">
            <label class="param-label">填入参数</label>
            <el-input v-model="deleteAccountParam" placeholder="请输入参数" />
          </div>

          <template v-if="isMonkeyTest">
            <div class="param-group">
              <label class="param-label">测试目标</label>
              <el-select
                v-model="monkeyTarget"
                class="param-select"
                filterable
                clearable
                :loading="monkeyPackagesLoading"
                placeholder="选择已连接设备上的三方 App 包名"
              >
                <el-option
                  v-for="app in monkeyPackages"
                  :key="app.name"
                  :label="app.name"
                  :value="app.name"
                />
              </el-select>
            </div>
            <div class="param-group">
              <label class="param-label">测试设备</label>
              <div class="monkey-device-row">
                <el-select v-model="monkeyDevice" class="param-select" placeholder="选择已授权 Android 设备">
                  <el-option
                    v-for="device in monkeyDevices"
                    :key="device.id"
                    :label="`${device.model || device.id} · ${device.source === 'wifi' ? 'Wi-Fi' : 'USB'} (${device.status})`"
                    :value="device.id"
                    :disabled="device.status !== 'device'"
                  />
                </el-select>
                <el-button size="small" @click="fetchMonkeyDevices">刷新设备</el-button>
              </div>
              <div class="monkey-wireless-actions">
                <el-button
                  v-if="permissionStore.canAccess('dashboard.monkey_test.visible')"
                  size="small"
                  @click="monkeyWirelessDialogVisible = true"
                >
                  无线连接设备
                </el-button>
                <el-button
                  v-if="permissionStore.canAccess('dashboard.monkey_test.visible')"
                  size="small"
                  :disabled="!monkeyDevices.some(device => device.id === monkeyDevice && device.source === 'wifi')"
                  @click="disconnectSelectedMonkeyWirelessADB"
                >
                  断开 Wi-Fi
                </el-button>
              </div>
            </div>
            <div class="param-row">
              <div class="param-group half">
                <label class="param-label">事件数</label>
                <el-input-number
                  v-model="monkeyEventTotal"
                  :min="100"
                  :max="20000"
                  :step="100"
                  controls-position="right"
                  class="param-number"
                />
              </div>
              <div class="param-group half">
                <label class="param-label">时长（秒）</label>
                <el-input-number
                  v-model="monkeyDurationSec"
                  :min="60"
                  :max="21600"
                  :step="300"
                  controls-position="right"
                  class="param-number"
                />
              </div>
            </div>
            <div class="param-row">
              <div class="param-group half">
                <label class="param-label">Throttle（毫秒）</label>
                <el-input-number
                  v-model="monkeyThrottleMs"
                  :min="300"
                  :max="3000"
                  :step="100"
                  :value-on-clear="300"
                  controls-position="right"
                  class="param-number"
                />
                <span class="param-help">每次随机事件间隔，可输入 300～3000ms。</span>
              </div>
              <div class="param-group half">
                <label class="param-label">测试精度</label>
                <el-button
                  class="monkey-mode-switch"
                  @click="toggleMonkeyPrecisionMode"
                >
                  {{ monkeyPrecisionLabel }}
                </el-button>
              </div>
            </div>
            <div class="param-group">
              <label class="param-label">随机策略</label>
              <el-select v-model="monkeyStrategy" class="param-select">
                <el-option
                  v-for="item in monkeyStrategyOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </div>
            <div class="param-group">
              <label class="param-label">随机种子</label>
              <el-input v-model="monkeySeed" placeholder="数字 seed；非数字会由后端稳定转换" />
            </div>
            <div class="param-group">
              <el-checkbox v-model="monkeyDenseSamplingEnabled" class="monkey-option-checkbox">
                异常加密采样（warning 5s/张持续60s，critical 2s/张持续120s）
              </el-checkbox>
            </div>
            <div class="param-group">
              <el-checkbox v-model="monkeyContinueAfterCrash" class="monkey-option-checkbox">
                Crash 后继续执行（持续采样并记录后续问题）
              </el-checkbox>
            </div>
            <div class="monkey-risk-card">
              <span class="monkey-risk-card__label">运行级别</span>
              <strong>{{ monkeyRiskLevel }}</strong>
              <span class="monkey-risk-card__description">
                ADB {{ monkeyAdbAvailable ? `已就绪：${monkeyAdbPath}` : '未就绪：请确认 Android Studio SDK Platform-Tools 已安装' }}。
                当前每 {{ monkeyScreenshotIntervalSec }} 秒截图一次，执行后生成真实截图节点。
              </span>
            </div>
            <div class="monkey-command-preview">
              <span class="monkey-risk-card__label">ADB 命令预览</span>
              <pre>{{ monkeyCommandPreview }}</pre>
              <pre>{{ monkeyScreenshotCommand }}</pre>
            </div>
          </template>

          <!-- 针对短剧类接口测试，展示专属参数设置 -->
          <template v-if="isShortDramaApiTest">
            <div
              v-for="suite in apiTestSuites"
              :key="suite.id"
              class="short-drama-config-panel"
              :class="{
                'short-drama-config-panel--collapsed': !suite.isExpanded,
                'short-drama-config-panel--selected': selectedApiTestSuiteId === suite.id
              }"
            >
              <div class="short-drama-config-panel__header" @click="selectApiTestSuite(suite)">
                <div class="short-drama-test-name">
                  <el-input
                    v-if="suite.isNameEditing"
                    v-model="suite.name"
                    size="small"
                    placeholder="请输入接口测试名称"
                    class="short-drama-test-name-input"
                    @click.stop
                    @blur="finishApiTestSuiteNameEditing(suite)"
                    @keyup.enter="finishApiTestSuiteNameEditing(suite)"
                  />
                  <template v-else>
                    <span class="short-drama-test-name__text">{{ suite.name }}</span>
                    <el-button
                      class="short-drama-test-name__edit"
                      link
                      type="primary"
                      :icon="Edit"
                      @click.stop="startApiTestSuiteNameEditing(suite)"
                    />
                  </template>
                </div>
                <div class="short-drama-config-panel__actions">
                  <el-button v-if="suite.isExpanded" type="primary" size="small" @click.stop="addApiInterface(suite)" class="add-api-btn">
                    新增接口
                  </el-button>
                  <el-button
                    size="small"
                    text
                    bg
                    class="suite-toggle-btn"
                    @click.stop="toggleApiTestSuite(suite)"
                  >
                    {{ suite.isExpanded ? '收起' : '展开' }}
                    <el-icon class="suite-toggle-btn__icon" :class="{ 'suite-toggle-btn__icon--expanded': suite.isExpanded }">
                      <ArrowRight />
                    </el-icon>
                  </el-button>
                </div>
              </div>

              <template v-if="suite.isExpanded">
              <div class="short-drama-settings-grid">
                <label class="short-drama-setting">
                  <span class="short-drama-setting__label">执行环境</span>
                  <el-select v-model="testServer" placeholder="选择服务器" size="small" class="short-drama-setting__control">
                    <el-option
                      v-for="item in serverOptions"
                      :key="item.value"
                      :label="item.label"
                      :value="item.value"
                    />
                  </el-select>
                </label>
                <label class="short-drama-setting">
                  <span class="short-drama-setting__label">项目</span>
                  <el-select v-model="projectName" placeholder="选择项目" size="small" class="short-drama-setting__control">
                    <el-option label="ShortsWave" value="ShortsWave" />
                    <el-option label="NovelNova" value="NovelNova" />
                  </el-select>
                </label>
              </div>

              <!-- 接口卡片列表 -->
              <div class="api-interface-list">
                <div
                  v-for="item in suite.interfaces"
                  :key="item.id"
                  class="api-interface-card"
                  :class="{
                    'api-interface-card--active': expandedInterfaceId === item.id,
                    'api-interface-card--selected': selectedApiInterfaceId === item.id,
                    'card-status--success': item.status === 'success',
                    'card-status--failed': item.status === 'failed',
                    'card-status--running': item.status === 'running'
                  }"
                >
                  <!-- 卡片头部：折叠点击区、方法、名称、快捷操作 -->
                  <div class="api-card-header" @click="selectApiInterface(suite, item)">
                    <div class="api-header-left">
                      <span class="method-badge" :class="item.method">{{ item.method }}</span>
                      <div class="api-name-editor">
                        <el-input
                          v-if="editingInterfaceNameId === item.id"
                          v-model="item.name"
                          size="small"
                          placeholder="请输入接口名称"
                          class="api-name-input"
                          autofocus
                          @click.stop
                          @blur="finishInterfaceNameEditing(item)"
                          @keyup.enter="finishInterfaceNameEditing(item)"
                        />
                        <template v-else>
                          <span class="api-name-text" :title="item.name">{{ item.name }}</span>
                          <el-button
                            class="api-name-edit-btn"
                            link
                            type="primary"
                            :icon="Edit"
                            @click.stop="startInterfaceNameEditing(item.id)"
                          />
                        </template>
                      </div>
                    </div>
                    <div class="api-header-right">
                      <span class="api-status-chip" :class="`api-status-chip--${item.status}`">
                        {{ getInterfaceStatusText(item.status) }}
                      </span>
                      <el-icon class="action-icon delete-icon" title="删除" @click.stop="removeApiInterface(item.id)"><Delete /></el-icon>
                      <el-icon
                        class="arrow-icon"
                        :class="{ 'arrow-rotated': expandedInterfaceId === item.id }"
                        title="展开/收起"
                        @click.stop="toggleInterfaceExpand(item.id)"
                      ><ArrowRight /></el-icon>
                    </div>
                  </div>

                  <!-- 卡片主体：折叠展开编辑区 -->
                  <div v-show="expandedInterfaceId === item.id" class="api-card-body">
                    <div class="form-row method-url-row">
                      <div class="form-item method-col">
                        <label class="inner-label">请求方法</label>
                        <el-select v-model="item.method" size="small" class="method-select">
                          <el-option label="GET" value="GET" />
                          <el-option label="POST" value="POST" />
                          <el-option label="PUT" value="PUT" />
                          <el-option label="DELETE" value="DELETE" />
                          <el-option label="PATCH" value="PATCH" />
                        </el-select>
                      </div>
                      <div class="form-item url-col">
                        <label class="inner-label">请求接口</label>
                        <el-input
                          v-model="item.url"
                          size="small"
                          placeholder="/login/anonymous"
                          @blur="item.url = normalizeInterfacePath(item.url); persistShortDramaApiConfig()"
                        />
                      </div>
                    </div>

                    <div class="form-row">
                      <div class="form-item">
                        <label class="inner-label">请求头</label>
                        <div
                          class="json-command-preview"
                          :class="{ 'json-command-preview--editing': isJsonBlockEditing(item.id, 'headers') }"
                          title="双击修改请求头"
                          @dblclick="startJsonBlockEditing(item.id, 'headers')"
                        >
                          <span class="json-command-preview__label">
                            {{ isJsonBlockEditing(item.id, 'headers') ? '请求头编辑' : '请求头预览' }}
                          </span>
                          <el-input
                            v-if="isJsonBlockEditing(item.id, 'headers')"
                            v-model="item.headers"
                            type="textarea"
                            :rows="getJsonEditorRows(item.headers, 10)"
                            resize="none"
                            wrap="off"
                            size="small"
                            placeholder='{"Content-Type": "application/json", "X-Token": "xxx"}'
                            class="monospace-textarea json-command-editor"
                            autofocus
                            @blur="stopJsonBlockEditing"
                          />
                          <pre v-else class="json-command-preview__lines">
                            <span
                              v-for="(line, lineIndex) in getJsonPreviewLines(item.headers, '未配置请求头')"
                              :key="lineIndex"
                              class="json-command-preview__line"
                            >{{ line || ' ' }}</span>
                          </pre>
                        </div>
                      </div>
                    </div>

                    <div class="form-row" v-if="item.method !== 'GET'">
                      <div class="form-item">
                        <label class="inner-label">请求体</label>
                        <div
                          class="json-command-preview"
                          :class="{ 'json-command-preview--editing': isJsonBlockEditing(item.id, 'body') }"
                          title="双击修改请求体"
                          @dblclick="startJsonBlockEditing(item.id, 'body')"
                        >
                          <span class="json-command-preview__label">
                            {{ isJsonBlockEditing(item.id, 'body') ? '请求体编辑' : '请求体预览' }}
                          </span>
                          <el-input
                            v-if="isJsonBlockEditing(item.id, 'body')"
                            v-model="item.body"
                            type="textarea"
                            :rows="getJsonEditorRows(item.body, 6)"
                            resize="none"
                            wrap="off"
                            size="small"
                            placeholder='{"key": "value"}'
                            class="monospace-textarea json-command-editor"
                            autofocus
                            @blur="stopJsonBlockEditing"
                          />
                          <pre v-else class="json-command-preview__lines">
                            <span
                              v-for="(line, lineIndex) in getJsonPreviewLines(item.body, '未配置请求体')"
                              :key="lineIndex"
                              class="json-command-preview__line"
                            >{{ line || ' ' }}</span>
                          </pre>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
                <div v-if="suite.interfaces.length === 0" class="api-interface-empty">
                  暂无接口，点击右上角新增接口开始配置
                </div>
              </div>
              </template>
            </div>
          </template>

          <!-- 针对业务自检工具，隐藏原本的链接输入框 -->
          <div v-if="!isDramaCheck && !isMonkeyTest && !isShortDramaApiTest" class="param-group">
            <label class="param-label">
              <span class="link-icon">🔗</span> 测试链接
            </label>
            <el-input 
              v-model="projectName" 
              :disabled="!isWebFrontendStressTest" 
              :class="{ 'param-input-disabled': !isWebFrontendStressTest && !isWebFrontendStressTest }" 
            />
          </div>

          <div v-if="!isWebFrontendStressTest && !isMonkeyTest && !isShortDramaApiTest" class="param-row">
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
      <div class="log-panel" :class="{ 'log-panel--short-drama': isShortDramaApiTest }" style="position: relative;">
        <!-- 日志顶栏 -->
        <div class="log-header">
          <div class="log-status-info">
            <span class="dot" :class="{ 'dot-active': visibleStatus === 'Executing', 'dot-ready': visibleStatus !== 'Executing' }"></span>
            <span class="status-text">Status: {{ visibleStatus }}</span>
            <span class="divider">|</span>
            <span class="uptime-text">UPTIME: <span class="time-val">{{ formatTime(visibleUptime) }}</span></span>
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
        <div v-if="monkeyGraphReady" class="monkey-hologram">
          <MonkeyHologram3D
            :nodes="monkeyGraphNodes"
            :edges="monkeyGraphEdges"
            :selected-id="selectedMonkeyNode?.id"
            :target-app="monkeyTarget"
            @select="selectedMonkeyNode = $event"
          />
          <div class="monkey-preview-panel">
            <div class="monkey-preview-panel__header">
              <span>{{ selectedMonkeyNode?.title || '截图预览' }}</span>
              <strong :class="`risk-text risk-text--${selectedMonkeyNode?.risk || 'normal'}`">
                {{ selectedMonkeyNode?.risk || 'normal' }}
              </strong>
            </div>
            <img
              v-if="selectedMonkeyImageUrl"
              :src="selectedMonkeyImageUrl"
              :alt="selectedMonkeyNode?.title || 'Monkey 截图预览'"
              class="monkey-preview-panel__image"
            />
            <div v-else class="monkey-preview-panel__empty">
              {{ selectedMonkeyImageLoading ? '截图加载中...' : '图片已清理或暂不可用' }}
            </div>
            <div v-if="selectedMonkeyNode" class="monkey-preview-panel__meta">
              <span>事件：{{ selectedMonkeyNode.event }}</span>
              <span>页面：{{ selectedMonkeyNode.activity }}</span>
              <span>来源：adb exec-out screencap -p</span>
              <MonkeyNodeSummary :summary="selectedMonkeyNode.summary" :evidence="selectedMonkeyNode.evidence" />
            </div>
          </div>
        </div>
        <div v-else-if="isShortDramaApiTest" class="short-drama-results-container">
          <!-- 自定义 Tab 头部 -->
          <div class="short-drama-tabs-header">
            <button
              class="sd-tab-btn"
              :class="{ active: shortDramaActiveTab === 'pipeline' }"
              @click="shortDramaActiveTab = 'pipeline'"
            >
              <el-icon><Tickets /></el-icon> 链路执行状态
            </button>
            <button
              class="sd-tab-btn"
              :class="{ active: shortDramaActiveTab === 'payload' }"
              @click="shortDramaActiveTab = 'payload'"
            >
              <el-icon><DocumentIcon /></el-icon> 接口报文详情
            </button>
            <button
              class="sd-tab-btn"
              :class="{ active: shortDramaActiveTab === 'latency' }"
              @click="shortDramaActiveTab = 'latency'"
            >
              <el-icon><Odometer /></el-icon> 时延看板
            </button>
            <button
              class="sd-tab-btn"
              :class="{ active: shortDramaActiveTab === 'logs' }"
              @click="shortDramaActiveTab = 'logs'"
            >
              <el-icon><Cpu /></el-icon> 原始日志
            </button>
          </div>

          <!-- Tab 内容区域 -->
          <div class="short-drama-tab-body">
            <!-- 1. Pipeline Tab -->
            <div v-if="shortDramaActiveTab === 'pipeline'" class="pipeline-view">
              <div class="pipeline-intro">
                <h3>短剧测试链路流水线</h3>
                <p>点击每个管道节点可查看详细的 HTTP 请求与响应报文内容。</p>
              </div>
              <div class="pipeline-list">
                <div
                  v-for="(step, idx) in activePipelineSteps"
                  :key="idx"
                  class="pipeline-card"
                  :class="[
                    `pipeline-card--${step.status}`,
                    { active: selectedApiInterfaceId === step.id }
                  ]"
                  @click="selectPipelineStep(idx)"
                >
                  <div class="pipeline-card__indicator">
                    <span v-if="step.status === 'pending'" class="status-dot pending"></span>
                    <span v-else-if="step.status === 'running'" class="status-spinner"></span>
                    <el-icon v-else-if="step.status === 'success'" class="status-icon success" color="#10b981"><Check /></el-icon>
                    <el-icon v-else-if="step.status === 'failed'" class="status-icon failed" color="#ef4444"><Close /></el-icon>
                  </div>
                  <div class="pipeline-card__details">
                    <div class="pipeline-card__title-row">
                      <strong class="step-name">{{ step.name }}</strong>
                      <span v-if="step.latency" class="step-latency">{{ step.latency }}ms</span>
                    </div>
                    <div class="pipeline-card__meta-row">
                      <span class="step-badge" :class="step.method">{{ step.method }}</span>
                      <span class="step-path">{{ getInterfaceDisplayUrl(step) }}</span>
                      <span v-if="step.code" class="step-code" :class="`code-${step.code}`">HTTP {{ step.code }}</span>
                    </div>
                  </div>
                  <div class="pipeline-card__action">
                    <el-icon><ArrowRight /></el-icon>
                  </div>
                </div>
              </div>
            </div>

            <!-- 2. Payload Detail Tab -->
            <div v-else-if="shortDramaActiveTab === 'payload'" class="payload-view">
              <div v-if="selectedStepForPayload" class="payload-details-wrapper">
                <div class="payload-header-bar">
                  <div>
                    <span class="payload-header-bar__eyebrow">Payload Detail</span>
                    <strong>{{ selectedStepForPayload.name }} 报文详情</strong>
                  </div>
                  <span class="payload-badge" :class="selectedStepForPayload.method">
                    {{ selectedStepForPayload.method }} {{ getInterfaceDisplayUrl(selectedStepForPayload) }}
                  </span>
                </div>
                <div class="payload-split-container">
                  <div class="payload-box">
                    <div class="payload-box-title">Request Payload</div>
                    <pre class="json-code-block json-code-block--request">{{ formatJSON(selectedStepForPayload.requestData) }}</pre>
                  </div>
                  <div class="payload-box">
                    <div class="payload-box-title">Response Payload</div>
                    <pre class="json-code-block json-code-block--response">{{ formatJSON(selectedStepForPayload.responseData) }}</pre>
                  </div>
                </div>
              </div>
              <div v-else class="payload-empty-state">
                <el-empty description="请先在链路视图中选择一个步骤，或点击 Execute 运行测试" />
              </div>
            </div>

            <!-- 3. Latency Tab -->
            <div v-else-if="shortDramaActiveTab === 'latency'" class="latency-view">
              <div class="latency-view__header">
                <span class="payload-header-bar__eyebrow">Latency Board</span>
                <h3>链路接口时延看板</h3>
              </div>
              <div class="latency-chart-container">
                <div
                  v-for="(step, idx) in activePipelineSteps"
                  :key="idx"
                  class="latency-row"
                  :class="{ 'latency-row--selected': selectedApiInterfaceId === step.id }"
                  @click="selectPipelineStep(idx)"
                >
                  <div class="latency-row__info">
                    <span class="latency-row__name">{{ step.name }}</span>
                    <span class="latency-row__val" v-if="step.latency">{{ step.latency }} ms</span>
                    <span class="latency-row__val latency-row__val--empty" v-else>未开始</span>
                  </div>
                  <div class="latency-progress-bg">
                    <div
                      class="latency-progress-bar"
                      :class="getLatencyClass(step.latency)"
                      :style="{ width: step.latency ? `${Math.min(100, (step.latency / 800) * 100)}%` : '0%' }"
                    ></div>
                  </div>
                </div>
                <div class="latency-threshold-markers">
                  <span class="marker-ok"><span></span> 健康 (&lt;200ms)</span>
                  <span class="marker-warn"><span></span> 警告 (200-500ms)</span>
                  <span class="marker-err"><span></span> 延迟过高 (&gt;500ms)</span>
                </div>
              </div>
            </div>

            <!-- 4. Logs Tab -->
            <div v-else-if="shortDramaActiveTab === 'logs'" class="log-content-wrapper" style="height: 100%;">
              <div class="log-content" ref="logContainer" style="height: 100%; overflow-y: auto;">
                <div v-for="(log, idx) in visibleLogs" :key="idx" class="log-line">
                  {{ log }}
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-else-if="visibleReportUrl" class="report-container">
          <iframe :src="visibleReportUrl" class="report-iframe" frameborder="0"></iframe>
        </div>
        <div v-else class="log-content" ref="logContainer">
          <div v-for="(log, idx) in visibleLogs" :key="idx" class="log-line">
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
          <span class="stat-value">{{ formatTime(visibleDuration) }}</span>
        </div>
        <div class="stat-item">
          <span class="stat-label">STATUS</span>
          <span class="stat-value capitalize">{{ visibleStatus.toLowerCase() === 'ready' ? 'Idle' : visibleStatus }}</span>
        </div>
      </div>
      <div class="bottom-right">
        <button 
          class="btn-stop" 
          :disabled="visibleStatus !== 'Executing'"
          @click="stopExecution"
        >
          <el-icon><VideoPause /></el-icon>
          Stop
        </button>
        <button 
          class="btn-execute" 
          :disabled="visibleStatus === 'Executing'"
          @click="startExecution"
        >
          <el-icon><VideoPlay /></el-icon>
          Execute
        </button>
      </div>
    </div>

    <el-dialog
      v-model="monkeyWirelessDialogVisible"
      title="无线连接 Android 设备"
      width="520px"
      append-to-body
    >
      <div class="monkey-wireless-dialog">
        <section :class="['monkey-wireless-step', { 'is-completed': monkeyWirelessPaired }]">
          <div class="monkey-wireless-step__header">
            <strong>Step 1 · 配对设备</strong>
            <span v-if="monkeyWirelessPaired" class="monkey-wireless-step__completed">已完成</span>
          </div>
          <p>在手机中打开“开发者选项 > 无线调试 > 使用配对码配对设备”，填写手机显示的配对地址和 6 位配对码。</p>
          <el-input v-model="monkeyWirelessPairAddress" placeholder="配对地址，例如 192.168.1.86:37091" />
          <el-input v-model="monkeyWirelessPairingCode" maxlength="6" placeholder="6 位配对码" show-word-limit />
          <el-button
            v-if="permissionStore.canAccess('dashboard.monkey_test.visible')"
            type="primary"
            :loading="monkeyWirelessSubmitting"
            @click="pairMonkeyWirelessADB"
          >
            配对
          </el-button>
        </section>
        <section :class="['monkey-wireless-step', { 'is-active': monkeyWirelessPaired }]">
          <div class="monkey-wireless-step__header">
            <strong>Step 2 · 连接设备</strong>
            <span v-if="monkeyWirelessPaired" class="monkey-wireless-step__current">请继续连接</span>
          </div>
          <p>配对成功后，返回手机无线调试页面，填写页面顶部显示的连接地址。连接端口通常与配对端口不同。</p>
          <p v-if="monkeyWirelessPaired" class="monkey-wireless-step__guide">配对已完成。现在请填写手机无线调试页面顶部的连接地址。</p>
          <el-input v-model="monkeyWirelessConnectAddress" placeholder="连接地址，例如 192.168.1.86:39147" />
          <el-button
            v-if="permissionStore.canAccess('dashboard.monkey_test.visible')"
            type="primary"
            :loading="monkeyWirelessSubmitting"
            @click="connectMonkeyWirelessADB"
          >
            连接并刷新设备
          </el-button>
        </section>
      </div>
    </el-dialog>
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

.sidebar-panel--short-drama {
  width: clamp(430px, 28vw, 520px);
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

.badge-ready--short-drama {
  font-size: 16.5px;
  font-weight: 800;
  padding: 6px 14px;
  border-radius: 16px;
  letter-spacing: 0;
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

.add-api-test-btn {
  height: 28px;
  margin-left: auto;
  padding: 0 10px;
  border-radius: 7px;
  font-size: 12px;
  font-weight: 800;
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
  min-width: 0;
}

.param-label {
  font-size: 12.5px;
  font-weight: 600;
  color: #1e293b;
  display: flex;
  align-items: center;
  gap: 4px;
}

.param-help {
  color: #94a3b8;
  font-size: 11px;
  line-height: 1.45;
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

.param-number {
  width: 100%;
}

.monkey-device-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
}

.monkey-wireless-actions {
  display: flex;
  gap: 8px;
}

.monkey-mode-switch {
  width: 100%;
  height: 32px;
  margin: 0;
  justify-content: center;
}

.monkey-wireless-dialog {
  display: grid;
  gap: 20px;
}

.monkey-wireless-step {
  display: grid;
  gap: 10px;
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.monkey-wireless-step.is-completed {
  border-color: #86efac;
  background: #f0fdf4;
}

.monkey-wireless-step.is-active {
  border-color: #60a5fa;
  background: #eff6ff;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}

.monkey-wireless-step__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.monkey-wireless-step__completed,
.monkey-wireless-step__current {
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
}

.monkey-wireless-step__completed {
  color: #15803d;
  background: #dcfce7;
}

.monkey-wireless-step__current {
  color: #1d4ed8;
  background: #dbeafe;
}

.monkey-wireless-step p {
  margin: 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.6;
}

.monkey-wireless-step .monkey-wireless-step__guide {
  color: #1d4ed8;
  font-weight: 600;
}

.monkey-option-checkbox {
  width: 100%;
  height: auto;
  align-items: flex-start;
  white-space: normal;
}

.monkey-option-checkbox :deep(.el-checkbox__input) {
  margin-top: 3px;
}

.monkey-option-checkbox :deep(.el-checkbox__label) {
  min-width: 0;
  padding-left: 8px;
  white-space: normal;
  overflow-wrap: anywhere;
  line-height: 1.55;
}

.monkey-risk-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  padding: 12px;
  border-radius: 10px;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
  color: #166534;
  font-size: 12.5px;
  line-height: 1.5;
}

.monkey-risk-card__description {
  min-width: 0;
  max-width: 100%;
  white-space: normal;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.monkey-risk-card__label {
  color: #16a34a;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0;
}

.monkey-command-preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  border-radius: 10px;
  background: #0f172a;
  border: 1px solid #1e293b;
  color: #d1fae5;
}

.monkey-command-preview pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 11px;
  line-height: 1.5;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
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

.log-panel--short-drama {
  min-width: 0;
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
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

/* ==================== Monkey 截图全息图谱 ==================== */
.monkey-hologram {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 16px;
  padding: 16px;
  background:
    radial-gradient(circle at 50% 46%, rgba(45, 212, 191, 0.24), transparent 32%),
    radial-gradient(circle at 72% 18%, rgba(99, 102, 241, 0.2), transparent 28%),
    radial-gradient(circle at 18% 80%, rgba(14, 165, 233, 0.18), transparent 30%),
    linear-gradient(135deg, #020617 0%, #07111f 48%, #0f172a 100%);
  overflow: hidden;
}

.monkey-hologram__stage-wrap {
  position: relative;
  min-width: 0;
  min-height: 0;
  perspective: 1100px;
  border-radius: 22px;
}

.monkey-hologram__stage {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 0;
  border: 1px solid rgba(103, 232, 249, 0.22);
  border-radius: 22px;
  overflow: hidden;
  background:
    linear-gradient(rgba(125, 211, 252, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(125, 211, 252, 0.055) 1px, transparent 1px),
    radial-gradient(circle at 50% 50%, rgba(34, 211, 238, 0.1), transparent 42%);
  background-size: 30px 30px, 30px 30px, 100% 100%;
  transform: rotateX(8deg) rotateY(-8deg);
  transform-style: preserve-3d;
  box-shadow:
    inset 0 0 62px rgba(34, 211, 238, 0.12),
    inset 0 0 140px rgba(15, 23, 42, 0.62),
    0 28px 70px rgba(0, 0, 0, 0.35);
}

.monkey-hologram__stage::before {
  content: "";
  position: absolute;
  inset: 10%;
  border-radius: 50%;
  border: 1px solid rgba(125, 211, 252, 0.2);
  transform: translateZ(-70px) rotateX(68deg);
  box-shadow: 0 0 58px rgba(34, 211, 238, 0.18);
}

.monkey-hologram__stage::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(110deg, transparent 0%, rgba(255, 255, 255, 0.12) 44%, transparent 52%),
    radial-gradient(circle at 50% 52%, transparent 0 36%, rgba(34, 211, 238, 0.08) 37%, transparent 48%);
  mix-blend-mode: screen;
}

.monkey-hologram__aura {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 44%;
  aspect-ratio: 1;
  border-radius: 50%;
  transform: translate(-50%, -50%) translateZ(-40px);
  background: radial-gradient(circle, rgba(45, 212, 191, 0.34), rgba(14, 165, 233, 0.08) 46%, transparent 68%);
  filter: blur(4px);
}

.monkey-orbit {
  position: absolute;
  left: 50%;
  top: 50%;
  width: 62%;
  aspect-ratio: 1;
  border-radius: 50%;
  border: 1px solid rgba(125, 211, 252, 0.32);
  transform-style: preserve-3d;
  box-shadow: 0 0 24px rgba(34, 211, 238, 0.13);
  pointer-events: none;
}

.monkey-orbit--one {
  transform: translate(-50%, -50%) rotateX(66deg) rotateZ(12deg) translateZ(-26px);
}

.monkey-orbit--two {
  width: 76%;
  border-color: rgba(167, 139, 250, 0.28);
  transform: translate(-50%, -50%) rotateX(58deg) rotateY(44deg) rotateZ(-20deg) translateZ(-44px);
}

.monkey-orbit--three {
  width: 48%;
  border-color: rgba(52, 211, 153, 0.3);
  transform: translate(-50%, -50%) rotateX(72deg) rotateY(-42deg) rotateZ(48deg) translateZ(24px);
}

.monkey-hologram__edges {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.monkey-hologram__edge {
  stroke: rgba(103, 232, 249, 0.5);
  stroke-width: 0.28;
  filter: drop-shadow(0 0 5px rgba(34, 211, 238, 0.72));
}

.monkey-node {
  position: absolute;
  width: auto;
  height: auto;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  transform: translate(-50%, -50%);
  transform-style: preserve-3d;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  color: #c4f1ff;
  animation: monkey-node-float 4.8s ease-in-out infinite;
  animation-delay: var(--node-delay);
}

.monkey-node:hover {
  transform: translate(-50%, -50%) translateZ(calc(var(--node-depth) + 30px)) scale(1.12);
}

.monkey-node__core {
  position: relative;
  z-index: 2;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background:
    radial-gradient(circle at 32% 28%, #ffffff 0 8%, #86efac 18%, #22c55e 58%, #064e3b 100%);
  box-shadow:
    0 0 0 8px rgba(34, 197, 94, 0.12),
    0 0 22px rgba(34, 197, 94, 0.9),
    0 12px 26px rgba(0, 0, 0, 0.32);
}

.monkey-node__halo {
  position: absolute;
  top: 8px;
  width: 46px;
  height: 18px;
  border-radius: 50%;
  border: 1px solid rgba(187, 247, 208, 0.52);
  transform: rotateX(72deg) translateZ(-10px);
  filter: drop-shadow(0 0 8px rgba(34, 197, 94, 0.5));
  pointer-events: none;
}

.monkey-node--warning .monkey-node__core {
  background:
    radial-gradient(circle at 32% 28%, #ffffff 0 8%, #fde68a 18%, #f59e0b 58%, #78350f 100%);
  box-shadow: 0 0 0 8px rgba(245, 158, 11, 0.14), 0 0 32px rgba(245, 158, 11, 0.9), 0 12px 26px rgba(0, 0, 0, 0.32);
}

.monkey-node--warning .monkey-node__halo {
  border-color: rgba(253, 230, 138, 0.58);
  filter: drop-shadow(0 0 8px rgba(245, 158, 11, 0.62));
}

.monkey-node--critical .monkey-node__core {
  background:
    radial-gradient(circle at 32% 28%, #ffffff 0 8%, #fecaca 18%, #ef4444 58%, #7f1d1d 100%);
  box-shadow: 0 0 0 9px rgba(239, 68, 68, 0.16), 0 0 38px rgba(239, 68, 68, 0.98), 0 12px 26px rgba(0, 0, 0, 0.34);
}

.monkey-node--critical .monkey-node__halo {
  border-color: rgba(252, 165, 165, 0.62);
  filter: drop-shadow(0 0 9px rgba(239, 68, 68, 0.68));
}

.monkey-node--active .monkey-node__core {
  outline: 2px solid rgba(255, 255, 255, 0.86);
  outline-offset: 5px;
}

.monkey-node__label {
  position: relative;
  z-index: 3;
  padding: 3px 8px;
  border-radius: 999px;
  background: rgba(2, 6, 23, 0.62);
  border: 1px solid rgba(125, 211, 252, 0.24);
  backdrop-filter: blur(10px);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0;
}

@keyframes monkey-node-float {
  0%, 100% {
    transform: translate(-50%, -50%) translateZ(var(--node-depth));
  }
  50% {
    transform: translate(-50%, calc(-50% - 8px)) translateZ(calc(var(--node-depth) + 18px));
  }
}

.monkey-preview-panel {
  min-width: 0;
  min-height: 0;
  max-height: 100%;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-sizing: border-box;
  padding: 16px;
  overflow-y: auto;
  overscroll-behavior: contain;
  scrollbar-gutter: stable;
  border-radius: 22px;
  background:
    linear-gradient(180deg, rgba(15, 23, 42, 0.76), rgba(2, 6, 23, 0.7));
  border: 1px solid rgba(125, 211, 252, 0.26);
  color: #e2e8f0;
  backdrop-filter: blur(18px);
  box-shadow:
    inset 0 0 22px rgba(14, 165, 233, 0.08),
    0 24px 55px rgba(0, 0, 0, 0.28);
}

.monkey-preview-panel__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  font-weight: 800;
}

.risk-text {
  text-transform: uppercase;
  font-size: 11px;
}

.risk-text--normal { color: #86efac; }
.risk-text--warning { color: #fcd34d; }
.risk-text--critical { color: #fca5a5; }
.risk-text--unknown { color: #cbd5e1; }

.monkey-preview-panel__image {
  width: 100%;
  min-height: 0;
  flex-shrink: 0;
  max-height: 48vh;
  border-radius: 18px;
  object-fit: contain;
  border: 1px solid rgba(148, 163, 184, 0.2);
  box-shadow: 0 18px 42px rgba(0, 0, 0, 0.42);
}

.monkey-preview-panel__empty {
  display: grid;
  min-height: 180px;
  place-items: center;
  border: 1px dashed rgba(148, 163, 184, 0.32);
  border-radius: 18px;
  color: #94a3b8;
  background: rgba(15, 23, 42, 0.42);
}

.monkey-preview-panel__meta {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: #94a3b8;
  font-size: 12px;
  line-height: 1.5;
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

/* ==================== 短剧类接口测试专属样式 ==================== */
.short-drama-config-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #f8fafc;
}

.short-drama-config-panel--collapsed {
  gap: 0;
  background: #ffffff;
}

.short-drama-config-panel--selected {
  border-color: #3b82f6;
  box-shadow: 0 10px 24px rgba(59, 130, 246, 0.1);
}

.short-drama-config-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  cursor: pointer;
}

.short-drama-config-panel__actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  flex-shrink: 0;
}

.short-drama-config-panel__eyebrow,
.payload-header-bar__eyebrow {
  display: block;
  margin-bottom: 4px;
  color: #64748b;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.short-drama-config-panel__title {
  margin: 0;
  color: #0f172a;
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 0;
}

.short-drama-test-name {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  gap: 6px;
}

.short-drama-test-name__text {
  overflow: hidden;
  max-width: 280px;
  color: #0f172a;
  font-size: 16px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.short-drama-test-name__edit {
  width: 22px;
  height: 22px;
  padding: 0;
}

.short-drama-test-name-input {
  width: 240px;
}

.short-drama-test-name-input :deep(.el-input__wrapper) {
  padding: 0 8px;
  border-radius: 8px;
  box-shadow: 0 0 0 1px #e2e8f0 inset;
}

.short-drama-test-name-input :deep(.el-input__inner) {
  color: #0f172a;
  font-size: 16px;
  font-weight: 800;
}

.add-api-btn {
  height: 22px;
  min-width: auto;
  padding: 0 8px;
  flex-shrink: 0;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
}

.suite-toggle-btn {
  height: 22px;
  min-width: auto;
  padding: 0 8px;
  border-radius: 6px;
  color: #64748b;
  font-size: 11px;
  font-weight: 800;
}

.suite-toggle-btn__icon {
  margin-left: 3px;
  transition: transform 0.2s ease;
}

.suite-toggle-btn__icon--expanded {
  transform: rotate(90deg);
}

.short-drama-settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.short-drama-setting {
  min-width: 0;
  padding: 10px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
}

.short-drama-setting__label {
  display: block;
  margin-bottom: 7px;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.short-drama-setting__control {
  width: 100%;
}

.api-interface-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.api-interface-empty {
  padding: 18px 12px;
  border: 1px dashed #cbd5e1;
  border-radius: 10px;
  background: #ffffff;
  color: #94a3b8;
  font-size: 12px;
  font-weight: 700;
  text-align: center;
}

.api-interface-card {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #ffffff;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.api-interface-card:hover {
  border-color: #cbd5e1;
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.05);
}

.api-interface-card--active {
  border-color: #93c5fd;
  box-shadow: 0 10px 24px rgba(59, 130, 246, 0.08);
}

.api-interface-card--selected {
  border-color: #2563eb;
  box-shadow: 0 10px 24px rgba(37, 99, 235, 0.12);
}

.api-interface-card--selected .api-card-header {
  background: #eff6ff;
}

.api-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 58px;
  padding: 12px 14px;
  cursor: pointer;
}

.api-header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.api-name-editor {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  gap: 6px;
}

.api-name-text {
  overflow: hidden;
  max-width: 220px;
  color: #0f172a;
  font-size: 14px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.api-name-edit-btn {
  width: 20px;
  height: 20px;
  padding: 0;
}

.api-name-input {
  width: 210px;
}

.api-name-input :deep(.el-input__wrapper) {
  padding: 0 8px;
  border-radius: 8px;
  box-shadow: 0 0 0 1px #e2e8f0 inset;
}

.api-name-input :deep(.el-input__inner) {
  color: #0f172a;
  font-size: 14px;
  font-weight: 800;
}

.api-header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.method-badge,
.step-badge,
.payload-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 46px;
  height: 22px;
  padding: 0 8px;
  border-radius: 6px;
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 11px;
  font-weight: 800;
}

.method-badge.GET,
.step-badge.GET,
.payload-badge.GET {
  background: #e0f2fe;
  color: #0369a1;
}

.method-badge.POST,
.step-badge.POST,
.payload-badge.POST {
  background: #dcfce7;
  color: #15803d;
}

.method-badge.PUT,
.step-badge.PUT,
.payload-badge.PUT,
.method-badge.PATCH,
.step-badge.PATCH,
.payload-badge.PATCH {
  background: #fef3c7;
  color: #92400e;
}

.method-badge.DELETE,
.step-badge.DELETE,
.payload-badge.DELETE {
  background: #fee2e2;
  color: #b91c1c;
}

.api-status-chip {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 800;
}

.api-status-chip--pending {
  background: #f1f5f9;
  color: #64748b;
}

.api-status-chip--running {
  background: #dbeafe;
  color: #1d4ed8;
}

.api-status-chip--success {
  background: #dcfce7;
  color: #15803d;
}

.api-status-chip--failed {
  background: #fee2e2;
  color: #b91c1c;
}

.action-icon,
.arrow-icon {
  color: #94a3b8;
  cursor: pointer;
  transition: color 0.2s ease, transform 0.2s ease;
}

.action-icon:hover {
  color: #2563eb;
}

.delete-icon:hover {
  color: #dc2626;
}

.arrow-icon {
  font-size: 14px;
}

.arrow-rotated {
  transform: rotate(90deg);
}

.api-card-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 0 14px 14px;
  border-top: 1px solid #f1f5f9;
}

.form-row {
  display: flex;
  gap: 10px;
}

.form-item {
  min-width: 0;
  flex: 1;
}

.method-url-row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 10px;
  align-items: flex-start;
}

.method-col {
  min-width: 0;
}

.url-col {
  min-width: 0;
}

.inner-label {
  display: block;
  margin-bottom: 6px;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.method-select {
  width: 100%;
}

.monospace-textarea :deep(.el-textarea__inner) {
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.5;
}

.json-command-preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  padding: 12px;
  border: 1px solid #1e293b;
  border-radius: 10px;
  background: #0f172a;
  color: #d1fae5;
  cursor: text;
}

.json-command-preview--editing {
  border-color: #2563eb;
}

.json-command-preview:hover {
  border-color: #334155;
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.08);
}

.json-command-preview__label {
  color: #16a34a;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0;
}

.json-command-preview__lines {
  min-width: 0;
  margin: 0;
  color: inherit;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 11px;
  line-height: 1.5;
}

.json-command-preview__line {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: pre;
}

.json-command-editor {
  min-width: 0;
}

.json-command-editor :deep(.el-textarea__inner) {
  min-width: 100%;
  padding: 0 0 8px;
  border: none;
  background: transparent;
  box-shadow: none;
  color: #d1fae5;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 11px;
  line-height: 1.5;
  overflow-x: auto;
  overflow-y: hidden;
  white-space: pre;
  word-break: normal;
}

.json-command-editor :deep(.el-textarea__inner:focus) {
  box-shadow: none;
}

.short-drama-results-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}

.short-drama-tabs-header {
  display: flex;
  background: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
  padding: 8px 16px;
  gap: 8px;
  flex-shrink: 0;
}

.sd-tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding: 0 16px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  color: #64748b;
  background: transparent;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
}

.sd-tab-btn:hover {
  background: #f1f5f9;
  color: #0f172a;
}

.sd-tab-btn.active {
  background: #3b82f6;
  color: #ffffff;
  box-shadow: 0 2px 4px rgba(59, 130, 246, 0.2);
}

.short-drama-tab-body {
  flex: 1;
  overflow-y: auto;
  padding: 18px;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.pipeline-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.pipeline-intro h3 {
  margin: 0 0 4px 0;
  font-size: 16px;
  color: #0f172a;
}

.pipeline-intro p {
  margin: 0;
  font-size: 13px;
  color: #64748b;
}

.pipeline-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.pipeline-card {
  display: flex;
  align-items: center;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 14px 16px;
  cursor: pointer;
  transition: all 0.2s;
  background: #ffffff;
}

.pipeline-card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
  border-color: #cbd5e1;
}

.pipeline-card.active {
  border-color: #3b82f6;
  background: #f8fafc;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.06);
}

.pipeline-card--pending {
  border-left: 4px solid #cbd5e1;
}

.pipeline-card--running {
  border-left: 4px solid #3b82f6;
}

.pipeline-card--success {
  border-left: 4px solid #10b981;
}

.pipeline-card--failed {
  border-left: 4px solid #ef4444;
}

.pipeline-card__indicator {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  margin-right: 14px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.status-dot.pending {
  background: #94a3b8;
}

.status-spinner {
  width: 16px;
  height: 16px;
  border: 2px solid #e2e8f0;
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.status-icon {
  font-size: 18px;
}

.pipeline-card__details {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.pipeline-card__title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.step-name {
  font-size: 14px;
  color: #1e293b;
  font-weight: 700;
}

.step-latency {
  font-size: 12px;
  color: #94a3b8;
  font-weight: 500;
}

.pipeline-card__meta-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.step-badge {
  min-width: 42px;
  height: 20px;
  font-size: 10px;
}

.step-path {
  overflow: hidden;
  max-width: 420px;
  color: #64748b;
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.step-code {
  font-weight: 600;
}

.step-code.code-200 {
  color: #10b981;
}

.step-code:not(.code-200) {
  color: #ef4444;
}

.pipeline-card__action {
  color: #94a3b8;
  margin-left: 8px;
  display: flex;
  align-items: center;
}

.latency-progress-bar.latency-ok {
  background: #10b981;
}

.latency-progress-bar.latency-warn {
  background: #f59e0b;
}

.latency-progress-bar.latency-error {
  background: #ef4444;
}

.payload-view,
.payload-details-wrapper,
.latency-view {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
}

.payload-details-wrapper {
  gap: 12px;
}

.payload-header-bar,
.latency-view__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.payload-header-bar strong,
.latency-view__header h3 {
  display: block;
  margin: 0;
  color: #0f172a;
  font-size: 15px;
  font-weight: 800;
}

.payload-badge {
  overflow: hidden;
  max-width: 360px;
  justify-content: flex-start;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.payload-split-container {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 14px;
  flex: 1;
  min-height: 0;
}

.payload-box {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  gap: 8px;
}

.payload-box-title {
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.json-code-block {
  flex: 1;
  min-height: 0;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border: 1px solid #1e293b;
  border-radius: 8px;
  background: #0f172a;
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.json-code-block--request {
  color: #38bdf8;
}

.json-code-block--response {
  color: #34d399;
}

.payload-empty-state {
  padding: 40px;
  text-align: center;
}

.latency-view {
  gap: 14px;
}

.latency-chart-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #ffffff;
}

.latency-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
  border-radius: 8px;
  cursor: pointer;
}

.latency-row--selected {
  background: #eff6ff;
}

.latency-row__info {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  font-weight: 700;
}

.latency-row__name {
  overflow: hidden;
  color: #475569;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.latency-row__val {
  flex-shrink: 0;
  color: #2563eb;
}

.latency-row__val--empty {
  color: #94a3b8;
}

.latency-progress-bg {
  height: 8px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  background: #f1f5f9;
}

.latency-progress-bar {
  height: 100%;
  border-radius: 999px;
  transition: width 0.3s ease;
}

.latency-threshold-markers {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-top: 2px;
  padding-top: 10px;
  border-top: 1px dashed #e2e8f0;
  color: #94a3b8;
  font-size: 11px;
}

.latency-threshold-markers span {
  display: flex;
  align-items: center;
  gap: 5px;
}

.latency-threshold-markers span span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.marker-ok span {
  background: #10b981;
}

.marker-warn span {
  background: #f59e0b;
}

.marker-err span {
  background: #ef4444;
}

@media (max-width: 1280px) {
  .short-drama-settings-grid,
  .payload-split-container {
    grid-template-columns: 1fr;
  }
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
