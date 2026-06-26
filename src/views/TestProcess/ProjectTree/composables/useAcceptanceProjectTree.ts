import { computed, shallowRef } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { buildBackendUrl } from '@/utils/runtimeUrl'
import { retryFetch } from '@/utils/retryFetch'
import type {
  AcceptanceReportRecord,
  ProjectMemoColor,
  ProjectMemoItem,
  ProjectMemoRecord,
  ProjectTreeNode,
  ProjectVersionNode
} from '../types'

const EMPTY_PROJECT_CODE = '未填写项目代码'
const EMPTY_PROJECT_NAME = '未命名项目'
const EMPTY_VERSION = '未填写版本'
const PROJECT_MEMOS_STORAGE_KEY = 'testcenter.projectTree.projectMemos'

const normalizeText = (value?: string) => (value || '').trim()

const normalizeDateText = (value: string) => value.replace(/\./g, '-').replace(/\//g, '-')

const getTestEndTime = (value?: string) => {
  const text = normalizeText(value)
  if (!text) return ''

  const parts = text
    .split(/(?:～|~|至|到|—|–| - )/)
    .map((item) => normalizeText(item))
    .filter(Boolean)

  return normalizeDateText(parts[parts.length - 1] || text)
}

const getReportTime = (report: Partial<AcceptanceReportRecord>) => (
  getTestEndTime(report.test_time) ||
  normalizeText(report.updated_at) ||
  normalizeText(report.created_at) ||
  '-'
)

const compareByTimeDesc = (a: string, b: string) => {
  const left = Date.parse(a)
  const right = Date.parse(b)

  if (Number.isNaN(left) && Number.isNaN(right)) return 0
  if (Number.isNaN(left)) return 1
  if (Number.isNaN(right)) return -1
  return right - left
}

const parseVersionParts = (version: string) => {
  const normalized = normalizeText(version)
  const numericParts = normalized.match(/\d+/g)?.map((item) => Number(item)) || []
  return {
    normalized,
    numericParts
  }
}

const compareVersionDesc = (a: string, b: string) => {
  const left = parseVersionParts(a)
  const right = parseVersionParts(b)
  const maxLength = Math.max(left.numericParts.length, right.numericParts.length)

  for (let index = 0; index < maxLength; index += 1) {
    const leftPart = left.numericParts[index] ?? 0
    const rightPart = right.numericParts[index] ?? 0
    if (leftPart !== rightPart) {
      return rightPart - leftPart
    }
  }

  return right.normalized.localeCompare(left.normalized, undefined, {
    numeric: true,
    sensitivity: 'base'
  })
}

const buildProjectTree = (reports: AcceptanceReportRecord[]) => {
  const projectMap = new Map<string, Map<string, AcceptanceReportRecord[]>>()

  reports.forEach((report) => {
    const projectCode = normalizeText(report.project_code) || EMPTY_PROJECT_CODE
    const version = normalizeText(report.version) || EMPTY_VERSION

    if (!projectMap.has(projectCode)) {
      projectMap.set(projectCode, new Map())
    }

    const versionMap = projectMap.get(projectCode)
    if (!versionMap?.has(version)) {
      versionMap?.set(version, [])
    }

    versionMap?.get(version)?.push(report)
  })

  return Array.from(projectMap.entries()).map<ProjectTreeNode>(([projectCode, versionMap]) => {
    const versions = Array.from(versionMap.entries()).map<ProjectVersionNode>(([version, versionReports]) => {
      const sortedReports = [...versionReports].sort((a, b) => compareByTimeDesc(getReportTime(a), getReportTime(b)))

      return {
        version,
        reportCount: sortedReports.length,
        latestTestTime: getReportTime(sortedReports[0] || {}),
        reports: sortedReports
      }
    }).sort((a, b) => {
      const versionOrder = compareVersionDesc(a.version, b.version)
      return versionOrder || compareByTimeDesc(a.latestTestTime, b.latestTestTime)
    })

    const allReports = versions.flatMap((version) => version.reports)
    const firstNamedReport = allReports.find((report) => normalizeText(report.project_name))

    return {
      projectCode,
      projectName: normalizeText(firstNamedReport?.project_name) || EMPTY_PROJECT_NAME,
      reportCount: allReports.length,
      versionCount: versions.length,
      latestTestTime: versions[0]?.latestTestTime || '-',
      versions
    }
  }).sort((a, b) => compareByTimeDesc(a.latestTestTime, b.latestTestTime))
}

const loadProjectMemos = (): Record<string, ProjectMemoRecord> => {
  try {
    const raw = window.localStorage.getItem(PROJECT_MEMOS_STORAGE_KEY)
    const parsed = raw ? JSON.parse(raw) : {}
    if (!parsed || typeof parsed !== 'object') return {}

    const migrated: Record<string, ProjectMemoRecord> = {}
    Object.keys(parsed).forEach((key) => {
      const val = parsed[key]
      if (!val) return

      if (typeof val === 'string') {
        const time = new Date().toISOString()
        migrated[key] = {
          items: [{ id: `memo-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`, content: val, color: 'green', updatedAt: time, history: [] }],
          updatedAt: time
        }
      } else if (typeof val === 'object') {
        if (Array.isArray(val.items)) {
          migrated[key] = {
            items: val.items.map((item: any) => ({
              id: item.id || `memo-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`,
              content: item.content || '',
              color: item.color || 'green',
              updatedAt: item.updatedAt || new Date().toISOString(),
              history: Array.isArray(item.history)
                ? item.history.map((historyItem: any) => ({
                    content: historyItem.content || '',
                    color: historyItem.color || item.color || 'green',
                    modifiedAt: historyItem.modifiedAt || historyItem.updatedAt || new Date().toISOString()
                  })).filter((historyItem: any) => normalizeText(historyItem.content))
                : []
            })),
            updatedAt: val.updatedAt || new Date().toISOString()
          }
        } else if (typeof val.content === 'string') {
          const time = val.updatedAt || new Date().toISOString()
          migrated[key] = {
            items: [{ id: `memo-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`, content: val.content, color: 'green', updatedAt: time, history: [] }],
            updatedAt: time
          }
        } else {
          migrated[key] = {
            items: [],
            updatedAt: new Date().toISOString()
          }
        }
      }
    })
    return migrated
  } catch {
    return {}
  }
}

const saveProjectMemos = (memos: Record<string, ProjectMemoRecord>) => {
  window.localStorage.setItem(PROJECT_MEMOS_STORAGE_KEY, JSON.stringify(memos))
}

const mergeProjectMemoRecords = (
  remoteRecords: Record<string, ProjectMemoRecord>,
  localRecords: Record<string, ProjectMemoRecord>
) => {
  const merged = { ...remoteRecords }
  const migratedProjectCodes: string[] = []

  Object.entries(localRecords).forEach(([projectCode, localRecord]) => {
    const remoteRecord = merged[projectCode]
    if (!remoteRecord) {
      merged[projectCode] = localRecord
      migratedProjectCodes.push(projectCode)
      return
    }

    const existingIDs = new Set(remoteRecord.items.map((item) => item.id))
    const existingContents = new Set(remoteRecord.items.map((item) => `${normalizeText(item.content)}::${item.color}`))
    const missingItems = localRecord.items.filter((item) => (
      !existingIDs.has(item.id) &&
      !existingContents.has(`${normalizeText(item.content)}::${item.color}`)
    ))
    if (!missingItems.length) return

    merged[projectCode] = {
      items: [...remoteRecord.items, ...missingItems],
      updatedAt: new Date().toISOString()
    }
    migratedProjectCodes.push(projectCode)
  })

  return { merged, migratedProjectCodes }
}

const isTTminsProject = (project: ProjectTreeNode) => {
  const projectText = `${project.projectCode} ${project.projectName}`.toLowerCase()
  return projectText.includes('ttmins')
}

const hasSameMemoItem = (items: ProjectMemoItem[], source: ProjectMemoItem) => {
  const sourceContent = normalizeText(source.content)
  if (!sourceContent) return true
  return items.some((item) => normalizeText(item.content) === sourceContent && item.color === source.color)
}

const copyA1160MemosToTTminsProjects = (
  projectTree: ProjectTreeNode[],
  projectMemos: Record<string, ProjectMemoRecord>
) => {
  const sourceRecord = projectMemos.A1160
  const sourceItems = (sourceRecord?.items || [])
    .filter((item) => normalizeText(item.content))
    .slice(0, 4)

  if (!sourceItems.length) return projectMemos

  const now = new Date().toISOString()
  let changed = false
  const next = { ...projectMemos }

  projectTree
    .filter((project) => project.projectCode !== 'A1160' && isTTminsProject(project))
    .forEach((project) => {
      const currentRecord = next[project.projectCode] || { items: [], updatedAt: '' }
      const currentItems = currentRecord.items || []
      const missingItems = sourceItems.filter((item) => !hasSameMemoItem(currentItems, item))

      if (!missingItems.length) return

      next[project.projectCode] = {
        items: [
          ...currentItems,
          ...missingItems.map((item, index) => ({
            id: `memo-${Date.now()}-${project.projectCode}-${index}-${Math.random().toString(36).substring(2, 8)}`,
            content: item.content,
            color: item.color,
            updatedAt: now,
            history: [],
            kind: item.kind || (item.color === 'blue' ? 'ai' : 'manual'),
            configKey: item.configKey,
            sourceReportId: item.sourceReportId,
            sourceHash: item.sourceHash
          }))
        ],
        updatedAt: now
      }
      changed = true
    })

  return changed ? next : projectMemos
}


export function useAcceptanceProjectTree() {
  const authStore = useAuthStore()
  const reports = shallowRef<AcceptanceReportRecord[]>([])
  const projectMemos = shallowRef<Record<string, ProjectMemoRecord>>(loadProjectMemos())
  const loading = shallowRef(false)
  const keyword = shallowRef('')
  const selectedProjectCode = shallowRef('')

  const projectTree = computed(() => buildProjectTree(reports.value))

  const projectOptions = computed(() => projectTree.value.map((project) => ({
    label: `${project.projectCode}｜${project.projectName}`,
    value: project.projectCode,
    reportCount: project.reportCount
  })))

  const filteredProjectTree = computed(() => {
    const query = keyword.value.trim().toLowerCase()
    const projectScopedTree = selectedProjectCode.value
      ? projectTree.value.filter((project) => project.projectCode === selectedProjectCode.value)
      : projectTree.value.slice(0, 1)

    if (!query) return projectScopedTree

    return projectScopedTree
      .map<ProjectTreeNode | null>((project) => {
        const projectMatched = [
          project.projectCode,
          project.projectName
        ].some((value) => value.toLowerCase().includes(query))

        const versions = project.versions
          .map<ProjectVersionNode | null>((version) => {
            const versionMatched = version.version.toLowerCase().includes(query)
            const reports = version.reports.filter((report) => [
              report.test_time,
              report.update_requirements,
              report.reporter,
              report.test_conclusion
            ].some((value) => normalizeText(value).toLowerCase().includes(query)))

            if (projectMatched || versionMatched) return version
            if (!reports.length) return null

            return {
              ...version,
              reportCount: reports.length,
              reports
            }
          })
          .filter((version): version is ProjectVersionNode => Boolean(version))

        if (!versions.length) return null

        return {
          ...project,
          reportCount: versions.reduce((total, version) => total + version.reportCount, 0),
          versionCount: versions.length,
          versions
        }
      })
      .filter((project): project is ProjectTreeNode => Boolean(project))
  })

  const stats = computed(() => ({
    projectCount: filteredProjectTree.value.length,
    versionCount: filteredProjectTree.value.reduce((total, project) => total + project.versionCount, 0),
    reportCount: filteredProjectTree.value.reduce((total, project) => total + project.reportCount, 0)
  }))

  const selectedProject = computed(() => {
    if (!selectedProjectCode.value) return null
    return projectTree.value.find((project) => project.projectCode === selectedProjectCode.value) || null
  })

  const selectedProjectMemo = computed(() => {
    if (!selectedProjectCode.value) return null
    return projectMemos.value[selectedProjectCode.value] || null
  })

  const persistProjectMemoRecord = async (projectCode: string, record?: ProjectMemoRecord) => {
    const targetRecord = record || projectMemos.value[projectCode] || {
      items: [],
      updatedAt: new Date().toISOString()
    }
    try {
      const response = await retryFetch(buildBackendUrl('/api/acceptance-reports/project-configs'), {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
          Authorization: authStore.token
        },
        body: JSON.stringify({
          project_code: projectCode,
          items: targetRecord.items,
          updated_at: targetRecord.updatedAt
        })
      })
      if (!response.ok) {
        throw new Error(await response.text())
      }
    } catch (error) {
      console.error('Failed to persist project config record', error)
      ElMessage.error('项目配置记录同步失败，请稍后重试')
    }
  }

  const addProjectMemoItem = (content = '', color: Exclude<ProjectMemoColor, 'blue'> = 'green') => {
    if (!selectedProjectCode.value) return null

    const next = { ...projectMemos.value }
    const currentRecord = next[selectedProjectCode.value] || { items: [], updatedAt: '' }
    const newItem: ProjectMemoItem = {
      id: `memo-${Date.now()}-${Math.random().toString(36).substring(2, 11)}`,
      content,
      color,
      updatedAt: new Date().toISOString(),
      history: [],
      kind: 'manual'
    }

    next[selectedProjectCode.value] = {
      items: [...currentRecord.items, newItem],
      updatedAt: new Date().toISOString()
    }

    projectMemos.value = next
    saveProjectMemos(next)
    void persistProjectMemoRecord(selectedProjectCode.value, next[selectedProjectCode.value])
    return newItem
  }

  const updateProjectMemoItem = (itemId: string, updates: Partial<Omit<ProjectMemoItem, 'id'>>) => {
    if (!selectedProjectCode.value) return

    const next = { ...projectMemos.value }
    const currentRecord = next[selectedProjectCode.value]
    if (!currentRecord) return

    const updatedItems = currentRecord.items.map((item) => {
      if (item.id === itemId) {
        const nextContent = typeof updates.content === 'string' ? updates.content : item.content
        const nextColor = item.kind === 'ai' ? 'blue' : updates.color || item.color
        const modifiedAt = new Date().toISOString()
        const contentChanged = nextContent !== item.content || nextColor !== item.color
        return {
          ...item,
          ...updates,
          history: contentChanged
            ? [
                ...(item.history || []),
                {
                  content: item.content,
                  color: item.color,
                  modifiedAt
                }
              ]
            : item.history || [],
          updatedAt: contentChanged ? modifiedAt : item.updatedAt
        }
      }
      return item
    })

    next[selectedProjectCode.value] = {
      items: updatedItems,
      updatedAt: new Date().toISOString()
    }

    projectMemos.value = next
    saveProjectMemos(next)
    void persistProjectMemoRecord(selectedProjectCode.value, next[selectedProjectCode.value])
  }

  const reorderProjectMemoItems = (sourceItemId: string, targetItemId: string) => {
    if (!selectedProjectCode.value || sourceItemId === targetItemId) return

    const next = { ...projectMemos.value }
    const currentRecord = next[selectedProjectCode.value]
    if (!currentRecord) return

    const currentItems = [...currentRecord.items]
    const sourceIndex = currentItems.findIndex((item) => item.id === sourceItemId)
    const targetIndex = currentItems.findIndex((item) => item.id === targetItemId)
    if (sourceIndex < 0 || targetIndex < 0) return

    const [sourceItem] = currentItems.splice(sourceIndex, 1)
    if (!sourceItem) return
    currentItems.splice(targetIndex, 0, sourceItem)

    next[selectedProjectCode.value] = {
      items: currentItems,
      updatedAt: new Date().toISOString()
    }

    projectMemos.value = next
    saveProjectMemos(next)
    void persistProjectMemoRecord(selectedProjectCode.value, next[selectedProjectCode.value])
  }

  const deleteProjectMemoItem = (itemId: string) => {
    if (!selectedProjectCode.value) return

    const next = { ...projectMemos.value }
    const currentRecord = next[selectedProjectCode.value]
    if (!currentRecord) return

    const updatedItems = currentRecord.items.filter((item) => item.id !== itemId)

    if (updatedItems.length > 0) {
      next[selectedProjectCode.value] = {
        items: updatedItems,
        updatedAt: new Date().toISOString()
      }
    } else {
      delete next[selectedProjectCode.value]
    }

    projectMemos.value = next
    saveProjectMemos(next)
    void persistProjectMemoRecord(
      selectedProjectCode.value,
      next[selectedProjectCode.value] || { items: [], updatedAt: new Date().toISOString() }
    )
  }

  const pasteProjectMemoItem = (sourceItem: Pick<ProjectMemoItem, 'content' | 'color'>, targetProjectCodes: string[]) => {
    const content = normalizeText(sourceItem.content)
    if (!content) return 0

    const uniqueTargetCodes = Array.from(new Set(
      targetProjectCodes
        .map((projectCode) => normalizeText(projectCode))
        .filter((projectCode) => projectCode && projectCode !== selectedProjectCode.value)
    ))
    if (!uniqueTargetCodes.length) return 0

    const now = new Date().toISOString()
    const next = { ...projectMemos.value }

    uniqueTargetCodes.forEach((projectCode, index) => {
      const currentRecord = next[projectCode] || { items: [], updatedAt: '' }
      const newItem: ProjectMemoItem = {
        id: `memo-${Date.now()}-${projectCode}-${index}-${Math.random().toString(36).substring(2, 8)}`,
        content,
        color: sourceItem.color,
        updatedAt: now,
        history: [],
        kind: sourceItem.color === 'blue' ? 'ai' : 'manual'
      }

      next[projectCode] = {
        items: [...(currentRecord.items || []), newItem],
        updatedAt: now
      }
    })

    projectMemos.value = next
    saveProjectMemos(next)
    uniqueTargetCodes.forEach((projectCode) => {
      void persistProjectMemoRecord(projectCode, next[projectCode])
    })
    return uniqueTargetCodes.length
  }

  const fetchProjectMemos = async () => {
    try {
      const response = await retryFetch(buildBackendUrl('/api/acceptance-reports/project-configs'), {
        credentials: 'include',
        headers: {
          Authorization: authStore.token
        }
      })
      if (!response.ok) {
        throw new Error(await response.text())
      }

      const data = await response.json()
      const remoteRecords: Record<string, ProjectMemoRecord> = {}
      if (Array.isArray(data)) {
        data.forEach((record) => {
          const projectCode = normalizeText(record?.project_code)
          if (!projectCode) return
          remoteRecords[projectCode] = {
            items: Array.isArray(record.items) ? record.items : [],
            updatedAt: record.updated_at || new Date().toISOString()
          }
        })
      }

      const { merged, migratedProjectCodes } = mergeProjectMemoRecords(remoteRecords, projectMemos.value)
      projectMemos.value = merged
      saveProjectMemos(merged)
      migratedProjectCodes.forEach((projectCode) => {
        void persistProjectMemoRecord(projectCode, merged[projectCode])
      })
    } catch (error) {
      console.warn('Project config API unavailable, using local memo cache', error)
    }
  }

  const fetchReports = async () => {
    if (loading.value) return
    loading.value = true
    try {
      const response = await retryFetch(buildBackendUrl('/api/acceptance-reports/list'), {
        credentials: 'include',
        headers: {
          Authorization: authStore.token
        }
      })

      if (!response.ok) {
        const rawText = await response.text()
        throw new Error(rawText || `请求失败 (${response.status})`)
      }

      const data = await response.json()
      reports.value = Array.isArray(data) ? data : []
      void fetchProjectMemos()
      const nextProjectTree = buildProjectTree(reports.value)
      const syncedProjectMemos = copyA1160MemosToTTminsProjects(nextProjectTree, projectMemos.value)
      if (syncedProjectMemos !== projectMemos.value) {
        projectMemos.value = syncedProjectMemos
        saveProjectMemos(syncedProjectMemos)
      }
      if (!nextProjectTree.some((project) => project.projectCode === selectedProjectCode.value)) {
        selectedProjectCode.value = nextProjectTree[0]?.projectCode || ''
      }
    } catch (error) {
      console.error('Failed to fetch acceptance project tree', error)
      ElMessage.error('项目树数据加载失败，请稍后重试')
    } finally {
      loading.value = false
    }
  }

  return {
    keyword,
    selectedProjectCode,
    loading,
    reports,
    projectMemos,
    projectOptions,
    allProjects: projectTree,
    projectTree: filteredProjectTree,
    selectedProject,
    selectedProjectMemo,
    stats,
    addProjectMemoItem,
    updateProjectMemoItem,
    reorderProjectMemoItems,
    deleteProjectMemoItem,
    pasteProjectMemoItem,
    fetchReports
  }
}
