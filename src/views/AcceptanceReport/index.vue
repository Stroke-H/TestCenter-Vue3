<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, watch, markRaw, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import { Plus, Search, Calendar, User, Money, Loading, Close, DocumentCopy } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { retryFetch } from '@/utils/retryFetch'
import { notifyAcceptanceReportsChanged } from '@/utils/acceptanceReportEvents'

defineOptions({ name: 'AcceptanceReport' })

// --- API Service ---
const API_BASE = '/api'
const authStore = useAuthStore()
const route = useRoute()

interface ProjectOption {
  id: string
  project_code: string
  project_name: string
  short_code?: string
  wiki_url?: string
}

interface DeviceOption {
  id: string
  device_name: string
  os: string
  model: string
  allowed_app: string
  created_at?: string
}

// ===== State =====
const stats = ref([
  { title: '报告总数', value: 0, icon: markRaw(Calendar), color: '#3b82f6', bgColor: '#eff6ff' },
  { title: '测试负责人', value: 0, icon: markRaw(User), color: '#eab308', bgColor: '#fefce8' },
  { title: '覆盖项目数', value: 0, icon: markRaw(Money), color: '#f97316', bgColor: '#fff7ed' }
])

const recentReports = ref<any[]>([])
const historyProjects = ref<any[]>([])
const searchQuery = ref('')
const categoryFilter = ref('')
const selectedProjectCodeFilter = ref('')
const projects = ref<ProjectOption[]>([])
const devices = ref<DeviceOption[]>([])
const testTimeRange = ref<string[]>([])
const selectedTestDevices = ref<string[]>([])
const deviceSelectRef = ref<any>(null)
const deviceFieldRef = ref<HTMLElement | null>(null)
const deviceAssociationVisible = ref(false)
const deviceAssociationSearch = ref('')
const pendingDeviceIds = ref<string[]>([])
const associatingDevices = ref(false)
const deviceAssociationPanelStyle = ref<Record<string, string>>({})
const autoFetchingDefectTitles = ref(false)
const autoFetchDefectTitlesError = ref('')
const requirementsPickerVisible = ref(false)
const requirementsLoading = ref(false)
const requirementsLoadError = ref('')
const requirementsProjectCode = ref('')
const requirementsReportID = ref('')
const requirementsReports = ref<any[]>([])
let autoFetchDefectTitlesTimer: ReturnType<typeof setTimeout> | null = null
let autoFetchDefectTitlesSeq = 0

// --- Preview State ---
const previewVisible = ref(false)
const currentPreview = ref<any>(null)
const previewMode = ref<'view' | 'create' | 'edit'>('view')
const sendingToFeishu = ref(false)
const syncingToCloudDoc = ref(false)
const reportForm = ref<any>({
  project_name: '',
  project_code: '',
  version: '',
  reporter: '',
  test_owner: '',
  test_time: '',
  test_env: '',
  test_devices: '',
  test_conclusion: 'Pass',
  update_requirements: '',
  bug_submission_status: '',
  bug_fix_status: '',
  status: 'Completed'
})

// --- Fetch Logic ---
const fetchReports = async () => {
  try {
    const res = await retryFetch(`${API_BASE}/acceptance-reports/list`, {
      credentials: 'include',
      headers: {
        'Authorization': authStore.token
      }
    })
    if (!res.ok) {
      const rawText = await res.text()
      throw new Error(rawText || `请求失败 (${res.status})`)
    }
    const data = await res.json()
    if (data && Array.isArray(data)) {
      recentReports.value = data.map(r => ({
        ...r, // Keep original data for preview
        avatar: r.reporter ? r.reporter.charAt(0).toUpperCase() : 'R',
        reporter_display: `${r.reporter} | ${r.project_name}`,
        date: r.created_at ? new Date(r.created_at).toISOString().split('T')[0] : '-',
        status: r.status || 'Completed'
      }))

      // Update Stats
      if (stats.value[0]) {
        stats.value[0].value = data.length
      }
      if (stats.value[1]) {
        const uniqueReporters = new Set(
          data
            .map((r: any) => (r.reporter || '').trim())
            .filter((reporter: string) => reporter)
        )
        stats.value[1].value = uniqueReporters.size
      }
      
      // Update History Projects (Count per Code)
      const counts: Record<string, number> = {}
      data.forEach((r: any) => {
        counts[r.project_code] = (counts[r.project_code] || 0) + 1
      })
      historyProjects.value = Object.entries(counts).map(([id, count]) => ({ id, count }))
      if (stats.value[2]) {
        stats.value[2].value = historyProjects.value.length
      }
    }
  } catch (err: any) {
    console.error('Failed to fetch reports', err)
    if (String(err?.message || '').includes('Unauthorized')) {
      ElMessage.warning('当前登录态未就绪或已失效，请重新登录后重试')
    }
  }
}

const fetchProjects = async () => {
  try {
    const res = await retryFetch(`${API_BASE}/config/projects`)
    const data = await res.json()
    projects.value = Array.isArray(data)
      ? data.map((item: ProjectOption) => ({
          ...item,
          project_code: String(item.project_code || '').trim()
        }))
      : []
  } catch (err) {
    console.error('Failed to fetch projects', err)
  }
}

const fetchDevices = async () => {
  try {
    const res = await retryFetch(`${API_BASE}/config/devices`)
    const data = await res.json()
    devices.value = Array.isArray(data) ? data : []
  } catch (err) {
    console.error('Failed to fetch devices', err)
  }
}

const selectedProject = computed(() => {
  if (reportForm.value.project_code) {
    return projects.value.find(item => item.project_code === reportForm.value.project_code) || null
  }
  if (reportForm.value.project_name) {
    return projects.value.find(item => item.project_name === reportForm.value.project_name) || null
  }
  return null
})

const currentPreviewProject = computed(() => {
  if (!currentPreview.value?.project_code && !currentPreview.value?.project_name) return null

  return projects.value.find(item =>
    item.project_code === currentPreview.value?.project_code ||
    item.project_name === currentPreview.value?.project_name
  ) || null
})

const filteredDevices = computed(() => {
  const projectCode = String(reportForm.value.project_code || '').trim()
  if (!projectCode) return []
  return devices.value.filter((item) => {
    const allowedApps = (item.allowed_app || '').split(',').map(app => app.trim()).filter(Boolean)
    return allowedApps.includes(projectCode)
  })
})

const getDeviceProjectCodes = (device: DeviceOption) => {
  return (device.allowed_app || '').split(',').map(code => code.trim()).filter(Boolean)
}

const isDeviceAssociatedWithCurrentProject = (device: DeviceOption) => {
  const projectCode = String(reportForm.value.project_code || '').trim()
  return !!projectCode && getDeviceProjectCodes(device).includes(projectCode)
}

