import request from '@/api/request'
import type {
  Defect,
  DefectDetail,
  DefectDevice,
  DefectFieldDefinition,
  DefectFormValue,
  DefectMeta,
  DefectMemberDefaultRole,
  DefectProjectPermission,
  DefectStats,
  DefectStatus
} from './types'

export interface DefectListParams {
  [key: string]: string | number | undefined
  search?: string
  project_code?: string
  found_version?: string
  status?: string
  severity?: string
  priority?: string
  assignee_id?: string
  reporter_id?: string
  page: number
  page_size: number
}

export interface DefectListResult {
  items: Defect[]
  total: number
  page: number
  page_size: number
}

export interface DefectTransitionPayload {
  action: 'confirm' | 'resolve' | 'close' | 'reopen'
  assignee_id?: string
  resolution?: string
  resolved_version?: string
  comment?: string
}

export interface DefectBatchPayload {
  ids: string[]
  action: 'assign' | 'severity' | 'priority' | 'confirm' | 'resolve' | 'close' | 'reopen' | 'archive'
  assignee_id?: string
  severity?: number
  priority?: number
  resolution?: string
  resolved_version?: string
  comment?: string
}

export interface DefectBatchResult {
  id: string
  defect_no: string
  success: boolean
  error?: string
}

export const defectApi = {
  updateTestingVersion: (project: string, payload: { testing: string; expected_testing: string; expected_online: string }) => request.put(`/defects/testing-version/${encodeURIComponent(project)}`, payload) as Promise<{online: string; testing: string; versions: string[]}>,
  saveVersions: (project: string, stages: { online: string; testing: string }) => request.put(`/defects/versions/${encodeURIComponent(project)}`, stages),
  meta: () => request.get('/defects/meta') as Promise<DefectMeta>,
  devices: () => request.get('/config/devices') as Promise<DefectDevice[]>,
  stats: (params: { project_code?: string; date_from?: string; date_to?: string }) => request.get('/defects/stats', { params }) as Promise<DefectStats>,
  list: (params: DefectListParams) => request.get('/defects', { params }) as Promise<DefectListResult>,
  detail: (id: string) => request.get(`/defects/${id}`) as Promise<DefectDetail>,
  create: (payload: DefectFormValue) => request.post('/defects', payload) as Promise<Defect>,
  batchCreate: (items: DefectFormValue[]) => request.post('/defects/batch-create', { items }) as Promise<{ items: Defect[] }>,
  update: (id: string, payload: DefectFormValue) => request.put(`/defects/${id}`, payload) as Promise<Defect>,
  setStatus: (id: string, payload: { status: DefectStatus; row_version: number }) => request.put(`/defects/${id}/status`, payload) as Promise<Defect>,
  transition: (id: string, payload: DefectTransitionPayload) => request.post(`/defects/${id}/transition`, payload) as Promise<Defect>,
  batch: (payload: DefectBatchPayload) => request.post('/defects/batch', payload) as Promise<{ results: DefectBatchResult[] }>,
  fields: () => request.get('/defects/fields') as Promise<DefectFieldDefinition[]>,
  createField: (payload: DefectFieldDefinition) => request.post('/defects/fields', payload) as Promise<DefectFieldDefinition>,
  updateField: (id: string, payload: DefectFieldDefinition) => request.put(`/defects/fields/${id}`, payload) as Promise<DefectFieldDefinition>,
  projectPermissions: () => request.get('/defects/project-permissions') as Promise<DefectProjectPermission[]>,
  saveProjectPermission: (projectCode: string, payload: Pick<DefectProjectPermission, 'permission_mode' | 'members'>) =>
    request.put(`/defects/project-permissions/${encodeURIComponent(projectCode)}`, payload),
  memberDefaultRoles: () => request.get('/defects/member-default-roles') as Promise<DefectMemberDefaultRole[]>,
  saveMemberDefaultRoles: (members: DefectMemberDefaultRole[]) => request.put('/defects/member-default-roles', { members }),
  comment: (id: string, content: string) => request.post(`/defects/${id}/comments`, { content }),
  upload: (id: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request.post(`/defects/${id}/attachments`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 30000
    })
  },
  deleteAttachment: (id: string, attachmentId: string) => request.delete(`/defects/${id}/attachments/${attachmentId}`),
  attachmentBlob: (id: string, attachmentId: string) => request.get(`/defects/${id}/attachments/${attachmentId}`, {
    responseType: 'blob',
    timeout: 30000
  }) as Promise<Blob>
}
