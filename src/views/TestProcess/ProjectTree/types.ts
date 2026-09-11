export interface AcceptanceReportRecord {
  id: string
  project_name?: string
  project_code?: string
  version?: string
  reporter?: string
  test_time?: string
  update_requirements?: string
  test_conclusion?: string
  status?: string
  created_at?: string
  updated_at?: string
}

export interface ProjectVersionNode {
  version: string
  reportCount: number
  latestTestTime: string
  reports: AcceptanceReportRecord[]
}

export interface ProjectTreeNode {
  projectCode: string
  projectName: string
  reportCount: number
  versionCount: number
  latestTestTime: string
  versions: ProjectVersionNode[]
}

export interface ProjectMemoItem {
  feature?: string
  audience?: string
  platform?: string
  variant?: string
  value?: string
  previousValue?: string
  category?: string
  version?: string
  evidence?: string
  removed?: boolean
  id: string
  content: string
  color: ProjectMemoColor
  updatedAt: string
  history?: ProjectMemoHistoryItem[]
  kind?: 'manual' | 'ai'
  configKey?: string
  sourceReportId?: string
  sourceHash?: string
}

export type ProjectMemoColor = 'green' | 'red' | 'orange' | 'blue'

export interface ProjectMemoHistoryItem {
  content: string
  color: ProjectMemoColor
  modifiedAt: string
}

export interface ProjectMemoRecord {
  schema?: number
  current_version?: string
  versions?: ProjectConfigVersion[]
  warnings?: string[]
  legacy_items?: ProjectMemoItem[]
  items: ProjectMemoItem[]
  updatedAt: string
}

export interface ProjectConfigVersion {
  version: string
  submittedAt: string
  reportId: string
  items: ProjectMemoItem[]
}