const deviceAssociationItems = computed(() => {
  const keyword = deviceAssociationSearch.value.trim().toLowerCase()

  return devices.value
    .filter((device) => {
      if (!keyword) return true
      return [device.device_name, device.model, device.os, device.id]
        .some(value => String(value || '').toLowerCase().includes(keyword))
    })
    .map(device => ({
      device,
      associated: isDeviceAssociatedWithCurrentProject(device)
    }))
    .sort((left, right) => {
      if (left.associated !== right.associated) return left.associated ? 1 : -1
      return left.device.device_name.localeCompare(right.device.device_name, 'zh-CN')
    })
})

const updateDeviceAssociationPanelPosition = () => {
  if (!deviceAssociationVisible.value || !deviceFieldRef.value) return

  const fieldRect = deviceFieldRef.value.getBoundingClientRect()
  const dialog = deviceFieldRef.value.closest('.el-dialog') as HTMLElement | null
  const dialogRect = dialog?.getBoundingClientRect() || fieldRect
  const viewportPadding = 12
  const gap = 6
  const preferredWidth = 340
  const panelWidth = Math.min(preferredWidth, window.innerWidth - viewportPadding * 2)
  let left = dialogRect.right + gap

  if (left + panelWidth > window.innerWidth - viewportPadding) {
    left = Math.max(viewportPadding, dialogRect.left - panelWidth - gap)
  }

  const top = Math.max(viewportPadding, fieldRect.top)
  deviceAssociationPanelStyle.value = {
    left: `${Math.round(left)}px`,
    top: `${Math.round(top)}px`,
    width: `${Math.round(panelWidth)}px`,
    maxHeight: `${Math.max(280, Math.round(window.innerHeight - top - viewportPadding))}px`
  }
}

const closeDeviceAssociationPanel = () => {
  if (associatingDevices.value) return
  deviceAssociationVisible.value = false
  deviceAssociationSearch.value = ''
  pendingDeviceIds.value = []
}

const openDeviceAssociationPanel = async () => {
  if (!reportForm.value.project_code) {
    ElMessage.warning('请先选择项目，再添加关联设备')
    return
  }

  pendingDeviceIds.value = []
  deviceAssociationSearch.value = ''
  deviceAssociationVisible.value = true
  await nextTick()
  updateDeviceAssociationPanelPosition()
}

const togglePendingDevice = (deviceId: string, associated: boolean) => {
  if (associated || associatingDevices.value) return
  if (pendingDeviceIds.value.includes(deviceId)) {
    pendingDeviceIds.value = pendingDeviceIds.value.filter(id => id !== deviceId)
    return
  }
  pendingDeviceIds.value = [...pendingDeviceIds.value, deviceId]
}

const saveDeviceAssociations = async () => {
  const projectCode = String(reportForm.value.project_code || '').trim()
  if (!projectCode) {
    ElMessage.warning('请先选择项目')
    return
  }
  if (pendingDeviceIds.value.length === 0) {
    ElMessage.warning('请至少选择一台设备')
    return
  }

  associatingDevices.value = true
  const succeededIds: string[] = []
  const failedIds: string[] = []

  for (const deviceId of pendingDeviceIds.value) {
    const device = devices.value.find(item => item.id === deviceId)
    if (!device || isDeviceAssociatedWithCurrentProject(device)) continue

    const updatedDevice = {
      ...device,
      allowed_app: [...new Set([...getDeviceProjectCodes(device), projectCode])].join(',')
    }

    try {
      const res = await fetch(`${API_BASE}/config/devices`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updatedDevice)
      })
      if (!res.ok) {
        const data = await res.json().catch(() => null)
        throw new Error(data?.error || `关联失败 (${res.status})`)
      }
      const index = devices.value.findIndex(item => item.id === deviceId)
      if (index >= 0) devices.value[index] = updatedDevice
      succeededIds.push(deviceId)
    } catch (err) {
      console.error(`Failed to associate device ${deviceId}`, err)
      failedIds.push(deviceId)
    }
  }

  associatingDevices.value = false
  await fetchDevices()

  if (failedIds.length > 0) {
    pendingDeviceIds.value = failedIds
    ElMessage.error(`已关联 ${succeededIds.length} 台设备，${failedIds.length} 台关联失败，请重试`)
    return
  }

  deviceAssociationVisible.value = false
  deviceAssociationSearch.value = ''
  pendingDeviceIds.value = []
  ElMessage.success(`已为 ${projectCode} 关联 ${succeededIds.length} 台设备`)
  await nextTick()
  deviceSelectRef.value?.focus?.()
}

const syncProjectByCode = (code: string) => {
  const matched = projects.value.find(item => item.project_code === code)
  if (matched) {
    reportForm.value.project_name = matched.project_name
  }
}

const syncProjectByName = (name: string) => {
  const matched = projects.value.find(item => item.project_name === name)
  if (matched) {
    reportForm.value.project_code = matched.project_code
  }
}

const handleRowClick = (row: any) => {
  previewMode.value = 'view'
  currentPreview.value = row
  previewVisible.value = true
}

const openReportById = (reportId: string) => {
  const matchedReport = recentReports.value.find((report) => report.id === reportId)
  if (matchedReport) {
    handleRowClick(matchedReport)
  }
}

const canSendToFeishu = computed(() => {
  return previewMode.value === 'view' &&
    !!currentPreview.value?.id &&
    !!authStore.user?.username &&
    authStore.user.username === currentPreview.value?.reporter
})

const canSyncToCloudDoc = computed(() => {
  return canSendToFeishu.value && !!currentPreviewProject.value?.wiki_url?.trim()
})

const canEditReport = computed(() => {
  return previewMode.value === 'view' &&
    !!currentPreview.value?.id &&
    !!authStore.user?.username &&
    authStore.user.username === currentPreview.value?.reporter
})

