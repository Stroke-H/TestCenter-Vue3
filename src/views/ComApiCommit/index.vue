<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { onBeforeRouteLeave, useRoute, useRouter } from 'vue-router'
import { useDramaRunStore, usePermissionStore, useReportStore, useSubtitleRunStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import { buildBackendUrl, buildBackendWsUrl, normalizeBackendUrl } from '@/utils/runtimeUrl'
import MonkeyHologram3D from './components/MonkeyHologram3D.vue'
import MonkeyNodeSummary from './components/MonkeyNodeSummary.vue'
import DramaRulesDialog from './components/DramaRulesDialog.vue'
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
  ArrowDown,
  ArrowRight
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const reportStore = useReportStore()
const authStore = useAuthStore()
const dramaRunStore = useDramaRunStore()
const subtitleRunStore = useSubtitleRunStore()
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
const showDramaRulesEntry = toolTypeName === '剧集播放接口测试'
const dramaRulesVisible = ref(false)
const isSubtitleCheck = toolTypeName === '剧集外挂字幕测试' || toolTypeName.includes('外挂字幕')
// 是否是Web前端压测
const isWebFrontendStressTest = toolTypeName === 'WebFrontend性能' || toolTypeName === 'Web前端压测'
// 是否是 Monkey 稳定性测试 demo
const isMonkeyTest = toolTypeName === 'Monkey测试'
// 是否是删除账号工具
const isDeleteAccount = toolTypeName === '删除账号'
const deleteAccountActiveTab = ref<'status' | 'payload' | 'logs'>('status')

// ===== 短剧类接口测试专属状态 =====
const isShortDramaApiTest = computed(() => toolTypeName === '短剧类接口测试')
const shortDramaActiveTab = ref('pipeline')
// ===== 通用接口测试：类型定义与状态管理 =====
type ApiVariableSource = 'body' | 'header' | 'cookie' | 'text'

interface ApiVariableExtractor {
  id: number
  name: string
  source: ApiVariableSource
  path: string
  required: boolean
  sensitive: boolean
}

interface ApiRuntimeVariable {
  name: string
  value: unknown
  sourceCaseId: number
  sourceCaseName: string
  sensitive: boolean
}

interface ApiExtractedVariable {
  name: string
  displayValue: string
  source: ApiVariableSource
  path: string
  sensitive: boolean
}

interface ApiProxyEnvelope {
  status: number
  headers: Record<string, string[]>
  cookies: Record<string, string>
  body: unknown
}

// ApiInterface 描述一个可测试的 HTTP 接口条目
interface ApiInterface {
  id: number          // 唯一标识
  name: string        // 接口名称（用户自定义）
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'  // HTTP 方法
  url: string         // 请求接口路径
  headers: string     // JSON 格式的请求头
  body: string        // JSON 格式的请求体
  extractors: ApiVariableExtractor[]
  status: 'pending' | 'running' | 'success' | 'failed' | 'skipped'  // 执行状态
  code?: number       // HTTP 响应状态码
  latency?: number    // 响应耗时（毫秒）
  errorMsg?: string   // 错误信息
  requestData?: any   // 发送的请求数据（用于 Payload 展示）
  responseData?: any  // 接收的响应数据（用于 Payload 展示）
  extractedVariables?: ApiExtractedVariable[]
}

interface ApiTestSuite {
  id: number
  name: string
  interfaces: ApiInterface[]
  isExpanded: boolean
  isNameEditing: boolean
}

type ApiEnvironment = 'test' | 'prod' | 'gray'
type DeleteAccountEnvironment = ApiEnvironment | 'custom'
type ApiJsonField = 'headers' | 'body'

const SHORT_DRAMA_API_CONFIG_STORAGE_KEY = 'testcenter.shortDramaApi.interfaces.v1'
const DELETE_ACCOUNT_API_CONFIG_STORAGE_KEY = 'testcenter.deleteAccountApi.singleCase.v1'

// 自增 ID 计数器，确保每个接口条目有唯一标识
let apiInterfaceIdCounter = 1
let apiTestSuiteIdCounter = 1
let apiVariableExtractorIdCounter = 1

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
  extractors: [],
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
  extractors: [{
    id: apiVariableExtractorIdCounter++,
    name: 'session_token',
    source: 'body',
    path: '$.data.session_token',
    required: true,
    sensitive: true
  }],
  status: 'pending'
})

const createDeleteAccountLoginCase = (): ApiInterface => ({
  id: -1,
  name: '匿名登录',
  method: 'POST',
  url: '/login/anonymous',
  headers: JSON.stringify(shortDramaAnonymousLoginHeaders, null, 2),
  body: '{}',
  extractors: [],
  status: 'pending'
})

const createDeleteAccountDeleteCase = (): ApiInterface => ({
  id: -2,
  name: '删除账号',
  method: 'GET',
  url: '/user/delete',
  headers: JSON.stringify({
    ...shortDramaAnonymousLoginHeaders,
    'X-SESSION-TOKEN': '{{session_token}}'
  }, null, 2),
  body: '',
  extractors: [],
  status: 'pending'
})

const createApiTestSuite = (name: string, interfaces: ApiInterface[], isNameEditing = false, isExpanded = true): ApiTestSuite => ({
  id: apiTestSuiteIdCounter++,
  name,
  interfaces,
  isExpanded,
  isNameEditing
})

const isSensitiveVariableName = (name: string) => /(token|secret|password|cookie|authorization|session|key)/i.test(name)

const normalizeApiVariableExtractors = (value: unknown): ApiVariableExtractor[] => {
  if (!Array.isArray(value)) return []
  const supportedSources: ApiVariableSource[] = ['body', 'header', 'cookie', 'text']
  return value
    .filter(item => item && typeof item === 'object')
    .map((item: Partial<ApiVariableExtractor>) => {
      const id = typeof item.id === 'number' ? item.id : apiVariableExtractorIdCounter++
      apiVariableExtractorIdCounter = Math.max(apiVariableExtractorIdCounter, id + 1)
      const name = typeof item.name === 'string' ? item.name.trim() : ''
      return {
        id,
        name,
        source: supportedSources.includes(item.source as ApiVariableSource) ? item.source as ApiVariableSource : 'body',
        path: typeof item.path === 'string' ? item.path.trim() : '',
        required: item.required !== false,
        sensitive: typeof item.sensitive === 'boolean' ? item.sensitive : isSensitiveVariableName(name)
      }
    })
}

