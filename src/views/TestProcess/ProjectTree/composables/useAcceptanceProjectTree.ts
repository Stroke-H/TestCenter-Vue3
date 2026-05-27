import { computed, shallowRef } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { buildBackendUrl } from '@/utils/runtimeUrl'
import { retryFetch } from '@/utils/retryFetch'
import type { AcceptanceReportRecord, ProjectTreeNode, ProjectVersionNode } from '../types'

const EMPTY_PROJECT_CODE = '未填写项目代码'
const EMPTY_PROJECT_NAME = '未命名项目'
const EMPTY_VERSION = '未填写版本'

const normalizeText = (value?: string) => (value || '').trim()

const getReportTime = (report: Partial<AcceptanceReportRecord>) => (
  normalizeText(report.test_time) ||
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
    }).sort((a, b) => compareByTimeDesc(a.latestTestTime, b.latestTestTime))

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

export function useAcceptanceProjectTree() {
  const authStore = useAuthStore()
  const reports = shallowRef<AcceptanceReportRecord[]>([])
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

  const fetchReports = async () => {
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
      const nextProjectTree = buildProjectTree(reports.value)
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
    projectOptions,
    projectTree: filteredProjectTree,
    stats,
    fetchReports
  }
}