const filteredReports = computed(() => {
  const keyword = searchQuery.value.trim().toLowerCase()

  return recentReports.value.filter((report) => {
    if (selectedProjectCodeFilter.value && report.project_code !== selectedProjectCodeFilter.value) {
      return false
    }

    if (!keyword) return true

    const searchableText = [
      report.reporter,
      report.project_name,
      report.project_code,
      report.reporter_display
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()

    return searchableText.includes(keyword)
  })
})

const toggleProjectCodeFilter = (projectCode: string) => {
  selectedProjectCodeFilter.value = selectedProjectCodeFilter.value === projectCode ? '' : projectCode
}

const showDefectTitlesLoading = computed(() => {
  return previewMode.value === 'create' && autoFetchingDefectTitles.value
})

const formatManagedDefectTitles = (titles: string[]) => {
  const normalized = titles.map(title => String(title || '').trim()).filter(Boolean)
  return normalized.length > 0
    ? normalized.map((title, index) => `${index + 1}. ${title}`).join('\n')
    : '无'
}

const normalizeProjectCode = (value: string) => value.trim().toUpperCase()

const getReportTimestamp = (report: any) => {
  const parsedTime = Date.parse(report?.updated_at || report?.created_at || '')
  if (!Number.isNaN(parsedTime)) return parsedTime

  const idTime = String(report?.id || '').match(/\d{10,}/)?.[0]
  return idTime ? Number(idTime) : 0
}

const requirementsProjects = computed(() => {
  const options = new Map<string, { code: string; name: string }>()
  for (const project of projects.value) {
    const code = normalizeProjectCode(project.project_code || '')
    if (code) options.set(code, { code, name: project.project_name })
  }
  for (const report of requirementsReports.value) {
    const code = normalizeProjectCode(report.project_code || '')
    if (code && !options.has(code)) options.set(code, { code, name: report.project_name || '' })
  }
  return [...options.values()].sort((a, b) => a.code.localeCompare(b.code, undefined, { numeric: true }))
})

const selectableRequirementReports = computed(() => requirementsReports.value
  .filter(report => normalizeProjectCode(String(report.project_code || '')) === requirementsProjectCode.value
    && String(report.update_requirements || '').trim())
  .sort((a, b) => getReportTimestamp(b) - getReportTimestamp(a)))

const selectedRequirementReport = computed(() => selectableRequirementReports.value
  .find(report => report.id === requirementsReportID.value))

const formatRequirementReportTime = (report: any) => {
  const timestamp = getReportTimestamp(report)
  return timestamp ? new Date(timestamp).toLocaleString('zh-CN', { hour12: false }) : '时间未记录'
}

const openRequirementsPicker = async () => {
  requirementsProjectCode.value = ''
  requirementsReportID.value = ''
  requirementsReports.value = []
  requirementsLoadError.value = ''
  requirementsPickerVisible.value = true
  requirementsLoading.value = true
  try {
    const response = await retryFetch(`${API_BASE}/acceptance-reports/list`, {
      credentials: 'include', headers: { Authorization: authStore.token }
    })
    if (!response.ok) throw new Error(`加载历史报告失败 (${response.status})`)
    const data = await response.json()
    if (!Array.isArray(data)) throw new Error('历史报告数据格式异常')
    requirementsReports.value = data
    const latestReport = [...data]
      .filter(report => String(report.project_code || '').trim())
      .sort((a, b) => getReportTimestamp(b) - getReportTimestamp(a))[0]
    requirementsProjectCode.value = normalizeProjectCode(String(latestReport?.project_code || ''))
  } catch (error: any) {
    requirementsLoadError.value = error?.message || '加载历史报告失败，请重试'
  } finally {
    requirementsLoading.value = false
  }
}

const importRequirements = async () => {
  const content = String(selectedRequirementReport.value?.update_requirements || '')
  if (!content.trim()) return
  const existing = String(reportForm.value.update_requirements || '')
  if (existing.trim() && existing !== content) {
    try {
      await ElMessageBox.confirm('带入后将替换当前已填写的测试需求点，是否继续？', '带入历史需求', {
        confirmButtonText: '确认替换', cancelButtonText: '取消', type: 'warning'
      })
    } catch { return }
  }
  reportForm.value.update_requirements = content
  requirementsPickerVisible.value = false
  ElMessage.success('历史需求点已带入')
}

watch(requirementsProjectCode, () => { requirementsReportID.value = '' })

const latestProjectVersion = computed(() => {
  if (previewMode.value !== 'create') return ''

  const projectCode = normalizeProjectCode(reportForm.value.project_code || '')
  if (!projectCode) return ''

  return recentReports.value
    .filter((report) =>
      normalizeProjectCode(report.project_code || '') === projectCode &&
      String(report.version || '').trim()
    )
    .sort((a, b) => getReportTimestamp(b) - getReportTimestamp(a))[0]
    ?.version || ''
})

const versionInputPlaceholder = computed(() => {
  return latestProjectVersion.value ? `最近版本 ${latestProjectVersion.value}` : '例如 2.58.0'
})

const clearAutoFetchDefectTitlesTimer = () => {
  if (autoFetchDefectTitlesTimer) {
    clearTimeout(autoFetchDefectTitlesTimer)
    autoFetchDefectTitlesTimer = null
  }
}

const fetchDefectTitlesForReport = async (projectCode: string, version: string, seq: number) => {
  autoFetchingDefectTitles.value = true
  autoFetchDefectTitlesError.value = ''

  try {
    const res = await fetch(`${API_BASE}/acceptance-reports/fetch-defect-titles`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({ project_code: projectCode, version })
    })
    const data = await res.json().catch(() => null)
    if (!res.ok) throw new Error(data?.error || '匹配缺陷管理数据失败')

    const stillCurrentRequest = seq === autoFetchDefectTitlesSeq &&
      previewMode.value === 'create' &&
      reportForm.value.project_code === projectCode &&
      reportForm.value.version === version

    if (!stillCurrentRequest) return
    reportForm.value.bug_fix_status = formatManagedDefectTitles(
      Array.isArray(data?.titles) ? data.titles : []
    )
  } catch (err: any) {
    if (seq !== autoFetchDefectTitlesSeq) return
    console.error('Failed to match managed defects for acceptance report', err)
    autoFetchDefectTitlesError.value = err?.message || '匹配缺陷管理数据失败'
    ElMessage.error(autoFetchDefectTitlesError.value)
  } finally {
    if (seq === autoFetchDefectTitlesSeq) {
      autoFetchingDefectTitles.value = false
    }
  }
}

const scheduleAutoFetchDefectTitles = () => {
  clearAutoFetchDefectTitlesTimer()
  autoFetchDefectTitlesSeq += 1

  const projectCode = (reportForm.value.project_code || '').trim()
  const version = (reportForm.value.version || '').trim()
  if (previewMode.value !== 'create' || !projectCode || !version) {
    autoFetchingDefectTitles.value = false
    autoFetchDefectTitlesError.value = ''
    return
  }

  const seq = autoFetchDefectTitlesSeq
  autoFetchDefectTitlesTimer = setTimeout(() => {
    fetchDefectTitlesForReport(projectCode, version, seq)
  }, 2000)
}

const openCreateReport = () => {
  closeDeviceAssociationPanel()
  previewMode.value = 'create'
  currentPreview.value = null
  reportForm.value = {
    project_name: '',
    project_code: '',
    version: '',
    reporter: authStore.user?.username || '',
    test_owner: authStore.user?.username || '',
    test_time: '',
    test_env: '',
    test_devices: '',
    test_conclusion: 'Pass',
    update_requirements: '',
    bug_submission_status: '',
    bug_fix_status: '',
    status: 'Completed'
  }
  testTimeRange.value = []
  selectedTestDevices.value = []
  previewVisible.value = true
}

