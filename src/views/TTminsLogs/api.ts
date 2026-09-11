import request from '@/api/request'

export interface TTminsLogInfo {
  version: string
  integrity: string
  script_path: string
  script_options: Array<{ label: string; url: string }>
}

export interface TTminsLogTarget {
  id: string
  title: string
  url: string
  ip: string
  user_agent: string
  connected_at: string
  inspecting: boolean
}

export const ttminsLogsApi = {
  info: () => request.get('/ttmins-logs/info') as Promise<TTminsLogInfo>,
  targets: () => request.get('/ttmins-logs/targets') as Promise<{ targets: TTminsLogTarget[]; timestamp: number }>,
  inspect: (id: string) => request.post(`/ttmins-logs/targets/${encodeURIComponent(id)}/inspect`) as Promise<{ client_path: string; inspector_path: string }>,
  disconnect: (id: string) => request.delete(`/ttmins-logs/targets/${encodeURIComponent(id)}`)
}
