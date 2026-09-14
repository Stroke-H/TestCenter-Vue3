import type {
  ProjectMemoItem,
  ProjectMemoRecord,
  ProjectVersionNode
} from '../types'

export interface ParsedConfigItem {
  id: string
  feature: string
  category: string
  scope: string
  value: string
  previousValue: string
  version: string
  evidence?: string
  sourceReportId?: string
  removed: boolean
  updatedAt: string
  kind: 'manual' | 'ai'
  raw: ProjectMemoItem
}

export interface VersionChangeSummary {
  version: string
  submittedAt?: string
  reportId?: string
  testTime?: string
  added: ParsedConfigItem[]
  modified: ParsedConfigItem[]
  deprecated: ParsedConfigItem[]
  totalChanges: number
}

export const parseVersionParts = (version: string) => {
  const normalized = (version || '').trim()
  const numericParts = normalized.match(/\d+/g)?.map((item) => Number(item)) || []
  return {
    normalized,
    numericParts
  }
}

export const compareVersionDesc = (a: string, b: string) => {
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

export const parseSingleConfigItem = (item: ProjectMemoItem, defaultVersion?: string): ParsedConfigItem => {
  const feature = (item.feature || '').trim() || (item.content || '').trim() || '未命名配置项'
  const category = (item.category || '').trim() || '通用配置'
  const scope = [item.audience, item.platform, item.variant].filter(Boolean).join(' / ') || '全量 / 通用'
  const value = (item.value || '').trim() || (item.content || '').trim() || '已配置'
  const previousValue = (item.previousValue || '').trim()
  const version = (item.version || '').trim() || defaultVersion || '初始版本'

  return {
    id: item.id,
    feature,
    category,
    scope,
    value,
    previousValue,
    version,
    evidence: item.evidence,
    sourceReportId: item.sourceReportId,
    removed: Boolean(item.removed),
    updatedAt: item.updatedAt || '',
    kind: item.kind || 'manual',
    raw: item
  }
}

/**
 * 提取当前生效配置（过滤已废除项）
 */
export const deriveActiveConfigs = (record?: ProjectMemoRecord): ParsedConfigItem[] => {
  if (!record || !Array.isArray(record.items)) return []

  return record.items
    .filter((item) => !item.removed)
    .map((item) => parseSingleConfigItem(item, record.current_version))
}

/**
 * 提取历史废弃配置
 */
export const deriveDeprecatedConfigs = (record?: ProjectMemoRecord): ParsedConfigItem[] => {
  if (!record || !Array.isArray(record.items)) return []

  return record.items
    .filter((item) => item.removed)
    .map((item) => parseSingleConfigItem(item, record.current_version))
}

/**
 * 提取所有可用分类清单
 */
export const deriveCategories = (items: ParsedConfigItem[]): string[] => {
  const set = new Set<string>()
  items.forEach((item) => {
    if (item.category) set.add(item.category)
  })
  return Array.from(set)
}

/**
 * 派生版本演进时间轴
 */
export const deriveVersionTimeline = (
  record?: ProjectMemoRecord,
  projectVersionNodes?: ProjectVersionNode[]
): VersionChangeSummary[] => {
  // 1. 收集所有版本号
  const versionSet = new Set<string>()

  if (record?.current_version) {
    versionSet.add(record.current_version)
  }

  if (record?.versions && Array.isArray(record.versions)) {
    record.versions.forEach((v) => {
      if (v.version) versionSet.add(v.version)
    })
  }

  if (record?.items && Array.isArray(record.items)) {
    record.items.forEach((item) => {
      if (item.version) versionSet.add(item.version)
    })
  }

  if (projectVersionNodes && Array.isArray(projectVersionNodes)) {
    projectVersionNodes.forEach((node) => {
      if (node.version) versionSet.add(node.version)
    })
  }

  const sortedVersions = Array.from(versionSet).filter(Boolean).sort(compareVersionDesc)

  // 映射各版本对应的测试时间与报告编号
  const versionMetaMap = new Map<string, { testTime?: string; reportId?: string }>()
  if (projectVersionNodes) {
    projectVersionNodes.forEach((node) => {
      versionMetaMap.set(node.version, {
        testTime: node.latestTestTime,
        reportId: node.reports?.[0]?.id
      })
    })
  }

  // 2. 为每个版本归纳 added / modified / deprecated
  return sortedVersions.map((ver) => {
    const meta = versionMetaMap.get(ver)
    const ledgerVer = record?.versions?.find((v) => v.version === ver)

    // 优先从 ledgerVer.items 或 record.items 中提取归属于该版本的 items
    let candidateItems: ProjectMemoItem[] = []

    if (ledgerVer && Array.isArray(ledgerVer.items)) {
      candidateItems = ledgerVer.items
    } else if (record?.items) {
      candidateItems = record.items.filter((item) => (item.version || '').trim() === ver)
    }

    const added: ParsedConfigItem[] = []
    const modified: ParsedConfigItem[] = []
    const deprecated: ParsedConfigItem[] = []

    candidateItems.forEach((item) => {
      const parsed = parseSingleConfigItem(item, ver)
      if (parsed.removed) {
        deprecated.push(parsed)
      } else if (parsed.previousValue && parsed.previousValue !== parsed.value) {
        modified.push(parsed)
      } else {
        added.push(parsed)
      }
    })

    const totalChanges = added.length + modified.length + deprecated.length

    return {
      version: ver,
      submittedAt: ledgerVer?.submittedAt,
      reportId: ledgerVer?.reportId || meta?.reportId,
      testTime: meta?.testTime,
      added,
      modified,
      deprecated,
      totalChanges
    }
  })
}