const openEditReport = () => {
  if (!currentPreview.value) return

  closeDeviceAssociationPanel()
  previewMode.value = 'edit'
  reportForm.value = {
    id: currentPreview.value.id,
    project_name: currentPreview.value.project_name || '',
    project_code: currentPreview.value.project_code || '',
    version: currentPreview.value.version || '',
    reporter: currentPreview.value.reporter || authStore.user?.username || '',
    test_owner: currentPreview.value.test_owner || currentPreview.value.reporter || authStore.user?.username || '',
    test_time: currentPreview.value.test_time || '',
    test_env: getPreviewTestEnv(currentPreview.value) === 'N/A' ? '' : getPreviewTestEnv(currentPreview.value),
    test_devices: getPreviewTestDevices(currentPreview.value) === 'N/A' ? '' : getPreviewTestDevices(currentPreview.value),
    test_conclusion: currentPreview.value.test_conclusion || 'Pass',
    update_requirements: currentPreview.value.update_requirements || '',
    bug_submission_status: currentPreview.value.bug_submission_status || '',
    bug_fix_status: currentPreview.value.bug_fix_status || '',
    status: currentPreview.value.status || 'Completed',
    created_at: currentPreview.value.created_at || '',
    updated_at: currentPreview.value.updated_at || ''
  }

  const testTime = String(currentPreview.value.test_time || '')
  testTimeRange.value = testTime.includes('～') ? testTime.split('～').map((item: string) => item.trim()) : []

  const testDevices = String(currentPreview.value.test_devices || '')
  selectedTestDevices.value = testDevices
    ? testDevices.split('、').map((item: string) => item.trim()).filter(Boolean)
    : []
}

const saveReport = async () => {
  if (!reportForm.value.project_name || !reportForm.value.project_code || !reportForm.value.version) {
    ElMessage.warning('请至少填写项目名称、项目代码和版本号')
    return
  }

  try {
    const isEditMode = previewMode.value === 'edit'
    const payload = {
      ...reportForm.value,
      id: reportForm.value.id || `AR_${Date.now()}`,
      reporter: reportForm.value.reporter || authStore.user?.username || 'Manual Report',
      test_owner: reportForm.value.test_owner || reportForm.value.reporter || authStore.user?.username || 'Manual Report',
      test_devices: selectedTestDevices.value.join('、'),
      created_at: reportForm.value.created_at || new Date().toISOString(),
      updated_at: new Date().toISOString(),
      status: reportForm.value.status || 'Completed'
    }

    const res = await fetch(`${API_BASE}/acceptance-reports/save`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify(payload)
    })

    if (!res.ok) {
      const data = await res.json().catch(() => null)
      throw new Error(data?.error || 'Save failed')
    }

    const saved = await res.json()
    if (saved.version_sync_warning) ElMessage.warning(saved.version_sync_warning)
    else ElMessage.success(isEditMode ? '验收报告修改成功' : '验收报告创建成功')
    notifyAcceptanceReportsChanged()
    previewVisible.value = false
    previewMode.value = 'view'
    fetchReports()
  } catch (err: any) {
    console.error('Failed to save acceptance report', err)
    ElMessage.error(err?.message || '保存验收报告失败')
  }
}

const sendReportToFeishu = async () => {
  if (!currentPreview.value?.id) return

  sendingToFeishu.value = true
  try {
    const res = await fetch(`${API_BASE}/acceptance-reports/send-feishu`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({ id: currentPreview.value.id })
    })

    const rawText = await res.text()
    let data: any = null
    try {
      data = rawText ? JSON.parse(rawText) : null
    } catch {
      data = { error: rawText || 'Unexpected response format' }
    }

    if (!res.ok) {
      throw new Error(data?.error || 'Send failed')
    }

    ElMessage.success('已发送到飞书群')
  } catch (err: any) {
    console.error('Failed to send acceptance report to Feishu', err)
    ElMessage.error(err?.message || '发送到飞书失败')
  } finally {
    sendingToFeishu.value = false
  }
}

const syncReportToCloudDoc = async () => {
  if (!currentPreview.value?.id) return

  syncingToCloudDoc.value = true
  try {
    const res = await fetch(`${API_BASE}/acceptance-reports/sync-cloud-doc`, {
      method: 'POST',
      credentials: 'include',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({ id: currentPreview.value.id })
    })

    const rawText = await res.text()
    let data: any = null
    try {
      data = rawText ? JSON.parse(rawText) : null
    } catch {
      data = { error: rawText || 'Unexpected response format' }
    }

    if (!res.ok) {
      throw new Error(data?.error || 'Sync failed')
    }

    ElMessage.success('已同步到云文档')
  } catch (err: any) {
    console.error('Failed to sync acceptance report to cloud doc', err)
    ElMessage.error(err?.message || '同步到云文档失败')
  } finally {
    syncingToCloudDoc.value = false
  }
}

const getPreviewTestEnv = (report: any) => {
  const rawEnv = (report?.test_env || '').trim()
  if (!rawEnv) return 'N/A'

  const knownEnvs = ['测试服务器', '正式服务器']
  const matchedEnv = knownEnvs.find(item => rawEnv.includes(item))
  if (!matchedEnv) return rawEnv

  return matchedEnv
}

const getPreviewTestDevices = (report: any) => {
  const explicitDevices = (report?.test_devices || '').trim()
  if (explicitDevices) return explicitDevices

  const rawEnv = (report?.test_env || '').trim()
  if (!rawEnv) return 'N/A'

  const knownEnvs = ['测试服务器', '正式服务器']
  for (const env of knownEnvs) {
    if (rawEnv.includes(env)) {
      const cleaned = rawEnv
        .replace(env, '')
        .replace(/^[：:、,\s/-]+/, '')
        .trim()
      return cleaned || 'N/A'
    }
  }

  return 'N/A'
}

watch(testTimeRange, (range) => {
  if (!Array.isArray(range) || range.length !== 2) {
    reportForm.value.test_time = ''
    return
  }
  reportForm.value.test_time = `${range[0]}～${range[1]}`
})

watch(selectedTestDevices, (list) => {
  reportForm.value.test_devices = list.join('、')
})

watch(() => reportForm.value.project_code, () => {
  closeDeviceAssociationPanel()
  const allowedNames = new Set(filteredDevices.value.map(item => item.device_name))
  selectedTestDevices.value = selectedTestDevices.value.filter(name => allowedNames.has(name))
})

watch(previewVisible, (visible) => {
  if (!visible) {
    closeDeviceAssociationPanel()
    requirementsPickerVisible.value = false
  }
})

