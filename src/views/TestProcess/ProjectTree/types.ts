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

export interface ProjectMemoRecord {
  content: string
  updatedAt: string
}
