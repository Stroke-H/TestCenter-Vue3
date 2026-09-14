import { v4 as uuidv4 } from 'uuid'

export type DefectStatus = 'new' | 'active' | 'resolved' | 'closed'

export interface DefectStep {
  id: string
  content: string
}

export interface DefectEnvironment {
  platform: string
  environment: string
  app_version: string
  os_version: string
  device_model: string
  network: string
}

export interface Defect {
	allowed_actions?: Record<string, boolean>
  id: string
  defect_no: string
  title: string
  project_code: string
  project_name: string
  found_version: string
  resolved_version: string
  defect_type: string
  severity: number
  priority: number
  status: DefectStatus
  resolution: string
  reporter_id: string
  reporter_name: string
  assignee_id: string
  assignee_name: string
  verifier_id: string
  verifier_name: string
  due_date: string
  precondition: string
  steps: DefectStep[]
  actual_result: string
  expected_result: string
  description: string
  environment: DefectEnvironment
  tags: string[]
  custom_fields: Record<string, any>
  reopen_count: number
  row_version: number
  archived: boolean
  resolved_at: string
  closed_at: string
  created_at: string
  updated_at: string
}

export interface DefectComment {
  id: string
  defect_id: string
  author_id: string
  author_name: string
  content: string
  created_at: string
}

export interface DefectHistory {
  id: string
  defect_id: string
  action: string
  operator_id: string
  operator_name: string
  comment: string
  created_at: string
}

export interface DefectAttachment {
  id: string
  defect_id: string
  original_name: string
  mime_type: string
  size: number
  uploader_id: string
  uploader_name: string
  created_at: string
  download_url: string
}

export interface DefectDetail {
  defect: Defect
  comments: DefectComment[]
  history: DefectHistory[]
  attachments: DefectAttachment[]
}

export interface DefectProject {
  id: string
  project_code: string
  project_name: string
  short_code?: string
}

export interface DefectDevice {
  id: string
  device_name: string
  os: string
  model: string
  allowed_app: string
}

export interface DefectAccount {
  id: string
  username: string
  nickname: string
  avatar?: string
}

export interface DefectMeta {
  project_version_stages?: Record<string, { online: string; testing: string; versions: string[]; rule?: string }>
  project_versions?: Record<string, string[]>
	project_actions?: Record<string, Record<string, boolean>>
  projects: DefectProject[]
  devices?: DefectDevice[]
  accounts: DefectAccount[]
  permissions: Record<string, boolean>
  fields: DefectFieldDefinition[]
}

export interface DefectFieldDefinition {
  id: string
  field_key: string
  name: string
  field_type: 'text' | 'textarea' | 'number' | 'select' | 'multi_select' | 'date' | 'user' | 'boolean' | 'url'
  project_code: string
  required: boolean
  enabled: boolean
  list_visible: boolean
  filterable: boolean
  options: string[]
  sort_order: number
  created_by?: string
  created_at?: string
  updated_at?: string
}

export interface DefectProjectMember {
  user_id: string
  username?: string
  nickname?: string
  role_key: 'lead' | 'tester' | 'developer' | 'product' | 'viewer'
}

export interface DefectProjectPermission {
  project_code: string
  project_name: string
  permission_mode: 'open'
  members: DefectProjectMember[]
  updated_at?: string
}

export interface DefectMemberDefaultRole {
  user_id: string
  username: string
  nickname?: string
  default_role: 'tester' | 'developer' | 'product'
}

export interface DefectStats {
  total: number
  new: number
  active: number
  resolved: number
  closed: number
  overdue: number
  reopened: number
  closure_rate: number
  average_resolve_hours: number
  status_distribution: Array<{ key: string; label: string; count: number }>
  severity_distribution: Array<{ key: string; label: string; count: number }>
  project_distribution: Array<{ key: string; label: string; count: number }>
  assignee_distribution: Array<{ key: string; label: string; count: number }>
  trend: Array<{ date: string; created: number; closed: number }>
}

export interface DefectFormValue {
  title: string
  project_code: string
  found_version: string
  defect_type: string
  severity: number
  priority: number
  assignee_id: string
  verifier_id: string
  due_date: string
  precondition: string
  steps: DefectStep[]
  actual_result: string
  expected_result: string
  description: string
  environment: DefectEnvironment
  tags: string[]
  custom_fields: Record<string, any>
  row_version: number
}

export const defectStatusMeta: Record<DefectStatus, { label: string; tone: string }> = {
  new: { label: '待确认', tone: 'slate' },
  active: { label: '处理中', tone: 'blue' },
  resolved: { label: '待验证', tone: 'amber' },
  closed: { label: '已关闭', tone: 'green' }
}

export const severityLabels: Record<number, string> = {
  1: '致命',
  2: '严重',
  3: '一般',
  4: '轻微'
}

export const priorityLabels: Record<number, string> = {
  1: 'P1 紧急',
  2: 'P2 高',
  3: 'P3 中',
  4: 'P4 低'
}

export const defectTypeOptions = [
  { value: 'function', label: '功能问题' },
  { value: 'ui', label: '界面问题' },
  { value: 'performance', label: '性能问题' },
  { value: 'compatibility', label: '兼容性问题' },
  { value: 'security', label: '安全问题' },
  { value: 'data', label: '数据问题' },
  { value: 'other', label: '其他问题' }
]

export function emptyDefectForm(): DefectFormValue {
  return {
    title: '',
    project_code: '',
    found_version: '',
    defect_type: 'function',
    severity: 3,
    priority: 3,
    assignee_id: '',
    verifier_id: '',
    due_date: '',
    precondition: '',
    steps: [{ id: uuidv4(), content: '' }],
    actual_result: '',
    expected_result: '',
    description: '',
    environment: {
      platform: '',
      environment: '',
      app_version: '',
      os_version: '',
      device_model: '',
      network: ''
    },
    tags: [],
    custom_fields: {},
    row_version: 0
  }
}

export function defectToForm(defect: Defect): DefectFormValue {
  return {
    title: defect.title,
    project_code: defect.project_code,
    found_version: defect.found_version,
    defect_type: defect.defect_type,
    severity: defect.severity,
    priority: defect.priority,
    assignee_id: defect.assignee_id,
    verifier_id: defect.verifier_id,
    due_date: defect.due_date,
    precondition: defect.precondition,
    steps: defect.steps?.length ? defect.steps.map((item) => ({ ...item })) : [{ id: uuidv4(), content: '' }],
    actual_result: defect.actual_result,
    expected_result: defect.expected_result,
    description: defect.description,
    environment: { ...defect.environment },
    tags: [...(defect.tags || [])],
    custom_fields: { ...(defect.custom_fields || {}) },
    row_version: defect.row_version
  }
}