watch(
  () => [previewMode.value, reportForm.value.project_name, reportForm.value.project_code, reportForm.value.version],
  scheduleAutoFetchDefectTitles
)

watch(
  () => [route.query.reportId, recentReports.value.length],
  ([reportId]) => {
    if (typeof reportId === 'string' && reportId) {
      openReportById(reportId)
    }
  },
  { immediate: true }
)

onMounted(async () => {
  window.addEventListener('resize', updateDeviceAssociationPanelPosition)
  window.addEventListener('scroll', updateDeviceAssociationPanelPosition, true)
  await Promise.all([fetchProjects(), fetchDevices()])
  if (!authStore.isLoggedIn && authStore.status !== 'anonymous') {
    await authStore.fetchMe()
  }
  if (authStore.isLoggedIn) {
    await fetchReports()
  }
})

onBeforeUnmount(() => {
  clearAutoFetchDefectTitlesTimer()
  window.removeEventListener('resize', updateDeviceAssociationPanelPosition)
  window.removeEventListener('scroll', updateDeviceAssociationPanelPosition, true)
})
</script>

<template>
  <div class="acceptance-container">
    <!-- Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h2 class="page-title">验收报告</h2>
          <span class="page-badge">Acceptance Reports</span>
        </div>
        <p class="page-subtitle">查看和管理各项目版本的提测验收结论、需求要点与缺陷修复验证情况</p>
      </div>
      <div class="header-right">
        <el-button type="primary" :icon="Plus" class="new-report-btn" @click="openCreateReport">
          新建验收报告
        </el-button>
      </div>
    </div>

    <!-- Stats Grid -->
    <el-row :gutter="16" class="stats-row">
      <el-col :span="8" v-for="(stat, index) in stats" :key="index">
        <div class="stat-card" :class="'stat-card--' + index" :style="{ '--accent-color': stat.color }">
          <div class="stat-content">
            <div class="stat-info">
              <span class="stat-title">{{ stat.title }}</span>
              <span class="stat-value">{{ stat.value }}</span>
            </div>
            <div class="stat-icon-wrapper" :style="{ backgroundColor: stat.bgColor, color: stat.color }">
              <el-icon class="stat-icon"><component :is="stat.icon" /></el-icon>
            </div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- Main Data Area -->
    <el-row :gutter="24" class="content-row">
      <!-- Left Column: Recent Report -->
      <el-col :span="16">
        <el-card class="content-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <h3 class="card-title">最近报告</h3>
              <div class="header-actions">
                <el-input
                  v-model="searchQuery"
                  placeholder="按报告人或项目筛选"
                  :prefix-icon="Search"
                  class="search-input"
                  clearable
                />
                <el-select v-model="categoryFilter" placeholder="分类" class="filter-select">
                  <el-option label="全部分类" value="" />
                  <el-option label="移动端" value="mobile" />
                  <el-option label="Web 端" value="web" />
                </el-select>
              </div>
            </div>
          </template>
          
          <div class="table-scroll-container">
            <el-table 
              :data="filteredReports" 
              style="width: 100%" 
              class="custom-table" 
              :row-style="{ height: '60px', cursor: 'pointer' }"
              @row-click="handleRowClick"
            >
              <el-table-column label="头像" width="70">
                <template #default="{ row }">
                  <div class="avatar-circle">{{ row.avatar }}</div>
                </template>
              </el-table-column>
              <el-table-column prop="reporter_display" label="报告人 / 项目" min-width="250">
                <template #default="{ row }">
                  <span class="reporter-text">{{ row.reporter_display }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="date" label="报告日期" width="150" />
              <el-table-column prop="status" label="状态" width="120">
                <template #default="{ row }">
                  <span class="status-badge" :class="row.status.toLowerCase().replace(' ', '-')">
                    {{ row.status }}
                  </span>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-card>
      </el-col>

      <!-- Right Column: History Project -->
      <el-col :span="8">
        <el-card class="content-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <h3 class="card-title">项目报告分布</h3>
            </div>
          </template>
          
            <div class="history-list scroll-container">
              <div
                v-for="item in historyProjects"
                :key="item.id"
                class="history-item"
                :class="{ 'history-item--active': selectedProjectCodeFilter === item.id }"
                @click="toggleProjectCodeFilter(item.id)"
              >
                <div class="project-id">
                  <span class="id-dot"></span>
                  {{ item.id }}
                </div>
                <div class="count-bubble">{{ item.count }}</div>
              </div>
            </div>
        </el-card>
      </el-col>
    </el-row>

    <!-- Preview Dialog -->
    <el-dialog
      v-model="previewVisible"
      :title="previewMode === 'create'
        ? '新建验收报告'
        : previewMode === 'edit'
          ? `编辑验收报告 - ${currentPreview?.project_name || reportForm.project_name || '详情'}`
          : `报告预览 - ${currentPreview?.project_name || '详情'}`"
      width="600px"
      destroy-on-close
      class="preview-dialog"
    >
      <div v-if="previewMode === 'view' && currentPreview" class="preview-content">
        <div class="preview-section">
          <div class="preview-item">
            <span class="label">项目代码:</span>
            <span class="value font-bold">{{ currentPreview.project_code }}</span>
          </div>
          <div class="preview-item">
            <span class="label">版本号:</span>
            <span class="value">v{{ currentPreview.version }}</span>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">测试负责人:</span>
            <span class="value">{{ currentPreview.reporter }}</span>
          </div>
          <div class="preview-item">
            <span class="label">测试时间:</span>
            <span class="value">{{ currentPreview.test_time || currentPreview.created_at?.split('T')[0] }}</span>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">测试环境:</span>
            <span class="value">{{ getPreviewTestEnv(currentPreview) }}</span>
          </div>
          <div class="preview-item">
            <span class="label">测试设备:</span>
            <span class="value">{{ getPreviewTestDevices(currentPreview) }}</span>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">测试结论:</span>
            <el-tag :type="currentPreview.test_conclusion === 'Pass' ? 'success' : 'danger'" size="small" effect="dark">
              {{ currentPreview.test_conclusion || '未知' }}
            </el-tag>
          </div>
        </div>

        <el-divider border-style="dashed" />

        <div class="preview-grid">
          <div class="preview-detail">
            <h4 class="detail-title">测试需求点 (Acceptance Requirements)</h4>
            <pre class="detail-text">{{ currentPreview.update_requirements || '无需求说明' }}</pre>
          </div>

          <div class="preview-detail">
            <h4 class="detail-title">当前版本缺陷提交及修复情况</h4>
            <pre class="detail-text">{{ currentPreview.bug_fix_status || '无' }}</pre>

            <h4 class="detail-title detail-title--spaced">历史版本遗留缺陷修复情况</h4>
            <pre class="detail-text">{{ currentPreview.bug_submission_status || '无' }}</pre>
          </div>
        </div>
      </div>
      <div v-else class="preview-content">
        <div class="preview-section">
          <div class="preview-item">
            <span class="label">项目名称:</span>
            <el-select
              v-model="reportForm.project_name"
              filterable
              placeholder="请选择项目名称"
              style="width: 100%"
              @change="syncProjectByName"
            >
              <el-option
                v-for="project in projects"
                :key="project.id || project.project_code"
                :label="project.project_name"
                :value="project.project_name"
              />
            </el-select>
          </div>
          <div class="preview-item">
            <span class="label">项目代码:</span>
            <el-select
              v-model="reportForm.project_code"
              filterable
              placeholder="请选择项目代码"
              style="width: 100%"
              @change="syncProjectByCode"
            >
              <el-option
                v-for="project in projects"
                :key="project.id || project.project_code"
                :label="project.project_code"
                :value="project.project_code"
              />
            </el-select>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">版本号:</span>
            <el-input v-model="reportForm.version" :placeholder="versionInputPlaceholder" />
          </div>
          <div class="preview-item">
            <span class="label">测试时间:</span>
            <el-date-picker
              v-model="testTimeRange"
              type="daterange"
              range-separator="~"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              style="width: 100%"
            />
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">报告人:</span>
            <el-input v-model="reportForm.reporter" placeholder="请输入报告人" />
          </div>
          <div class="preview-item">
            <span class="label">测试负责人:</span>
            <el-input v-model="reportForm.test_owner" placeholder="请输入测试负责人" />
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">测试环境:</span>
            <el-select v-model="reportForm.test_env" placeholder="请选择测试环境" style="width: 100%">
              <el-option label="测试服务器" value="测试服务器" />
              <el-option label="正式服务器" value="正式服务器" />
            </el-select>
          </div>
          <div ref="deviceFieldRef" class="preview-item">
            <span class="label">测试设备:</span>
            <el-select
              ref="deviceSelectRef"
              v-model="selectedTestDevices"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              placeholder="请选择测试设备"
              style="width: 100%"
            >
              <el-option
                v-for="device in filteredDevices"
                :key="device.id"
                :label="`${device.device_name}${device.model ? ` (${device.model})` : ''}`"
                :value="device.device_name"
              />
              <template v-if="previewMode === 'create'" #footer>
                <button
                  type="button"
                  class="associate-device-trigger"
                  @mousedown.prevent.stop
                  @click.prevent.stop="openDeviceAssociationPanel"
                >
                  <el-icon><Plus /></el-icon>
                  添加关联设备
                </button>
              </template>
            </el-select>
          </div>
        </div>

        <div class="preview-section">
          <div class="preview-item">
            <span class="label">测试结论:</span>
            <el-select v-model="reportForm.test_conclusion" style="width: 100%">
              <el-option label="Pass" value="Pass" />
              <el-option label="Fail" value="Fail" />
              <el-option label="Blocked" value="Blocked" />
            </el-select>
          </div>
          <div class="preview-item">
            <span class="label">项目匹配:</span>
            <span class="value">{{ selectedProject?.project_name && selectedProject?.project_code ? `${selectedProject.project_name} / ${selectedProject.project_code}` : '请选择项目' }}</span>
          </div>
        </div>

        <el-divider border-style="dashed" />

        <div class="preview-grid">
          <div class="preview-detail">
            <h4 class="detail-title detail-title--inline">
              <span>测试需求点 (Acceptance Requirements)</span>
              <el-button v-if="previewMode === 'create'" type="primary" plain size="small" :icon="DocumentCopy" @click="openRequirementsPicker">带入需求</el-button>
            </h4>
            <el-input
              v-model="reportForm.update_requirements"
              type="textarea"
              :rows="6"
              placeholder="请输入测试需求点、需求链接或验收范围"
            />
          </div>

          <div class="preview-detail">
            <h4 class="detail-title detail-title--inline">
              <span>历史版本遗留缺陷修复情况</span>
            </h4>
            <el-input
              v-model="reportForm.bug_submission_status"
              type="textarea"
              :rows="3"
              placeholder="请输入历史版本遗留缺陷的修复及验证情况"
            />

            <h4 class="detail-title detail-title--spaced detail-title--inline">
              <span>当前版本缺陷提交及修复情况</span>
              <span v-if="showDefectTitlesLoading" class="auto-fetch-status">
                <el-icon class="auto-fetch-status__icon"><Loading /></el-icon>
                正在自动拉取对应数据中，请稍等
              </span>
            </h4>
            <el-input
              v-model="reportForm.bug_fix_status"
              type="textarea"
              :rows="3"
              placeholder="请输入当前版本提交的缺陷、修复状态、链接或说明"
            />
          </div>
        </div>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="previewVisible = false">{{ previewMode === 'create' ? '取消' : '关闭' }}</el-button>
          <el-button
            v-if="canSendToFeishu"
            type="success"
            :loading="sendingToFeishu"
            @click="sendReportToFeishu"
          >
            发送到飞书
          </el-button>
          <el-button
            v-if="canSyncToCloudDoc"
            type="primary"
            :loading="syncingToCloudDoc"
            @click="syncReportToCloudDoc"
          >
            同步到云文档
          </el-button>
          <el-button v-if="canEditReport" type="primary" plain @click="openEditReport">编辑报告</el-button>
          <el-button v-if="previewMode === 'create' || previewMode === 'edit'" type="primary" @click="saveReport">
            {{ previewMode === 'edit' ? '保存修改' : '保存报告' }}
          </el-button>
        </span>
      </template>
    </el-dialog>

    <el-dialog v-model="requirementsPickerVisible" title="带入历史需求" width="min(720px, calc(100vw - 32px))" align-center append-to-body destroy-on-close>
      <div class="requirements-picker" v-loading="requirementsLoading">
        <label class="requirements-picker__label">选择项目</label>
        <el-select v-model="requirementsProjectCode" filterable clearable :disabled="requirementsLoading" placeholder="选择项目，或输入项目编号 / 名称搜索" class="requirements-picker__select">
          <el-option v-for="project in requirementsProjects" :key="project.code" :value="project.code" :label="`${project.code} · ${project.name}`" />
        </el-select>
        <el-alert v-if="requirementsLoadError" :title="requirementsLoadError" type="error" :closable="false" show-icon />
        <div class="requirements-picker__heading"><strong>历史报告需求点</strong><span>按报告更新时间从近到远</span></div>
        <div class="requirements-picker__reports" role="group" aria-label="选择历史报告">
          <button v-for="report in selectableRequirementReports" :key="report.id" type="button" class="requirements-report" :class="{ 'is-selected': requirementsReportID === report.id }" :aria-pressed="requirementsReportID === report.id" @click="requirementsReportID = report.id">
            <div class="requirements-report__meta"><strong>{{ report.version || '未填写版本号' }}</strong><time>{{ formatRequirementReportTime(report) }}</time></div>
            <span class="requirements-report__reporter">{{ report.reporter || report.test_owner || '未填写报告人' }}</span>
            <p>{{ report.update_requirements }}</p>
          </button>
          <el-empty v-if="!requirementsLoading && !requirementsLoadError && !selectableRequirementReports.length" :description="requirementsProjectCode ? '该项目暂无包含需求点的历史报告' : '请先选择项目'" :image-size="64" />
        </div>
        <div v-if="selectedRequirementReport" class="requirements-picker__preview"><strong>需求预览</strong><pre>{{ selectedRequirementReport.update_requirements }}</pre></div>
      </div>
      <template #footer><el-button @click="requirementsPickerVisible = false">取消</el-button><el-button type="primary" :disabled="!selectedRequirementReport || requirementsLoading" @click="importRequirements">带入选中需求</el-button></template>
    </el-dialog>

    <Teleport to="body">
      <section
        v-if="deviceAssociationVisible"
        class="device-association-panel"
        :style="deviceAssociationPanelStyle"
        aria-label="添加关联设备"
        @mousedown.stop
        @click.stop
      >
        <div class="device-association-panel__header">
          <div>
            <h3>添加关联设备</h3>
            <p>{{ reportForm.project_name || reportForm.project_code }}</p>
          </div>
          <el-button
            link
            :icon="Close"
            aria-label="关闭"
            :disabled="associatingDevices"
            @click="closeDeviceAssociationPanel"
          />
        </div>

        <el-input
          v-model="deviceAssociationSearch"
          :prefix-icon="Search"
          clearable
          placeholder="搜索设备名称、型号、系统或 ID"
          class="device-association-panel__search"
        />

        <div class="device-association-list">
          <div
            v-for="item in deviceAssociationItems"
            :key="item.device.id"
            class="device-association-item"
            :class="{ 'device-association-item--disabled': item.associated }"
            @click="togglePendingDevice(item.device.id, item.associated)"
          >
            <el-checkbox
              class="device-association-checkbox"
              :model-value="pendingDeviceIds.includes(item.device.id)"
              :disabled="item.associated"
              @click.stop
              @change="togglePendingDevice(item.device.id, item.associated)"
            >
              <span class="device-association-item__content">
                <span class="device-association-item__name">{{ item.device.device_name }}</span>
                <span class="device-association-item__meta">
                  {{ [item.device.os, item.device.model, item.device.id].filter(Boolean).join(' · ') }}
                </span>
              </span>
            </el-checkbox>
            <el-tag v-if="item.associated" type="info" size="small" effect="plain">已关联</el-tag>
          </div>
          <el-empty
            v-if="deviceAssociationItems.length === 0"
            description="没有匹配的测试设备"
            :image-size="64"
          />
        </div>

        <div class="device-association-panel__footer">
          <span>已选 {{ pendingDeviceIds.length }} 台</span>
          <div>
            <el-button :disabled="associatingDevices" @click="closeDeviceAssociationPanel">取消</el-button>
            <el-button
              type="primary"
              :loading="associatingDevices"
              :disabled="pendingDeviceIds.length === 0"
              @click="saveDeviceAssociations"
            >
              完成关联
            </el-button>
          </div>
        </div>
      </section>
    </Teleport>
  </div>
</template>

<style scoped>
.requirements-picker { display:flex; flex-direction:column; gap:12px; }
.requirements-picker__label { color:#334155; font-size:13px; font-weight:600; }
.requirements-picker__select { width:100%; }
.requirements-picker__heading { display:flex; justify-content:space-between; align-items:center; gap:12px; margin-top:6px; font-size:13px; color:#334155; }
.requirements-picker__heading span { font-size:12px; color:#94a3b8; }
.requirements-picker__reports { display:flex; flex-direction:column; gap:8px; max-height:260px; overflow-y:auto; }
.requirements-report { width:100%; text-align:left; padding:12px 14px; border:1px solid #e2e8f0; border-radius:10px; background:#fff; color:#334155; cursor:pointer; font:inherit; transition:border-color .2s,background .2s; }
.requirements-report:hover,.requirements-report:focus-visible { border-color:#93c5fd; background:#f8fafc; }
.requirements-report.is-selected { border-color:#3b82f6; background:#eff6ff; }
.requirements-report__meta { display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:8px; }
.requirements-report__meta strong { font-size:14px; color:#1e293b; }
.requirements-report__meta time,.requirements-report__reporter { font-size:12px; color:#64748b; }
.requirements-report p { margin:6px 0 0; font-size:12px; line-height:1.6; white-space:pre-wrap; overflow-wrap:anywhere; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }
.requirements-picker__preview { padding:14px; background:#f8fafc; border:1px solid #e2e8f0; border-radius:10px; }
.requirements-picker__preview strong { font-size:13px; color:#334155; }
.requirements-picker__preview pre { margin:8px 0 0; max-height:180px; overflow:auto; white-space:pre-wrap; overflow-wrap:anywhere; font-family:inherit; font-size:13px; line-height:1.7; color:#475569; }
.acceptance-container {
  padding: 8px;
  font-family: 'Inter', -apple-system, sans-serif;
}

/* Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: #1e293b;
}

.breadcrumb {
  margin-top: 4px;
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
}

.divider {
  margin: 0 4px;
  color: #cbd5e1;
}

.new-report-btn {
  border-radius: 8px;
  padding: 10px 20px;
  font-weight: 600;
  box-shadow: 0 4px 6px -1px rgba(59, 130, 246, 0.2);
}

/* Stats */
.stats-row {
  margin-bottom: 24px;
}

.stat-card {
  border-radius: 16px;
  border: 1px solid #f1f5f9;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.stat-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
  border-color: #3b82f6;
}

.stat-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px;
}

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-title {
  font-size: 14px;
  color: #64748b;
  font-weight: 600;
  margin-bottom: 8px;
}

.stat-value {
  font-size: 32px;
  font-weight: 800;
  color: #0f172a;
}

.stat-icon-wrapper {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-icon {
  font-size: 28px;
}

/* Main Content */
.content-row {
  margin-bottom: 24px;
}

.content-card {
  border-radius: 16px;
  border: 1px solid #f1f5f9;
  height: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #1e293b;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.search-input {
  width: 220px;
}

.filter-select {
  width: 140px;
}

/* Table */
.custom-table {
  --el-table-border-color: #f1f5f9;
  --el-table-header-bg-color: #f8fafc;
  --el-table-header-text-color: #64748b;
}

.avatar-circle {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: #e2e8f0;
  color: #475569;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 14px;
}

.reporter-text {
  font-weight: 600;
  color: #334155;
}

/* Badges */
.status-badge {
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  display: inline-block;
}

.status-badge.completed {
  background: #eff6ff;
  color: #3b82f6;
}

.status-badge.in-progress {
  background: #fefce8;
  color: #eab308;
}

.status-badge.pending {
  background: #f1f5f9;
  color: #64748b;
}

/* History List & Scroll */
.table-scroll-container,
.history-list {
  height: 480px; /* Approx 8 rows */
  overflow-y: auto;
  scrollbar-width: none; /* Firefox */
  -ms-overflow-style: none; /* IE/Edge */
}

.table-scroll-container::-webkit-scrollbar,
.history-list::-webkit-scrollbar {
  display: none; /* Chrome/Safari */
}

.history-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 8px 0;
}

.history-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: #f8fafc;
  border-radius: 12px;
  transition: background 0.2s;
  cursor: pointer;
}

.history-item:hover {
  background: #f1f5f9;
}

.history-item--active {
  background: #dbeafe;
  box-shadow: inset 0 0 0 1px #60a5fa;
}

.history-item--active .project-id {
  color: #1d4ed8;
}

.history-item--active .count-bubble {
  background: #3b82f6;
  color: #ffffff;
}

.project-id {
  display: flex;
  align-items: center;
  gap: 12px;
  font-weight: 600;
  color: #1e293b;
  font-size: 14px;
}

.id-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #3b82f6;
}

.count-bubble {
  background: #dbeafe;
  color: #2563eb;
  padding: 2px 10px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 700;
}

/* Preview Dialog */
.preview-content {
  color: #334155;
}

.preview-section {
  display: flex;
  justify-content: space-between;
  margin-bottom: 16px;
  gap: 20px;
}

.preview-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.label {
  font-size: 12px;
  color: #64748b;
  font-weight: 600;
  text-transform: uppercase;
}

.value {
  font-size: 15px;
  color: #1e293b;
}

.detail-title {
  font-size: 14px;
  font-weight: 700;
  color: #6366f1;
  margin-bottom: 8px;
}

.detail-title--inline {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.detail-title--spaced {
  margin-top: 16px;
}

.auto-fetch-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
}

.auto-fetch-status__icon {
  animation: auto-fetch-spin 1s linear infinite;
}

@keyframes auto-fetch-spin {
  from {
    transform: rotate(0deg);
  }

  to {
    transform: rotate(360deg);
  }
}

.preview-grid {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.preview-detail :deep(.el-textarea__inner),
.preview-item :deep(.el-input__wrapper),
.preview-item :deep(.el-select__wrapper) {
  border-radius: 10px;
}

.associate-device-trigger {
  width: 100%;
  min-height: 36px;
  padding: 8px 12px;
  border: 0;
  background: transparent;
  color: #2563eb;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 8px;
  font: inherit;
  font-size: 14px;
  cursor: pointer;
}

.associate-device-trigger:hover {
  background: #eff6ff;
}

.device-association-panel {
  position: fixed;
  z-index: 3200;
  min-height: 280px;
  display: flex;
  flex-direction: column;
  padding: 16px;
  border: 1px solid #dbe3ee;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 18px 45px rgba(15, 23, 42, 0.16);
  color: #1e293b;
}

.device-association-panel__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.device-association-panel__header h3 {
  margin: 0;
  font-size: 16px;
  line-height: 22px;
}

.device-association-panel__header p {
  margin: 2px 0 0;
  color: #64748b;
  font-size: 12px;
}

.device-association-panel__search {
  margin-bottom: 12px;
}

.device-association-list {
  min-height: 0;
  flex: 1;
  overflow-y: auto;
  border-top: 1px solid #eef2f7;
  border-bottom: 1px solid #eef2f7;
}

.device-association-item {
  min-height: 58px;
  padding: 9px 4px;
  display: flex;
  align-items: center;
  gap: 10px;
  border-bottom: 1px solid #f1f5f9;
  cursor: pointer;
}

.device-association-item:last-child {
  border-bottom: 0;
}

.device-association-item:hover {
  background: #f8fafc;
}

.device-association-item--disabled {
  background: #f8fafc;
  color: #94a3b8;
  cursor: not-allowed;
}

.device-association-checkbox {
  min-width: 0;
  flex: 1;
  height: auto;
  margin-right: 0;
}

.device-association-checkbox :deep(.el-checkbox__label) {
  min-width: 0;
  flex: 1;
}

.device-association-item__content {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.device-association-item__name,
.device-association-item__meta {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-association-item__name {
  color: #334155;
  font-size: 14px;
  font-weight: 600;
}

.device-association-item--disabled .device-association-item__name {
  color: #94a3b8;
}

.device-association-item__meta {
  color: #64748b;
  font-size: 12px;
}

.device-association-panel__footer {
  padding-top: 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: #64748b;
  font-size: 12px;
}

@media (max-width: 900px) {
  .device-association-panel {
    right: 12px;
    left: 12px !important;
    width: auto !important;
  }
}

.detail-text {
  background: #f8fafc;
  padding: 12px 16px;
  border-radius: 8px;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  color: #475569;
  border: 1px solid #e2e8f0;
  max-height: 200px;
  overflow-y: auto;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
  padding: 16px 22px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-title {
  margin: 0;
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
}

.page-badge {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  padding: 2px 8px;
  border-radius: 6px;
}

.page-subtitle {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.new-report-btn {
  padding: 10px 18px;
  border-radius: 9px;
  font-weight: 600;
}

.stat-card {
  padding: 18px 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
  transition: all 0.2s ease;
  border-left: 4px solid var(--accent-color, #3b82f6);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.stat-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.stat-info {
  display: flex;
  flex-direction: column;
}

.stat-title {
  font-size: 13px;
  color: #64748b;
  font-weight: 500;
  margin-bottom: 4px;
}

.stat-value {
  font-size: 26px;
  font-weight: 700;
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  line-height: 1.2;
}

.stat-icon-wrapper {
  width: 46px;
  height: 46px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-icon {
  font-size: 22px;
}

.content-card {
  border-radius: 14px;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.history-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: #f8fafc;
  border-radius: 10px;
  border: 1px solid transparent;
  transition: all 0.2s ease;
  cursor: pointer;
}

.history-item:hover {
  background: #f1f5f9;
}

.history-item--active {
  background: #eff6ff !important;
  border-color: #bfdbfe !important;
  box-shadow: 0 0 0 1px #93c5fd;
}

.history-item--active .project-id {
  color: #2563eb;
  font-weight: 700;
}

.count-bubble {
  background: #eff6ff;
  color: #2563eb;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  border: 1px solid #dbeafe;
}

.history-item--active .count-bubble {
  background: #2563eb;
  color: #ffffff;
  border-color: #2563eb;
}

</style>