const createApiVariableExtractor = (): ApiVariableExtractor => ({
  id: apiVariableExtractorIdCounter++,
  name: '',
  source: 'body',
  path: '$.data.session_token',
  required: true,
  sensitive: true
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

const normalizeDeleteAccountCustomDomain = (value: string) => {
  const rawValue = value.trim()
  if (!rawValue) throw new Error('请输入自定义域名')
  const candidate = /^[a-z][a-z\d+.-]*:\/\//i.test(rawValue) ? rawValue : `https://${rawValue}`
  const parsed = new URL(candidate)
  if (!['http:', 'https:'].includes(parsed.protocol)) {
    throw new Error('自定义域名仅支持 HTTP 或 HTTPS')
  }
  if (!parsed.hostname) throw new Error('自定义域名格式不正确')
  const basePath = parsed.pathname.replace(/\/+$/, '')
  return `${parsed.protocol}//${parsed.host}${basePath === '/' ? '' : basePath}`
}

const finishDeleteAccountCustomDomainEditing = () => {
  if (!deleteAccountCustomDomain.value.trim()) return
  try {
    deleteAccountCustomDomain.value = normalizeDeleteAccountCustomDomain(deleteAccountCustomDomain.value)
    persistDeleteAccountConfig()
  } catch (error: any) {
    ElMessage.warning(error.message || '自定义域名格式不正确')
  }
}

const getSavedApiVariableExtractors = (item: Partial<ApiInterface>) => {
  if (Array.isArray(item.extractors)) return normalizeApiVariableExtractors(item.extractors)
  if (normalizeInterfacePath(item.url || '') === '/login/anonymous') {
    return [{
      id: apiVariableExtractorIdCounter++,
      name: 'session_token',
      source: 'body' as ApiVariableSource,
      path: '$.data.session_token',
      required: true,
      sensitive: true
    }]
  }
  return []
}

const resetApiRuntimeState = (item: ApiInterface): ApiInterface => ({
  ...item,
  url: normalizeInterfacePath(item.url),
  status: 'pending',
  code: undefined,
  latency: undefined,
  errorMsg: undefined,
  requestData: undefined,
  responseData: undefined,
  extractedVariables: undefined
})

interface DeleteAccountSavedConfig {
  environment?: DeleteAccountEnvironment
  customDomain?: string
  project?: string
  cases?: Partial<ApiInterface>[]
  // 兼容上一版只有一张复合卡片的本地配置。
  case?: Partial<ApiInterface>
}

const loadSavedDeleteAccountConfig = (): DeleteAccountSavedConfig | null => {
  if (typeof window === 'undefined') return null
  try {
    const rawData = window.localStorage.getItem(DELETE_ACCOUNT_API_CONFIG_STORAGE_KEY)
    if (!rawData) return null
    const parsed = JSON.parse(rawData)
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch {
    return null
  }
}

const savedDeleteAccountConfig = loadSavedDeleteAccountConfig()
const deleteAccountEnvironment = ref<DeleteAccountEnvironment>(
  ['test', 'prod', 'gray', 'custom'].includes(savedDeleteAccountConfig?.environment || '')
    ? savedDeleteAccountConfig!.environment as DeleteAccountEnvironment
    : 'test'
)
const deleteAccountCustomDomain = ref(savedDeleteAccountConfig?.customDomain || '')
if (isDeleteAccount && ['ShortsWave', 'NovelNova'].includes(savedDeleteAccountConfig?.project || '')) {
  projectName.value = savedDeleteAccountConfig!.project!
}

const savedDeleteAccountCases = Array.isArray(savedDeleteAccountConfig?.cases)
  ? savedDeleteAccountConfig!.cases!
  : []
const savedDeleteAccountLoginCase = savedDeleteAccountCases[0] || savedDeleteAccountConfig?.case || {}
const savedDeleteAccountDeleteCase = savedDeleteAccountCases[1] || {}

const deleteAccountCases = ref<ApiInterface[]>([
  resetApiRuntimeState({
    ...createDeleteAccountLoginCase(),
    ...savedDeleteAccountLoginCase,
    id: -1,
    name: savedDeleteAccountLoginCase.name?.trim() || '匿名登录',
    method: savedDeleteAccountLoginCase.method || 'POST',
    url: normalizeInterfacePath(savedDeleteAccountLoginCase.url || '/login/anonymous'),
    headers: savedDeleteAccountLoginCase.headers || JSON.stringify(shortDramaAnonymousLoginHeaders, null, 2),
    body: savedDeleteAccountLoginCase.body ?? '{}',
    extractors: []
  }),
  resetApiRuntimeState({
    ...createDeleteAccountDeleteCase(),
    ...savedDeleteAccountDeleteCase,
    id: -2,
    name: savedDeleteAccountDeleteCase.name?.trim() || '删除账号',
    method: savedDeleteAccountDeleteCase.method || 'GET',
    url: normalizeInterfacePath(savedDeleteAccountDeleteCase.url || '/user/delete'),
    headers: savedDeleteAccountDeleteCase.headers || createDeleteAccountDeleteCase().headers,
    body: savedDeleteAccountDeleteCase.body ?? '',
    extractors: []
  })
])
const deleteAccountLoginCase = computed<ApiInterface>(() => deleteAccountCases.value[0]!)
const deleteAccountDeleteCase = computed<ApiInterface>(() => deleteAccountCases.value[1]!)
const deleteAccountSelectedCaseId = ref(-1)
const selectedDeleteAccountCase = computed<ApiInterface>(() => (
  deleteAccountCases.value.find(item => item.id === deleteAccountSelectedCaseId.value) || deleteAccountCases.value[0]!
))
const deleteAccountEditingBlock = ref<string | null>(null)
const deleteAccountLoginInfo = ref('')
const deleteAccountLoginInfoSummary = ref('粘贴抓取到的 Optional([...]) 或 JSON 信息后自动解析')
const deleteAccountLoginInfoState = ref<'idle' | 'success' | 'error'>('idle')
let lastSavedDeleteAccountConfigSnapshot = ''
let deleteAccountAbortController: AbortController | null = null
let deleteAccountManuallyStopped = false

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
        extractors: getSavedApiVariableExtractors(item),
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

const hasSavedShortDramaTestSuiteConfig = () => {
  if (typeof window === 'undefined') return false
  try {
    const rawData = window.localStorage.getItem(SHORT_DRAMA_API_CONFIG_STORAGE_KEY)
    if (!rawData) return false
    return Array.isArray(JSON.parse(rawData)?.testSuites)
  } catch {
    return false
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
              extractors: getSavedApiVariableExtractors(item),
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
const hasSavedShortDramaTestSuites = isShortDramaApiTest.value && hasSavedShortDramaTestSuiteConfig()
const savedShortDramaTestSuites = isShortDramaApiTest.value ? loadSavedShortDramaTestSuites() : []
const savedShortDramaInterfaces = isShortDramaApiTest.value && !hasSavedShortDramaTestSuites ? loadSavedShortDramaInterfaces() : []
const savedShortDramaTestName = isShortDramaApiTest.value ? loadSavedShortDramaTestName() : ''
if (isShortDramaApiTest.value && savedShortDramaTestName) {
  toolName.value = savedShortDramaTestName
}
const apiInterfaces = ref<ApiInterface[]>(
  savedShortDramaInterfaces.length > 0
    ? savedShortDramaInterfaces
    : hasSavedShortDramaTestSuites ? [] : [createShortDramaAnonymousLoginInterface()]
)
const apiTestSuites = ref<ApiTestSuite[]>(
  hasSavedShortDramaTestSuites
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

// 删除指定接口条目；最后一个接口删除后，自动删除所属接口测试。
const removeApiInterface = (id: number) => {
  const targetSuiteIndex = apiTestSuites.value.findIndex(suite => suite.interfaces.some(item => item.id === id))
  if (targetSuiteIndex < 0) return
  const targetSuite = apiTestSuites.value[targetSuiteIndex]
  if (!targetSuite) return
  const targetIndex = targetSuite.interfaces.findIndex(item => item.id === id)
  const wasSuiteSelected = selectedApiTestSuiteId.value === targetSuite.id
  if (targetIndex >= 0) {
    targetSuite.interfaces.splice(targetIndex, 1)
  }

  if (editingInterfaceNameId.value === id) editingInterfaceNameId.value = null
  if (editingJsonBlock.value?.startsWith(`${id}:`)) editingJsonBlock.value = null

  if (targetSuite.interfaces.length === 0) {
    const removedSuiteName = targetSuite.name
    apiTestSuites.value.splice(targetSuiteIndex, 1)
    if (wasSuiteSelected || !apiTestSuites.value.some(suite => suite.id === selectedApiTestSuiteId.value)) {
      const nextSuite = apiTestSuites.value[targetSuiteIndex] || apiTestSuites.value[targetSuiteIndex - 1] || null
      const nextInterface = nextSuite?.interfaces[0] || null
      selectedApiTestSuiteId.value = nextSuite?.id ?? null
      selectedApiInterfaceId.value = nextInterface?.id ?? null
      expandedInterfaceId.value = nextInterface?.id ?? null
      selectedPipelineStepIndex.value = nextInterface ? 0 : null
    }
    toolName.value = apiTestSuites.value[0]?.name || toolTypeName
    persistShortDramaApiConfig()
    ElMessage.success(`“${removedSuiteName}”已无接口，已自动删除该接口测试`)
    return
  }

  if (selectedApiInterfaceId.value === id) {
    const nextInterface = targetSuite.interfaces[targetIndex] || targetSuite.interfaces[targetIndex - 1] || targetSuite.interfaces[0]
    selectedApiInterfaceId.value = nextInterface?.id ?? null
    selectedPipelineStepIndex.value = nextInterface ? targetSuite.interfaces.indexOf(nextInterface) : null
  }
  if (expandedInterfaceId.value === id) {
    expandedInterfaceId.value = selectedApiInterfaceId.value
  }
  persistShortDramaApiConfig()
}

const buildShortDramaApiConfigSnapshot = () => JSON.stringify({
  environment: testServer.value,
  project: projectName.value,
  testName: apiTestSuites.value[0]?.name.trim() || toolName.value.trim() || toolTypeName,
  interfaces: apiTestSuites.value[0]?.interfaces.map(({ id, name, method, url, headers, body, extractors }) => ({
    id,
    name,
    method,
    url: normalizeInterfacePath(url),
    headers,
    body,
    extractors
  })) || [],
  testSuites: apiTestSuites.value.map(suite => ({
    id: suite.id,
    name: suite.name.trim() || `接口测试 ${suite.id}`,
    isExpanded: suite.isExpanded,
    interfaces: suite.interfaces.map(({ id, name, method, url, headers, body, extractors }) => ({
      id,
      name,
      method,
      url: normalizeInterfacePath(url),
      headers,
      body,
      extractors
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

const buildDeleteAccountConfigSnapshot = () => JSON.stringify({
  environment: deleteAccountEnvironment.value,
  customDomain: deleteAccountCustomDomain.value.trim(),
  project: projectName.value,
  cases: deleteAccountCases.value.map((item, index) => ({
    name: item.name.trim() || (index === 0 ? '匿名登录' : '删除账号'),
    method: item.method,
    url: normalizeInterfacePath(item.url),
    headers: item.headers,
    body: item.body
  }))
})

const persistDeleteAccountConfig = () => {
  if (!isDeleteAccount || typeof window === 'undefined') return
  const snapshot = buildDeleteAccountConfigSnapshot()
  if (snapshot === lastSavedDeleteAccountConfigSnapshot) return
  window.localStorage.setItem(DELETE_ACCOUNT_API_CONFIG_STORAGE_KEY, snapshot)
  lastSavedDeleteAccountConfigSnapshot = snapshot
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

const parseApiHeaders = (value: string): Record<string, unknown> => {
  if (!value.trim()) return {}
  const parsed = JSON.parse(value)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('请求头必须是 JSON 对象，例如 {"X-Token": "xxx"}')
  }
  return parsed as Record<string, unknown>
}

const decodeCapturedJSONString = (value: string) => {
  try {
    return JSON.parse(`"${value}"`)
  } catch {
    return value.replace(/\\"/g, '"').replace(/\\\\/g, '\\')
  }
}

const parseDeleteAccountLoginInfo = (rawValue: string): Record<string, unknown> => {
  let source = rawValue.trim()
  if (!source) throw new Error('请先粘贴匿名登录信息')

  // 兼容 Swift 控制台常见的 Optional(["key": "value"]) 输出。
  if (/^Optional\s*\(/i.test(source) && source.endsWith(')')) {
    source = source.replace(/^Optional\s*\(/i, '').slice(0, -1).trim()
  }
  source = source.replace(/\\_/g, '_')

  const jsonCandidate = source.startsWith('[') && source.endsWith(']')
    ? `{${source.slice(1, -1)}}`
    : source

  try {
    const parsed = JSON.parse(jsonCandidate)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
  } catch {
    // 非标准控制台文本继续使用键值对提取，避免单个转义字符导致整段无法识别。
  }

  const parsedPairs: Record<string, unknown> = {}
  const pairPattern = /"((?:\\.|[^"\\])*)"\s*:\s*(?:"((?:\\.|[^"\\])*)"|([^,\]\}\n]+))/g
  let match: RegExpExecArray | null
  while ((match = pairPattern.exec(source)) !== null) {
    const key = decodeCapturedJSONString(match[1] || '').trim()
    if (!key) continue
    const quotedValue = match[2]
    parsedPairs[key] = quotedValue !== undefined
      ? decodeCapturedJSONString(quotedValue)
      : String(match[3] || '').trim().replace(/^(?:null|nil)$/i, '')
  }

  if (Object.keys(parsedPairs).length === 0) {
    throw new Error('未识别到有效键值，请检查粘贴内容是否完整')
  }
  return parsedPairs
}

const applyDeleteAccountLoginInfo = (quiet = false) => {
  try {
    const parsedInfo = parseDeleteAccountLoginInfo(deleteAccountLoginInfo.value)
    const currentHeaders = parseApiHeaders(deleteAccountLoginCase.value.headers)
    const knownHeaderNames = new Map(
      [...Object.keys(shortDramaAnonymousLoginHeaders), ...Object.keys(currentHeaders)]
        .map(key => [key.toLowerCase(), key])
    )
    const aliases: Record<string, string> = {
      device_uuid: 'device-uuid',
      advertising_id: 'advertising-id',
      mobile_brand: 'mobile-brand',
      x_session_token: 'X-SESSION-TOKEN',
      'x-session-token': 'X-SESSION-TOKEN'
    }
    const managedHeaders = new Set(['content-length', 'host', 'connection', 'transfer-encoding'])
    let appliedCount = 0
    let ignoredCount = 0

    Object.entries(parsedInfo).forEach(([rawKey, rawValue]) => {
      const trimmedKey = rawKey.trim()
      if (!trimmedKey) return
      const lowerKey = trimmedKey.toLowerCase()
      if (managedHeaders.has(lowerKey)) {
        ignoredCount++
        return
      }
      if (rawValue !== null && typeof rawValue === 'object') return

      const normalizedKey = aliases[lowerKey] || knownHeaderNames.get(lowerKey) || trimmedKey
      currentHeaders[normalizedKey] = rawValue == null ? '' : String(rawValue)
      appliedCount++
    })

    if (appliedCount === 0) throw new Error('没有可用于匿名登录的请求头字段')

    deleteAccountLoginCase.value.headers = JSON.stringify(currentHeaders, null, 2)
    deleteAccountSelectedCaseId.value = deleteAccountLoginCase.value.id
    deleteAccountEditingBlock.value = null
    deleteAccountLoginInfoState.value = 'success'
    deleteAccountLoginInfoSummary.value = `已识别并带入 ${appliedCount} 项请求头${ignoredCount ? `，忽略 ${ignoredCount} 项由系统维护的字段` : ''}`
    persistDeleteAccountConfig()
    if (!quiet) ElMessage.success(deleteAccountLoginInfoSummary.value)
  } catch (error: any) {
    deleteAccountLoginInfoState.value = 'error'
    deleteAccountLoginInfoSummary.value = error.message || '匿名登录信息解析失败'
    if (!quiet) ElMessage.warning(deleteAccountLoginInfoSummary.value)
  }
}

const scheduleDeleteAccountLoginInfoParsing = () => {
  window.setTimeout(() => applyDeleteAccountLoginInfo(true), 0)
}

const finishJsonBlockEditing = (item: ApiInterface, field: ApiJsonField) => {
  const value = item[field].trim()
  if (value) {
    try {
      const parsed = field === 'headers' ? parseApiHeaders(value) : JSON.parse(value)
      item[field] = JSON.stringify(parsed, null, 2)
    } catch (error: any) {
      ElMessage.warning(`${field === 'headers' ? '请求头' : '请求体'} JSON 格式错误：${error.message}`)
      return false
    }
  }
  editingJsonBlock.value = null
  persistShortDramaApiConfig()
  return true
}

const isDeleteAccountJsonEditing = (item: ApiInterface, field: ApiJsonField) => (
  deleteAccountEditingBlock.value === getJsonBlockKey(item.id, field)
)

const startDeleteAccountJsonEditing = (item: ApiInterface, field: ApiJsonField) => {
  deleteAccountEditingBlock.value = getJsonBlockKey(item.id, field)
}

const finishDeleteAccountJsonEditing = (item: ApiInterface, field: ApiJsonField) => {
  const value = item[field].trim()
  if (value) {
    try {
      const parsed = field === 'headers' ? parseApiHeaders(value) : JSON.parse(value)
      item[field] = JSON.stringify(parsed, null, 2)
    } catch (error: any) {
      ElMessage.warning(`${field === 'headers' ? '请求头' : '请求体'} JSON 格式错误：${error.message}`)
      return false
    }
  }
  deleteAccountEditingBlock.value = null
  persistDeleteAccountConfig()
  return true
}

const getPreviousJsonSources = (suite: ApiTestSuite, item: ApiInterface, field: ApiJsonField) => {
  const currentIndex = suite.interfaces.findIndex(candidate => candidate.id === item.id)
  if (currentIndex <= 0) return []
  return suite.interfaces
    .slice(0, currentIndex)
    .filter(candidate => candidate[field].trim().length > 0)
    .slice(-6)
    .reverse()
}

const applyPreviousJsonSource = (
  suite: ApiTestSuite,
  item: ApiInterface,
  field: ApiJsonField,
  sourceId: number
) => {
  const source = suite.interfaces.find(candidate => candidate.id === sourceId)
  if (!source) return
  try {
    const parsed = field === 'headers' ? parseApiHeaders(source[field]) : JSON.parse(source[field])
    item[field] = JSON.stringify(parsed, null, 2)
    startJsonBlockEditing(item.id, field)
    persistShortDramaApiConfig()
    ElMessage.success(`已带入“${source.name}”的${field === 'headers' ? '请求头' : '请求体'}`)
  } catch (error: any) {
    ElMessage.warning(`“${source.name}”的${field === 'headers' ? '请求头' : '请求体'}格式无效：${error.message}`)
  }
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

const runtimeApiVariables = ref<Record<string, ApiRuntimeVariable>>({})
const jsonEditorRefs = new Map<string, any>()
const API_VARIABLE_NAME_PATTERN = /^[A-Za-z_][A-Za-z0-9_.-]*$/
const BLOCKED_API_VARIABLE_NAMES = new Set(['__proto__', 'prototype', 'constructor'])

const setJsonEditorRef = (id: number, field: ApiJsonField, element: any) => {
  const key = getJsonBlockKey(id, field)
  if (element) jsonEditorRefs.set(key, element)
  else jsonEditorRefs.delete(key)
}

const addApiVariableExtractor = (item: ApiInterface) => {
  item.extractors.push(createApiVariableExtractor())
  persistShortDramaApiConfig()
}

const removeApiVariableExtractor = (item: ApiInterface, extractorId: number) => {
  item.extractors = item.extractors.filter(extractor => extractor.id !== extractorId)
  persistShortDramaApiConfig()
}

const finishApiVariableExtractorEditing = (extractor: ApiVariableExtractor) => {
  extractor.name = extractor.name.trim()
  extractor.path = extractor.path.trim()
  if (isSensitiveVariableName(extractor.name)) extractor.sensitive = true
  persistShortDramaApiConfig()
}

const getApiVariableSourceLabel = (source: ApiVariableSource) => ({
  body: '响应体 JSON',
  header: '响应头',
  cookie: 'Cookie',
  text: '纯文本正则'
}[source])

const formatApiVariablePlaceholder = (name: string) => `{{${name}}}`

const getApiVariablePathPlaceholder = (source: ApiVariableSource) => ({
  body: '$.data.session_token',
  header: 'Authorization',
  cookie: 'session_id',
  text: 'token[=:]\\s*([^\\s]+)'
}[source])

const getAvailableApiVariables = (suite: ApiTestSuite, item: ApiInterface) => {
  const currentIndex = suite.interfaces.findIndex(candidate => candidate.id === item.id)
  if (currentIndex <= 0) return []
  const variables = suite.interfaces
    .slice(0, currentIndex)
    .flatMap(sourceCase => sourceCase.extractors
      .filter(extractor => extractor.name.trim())
      .map(extractor => ({
        ...extractor,
        sourceCaseId: sourceCase.id,
        sourceCaseName: sourceCase.name
      })))

  const latestByName = new Map<string, typeof variables[number]>()
  variables.forEach(variable => latestByName.set(variable.name, variable))
  return Array.from(latestByName.values())
}

const insertApiVariablePlaceholder = async (
  item: ApiInterface,
  field: ApiJsonField,
  variableName: string
) => {
  const placeholder = `{{${variableName}}}`
  const editor = jsonEditorRefs.get(getJsonBlockKey(item.id, field))
  const textarea = editor?.textarea as HTMLTextAreaElement | undefined

  if (!textarea) {
    ElMessage.warning('请先打开编辑状态，再插入变量')
    return
  }

  const start = textarea.selectionStart ?? item[field].length
  const end = textarea.selectionEnd ?? start
  item[field] = `${item[field].slice(0, start)}${placeholder}${item[field].slice(end)}`
  await nextTick()
  textarea.focus()
  textarea.setSelectionRange(start + placeholder.length, start + placeholder.length)
}

const isValidApiVariableName = (name: string) => (
  API_VARIABLE_NAME_PATTERN.test(name) && !BLOCKED_API_VARIABLE_NAMES.has(name)
)

const getRuntimeApiVariable = (name: string) => {
  if (!Object.prototype.hasOwnProperty.call(runtimeApiVariables.value, name)) {
    throw new Error(`变量 {{${name}}} 不存在，请确认前置 Case 已成功提取`)
  }
  return runtimeApiVariables.value[name]!
}

const stringifyTemplateValue = (value: unknown) => {
  if (typeof value === 'string') return value
  if (value === null || value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

const resolveApiTemplate = (value: unknown): unknown => {
  if (Array.isArray(value)) return value.map(resolveApiTemplate)
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, child]) => [key, resolveApiTemplate(child)]))
  }
  if (typeof value !== 'string') return value

  const exactMatch = value.match(/^\{\{\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*\}\}$/)
  if (exactMatch?.[1]) return getRuntimeApiVariable(exactMatch[1]).value

  return value.replace(/\{\{\s*([A-Za-z_][A-Za-z0-9_.-]*)\s*\}\}/g, (_match, name: string) => (
    stringifyTemplateValue(getRuntimeApiVariable(name).value)
  ))
}

const readApiJsonPath = (root: unknown, path: string): { found: boolean; value?: unknown } => {
  const expression = path.trim()
  if (expression === '$') return { found: true, value: root }
  if (!expression.startsWith('$')) throw new Error('JSONPath 必须以 $ 开头')

  const tokens: Array<string | number> = []
  let cursor = 1
  while (cursor < expression.length) {
    if (expression[cursor] === '.') {
      const start = ++cursor
      while (cursor < expression.length && expression[cursor] !== '.' && expression[cursor] !== '[') cursor++
      if (cursor === start) throw new Error('JSONPath 字段名不能为空')
      tokens.push(expression.slice(start, cursor))
      continue
    }
    if (expression[cursor] === '[') {
      const closeIndex = expression.indexOf(']', cursor)
      if (closeIndex < 0) throw new Error('JSONPath 缺少 ]')
      let token = expression.slice(cursor + 1, closeIndex).trim()
      if ((token.startsWith('"') && token.endsWith('"')) || (token.startsWith("'") && token.endsWith("'"))) {
        token = token.slice(1, -1)
        tokens.push(token)
      } else if (/^\d+$/.test(token)) {
        tokens.push(Number(token))
      } else {
        throw new Error(`暂不支持的 JSONPath 片段：[${token}]`)
      }
      cursor = closeIndex + 1
      continue
    }
    throw new Error(`JSONPath 在第 ${cursor + 1} 个字符附近无效`)
  }

  let current = root
  for (const token of tokens) {
    if (current === null || current === undefined || typeof current !== 'object') return { found: false }
    if (!Object.prototype.hasOwnProperty.call(current, token)) return { found: false }
    current = (current as any)[token]
  }
  return { found: true, value: current }
}

const addExtractorFromResponse = async (item: ApiInterface) => {
  if (item.responseData === undefined) {
    ElMessage.warning('请先执行该 Case，获取响应后再创建提取规则')
    return
  }

  try {
    const pathResult = await ElMessageBox.prompt(
      '输入需要提取字段的 JSONPath，例如 $.data.session_token',
      '从响应创建变量',
      {
        confirmButtonText: '下一步',
        cancelButtonText: '取消',
        inputValue: '$.data.session_token',
        inputPlaceholder: '$.data.session_token'
      }
    )
    const path = pathResult.value.trim()
    const extracted = readApiJsonPath(item.responseData, path)
    if (!extracted.found) {
      ElMessage.warning(`当前响应中没有找到 ${path}`)
      return
    }

    const pathSegments = path.match(/[A-Za-z_][A-Za-z0-9_-]*/g) || []
    const suggestedName = pathSegments[pathSegments.length - 1] || 'response_value'
    const nameResult = await ElMessageBox.prompt(
      `字段 ${path} 已找到，请设置后续 Case 使用的变量名`,
      '设置变量名称',
      {
        confirmButtonText: '创建变量',
        cancelButtonText: '取消',
        inputValue: suggestedName,
        inputPattern: API_VARIABLE_NAME_PATTERN,
        inputErrorMessage: '变量名需以字母或下划线开头，只能包含字母、数字、点、横线和下划线'
      }
    )
    const name = nameResult.value.trim()
    if (!isValidApiVariableName(name)) {
      ElMessage.warning('变量名格式无效')
      return
    }
    if (item.extractors.some(extractor => extractor.name === name)) {
      ElMessage.warning(`当前 Case 已存在变量 ${name}`)
      return
    }

    item.extractors.push({
      id: apiVariableExtractorIdCounter++,
      name,
      source: 'body',
      path,
      required: true,
      sensitive: isSensitiveVariableName(name)
    })
    persistShortDramaApiConfig()
    ElMessage.success(`已创建变量 ${formatApiVariablePlaceholder(name)}`)
  } catch (action) {
    if (action !== 'cancel' && action !== 'close') {
      ElMessage.error('创建变量失败')
    }
  }
}

const getCaseInsensitiveRecordValue = (record: Record<string, any>, key: string) => {
  const matchedKey = Object.keys(record || {}).find(candidate => candidate.toLowerCase() === key.toLowerCase())
  if (!matchedKey) return { found: false as const }
  const rawValue = record[matchedKey]
  return { found: true as const, value: Array.isArray(rawValue) ? rawValue[0] : rawValue }
}

const extractApiVariableValue = (extractor: ApiVariableExtractor, response: ApiProxyEnvelope) => {
  if (extractor.source === 'body') return readApiJsonPath(response.body, extractor.path)
  if (extractor.source === 'header') return getCaseInsensitiveRecordValue(response.headers, extractor.path)
  if (extractor.source === 'cookie') return getCaseInsensitiveRecordValue(response.cookies, extractor.path)

  const responseText = typeof response.body === 'string' ? response.body : JSON.stringify(response.body)
  const match = new RegExp(extractor.path).exec(responseText)
  if (!match) return { found: false }
  return { found: true, value: match[1] ?? match[0] }
}

const isEmptyApiVariableValue = (value: unknown) => value === undefined || value === null || value === ''

const maskApiVariableValue = (value: unknown, sensitive: boolean) => {
  const text = stringifyTemplateValue(value)
  if (!sensitive) return text
  if (text.length <= 8) return '****'
  return `${text.slice(0, 4)}****${text.slice(-4)}`
}

const redactApiPayload = (value: unknown, sensitiveValues: string[] = [], key = ''): unknown => {
  if (/(token|secret|password|cookie|authorization|session|api[-_]?key)/i.test(key)) return '****'
  if (Array.isArray(value)) return value.map(child => redactApiPayload(child, sensitiveValues))
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([childKey, child]) => [
      childKey,
      redactApiPayload(child, sensitiveValues, childKey)
    ]))
  }
  if (typeof value === 'string') {
    let redacted = value
    sensitiveValues.filter(Boolean).forEach(secret => {
      redacted = redacted.split(secret).join('****')
    })
    return redacted
  }
  return value
}

const extractApiRuntimeVariables = (step: ApiInterface, response: ApiProxyEnvelope) => {
  const requiredErrors: string[] = []
  const stagedVariables: Array<{ runtime: ApiRuntimeVariable; extracted: ApiExtractedVariable }> = []

  for (const extractor of step.extractors) {
    const name = extractor.name.trim()
    if (!isValidApiVariableName(name)) {
      const message = name ? `变量名“${name}”格式无效` : '存在未填写变量名的提取规则'
      if (extractor.required) requiredErrors.push(message)
      logs.value.push(`[WARN] ${step.name}: ${message}`)
      continue
    }
    if (!extractor.path.trim()) {
      const message = `变量 ${name} 未填写提取路径`
      if (extractor.required) requiredErrors.push(message)
      logs.value.push(`[WARN] ${step.name}: ${message}`)
      continue
    }

    try {
      const result = extractApiVariableValue(extractor, response)
      if (!result.found || isEmptyApiVariableValue(result.value)) {
        const message = `变量 ${name} 未提取到有效值（${getApiVariableSourceLabel(extractor.source)}：${extractor.path}）`
        if (extractor.required) requiredErrors.push(message)
        logs.value.push(`[WARN] ${step.name}: ${message}`)
        continue
      }

      stagedVariables.push({
        runtime: {
          name,
          value: result.value,
          sourceCaseId: step.id,
          sourceCaseName: step.name,
          sensitive: extractor.sensitive
        },
        extracted: {
          name,
          displayValue: maskApiVariableValue(result.value, extractor.sensitive),
          source: extractor.source,
          path: extractor.path,
          sensitive: extractor.sensitive
        }
      })
    } catch (error: any) {
      const message = `变量 ${name} 提取失败：${error.message}`
      if (extractor.required) requiredErrors.push(message)
      logs.value.push(`[WARN] ${step.name}: ${message}`)
    }
  }

  step.extractedVariables = stagedVariables.map(variable => variable.extracted)
  if (requiredErrors.length === 0) {
    stagedVariables.forEach(({ runtime }) => {
      const hadPreviousValue = Object.prototype.hasOwnProperty.call(runtimeApiVariables.value, runtime.name)
      runtimeApiVariables.value[runtime.name] = runtime
      logs.value.push(`[VARIABLE] ${runtime.name} 已${hadPreviousValue ? '更新' : '提取'}${runtime.sensitive ? '（已脱敏）' : ''}`)
    })
  } else if (stagedVariables.length > 0) {
    logs.value.push(`[WARN] ${step.name}: 因必填变量提取失败，本 Case 已提取值未写入后续运行上下文`)
  }

  return {
    requiredErrors,
    sensitiveValues: stagedVariables
      .filter(variable => variable.runtime.sensitive)
      .map(variable => stringifyTemplateValue(variable.runtime.value))
  }
}

const isApiProxyEnvelope = (value: any): value is ApiProxyEnvelope => (
  value && typeof value === 'object' && typeof value.status === 'number' &&
  value.headers && typeof value.headers === 'object' &&
  value.cookies && typeof value.cookies === 'object' && Object.prototype.hasOwnProperty.call(value, 'body')
)

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
    failed: '失败',
    skipped: '已跳过'
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
  if (activePipelineSteps.value.length === 0) {
    ElMessage.warning('当前没有可执行的接口，请先新增接口测试和接口')
    return
  }
  currentStatus.value = 'Executing'
  logs.value = [`[${new Date().toLocaleTimeString()}] 开始接口链路测试...`]
  runtimeApiVariables.value = {}
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
    s.extractedVariables = undefined
  })

  logs.value.push(`[INFO] 真实请求模式，接口数: ${activePipelineSteps.value.length}`)
  scrollToBottom()

  let hasFailed = false

  for (let index = 0; index < activePipelineSteps.value.length; index++) {
    const step = activePipelineSteps.value[index]
    if (!step) continue
    step.url = normalizeInterfacePath(step.url)
    const fullInterfaceUrlTemplate = getFullInterfaceUrl(step)
    step.status = 'running'
    shortDramaActiveTab.value = 'pipeline'
    selectedPipelineStepIndex.value = index
    selectedApiInterfaceId.value = step.id
    logs.value.push(`[${new Date().toLocaleTimeString()}] 正在执行: ${step.name} (${step.method} ${step.url || '未填写接口'})...`)
    scrollToBottom()

    const startTime = Date.now()

    if (!fullInterfaceUrlTemplate) {
      step.status = 'failed'
      step.code = 400
      step.errorMsg = '请求接口不能为空'
      logs.value.push(`[ERROR] 接口 [${step.name}] 执行失败: 请求接口不能为空`)
      hasFailed = true
      scrollToBottom()
      continue
    }

    let fullInterfaceUrl = ''
    let parsedHeaders: Record<string, unknown> = {}
    let parsedBody: unknown = {}
    try {
      fullInterfaceUrl = String(resolveApiTemplate(fullInterfaceUrlTemplate))
      parsedHeaders = resolveApiTemplate(parseApiHeaders(step.headers)) as Record<string, unknown>
      if (step.body && step.method !== 'GET') {
        parsedBody = resolveApiTemplate(JSON.parse(step.body))
      }
    } catch (e: any) {
      const isMissingVariable = String(e.message).includes('变量 {{')
      step.status = isMissingVariable ? 'skipped' : 'failed'
      step.errorMsg = e.message || '请求配置解析失败'
      logs.value.push(`[${isMissingVariable ? 'SKIP' : 'ERROR'}] 接口 [${step.name}] ${isMissingVariable ? '已跳过' : '执行失败'}: ${step.errorMsg}`)
      hasFailed = true
      scrollToBottom()
      continue
    }

    const sensitiveValuesBeforeRequest = Object.values(runtimeApiVariables.value)
      .filter(variable => variable.sensitive)
      .map(variable => stringifyTemplateValue(variable.value))
    step.requestData = redactApiPayload({
      url: fullInterfaceUrl,
      method: step.method,
      headers: parsedHeaders,
      body: parsedBody
    }, sensitiveValuesBeforeRequest)

    try {
      const response = await fetch(buildBackendUrl('/api/proxy'), {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          method: step.method,
          url: fullInterfaceUrl,
          headers: parsedHeaders,
          body: parsedBody,
          response_mode: 'envelope'
        })
      })

      const endTime = Date.now()
      step.latency = endTime - startTime
      step.code = response.status

      const contentType = response.headers.get('content-type') || ''
      let rawProxyResult: any
      if (contentType.includes('application/json')) {
        rawProxyResult = await response.json()
      } else {
        rawProxyResult = await response.text()
        try {
          rawProxyResult = JSON.parse(rawProxyResult)
        } catch {
          // 保留纯文本代理错误。
        }
      }

      const proxyResult: ApiProxyEnvelope = isApiProxyEnvelope(rawProxyResult)
        ? rawProxyResult
        : { status: response.status, headers: {}, cookies: {}, body: rawProxyResult }
      step.code = proxyResult.status

      if (response.ok) {
        const extractionResult = extractApiRuntimeVariables(step, proxyResult)
        const sensitiveValuesAfterResponse = Object.values(runtimeApiVariables.value)
          .filter(variable => variable.sensitive)
          .map(variable => stringifyTemplateValue(variable.value))
          .concat(extractionResult.sensitiveValues)
        step.responseData = redactApiPayload(proxyResult.body, sensitiveValuesAfterResponse)

        if (extractionResult.requiredErrors.length > 0) {
          step.status = 'failed'
          step.errorMsg = extractionResult.requiredErrors.join('；')
          logs.value.push(`[ERROR] 接口 [${step.name}] 响应变量提取失败: ${step.errorMsg}`)
          hasFailed = true
        } else {
          step.status = 'success'
          logs.value.push(`[SUCCESS] 接口 [${step.name}] 执行成功，HTTP ${proxyResult.status}`)
        }
      } else {
        step.status = 'failed'
        step.responseData = redactApiPayload(proxyResult.body)
        const responseBody = proxyResult.body as any
        step.errorMsg = responseBody?.msg || responseBody?.error || `接口返回 HTTP ${proxyResult.status}`
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

// 服务器配置档
const serverOptions = [
  { label: '测试服', value: 'test' },
  { label: '正式服', value: 'prod' },
  { label: '灰度服', value: 'gray' }
]
const deleteAccountServerOptions = [
  ...serverOptions,
  { label: '自定义', value: 'custom' }
]
const testServer = ref<ApiEnvironment>(
  isDeleteAccount && ['test', 'prod', 'gray'].includes(savedDeleteAccountConfig?.environment || '')
    ? savedDeleteAccountConfig!.environment as ApiEnvironment
    : loadSavedShortDramaEnvironment() || 'test'
)

watch(deleteAccountCases, () => {
  if (!isDeleteAccount) return
  try {
    const headers = parseApiHeaders(deleteAccountLoginCase.value.headers)
    const appID = String(headers.app || '')
    if (appID === 'com.novelnova.readstory') projectName.value = 'NovelNova'
    if (appID === 'com.company.shortsdrama.wave') projectName.value = 'ShortsWave'
  } catch {
    // 编辑中的 JSON 可能暂时不完整，完成编辑时会统一校验。
  }
  persistDeleteAccountConfig()
}, { deep: true, flush: 'post' })

watch([testServer, projectName, deleteAccountEnvironment, deleteAccountCustomDomain], () => {
  if (isDeleteAccount) persistDeleteAccountConfig()
})
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

const readDeleteAccountProxyResponse = async (response: Response) => {
  const contentType = response.headers.get('content-type') || ''
  if (contentType.includes('application/json')) return response.json()
  const text = await response.text()
  try {
    return JSON.parse(text)
  } catch {
    return { rawResponse: text }
  }
}

const escapeDeleteAccountHTML = (value: unknown) => String(value ?? '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#039;')

const runDeleteAccountNestedCases = async () => {
  const loginStep = deleteAccountLoginCase.value
  const deleteStep = deleteAccountDeleteCase.value
  if (!loginStep || !deleteStep) {
    ElMessage.error('删除账号 Case 配置不完整')
    return
  }

  loginStep.url = normalizeInterfacePath(loginStep.url)
  deleteStep.url = normalizeInterfacePath(deleteStep.url)
  if (!loginStep.url || !deleteStep.url) {
    ElMessage.warning('请完整填写匿名登录和删除账号接口')
    return
  }

  let loginHeaders: Record<string, unknown>
  let deleteHeadersConfig: Record<string, unknown>
  let loginBody: unknown = {}
  let deleteBody: unknown = undefined
  try {
    loginHeaders = parseApiHeaders(loginStep.headers)
    deleteHeadersConfig = parseApiHeaders(deleteStep.headers)
    if (loginStep.body.trim() && loginStep.method !== 'GET') loginBody = JSON.parse(loginStep.body)
    if (deleteStep.body.trim() && deleteStep.method !== 'GET') deleteBody = JSON.parse(deleteStep.body)
  } catch (error: any) {
    ElMessage.warning(`请求配置格式错误：${error.message}`)
    return
  }

  const projectDomains = domainMappings[projectName.value as keyof typeof domainMappings]
  let baseDomain = ''
  if (deleteAccountEnvironment.value === 'custom') {
    try {
      baseDomain = normalizeDeleteAccountCustomDomain(deleteAccountCustomDomain.value)
      deleteAccountCustomDomain.value = baseDomain
    } catch (error: any) {
      ElMessage.warning(error.message || '自定义域名格式不正确')
      return
    }
  } else {
    baseDomain = projectDomains?.[deleteAccountEnvironment.value] || ''
  }
  if (!baseDomain) {
    ElMessage.warning('当前项目或执行环境没有可用域名配置')
    return
  }

  const loginURL = `${baseDomain.replace(/\/$/, '')}${loginStep.url}`
  const deleteURL = `${baseDomain.replace(/\/$/, '')}${deleteStep.url}`
  const loginRequest = {
    url: loginURL,
    method: loginStep.method,
    headers: loginHeaders,
    body: loginBody
  }

  deleteAccountCases.value.forEach(item => {
    item.status = 'pending'
    item.code = undefined
    item.latency = undefined
    item.errorMsg = undefined
    item.requestData = undefined
    item.responseData = undefined
  })
  loginStep.status = 'running'
  loginStep.requestData = redactApiPayload(loginRequest)
  deleteAccountSelectedCaseId.value = loginStep.id
  currentStatus.value = 'Executing'
  deleteAccountManuallyStopped = false
  deleteAccountAbortController?.abort()
  deleteAccountAbortController = new AbortController()
  deleteAccountActiveTab.value = 'status'
  logs.value = [
    `[${new Date().toLocaleTimeString()}] 开始执行删除账号嵌套 Case 链路...`,
    `[CASE 1/2] ${loginStep.method} ${loginURL}`
  ]
  uptime.value = 0
  duration.value = 0
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    uptime.value++
    duration.value++
  }, 1000)
  scrollToBottom()

  let activeStep = loginStep
  try {
    const loginStartedAt = Date.now()
    const loginResponse = await fetch(buildBackendUrl('/api/proxy'), {
      method: 'POST',
      signal: deleteAccountAbortController.signal,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        method: loginStep.method,
        url: loginURL,
        headers: loginHeaders,
        body: loginBody
      })
    })
    const loginData = await readDeleteAccountProxyResponse(loginResponse)
    loginStep.latency = Date.now() - loginStartedAt
    loginStep.code = loginResponse.status
    loginStep.responseData = redactApiPayload(loginData)

    if (!loginResponse.ok) {
      throw new Error(loginData?.msg || loginData?.error || `匿名登录返回 HTTP ${loginResponse.status}`)
    }

    const userData = loginData?.data || {}
    const userID = userData.user_id || '未知'
    const userName = userData.user_name || '未知'
    const sessionToken = userData.session_token
    if (!sessionToken) throw new Error('登录响应中未找到有效 session_token')

    loginStep.status = 'success'
    loginStep.responseData = redactApiPayload(loginData, [String(sessionToken)])
    logs.value.push(`[CASE 1/2] 匿名登录成功，已识别用户 ${userID}，等待人工确认。`)
    scrollToBottom()

    await ElMessageBox.confirm(
      `<div class="delete-account-confirm-content">
        <div class="delete-account-confirm-lead">
          <span class="delete-account-confirm-badge"><i></i> CASE 1/2 已通过</span>
          <p>匿名登录成功，请确认账号身份后再执行下一步。</p>
        </div>
        <div class="delete-account-confirm-user-card">
          <div class="delete-account-confirm-user-card__title">
            <span class="delete-account-confirm-user-icon">ID</span>
            <div>
              <strong>待删除账号</strong>
              <small>请仔细核对以下账号信息</small>
            </div>
          </div>
          <div class="delete-account-confirm-user-row">
            <span>用户 ID</span>
            <code>${escapeDeleteAccountHTML(userID)}</code>
          </div>
          <div class="delete-account-confirm-user-row">
            <span>用户名</span>
            <strong>${escapeDeleteAccountHTML(userName)}</strong>
          </div>
        </div>
        <div class="delete-account-confirm-flow">
          <span class="is-finished">匿名登录</span>
          <i></i>
          <span class="is-danger">删除账号</span>
        </div>
        <div class="delete-account-confirm-warning">
          <span class="delete-account-confirm-warning__icon">!</span>
          <div>
            <strong>此操作不可撤销</strong>
            <span>继续后将永久抹除该账号及其关联数据，请谨慎操作。</span>
          </div>
        </div>
      </div>`,
      '确认执行删除账号',
      {
        confirmButtonText: '确认删除账号',
        cancelButtonText: '暂不执行',
        confirmButtonClass: 'delete-account-confirm-submit',
        cancelButtonClass: 'delete-account-confirm-cancel',
        dangerouslyUseHTMLString: true,
        center: false,
        customClass: 'delete-account-confirm-box'
      }
    )

    activeStep = deleteStep
    deleteStep.status = 'running'
    deleteAccountSelectedCaseId.value = deleteStep.id
    const deleteHeaders = {
      ...deleteHeadersConfig,
      'X-SESSION-TOKEN': sessionToken
    }
    const deleteRequest = {
      url: deleteURL,
      method: deleteStep.method,
      headers: deleteHeaders,
      ...(deleteBody !== undefined ? { body: deleteBody } : {})
    }
    deleteStep.requestData = redactApiPayload(deleteRequest, [String(sessionToken)])
    logs.value.push(`[CASE 2/2] 已确认，正在执行 ${deleteStep.method} ${deleteURL}`)
    scrollToBottom()

    const deleteStartedAt = Date.now()
    const deleteResponse = await fetch(buildBackendUrl('/api/proxy'), {
      method: 'POST',
      signal: deleteAccountAbortController.signal,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        method: deleteStep.method,
        url: deleteURL,
        headers: deleteHeaders,
        ...(deleteBody !== undefined ? { body: deleteBody } : {})
      })
    })
    const deleteData = await readDeleteAccountProxyResponse(deleteResponse)
    deleteStep.latency = Date.now() - deleteStartedAt
    deleteStep.code = deleteResponse.status
    deleteStep.responseData = redactApiPayload(deleteData)

    if (!deleteResponse.ok || (deleteData?.code !== 0 && deleteData?.msg !== 'success')) {
      throw new Error(deleteData?.msg || deleteData?.error || `删除账号返回 HTTP ${deleteResponse.status}`)
    }

    deleteStep.status = 'success'
    currentStatus.value = 'Finished'
    logs.value.push(`[SUCCESS] 两个嵌套 Case 均执行成功，用户 ${userID} 已注销。`)
    ElMessage.success('账号注销成功')
  } catch (error: any) {
    if (deleteAccountManuallyStopped) {
      deleteAccountCases.value.forEach(item => {
        if (item.status !== 'success') {
          item.status = 'skipped'
          item.errorMsg = '执行已手动终止'
        }
      })
      currentStatus.value = 'Stopped'
    } else if (error === 'cancel' || error === 'close') {
      deleteStep.status = 'skipped'
      deleteStep.errorMsg = '用户取消执行删除账号 Case'
      currentStatus.value = 'Stopped'
      logs.value.push(`[CANCEL] ${deleteStep.errorMsg}。`)
    } else {
      activeStep.status = 'failed'
      activeStep.errorMsg = error?.message || `${activeStep.name}执行异常`
      if (activeStep.id === loginStep.id) {
        deleteStep.status = 'skipped'
        deleteStep.errorMsg = '前置匿名登录 Case 失败'
      }
      currentStatus.value = 'Failed'
      logs.value.push(`[ERROR] ${activeStep.name}: ${activeStep.errorMsg}`)
      ElMessage.error(activeStep.errorMsg)
    }
  } finally {
    if (timer) clearInterval(timer)
    timer = null
    deleteAccountAbortController = null
    persistDeleteAccountConfig()
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
const visibleLogs = computed(() => {
  if (isDramaCheck) return dramaRunStore.logs
  if (isSubtitleCheck) return subtitleRunStore.logs
  return logs.value
})
const visibleReportUrl = computed(() => {
  if (isDramaCheck) return dramaRunStore.reportUrl
  if (isSubtitleCheck) return subtitleRunStore.reportUrl
  return reportUrl.value
})
const visibleDuration = computed(() => {
  if (isDramaCheck) return dramaRunStore.duration
  if (isSubtitleCheck) return subtitleRunStore.duration
  return duration.value
})
const visibleUptime = computed(() => {
  if (isDramaCheck) return dramaRunStore.uptime
  if (isSubtitleCheck) return subtitleRunStore.uptime
  return uptime.value
})
const visibleStatus = computed<ExecStatus>(() => {
  if (isSubtitleCheck) {
    if (subtitleRunStore.status === 'running') return 'Executing'
    if (subtitleRunStore.status === 'done') return 'Finished'
    if (subtitleRunStore.status === 'failed') return 'Failed'
    if (subtitleRunStore.status === 'stopped') return 'Stopped'
    return currentStatus.value
  }
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

  if (isDeleteAccount) {
    await runDeleteAccountNestedCases()
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
    dramaListUrl: profile.dramaListUrl,
    environment: testServer.value
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

  if (isSubtitleCheck) {
    await subtitleRunStore.start({
      email: profile.email,
      password: profile.password,
      loginUrl: profile.loginUrl,
      dramaListUrl: profile.dramaListUrl,
      toolName: toolName.value,
      author: authStore.user?.username || 'tester',
      environment: testServer.value
    })
    currentStatus.value = 'Ready'
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

  if (isDeleteAccount) {
    deleteAccountManuallyStopped = true
    deleteAccountAbortController?.abort()
    ElMessageBox.close()
    if (timer) clearInterval(timer)
    timer = null
    deleteAccountCases.value.forEach(item => {
      if (item.status !== 'success') {
        item.status = 'skipped'
        item.errorMsg = '执行已手动终止'
      }
    })
    currentStatus.value = 'Stopped'
    logs.value.push(`[${new Date().toLocaleTimeString()}] 删除账号嵌套 Case 链路已手动终止。`)
    scrollToBottom()
    return
  }

  if (isDramaCheck) {
    await dramaRunStore.stop()
    currentStatus.value = 'Stopped'
    return
  }

  if (isSubtitleCheck) {
    await subtitleRunStore.stop()
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
  persistDeleteAccountConfig()
  if (isDramaCheck && visibleStatus.value === 'Executing') {
    dramaRunStore.markBackground()
  } else if (currentStatus.value === 'Executing') {
    stopExecution()
  }
  router.push('/')
}

const persistApiToolConfigBeforeUnload = () => {
  persistShortDramaApiConfig()
  persistDeleteAccountConfig()
}

onMounted(() => {
  reportStore.fetchReports()
  fetchMonkeyDevices()
  if (isSubtitleCheck) {
    subtitleRunStore.recoverCurrentRun()
  }
  window.addEventListener('beforeunload', persistApiToolConfigBeforeUnload)
})

onUnmounted(() => {
  persistShortDramaApiConfig()
  persistDeleteAccountConfig()
  window.removeEventListener('beforeunload', persistApiToolConfigBeforeUnload)
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
  persistDeleteAccountConfig()
})
</script>

<template>
  <div class="com-api-container">
    <div class="main-content">
      
      <!-- 左侧信息区 -->
      <div
        class="sidebar-panel"
        :class="{
          'sidebar-panel--short-drama': isShortDramaApiTest,
          'sidebar-panel--delete-account': isDeleteAccount
        }"
      >
        <div class="status-badge-row">
          <span class="badge-ready" :class="{ 'badge-ready--short-drama': isShortDramaApiTest || isDeleteAccount }">
            {{ isShortDramaApiTest ? '接口配置列表' : isDeleteAccount ? '2 CASE 嵌套配置' : 'READY' }}
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
          <el-icon v-else-if="showDramaRulesEntry" class="info-icon" role="button" tabindex="0" aria-label="查看规则配置" title="查看规则配置" @click="dramaRulesVisible = true" @keydown.enter="dramaRulesVisible = true" @keydown.space.prevent="dramaRulesVisible = true"><Warning /></el-icon>
          <el-icon v-else class="info-icon"><Warning /></el-icon>
        </div>

        <div v-if="!isShortDramaApiTest" class="tool-title-section">
          <h1 class="tool-title">{{ toolName }}</h1>
          <p class="tool-desc">{{ toolDesc }}</p>
        </div>

        <div class="params-section">
          <template v-if="isDeleteAccount">
            <div class="delete-account-safety-tip">
              <el-icon><Warning /></el-icon>
              <span>两个 Case 按顺序嵌套执行：匿名登录成功并人工确认后，才会进入删除账号 Case。</span>
            </div>

            <div class="short-drama-settings-grid delete-account-settings-grid">
              <label class="short-drama-setting">
                <span class="short-drama-setting__label">执行环境</span>
                <el-select v-model="deleteAccountEnvironment" placeholder="选择服务器" size="small" class="short-drama-setting__control">
                  <el-option
                    v-for="server in deleteAccountServerOptions"
                    :key="server.value"
                    :label="server.label"
                    :value="server.value"
                  />
                </el-select>
              </label>
              <label
                class="short-drama-setting"
                :class="{ 'delete-account-project-setting--disabled': deleteAccountEnvironment === 'custom' }"
              >
                <span class="short-drama-setting__label">项目</span>
                <el-select
                  v-model="projectName"
                  placeholder="选择项目"
                  size="small"
                  class="short-drama-setting__control"
                  :disabled="deleteAccountEnvironment === 'custom'"
                >
                  <el-option label="ShortsWave" value="ShortsWave" />
                  <el-option label="NovelNova" value="NovelNova" />
                </el-select>
              </label>
            </div>

            <label v-if="deleteAccountEnvironment === 'custom'" class="short-drama-setting delete-account-custom-domain-setting">
              <span class="short-drama-setting__label">自定义域名</span>
              <el-input
                v-model="deleteAccountCustomDomain"
                size="small"
                class="short-drama-setting__control"
                placeholder="例如 https://api.example.com 或 http://192.168.1.10:8080"
                @blur="finishDeleteAccountCustomDomainEditing"
              />
              <small>执行时将使用该域名拼接匿名登录和删除账号接口路径</small>
            </label>

            <div class="delete-account-login-info-card">
              <div class="delete-account-login-info-card__header">
                <div>
                  <strong>匿名登录信息</strong>
                  <small>支持 Optional([...])、JSON 对象以及控制台键值文本</small>
                </div>
                <el-button
                  type="primary"
                  size="small"
                  :disabled="!deleteAccountLoginInfo.trim()"
                  @click="applyDeleteAccountLoginInfo()"
                >
                  解析并带入
                </el-button>
              </div>
              <el-input
                v-model="deleteAccountLoginInfo"
                type="textarea"
                :rows="5"
                resize="none"
                spellcheck="false"
                class="delete-account-login-info-input"
                placeholder="粘贴 Optional([&quot;app&quot;: &quot;...&quot;, &quot;device-uuid&quot;: &quot;...&quot;])"
                @paste="scheduleDeleteAccountLoginInfoParsing"
              />
              <div
                class="delete-account-login-info-card__result"
                :class="`delete-account-login-info-card__result--${deleteAccountLoginInfoState}`"
              >
                <span class="delete-account-login-info-card__dot"></span>
                <span>{{ deleteAccountLoginInfoSummary }}</span>
              </div>
              <p>解析结果会合并到下方“匿名登录”Case 的请求头；请求体保持接口要求的 JSON 结构。</p>
            </div>

            <template v-for="(deleteCase, deleteCaseIndex) in deleteAccountCases" :key="deleteCase.id">
              <div
                class="api-interface-card api-interface-card--active delete-account-case-card"
                :class="{ 'delete-account-case-card--selected': deleteAccountSelectedCaseId === deleteCase.id }"
              >
                <div class="api-card-header delete-account-case-card__header" @click="deleteAccountSelectedCaseId = deleteCase.id">
                  <div class="api-header-left">
                    <span class="delete-account-case-order">{{ deleteCaseIndex + 1 }}</span>
                    <span class="method-badge" :class="deleteCase.method">{{ deleteCase.method }}</span>
                    <div class="delete-account-case-card__title">
                      <strong>{{ deleteCase.name }}</strong>
                      <small>{{ deleteCaseIndex === 0 ? '获取 session_token，作为删除 Case 的前置步骤' : '自动接收登录 Case 的 session_token' }}</small>
                    </div>
                  </div>
                  <span class="api-status-chip" :class="`api-status-chip--${deleteCase.status}`">
                    {{ getInterfaceStatusText(deleteCase.status) }}
                  </span>
                </div>

                <div class="api-card-body delete-account-case-card__body">
                  <div class="form-row">
                    <div class="form-item">
                      <label class="inner-label">Case 名称</label>
                      <el-input v-model="deleteCase.name" size="small" @blur="persistDeleteAccountConfig" />
                    </div>
                  </div>
                  <div class="form-row method-url-row">
                    <div class="form-item method-col">
                      <label class="inner-label">请求方法</label>
                      <el-select v-model="deleteCase.method" size="small" class="method-select">
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
                        v-model="deleteCase.url"
                        size="small"
                        :placeholder="deleteCaseIndex === 0 ? '/login/anonymous' : '/user/delete'"
                        @blur="deleteCase.url = normalizeInterfacePath(deleteCase.url); persistDeleteAccountConfig()"
                      />
                    </div>
                  </div>

                  <div class="form-row">
                    <div class="form-item">
                      <div class="json-editor-heading">
                        <label class="inner-label">请求头</label>
                        <el-button
                          link
                          type="primary"
                          size="small"
                          :icon="Edit"
                          class="json-editor-action"
                          @mousedown.prevent
                          @click.stop="isDeleteAccountJsonEditing(deleteCase, 'headers') ? finishDeleteAccountJsonEditing(deleteCase, 'headers') : startDeleteAccountJsonEditing(deleteCase, 'headers')"
                        >
                          {{ isDeleteAccountJsonEditing(deleteCase, 'headers') ? '完成编辑' : '编辑请求头' }}
                        </el-button>
                      </div>
                      <div
                        class="json-command-preview"
                        :class="{ 'json-command-preview--editing': isDeleteAccountJsonEditing(deleteCase, 'headers') }"
                        @dblclick="startDeleteAccountJsonEditing(deleteCase, 'headers')"
                      >
                        <div class="json-command-preview__toolbar">
                          <span class="json-command-preview__label">
                            {{ isDeleteAccountJsonEditing(deleteCase, 'headers') ? '请求头编辑' : '请求头预览' }}
                          </span>
                          <span class="delete-account-isolation-badge">Case 独立配置</span>
                        </div>
                        <el-input
                          v-if="isDeleteAccountJsonEditing(deleteCase, 'headers')"
                          v-model="deleteCase.headers"
                          type="textarea"
                          :rows="getJsonEditorRows(deleteCase.headers, 12)"
                          resize="none"
                          wrap="off"
                          size="small"
                          class="monospace-textarea json-command-editor"
                          autofocus
                          @blur="finishDeleteAccountJsonEditing(deleteCase, 'headers')"
                        />
                        <pre v-else class="json-command-preview__lines">
                          <span
                            v-for="(line, lineIndex) in getJsonPreviewLines(deleteCase.headers, '未配置请求头')"
                            :key="lineIndex"
                            class="json-command-preview__line"
                          >{{ line || ' ' }}</span>
                        </pre>
                      </div>
                    </div>
                  </div>

                  <div v-if="deleteCase.method !== 'GET'" class="form-row">
                    <div class="form-item">
                      <div class="json-editor-heading">
                        <label class="inner-label">请求体</label>
                        <el-button
                          link
                          type="primary"
                          size="small"
                          :icon="Edit"
                          class="json-editor-action"
                          @mousedown.prevent
                          @click.stop="isDeleteAccountJsonEditing(deleteCase, 'body') ? finishDeleteAccountJsonEditing(deleteCase, 'body') : startDeleteAccountJsonEditing(deleteCase, 'body')"
                        >
                          {{ isDeleteAccountJsonEditing(deleteCase, 'body') ? '完成编辑' : '编辑请求体' }}
                        </el-button>
                      </div>
                      <div
                        class="json-command-preview"
                        :class="{ 'json-command-preview--editing': isDeleteAccountJsonEditing(deleteCase, 'body') }"
                        @dblclick="startDeleteAccountJsonEditing(deleteCase, 'body')"
                      >
                        <div class="json-command-preview__toolbar">
                          <span class="json-command-preview__label">
                            {{ isDeleteAccountJsonEditing(deleteCase, 'body') ? '请求体编辑' : '请求体预览' }}
                          </span>
                          <span class="delete-account-isolation-badge">Case 独立配置</span>
                        </div>
                        <el-input
                          v-if="isDeleteAccountJsonEditing(deleteCase, 'body')"
                          v-model="deleteCase.body"
                          type="textarea"
                          :rows="getJsonEditorRows(deleteCase.body, 6)"
                          resize="none"
                          wrap="off"
                          size="small"
                          class="monospace-textarea json-command-editor"
                          autofocus
                          @blur="finishDeleteAccountJsonEditing(deleteCase, 'body')"
                        />
                        <pre v-else class="json-command-preview__lines">
                          <span
                            v-for="(line, lineIndex) in getJsonPreviewLines(deleteCase.body, '未配置请求体')"
                            :key="lineIndex"
                            class="json-command-preview__line"
                          >{{ line || ' ' }}</span>
                        </pre>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
              <div v-if="deleteCaseIndex === 0" class="delete-account-nested-connector">
                <span>成功提取 session_token 后继续</span>
                <el-icon><ArrowDown /></el-icon>
              </div>
            </template>
          </template>

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
            <div v-if="apiTestSuites.length === 0" class="api-test-suite-empty">
              <span>暂无接口测试</span>
              <small>点击上方“新增接口测试”重新创建</small>
            </div>
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
                    'card-status--running': item.status === 'running',
                    'card-status--skipped': item.status === 'skipped'
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
                        <div class="json-editor-heading">
                          <label class="inner-label">请求头</label>
                          <el-button
                            link
                            type="primary"
                            size="small"
                            :icon="Edit"
                            class="json-editor-action"
                            @mousedown.prevent
                            @click.stop="isJsonBlockEditing(item.id, 'headers') ? finishJsonBlockEditing(item, 'headers') : startJsonBlockEditing(item.id, 'headers')"
                          >
                            {{ isJsonBlockEditing(item.id, 'headers') ? '完成编辑' : '编辑请求头' }}
                          </el-button>
                        </div>
                        <div
                          class="json-command-preview"
                          :class="{ 'json-command-preview--editing': isJsonBlockEditing(item.id, 'headers') }"
                          title="双击修改请求头"
                          @dblclick="startJsonBlockEditing(item.id, 'headers')"
                        >
                          <div class="json-command-preview__toolbar">
                            <span class="json-command-preview__label">
                              {{ isJsonBlockEditing(item.id, 'headers') ? '请求头编辑' : '请求头预览' }}
                            </span>
                            <div class="json-command-preview__actions">
                            <el-dropdown
                              v-if="isJsonBlockEditing(item.id, 'headers')"
                              trigger="click"
                              popper-class="api-variable-popper"
                              :disabled="getAvailableApiVariables(suite, item).length === 0"
                              @command="(variableName: string) => insertApiVariablePlaceholder(item, 'headers', variableName)"
                            >
                              <el-button
                                size="small"
                                round
                                class="json-variable-btn"
                                :disabled="getAvailableApiVariables(suite, item).length === 0"
                                @mousedown.prevent
                              >
                                {{ '{}' }} 插入变量
                                <el-icon class="json-quick-fill-btn__arrow"><ArrowDown /></el-icon>
                              </el-button>
                              <template #dropdown>
                                <el-dropdown-menu>
                                  <el-dropdown-item disabled class="api-variable-menu__header">可用的前置变量</el-dropdown-item>
                                  <el-dropdown-item
                                    v-for="variable in getAvailableApiVariables(suite, item)"
                                    :key="`${variable.sourceCaseId}:${variable.name}`"
                                    :command="variable.name"
                                  >
                                    <div class="api-variable-menu__item">
                                      <code>{{ formatApiVariablePlaceholder(variable.name) }}</code>
                                      <small>来自 {{ variable.sourceCaseName }}</small>
                                    </div>
                                  </el-dropdown-item>
                                </el-dropdown-menu>
                              </template>
                            </el-dropdown>
                            <el-dropdown
                              v-if="isJsonBlockEditing(item.id, 'headers')"
                              trigger="click"
                              popper-class="json-quick-fill-popper"
                              :show-timeout="0"
                              :hide-timeout="120"
                              :disabled="getPreviousJsonSources(suite, item, 'headers').length === 0"
                              @command="(sourceId: number) => applyPreviousJsonSource(suite, item, 'headers', sourceId)"
                            >
                              <el-button
                                size="small"
                                round
                                class="json-quick-fill-btn"
                                :disabled="getPreviousJsonSources(suite, item, 'headers').length === 0"
                                :title="getPreviousJsonSources(suite, item, 'headers').length > 0 ? '从前面的 case 带入请求头' : '前面的 case 暂无可用请求头'"
                                @mousedown.prevent
                              >
                                <el-icon><CopyDocument /></el-icon>
                                一键带入
                                <el-icon class="json-quick-fill-btn__arrow"><ArrowDown /></el-icon>
                              </el-button>
                              <template #dropdown>
                                <el-dropdown-menu>
                                  <el-dropdown-item disabled class="json-quick-fill-menu__header">
                                    <div class="json-quick-fill-menu__title">
                                      <span>选择来源 Case</span>
                                      <small>{{ getPreviousJsonSources(suite, item, 'headers').length }} 条可用</small>
                                    </div>
                                  </el-dropdown-item>
                                  <el-dropdown-item
                                    v-for="source in getPreviousJsonSources(suite, item, 'headers')"
                                    :key="source.id"
                                    :command="source.id"
                                  >
                                    <div class="json-quick-fill-option">
                                      <span :class="['json-quick-fill-method', `is-${source.method.toLowerCase()}`]">{{ source.method }}</span>
                                      <div class="json-quick-fill-option__copy">
                                        <strong>{{ source.name }}</strong>
                                        <small>{{ getInterfaceDisplayUrl(source) }}</small>
                                      </div>
                                      <el-icon class="json-quick-fill-option__arrow"><ArrowRight /></el-icon>
                                    </div>
                                  </el-dropdown-item>
                                </el-dropdown-menu>
                              </template>
                            </el-dropdown>
                            </div>
                          </div>
                          <el-input
                            v-if="isJsonBlockEditing(item.id, 'headers')"
                            :ref="(element: any) => setJsonEditorRef(item.id, 'headers', element)"
                            v-model="item.headers"
                            type="textarea"
                            :rows="getJsonEditorRows(item.headers, 10)"
                            resize="none"
                            wrap="off"
                            size="small"
                            placeholder='{"Content-Type": "application/json", "X-Token": "xxx"}'
                            class="monospace-textarea json-command-editor"
                            autofocus
                            @blur="finishJsonBlockEditing(item, 'headers')"
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
                        <div class="json-editor-heading">
                          <label class="inner-label">请求体</label>
                          <el-button
                            link
                            type="primary"
                            size="small"
                            :icon="Edit"
                            class="json-editor-action"
                            @mousedown.prevent
                            @click.stop="isJsonBlockEditing(item.id, 'body') ? finishJsonBlockEditing(item, 'body') : startJsonBlockEditing(item.id, 'body')"
                          >
                            {{ isJsonBlockEditing(item.id, 'body') ? '完成编辑' : '编辑请求体' }}
                          </el-button>
                        </div>
                        <div
                          class="json-command-preview"
                          :class="{ 'json-command-preview--editing': isJsonBlockEditing(item.id, 'body') }"
                          title="双击修改请求体"
                          @dblclick="startJsonBlockEditing(item.id, 'body')"
                        >
                          <div class="json-command-preview__toolbar">
                            <span class="json-command-preview__label">
                              {{ isJsonBlockEditing(item.id, 'body') ? '请求体编辑' : '请求体预览' }}
                            </span>
                            <div class="json-command-preview__actions">
                            <el-dropdown
                              v-if="isJsonBlockEditing(item.id, 'body')"
                              trigger="click"
                              popper-class="api-variable-popper"
                              :disabled="getAvailableApiVariables(suite, item).length === 0"
                              @command="(variableName: string) => insertApiVariablePlaceholder(item, 'body', variableName)"
                            >
                              <el-button
                                size="small"
                                round
                                class="json-variable-btn"
                                :disabled="getAvailableApiVariables(suite, item).length === 0"
                                @mousedown.prevent
                              >
                                {{ '{}' }} 插入变量
                                <el-icon class="json-quick-fill-btn__arrow"><ArrowDown /></el-icon>
                              </el-button>
                              <template #dropdown>
                                <el-dropdown-menu>
                                  <el-dropdown-item disabled class="api-variable-menu__header">可用的前置变量</el-dropdown-item>
                                  <el-dropdown-item
                                    v-for="variable in getAvailableApiVariables(suite, item)"
                                    :key="`${variable.sourceCaseId}:${variable.name}`"
                                    :command="variable.name"
                                  >
                                    <div class="api-variable-menu__item">
                                      <code>{{ formatApiVariablePlaceholder(variable.name) }}</code>
                                      <small>来自 {{ variable.sourceCaseName }}</small>
                                    </div>
                                  </el-dropdown-item>
                                </el-dropdown-menu>
                              </template>
                            </el-dropdown>
                            <el-dropdown
                              v-if="isJsonBlockEditing(item.id, 'body')"
                              trigger="click"
                              popper-class="json-quick-fill-popper"
                              :show-timeout="0"
                              :hide-timeout="120"
                              :disabled="getPreviousJsonSources(suite, item, 'body').length === 0"
                              @command="(sourceId: number) => applyPreviousJsonSource(suite, item, 'body', sourceId)"
                            >
                              <el-button
                                size="small"
                                round
                                class="json-quick-fill-btn"
                                :disabled="getPreviousJsonSources(suite, item, 'body').length === 0"
                                :title="getPreviousJsonSources(suite, item, 'body').length > 0 ? '从前面的 case 带入请求体' : '前面的 case 暂无可用请求体'"
                                @mousedown.prevent
                              >
                                <el-icon><CopyDocument /></el-icon>
                                一键带入
                                <el-icon class="json-quick-fill-btn__arrow"><ArrowDown /></el-icon>
                              </el-button>
                              <template #dropdown>
                                <el-dropdown-menu>
                                  <el-dropdown-item disabled class="json-quick-fill-menu__header">
                                    <div class="json-quick-fill-menu__title">
                                      <span>选择来源 Case</span>
                                      <small>{{ getPreviousJsonSources(suite, item, 'body').length }} 条可用</small>
                                    </div>
                                  </el-dropdown-item>
                                  <el-dropdown-item
                                    v-for="source in getPreviousJsonSources(suite, item, 'body')"
                                    :key="source.id"
                                    :command="source.id"
                                  >
                                    <div class="json-quick-fill-option">
                                      <span :class="['json-quick-fill-method', `is-${source.method.toLowerCase()}`]">{{ source.method }}</span>
                                      <div class="json-quick-fill-option__copy">
                                        <strong>{{ source.name }}</strong>
                                        <small>{{ getInterfaceDisplayUrl(source) }}</small>
                                      </div>
                                      <el-icon class="json-quick-fill-option__arrow"><ArrowRight /></el-icon>
                                    </div>
                                  </el-dropdown-item>
                                </el-dropdown-menu>
                              </template>
                            </el-dropdown>
                            </div>
                          </div>
                          <el-input
                            v-if="isJsonBlockEditing(item.id, 'body')"
                            :ref="(element: any) => setJsonEditorRef(item.id, 'body', element)"
                            v-model="item.body"
                            type="textarea"
                            :rows="getJsonEditorRows(item.body, 6)"
                            resize="none"
                            wrap="off"
                            size="small"
                            placeholder='{"key": "value"}'
                            class="monospace-textarea json-command-editor"
                            autofocus
                            @blur="finishJsonBlockEditing(item, 'body')"
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

                    <div class="api-variable-extractor-panel">
                      <div class="api-variable-extractor-panel__header">
                        <div>
                          <label class="inner-label">响应变量提取</label>
                          <small>请求成功后提取变量，供后续 Case 使用</small>
                        </div>
                        <el-button
                          size="small"
                          type="primary"
                          plain
                          class="api-variable-add-btn"
                          @click.stop="addApiVariableExtractor(item)"
                        >
                          添加变量
                        </el-button>
                      </div>

                      <div v-if="item.extractors.length === 0" class="api-variable-extractor-empty">
                        暂无提取规则。登录 Case 可在这里提取 session_token、access_token 或 Cookie。
                      </div>

                      <div
                        v-for="extractor in item.extractors"
                        :key="extractor.id"
                        class="api-variable-extractor-row"
                      >
                        <el-input
                          v-model="extractor.name"
                          size="small"
                          placeholder="变量名，如 session_token"
                          class="api-variable-name-input"
                          @blur="finishApiVariableExtractorEditing(extractor)"
                        />
                        <el-select
                          v-model="extractor.source"
                          size="small"
                          class="api-variable-source-select"
                          @change="persistShortDramaApiConfig()"
                        >
                          <el-option label="响应体 JSON" value="body" />
                          <el-option label="响应头" value="header" />
                          <el-option label="Cookie" value="cookie" />
                          <el-option label="纯文本正则" value="text" />
                        </el-select>
                        <el-input
                          v-model="extractor.path"
                          size="small"
                          :placeholder="getApiVariablePathPlaceholder(extractor.source)"
                          class="api-variable-path-input"
                          @blur="finishApiVariableExtractorEditing(extractor)"
                        />
                        <div class="api-variable-extractor-actions">
                        <el-tooltip content="提取失败时阻止依赖 Case 执行" placement="top">
                          <label class="api-variable-option">
                            <el-switch v-model="extractor.required" size="small" @change="persistShortDramaApiConfig()" />
                            <span>必填</span>
                          </label>
                        </el-tooltip>
                        <el-tooltip content="日志和报文中隐藏真实值" placement="top">
                          <label class="api-variable-option">
                            <el-switch v-model="extractor.sensitive" size="small" @change="persistShortDramaApiConfig()" />
                            <span>脱敏</span>
                          </label>
                        </el-tooltip>
                        <el-button
                          link
                          type="danger"
                          :icon="Delete"
                          title="删除提取规则"
                          class="api-variable-extractor-delete"
                          @click.stop="removeApiVariableExtractor(item, extractor.id)"
                        />
                        </div>
                      </div>

                      <div v-if="item.extractedVariables?.length" class="api-variable-runtime-result">
                        <span class="api-variable-runtime-result__label">本次已提取</span>
                        <span
                          v-for="variable in item.extractedVariables"
                          :key="variable.name"
                          class="api-variable-runtime-chip"
                        >
                          <code>{{ variable.name }}</code>
                          <span>{{ variable.displayValue }}</span>
                        </span>
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
          <div v-if="!isDramaCheck && !isSubtitleCheck && !isMonkeyTest && !isShortDramaApiTest && !isDeleteAccount" class="param-group">
            <label class="param-label">
              <span class="link-icon">🔗</span> 测试链接
            </label>
            <el-input 
              v-model="projectName" 
              :disabled="!isWebFrontendStressTest" 
              :class="{ 'param-input-disabled': !isWebFrontendStressTest && !isWebFrontendStressTest }" 
            />
          </div>

          <div v-if="!isWebFrontendStressTest && !isMonkeyTest && !isShortDramaApiTest && !isDeleteAccount" class="param-row">
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
      <div
        class="log-panel"
        :class="{
          'log-panel--short-drama': isShortDramaApiTest,
          'log-panel--delete-account': isDeleteAccount
        }"
        style="position: relative;"
      >
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
        <div v-else-if="isDeleteAccount" class="short-drama-results-container delete-account-results-container">
          <div class="short-drama-tabs-header delete-account-tabs-header">
            <button
              class="sd-tab-btn"
              :class="{ active: deleteAccountActiveTab === 'status' }"
              @click="deleteAccountActiveTab = 'status'"
            >
              <el-icon><Tickets /></el-icon> Case 状态
            </button>
            <button
              class="sd-tab-btn"
              :class="{ active: deleteAccountActiveTab === 'payload' }"
              @click="deleteAccountActiveTab = 'payload'"
            >
              <el-icon><DocumentIcon /></el-icon> 报文详情
            </button>
            <button
              class="sd-tab-btn"
              :class="{ active: deleteAccountActiveTab === 'logs' }"
              @click="deleteAccountActiveTab = 'logs'"
            >
              <el-icon><Cpu /></el-icon> 原始日志
            </button>
          </div>

          <div class="short-drama-tab-body">
            <div v-if="deleteAccountActiveTab === 'status'" class="pipeline-view delete-account-status-view">
              <div class="pipeline-intro">
                <span class="payload-header-bar__eyebrow">Isolated Nested Cases</span>
                <h3>删除账号嵌套执行链路</h3>
                <p>两个 Case 分别配置、顺序执行；匿名登录失败或未经人工确认时，删除账号 Case 不会发送请求。</p>
              </div>
              <div class="delete-account-stage-flow">
                <div class="delete-account-stage">
                  <span>01</span>
                  <div><strong>匿名登录</strong><small>使用卡片中的独立请求配置</small></div>
                </div>
                <el-icon><ArrowRight /></el-icon>
                <div class="delete-account-stage">
                  <span>02</span>
                  <div><strong>人工确认</strong><small>核对用户 ID 与用户名</small></div>
                </div>
                <el-icon><ArrowRight /></el-icon>
                <div class="delete-account-stage delete-account-stage--danger">
                  <span>03</span>
                  <div><strong>永久注销</strong><small>确认后才发送删除请求</small></div>
                </div>
              </div>
              <div class="delete-account-result-list">
                <template v-for="(deleteCase, deleteCaseIndex) in deleteAccountCases" :key="deleteCase.id">
                  <div
                    class="pipeline-card delete-account-result-card"
                    :class="[`pipeline-card--${deleteCase.status}`, { active: deleteAccountSelectedCaseId === deleteCase.id }]"
                    @click="deleteAccountSelectedCaseId = deleteCase.id; deleteAccountActiveTab = 'payload'"
                  >
                    <div class="pipeline-card__indicator">
                      <span v-if="deleteCase.status === 'pending'" class="status-dot pending"></span>
                      <span v-else-if="deleteCase.status === 'running'" class="status-spinner"></span>
                      <el-icon v-else-if="deleteCase.status === 'success'" class="status-icon success" color="#10b981"><Check /></el-icon>
                      <el-icon v-else-if="deleteCase.status === 'failed'" class="status-icon failed" color="#ef4444"><Close /></el-icon>
                      <el-icon v-else class="status-icon skipped" color="#f59e0b"><Warning /></el-icon>
                    </div>
                    <div class="pipeline-card__details">
                      <div class="pipeline-card__title-row">
                        <strong class="step-name">Case {{ deleteCaseIndex + 1 }} · {{ deleteCase.name }}</strong>
                        <span v-if="deleteCase.latency !== undefined" class="step-latency">{{ deleteCase.latency }}ms</span>
                      </div>
                      <div class="pipeline-card__meta-row">
                        <span class="step-badge" :class="deleteCase.method">{{ deleteCase.method }}</span>
                        <span class="step-path">{{ getInterfaceDisplayUrl(deleteCase) }}</span>
                        <span v-if="deleteCase.code" class="step-code" :class="`code-${deleteCase.code}`">HTTP {{ deleteCase.code }}</span>
                      </div>
                      <small v-if="deleteCase.errorMsg" class="delete-account-result-error">{{ deleteCase.errorMsg }}</small>
                    </div>
                    <div class="pipeline-card__action"><el-icon><ArrowRight /></el-icon></div>
                  </div>
                  <div v-if="deleteCaseIndex === 0" class="delete-account-result-connector">
                    <el-icon><ArrowDown /></el-icon>
                    <span>session_token + 人工确认</span>
                  </div>
                </template>
              </div>
            </div>

            <div v-else-if="deleteAccountActiveTab === 'payload'" class="payload-view">
              <div class="payload-details-wrapper">
                <div class="payload-header-bar">
                  <div>
                    <span class="payload-header-bar__eyebrow">Delete Account Payload</span>
                    <strong>{{ selectedDeleteAccountCase.name }} · 请求与响应</strong>
                  </div>
                  <span class="payload-badge" :class="selectedDeleteAccountCase.method">
                    {{ selectedDeleteAccountCase.method }} {{ getInterfaceDisplayUrl(selectedDeleteAccountCase) }}
                  </span>
                </div>
                <div class="payload-split-container">
                  <div class="payload-box">
                    <div class="payload-box-title">Request Payload</div>
                    <pre class="json-code-block json-code-block--request">{{ formatJSON(selectedDeleteAccountCase.requestData) }}</pre>
                  </div>
                  <div class="payload-box">
                    <div class="payload-box-title">Response Payload</div>
                    <pre class="json-code-block json-code-block--response">{{ formatJSON(selectedDeleteAccountCase.responseData) }}</pre>
                  </div>
                </div>
              </div>
            </div>

            <div v-else class="log-content-wrapper" style="height: 100%;">
              <div ref="logContainer" class="log-content" style="height: 100%; overflow-y: auto;">
                <div v-for="(log, idx) in visibleLogs" :key="idx" class="log-line">{{ log }}</div>
              </div>
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
                    <el-icon v-else class="status-icon skipped" color="#f59e0b"><Warning /></el-icon>
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
                    <div class="payload-box-title payload-box-title--with-action">
                      <span>Response Payload</span>
                      <el-button
                        link
                        type="success"
                        size="small"
                        :disabled="selectedStepForPayload.responseData === undefined"
                        @click="addExtractorFromResponse(selectedStepForPayload)"
                      >
                        提取为变量
                      </el-button>
                    </div>
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
          :disabled="visibleStatus === 'Executing' || (isShortDramaApiTest && activePipelineSteps.length === 0)"
          @click="startExecution"
        >
          <el-icon><VideoPlay /></el-icon>
          {{ isDeleteAccount ? '执行嵌套 Cases' : 'Execute' }}
        </button>
      </div>
    </div>

    <DramaRulesDialog v-if="showDramaRulesEntry" v-model="dramaRulesVisible" />
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

.sidebar-panel--delete-account {
  width: clamp(430px, 30vw, 540px);
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

.log-panel--delete-account {
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
.delete-account-safety-tip {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid #fed7aa;
  border-radius: 10px;
  background: #fff7ed;
  color: #9a3412;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.55;
}

.delete-account-safety-tip .el-icon {
  margin-top: 2px;
  flex-shrink: 0;
  color: #ea580c;
  font-size: 14px;
}

.delete-account-settings-grid {
  margin-top: 12px;
}

.delete-account-project-setting--disabled {
  border-color: #e5e7eb;
  background: #f1f5f9;
}

.delete-account-project-setting--disabled .short-drama-setting__label {
  color: #a8b1bf;
}

.delete-account-project-setting--disabled :deep(.el-select__wrapper) {
  background: #e9eef5;
  box-shadow: 0 0 0 1px #dde3eb inset;
}

.delete-account-custom-domain-setting {
  display: block;
  margin-top: 10px;
}

.delete-account-custom-domain-setting > small {
  display: block;
  margin-top: 7px;
  color: #94a3b8;
  font-size: 9px;
  line-height: 1.45;
}

.delete-account-login-info-card {
  display: flex;
  flex-direction: column;
  gap: 9px;
  margin-top: 12px;
  padding: 12px;
  border: 1px solid #dbe5f1;
  border-radius: 10px;
  background: linear-gradient(145deg, #f8fbff 0%, #ffffff 72%);
}

.delete-account-login-info-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.delete-account-login-info-card__header > div {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 3px;
}

.delete-account-login-info-card__header strong {
  color: #0f172a;
  font-size: 13px;
}

.delete-account-login-info-card__header small {
  color: #94a3b8;
  font-size: 10px;
}

.delete-account-login-info-card__header .el-button {
  flex-shrink: 0;
  border-radius: 7px;
  font-size: 11px;
  font-weight: 700;
}

.delete-account-login-info-input :deep(.el-textarea__inner) {
  padding: 10px 11px;
  border: 0;
  border-radius: 8px;
  background: #0f172a;
  color: #dbeafe;
  caret-color: #93c5fd;
  box-shadow: 0 0 0 1px #263449 inset;
  font-family: 'Menlo', 'Monaco', 'Courier New', monospace;
  font-size: 10px;
  line-height: 1.55;
}

.delete-account-login-info-input :deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #60a5fa inset, 0 0 0 3px rgba(59, 130, 246, 0.1);
}

.delete-account-login-info-card__result {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  color: #64748b;
  font-size: 10px;
  line-height: 1.45;
}

.delete-account-login-info-card__dot {
  width: 6px;
  height: 6px;
  flex-shrink: 0;
  border-radius: 50%;
  background: #94a3b8;
}

.delete-account-login-info-card__result--success {
  color: #15803d;
}

.delete-account-login-info-card__result--success .delete-account-login-info-card__dot {
  background: #22c55e;
}

.delete-account-login-info-card__result--error {
  color: #dc2626;
}

.delete-account-login-info-card__result--error .delete-account-login-info-card__dot {
  background: #ef4444;
}

.delete-account-login-info-card > p {
  margin: 0;
  color: #94a3b8;
  font-size: 9px;
  line-height: 1.5;
}

.delete-account-case-card {
  margin-top: 12px;
  border-color: #fed7aa;
  box-shadow: 0 10px 28px rgba(234, 88, 12, 0.08);
}

.delete-account-case-card--selected {
  border-color: #fb923c;
  box-shadow: 0 0 0 2px rgba(251, 146, 60, 0.12), 0 12px 30px rgba(234, 88, 12, 0.1);
}

.delete-account-case-card__header {
  cursor: pointer;
  background: linear-gradient(135deg, #fff7ed 0%, #ffffff 75%);
}

.delete-account-case-order {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  flex-shrink: 0;
  border-radius: 7px;
  background: #ffedd5;
  color: #c2410c;
  font-size: 10px;
  font-weight: 900;
}

.delete-account-case-card__title {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 3px;
}

.delete-account-case-card__title strong {
  color: #0f172a;
  font-size: 14px;
}

.delete-account-case-card__title small {
  overflow: hidden;
  color: #94a3b8;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.delete-account-case-card__body {
  padding-top: 12px;
}

.delete-account-nested-connector {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 8px 0 0;
  color: #c2410c;
  font-size: 10px;
  font-weight: 800;
}

.delete-account-nested-connector .el-icon {
  font-size: 16px;
}

.delete-account-isolation-badge {
  padding: 3px 7px;
  border: 1px solid rgba(251, 146, 60, 0.28);
  border-radius: 999px;
  background: rgba(234, 88, 12, 0.12);
  color: #fdba74;
  font-size: 9px;
  font-weight: 800;
}

.delete-account-tabs-header .sd-tab-btn.active {
  background: #ea580c;
  box-shadow: 0 2px 6px rgba(234, 88, 12, 0.2);
}

.delete-account-status-view {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.delete-account-stage-flow {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 20px minmax(0, 1fr) 20px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
}

.delete-account-stage-flow > .el-icon {
  justify-self: center;
  color: #cbd5e1;
}

.delete-account-stage {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #f8fafc;
}

.delete-account-stage > span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  border-radius: 8px;
  background: #e0f2fe;
  color: #0369a1;
  font-size: 10px;
  font-weight: 900;
}

.delete-account-stage > div {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 3px;
}

.delete-account-stage strong {
  color: #1e293b;
  font-size: 12px;
}

.delete-account-stage small {
  overflow: hidden;
  color: #94a3b8;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.delete-account-stage--danger {
  border-color: #fecaca;
  background: #fff1f2;
}

.delete-account-stage--danger > span {
  background: #fee2e2;
  color: #b91c1c;
}

.delete-account-result-card {
  margin-top: 4px;
}

.delete-account-result-list {
  display: flex;
  flex-direction: column;
}

.delete-account-result-connector {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-height: 34px;
  color: #c2410c;
  font-size: 10px;
  font-weight: 800;
}

.delete-account-result-error {
  margin-top: 4px;
  color: #dc2626;
  font-size: 11px;
}

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

.api-test-suite-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  padding: 28px 18px;
  border: 1px dashed #cbd5e1;
  border-radius: 12px;
  background: #f8fafc;
  color: #475569;
  font-size: 14px;
  font-weight: 800;
  text-align: center;
}

.api-test-suite-empty small {
  color: #94a3b8;
  font-size: 12px;
  font-weight: 600;
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

.api-status-chip--skipped {
  background: #fef3c7;
  color: #92400e;
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

.json-editor-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
}

.json-editor-heading .inner-label {
  margin-bottom: 0;
}

.json-editor-action {
  height: 22px;
  padding: 0 4px;
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

.json-command-preview__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 22px;
}

.json-command-preview__actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.json-quick-fill-btn {
  height: 26px;
  padding: 0 10px;
  border: 1px solid rgba(96, 165, 250, 0.34);
  background: rgba(37, 99, 235, 0.13);
  color: #93c5fd;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.01em;
  box-shadow: 0 5px 14px rgba(2, 6, 23, 0.2);
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease, transform 0.18s ease;
}

.json-quick-fill-btn:not(.is-disabled):hover {
  border-color: rgba(147, 197, 253, 0.72);
  background: rgba(37, 99, 235, 0.24);
  color: #dbeafe;
  transform: translateY(-1px);
}

.json-quick-fill-btn.is-disabled {
  border-color: rgba(71, 85, 105, 0.35);
  background: rgba(51, 65, 85, 0.12);
  color: #475569;
  box-shadow: none;
}

.json-quick-fill-btn__arrow {
  margin-left: 1px;
  font-size: 10px;
}

.json-variable-btn {
  height: 26px;
  padding: 0 10px;
  border: 1px solid rgba(52, 211, 153, 0.32);
  background: rgba(5, 150, 105, 0.12);
  color: #6ee7b7;
  font-size: 11px;
  font-weight: 800;
  box-shadow: 0 5px 14px rgba(2, 6, 23, 0.18);
}

.json-variable-btn:not(.is-disabled):hover {
  border-color: rgba(110, 231, 183, 0.7);
  background: rgba(5, 150, 105, 0.22);
  color: #d1fae5;
}

.json-variable-btn.is-disabled {
  border-color: rgba(71, 85, 105, 0.35);
  background: rgba(51, 65, 85, 0.12);
  color: #475569;
  box-shadow: none;
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

.api-variable-extractor-panel {
  min-width: 0;
  max-width: 100%;
  box-sizing: border-box;
  padding: 12px;
  border: 1px solid #dbe4f0;
  border-radius: 10px;
  background: linear-gradient(180deg, #f8fbff 0%, #ffffff 100%);
}

.api-variable-extractor-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
  flex-wrap: wrap;
}

.api-variable-extractor-panel__header > div { min-width: 0; flex: 1 1 150px; }
.api-variable-extractor-panel__header small { display: block; overflow-wrap: anywhere; }
.api-variable-add-btn { flex-shrink: 0; }

.api-variable-extractor-panel__header .inner-label {
  margin-bottom: 2px;
  color: #334155;
}

.api-variable-extractor-panel__header small {
  color: #94a3b8;
  font-size: 11px;
}

.api-variable-add-btn {
  height: 26px;
  padding: 0 10px;
  border-radius: 7px;
  font-size: 11px;
  font-weight: 800;
}

.api-variable-extractor-empty {
  overflow-wrap: anywhere;
  padding: 12px;
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.72);
  color: #94a3b8;
  font-size: 11px;
  line-height: 1.5;
  text-align: center;
}

.api-variable-extractor-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  align-items: center;
  gap: 7px;
  padding: 8px;
  border: 1px solid #e2e8f0;
  border-radius: 9px;
  background: #ffffff;
  min-width: 0;
}

.api-variable-extractor-row > .el-input,
.api-variable-extractor-row > .el-select { width: 100%; min-width: 0; }
.api-variable-path-input { grid-column: 1 / -1; }
.api-variable-extractor-actions {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 16px;
  min-width: 0;
  padding-top: 4px;
}
.api-variable-extractor-actions .api-variable-extractor-delete { margin-left: auto; flex-shrink: 0; }

.api-variable-extractor-row + .api-variable-extractor-row {
  margin-top: 7px;
}

.api-variable-name-input :deep(.el-input__inner),
.api-variable-path-input :deep(.el-input__inner) {
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 11px;
}

.api-variable-option {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #64748b;
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
}

.api-variable-runtime-result {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 9px;
  padding-top: 9px;
  border-top: 1px solid #e2e8f0;
}

.api-variable-runtime-result__label {
  color: #64748b;
  font-size: 11px;
  font-weight: 800;
}

.api-variable-runtime-chip {
  min-width: 0;
  max-width: 100%;
  box-sizing: border-box;
  flex-wrap: wrap;
  overflow-wrap: anywhere;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 7px;
  border-radius: 7px;
  background: #ecfdf5;
  color: #047857;
  font-size: 10px;
}

.api-variable-runtime-chip code {
  min-width: 0;
  font-weight: 900;
}

.api-variable-runtime-chip span {
  min-width: 0;
  color: #059669;
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
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

.pipeline-card--skipped {
  border-left: 4px solid #f59e0b;
  background: #fffbeb;
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

.payload-box-title--with-action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.payload-box-title--with-action .el-button {
  height: 20px;
  padding: 0;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0;
  text-transform: none;
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
  width: min(440px, calc(100vw - 32px)) !important;
  overflow: hidden;
  padding: 0 !important;
  border: 1px solid #e2e8f0 !important;
  border-radius: 14px !important;
  background: #ffffff !important;
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.18), 0 5px 16px rgba(15, 23, 42, 0.08) !important;
}

.delete-account-confirm-box .el-message-box__header {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 58px;
  padding: 0 52px !important;
  border-bottom: 1px solid #eef2f7;
}

.delete-account-confirm-box .el-message-box__title {
  font-size: 16px;
  font-weight: 800;
  color: #0f172a;
  letter-spacing: -0.01em;
  text-align: center;
}

.delete-account-confirm-box .el-message-box__headerbtn {
  top: 17px;
  right: 18px;
  width: 24px;
  height: 24px;
  border-radius: 7px;
  transition: background 0.18s ease;
}

.delete-account-confirm-box .el-message-box__headerbtn:hover {
  background: #f1f5f9;
}

.delete-account-confirm-box .el-message-box__headerbtn:hover .el-message-box__close {
  color: #334155;
}

.delete-account-confirm-box .el-message-box__content {
  padding: 18px 20px 16px !important;
}

.delete-account-confirm-box .el-message-box__container,
.delete-account-confirm-box .el-message-box__message {
  width: 100%;
}

.delete-account-confirm-content {
  display: flex;
  flex-direction: column;
  gap: 14px;
  color: #475569;
}

.delete-account-confirm-lead {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  text-align: center;
}

.delete-account-confirm-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 8px;
  border-radius: 999px;
  background: #ecfdf5;
  color: #15803d;
  font-size: 9px;
  font-weight: 900;
  letter-spacing: 0.04em;
}

.delete-account-confirm-badge i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  box-shadow: 0 0 0 3px rgba(34, 197, 94, 0.12);
}

.delete-account-confirm-lead p {
  max-width: 330px;
  margin: 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.55;
}

.delete-account-confirm-user-card {
  overflow: hidden;
  border: 1px solid #dbe5f1;
  border-radius: 11px;
  background: #f8fafc;
}

.delete-account-confirm-user-card__title {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 11px 12px;
  border-bottom: 1px solid #e2e8f0;
  background: linear-gradient(135deg, #eff6ff 0%, #f8fbff 72%);
}

.delete-account-confirm-user-icon {
  display: inline-flex;
  width: 30px;
  height: 30px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  background: #dbeafe;
  color: #1d4ed8;
  font: 900 9px/1 'Menlo', 'Monaco', monospace;
}

.delete-account-confirm-user-card__title > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.delete-account-confirm-user-card__title strong {
  color: #0f172a;
  font-size: 12px;
  font-weight: 800;
}

.delete-account-confirm-user-card__title small {
  color: #94a3b8;
  font-size: 9px;
}

.delete-account-confirm-user-row {
  display: grid;
  grid-template-columns: 78px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-bottom: 1px solid #e8edf3;
  background: #ffffff;
}

.delete-account-confirm-user-row:last-child {
  border-bottom: 0;
}

.delete-account-confirm-user-row > span {
  text-align: right;
  color: #94a3b8;
  font-size: 10px;
  font-weight: 700;
}

.delete-account-confirm-user-row code {
  overflow: hidden;
  color: #1d4ed8;
  font: 700 11px/1.4 'Menlo', 'Monaco', monospace;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.delete-account-confirm-user-row > strong {
  overflow: hidden;
  color: #1e293b;
  font-size: 12px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.delete-account-confirm-flow {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 2px 10px;
}

.delete-account-confirm-flow span {
  padding: 5px 9px;
  border-radius: 7px;
  font-size: 9px;
  font-weight: 800;
}

.delete-account-confirm-flow .is-finished {
  background: #ecfdf5;
  color: #15803d;
}

.delete-account-confirm-flow .is-danger {
  background: #fef2f2;
  color: #dc2626;
}

.delete-account-confirm-flow i {
  position: relative;
  width: 42px;
  height: 1px;
  background: #cbd5e1;
}

.delete-account-confirm-flow i::after {
  position: absolute;
  top: -3px;
  right: 0;
  width: 6px;
  height: 6px;
  border-top: 1px solid #94a3b8;
  border-right: 1px solid #94a3b8;
  content: '';
  transform: rotate(45deg);
}

.delete-account-confirm-warning {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  padding: 11px 12px;
  border: 1px solid #fecaca;
  border-radius: 9px;
  background: #fff7f7;
}

.delete-account-confirm-warning__icon {
  display: inline-flex;
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #fee2e2;
  color: #dc2626;
  font-size: 12px;
  font-weight: 900;
}

.delete-account-confirm-warning > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.delete-account-confirm-warning strong {
  color: #b91c1c;
  font-size: 11px;
}

.delete-account-confirm-warning div span {
  color: #7f1d1d;
  font-size: 10px;
  line-height: 1.5;
}

.delete-account-confirm-box .el-message-box__btns {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  padding: 14px 20px 18px !important;
  border-top: 1px solid #eef2f7;
  background: #f8fafc;
}

.delete-account-confirm-box .delete-account-confirm-submit,
.delete-account-confirm-box .delete-account-confirm-cancel {
  min-width: 104px;
  height: 36px;
  margin-left: 0 !important;
  padding: 0 16px !important;
  border-radius: 8px !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.delete-account-confirm-box .delete-account-confirm-cancel {
  border-color: #dbe5f1 !important;
  background: #ffffff !important;
  color: #64748b !important;
}

.delete-account-confirm-box .delete-account-confirm-cancel:hover {
  border-color: #bfdbfe !important;
  background: #f8fbff !important;
  color: #2563eb !important;
}

.delete-account-confirm-box .delete-account-confirm-submit {
  border-color: #ef4444 !important;
  background: #ef4444 !important;
  color: #ffffff !important;
  box-shadow: 0 4px 10px rgba(239, 68, 68, 0.22);
}

.delete-account-confirm-box .delete-account-confirm-submit:hover {
  border-color: #dc2626 !important;
  background: #dc2626 !important;
  box-shadow: 0 6px 14px rgba(220, 38, 38, 0.28);
  transform: translateY(-1px);
}

/* 短剧接口测试：一键带入下拉框会 teleport 到 body，需使用全局样式。 */
.json-quick-fill-popper.el-popper {
  overflow: hidden;
  min-width: 330px;
  padding: 0 !important;
  border: 1px solid #dbe4f0 !important;
  border-radius: 14px !important;
  background: rgba(255, 255, 255, 0.98) !important;
  box-shadow: 0 20px 48px rgba(15, 23, 42, 0.16), 0 4px 12px rgba(15, 23, 42, 0.06) !important;
  backdrop-filter: blur(14px);
}

.json-quick-fill-popper .el-popper__arrow::before {
  border-color: #dbe4f0 !important;
  background: #ffffff !important;
}

.json-quick-fill-popper .el-dropdown-menu {
  max-height: 360px;
  margin: 0;
  padding: 7px;
  overflow-y: auto;
  background: transparent;
}

.json-quick-fill-popper .el-dropdown-menu__item {
  min-height: 58px;
  margin: 3px 0;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 10px;
  color: #334155;
  line-height: 1.25;
  transition: border-color 0.16s ease, background 0.16s ease, transform 0.16s ease;
}

.json-quick-fill-popper .el-dropdown-menu__item:not(.is-disabled):focus,
.json-quick-fill-popper .el-dropdown-menu__item:not(.is-disabled):hover {
  border-color: #bfdbfe;
  background: linear-gradient(135deg, #eff6ff 0%, #f8fbff 100%);
  color: #1d4ed8;
  transform: translateX(2px);
}

.json-quick-fill-popper .el-dropdown-menu__item.json-quick-fill-menu__header {
  min-height: 42px;
  margin: 0 0 5px;
  padding: 6px 9px 9px;
  border-bottom: 1px solid #e8eef6;
  border-radius: 8px 8px 0 0;
  opacity: 1;
  cursor: default;
}

.json-quick-fill-menu__title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 16px;
}

.json-quick-fill-menu__title span {
  color: #0f172a;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.01em;
}

.json-quick-fill-menu__title small {
  padding: 3px 7px;
  border-radius: 999px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 10px;
  font-weight: 800;
}

.json-quick-fill-option {
  display: grid;
  grid-template-columns: 54px minmax(0, 1fr) 16px;
  align-items: center;
  width: 100%;
  min-width: 300px;
  gap: 10px;
}

.json-quick-fill-method {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 50px;
  height: 24px;
  padding: 0 6px;
  border-radius: 7px;
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 10px;
  font-weight: 900;
}

.json-quick-fill-method.is-get {
  background: #e0f2fe;
  color: #0369a1;
}

.json-quick-fill-method.is-post {
  background: #dcfce7;
  color: #15803d;
}

.json-quick-fill-method.is-put,
.json-quick-fill-method.is-patch {
  background: #fef3c7;
  color: #92400e;
}

.json-quick-fill-method.is-delete {
  background: #fee2e2;
  color: #b91c1c;
}

.json-quick-fill-option__copy {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 4px;
}

.json-quick-fill-option__copy strong,
.json-quick-fill-option__copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.json-quick-fill-option__copy strong {
  color: #1e293b;
  font-size: 13px;
  font-weight: 750;
}

.json-quick-fill-option__copy small {
  color: #94a3b8;
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 10px;
}

.json-quick-fill-option__arrow {
  color: #cbd5e1;
  font-size: 12px;
  transition: color 0.16s ease, transform 0.16s ease;
}

.json-quick-fill-popper .el-dropdown-menu__item:hover .json-quick-fill-option__arrow {
  color: #3b82f6;
  transform: translateX(2px);
}

.api-variable-popper.el-popper {
  min-width: 280px;
  overflow: hidden;
  padding: 0 !important;
  border: 1px solid #ccebdc !important;
  border-radius: 12px !important;
  background: rgba(255, 255, 255, 0.98) !important;
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.14), 0 4px 10px rgba(15, 23, 42, 0.05) !important;
}

.api-variable-popper .el-popper__arrow::before {
  border-color: #ccebdc !important;
  background: #ffffff !important;
}

.api-variable-popper .el-dropdown-menu {
  max-height: 320px;
  margin: 0;
  padding: 7px;
  overflow-y: auto;
}

.api-variable-popper .el-dropdown-menu__item {
  min-height: 48px;
  margin: 2px 0;
  padding: 7px 9px;
  border: 1px solid transparent;
  border-radius: 9px;
  color: #334155;
}

.api-variable-popper .el-dropdown-menu__item:not(.is-disabled):hover,
.api-variable-popper .el-dropdown-menu__item:not(.is-disabled):focus {
  border-color: #a7f3d0;
  background: #ecfdf5;
  color: #047857;
}

.api-variable-popper .el-dropdown-menu__item.api-variable-menu__header {
  min-height: 34px;
  color: #64748b;
  font-size: 11px;
  font-weight: 800;
  opacity: 1;
}

.api-variable-menu__item {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 245px;
  gap: 4px;
}

.api-variable-menu__item code {
  color: #047857;
  font-family: 'JetBrains Mono', 'Menlo', 'Monaco', 'Consolas', monospace;
  font-size: 12px;
  font-weight: 850;
}

.api-variable-menu__item small {
  overflow: hidden;
  color: #94a3b8;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
