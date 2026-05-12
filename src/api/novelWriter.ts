import request from '@/api/request'

const novelAIRequestConfig = {
  timeout: 120000
}

export interface NovelInfoCard {
  name: string
  description: string
}

export interface NovelCharacterState {
  name: string
  state: string
  location: string
}

export interface NovelChapterState {
  characters: NovelCharacterState[]
  plot_hooks: string[]
  plot_advances: string[]
}

export interface NovelTensionPoint {
  position: number
  value: number
  note: string
}

export interface NovelMaterials {
  raw_text: string
  character_raw: string
  world_raw: string
  conflict_raw: string
  reference_raw: string
}

export interface NovelExtractedInfo {
  characters: NovelInfoCard[]
  world_rules: NovelInfoCard[]
  conflicts: NovelInfoCard[]
  key_events: NovelInfoCard[]
  open_questions: string[]
}

export interface NovelChapterOutline {
  id: string
  title: string
  goal: string
  conflict: string
  hook: string
  summary: string
  before_state: NovelChapterState
  after_state: NovelChapterState
  must_happen: string[]
  tension_curve: NovelTensionPoint[]
  key_scenes: string[]
  new_hooks: string[]
}

export interface NovelOutline {
  logline: string
  acts: NovelInfoCard[]
  chapters: NovelChapterOutline[]
  generation_status: string
  target_chapters: number
  generated_chapters: number
  batch_size: number
  generation_error: string
}

export interface NovelStyleProfile {
  summary: string
  narration: string
  sentence: string
  dialogue: string
  rhythm: string
  do_rules: string[]
  avoid_rules: string[]
}

export interface NovelAuditItem {
  severity: string
  title: string
  detail: string
  suggestion: string
}

export interface NovelAuditReport {
  total_score: number
  ai_flavor_score: number
  character_score: number
  logic_score: number
  style_score: number
  issues: NovelAuditItem[]
  revision_advice: string
}

export interface NovelChapterVersion {
  id: string
  type: string
  content: string
  reason: string
  created_at: string
}

export interface NovelChapter {
  id: string
  outline_id: string
  title: string
  status: string
  content: string
  summary: string
  audit: NovelAuditReport
  versions: NovelChapterVersion[]
  created_at: string
  updated_at: string
}

export interface NovelMemory {
  chapter_summaries: NovelInfoCard[]
  character_states: NovelInfoCard[]
  open_hooks: NovelInfoCard[]
  timeline: NovelInfoCard[]
}

export interface NovelProject {
  id: string
  title: string
  genre: string
  target_words: number
  target_chapters: number
  status: string
  current_stage: string
  created_by: string
  created_at: string
  updated_at: string
  materials: NovelMaterials
  extracted: NovelExtractedInfo
  outline: NovelOutline
  style_profile: NovelStyleProfile
  chapters: NovelChapter[]
  memory: NovelMemory
}

export interface CreateNovelProjectPayload {
  title: string
  genre: string
  target_words: number
  target_chapters: number
  materials: NovelMaterials
}

export function emptyNovelMaterials(): NovelMaterials {
  return {
    raw_text: '',
    character_raw: '',
    world_raw: '',
    conflict_raw: '',
    reference_raw: ''
  }
}

export const novelWriterApi = {
  listProjects() {
    return request.get('/novel-writer/projects') as Promise<NovelProject[]>
  },
  createProject(payload: CreateNovelProjectPayload) {
    return request.post('/novel-writer/projects', payload) as Promise<NovelProject>
  },
  getProject(projectId: string) {
    return request.get(`/novel-writer/projects/${projectId}`) as Promise<NovelProject>
  },
  updateProject(project: NovelProject) {
    return request.put(`/novel-writer/projects/${project.id}`, project) as Promise<NovelProject>
  },
  deleteProject(projectId: string) {
    return request.delete(`/novel-writer/projects/${projectId}`) as Promise<void>
  },
  deleteProjectFallback(projectId: string) {
    return request.post(`/novel-writer/projects/${projectId}/delete`) as Promise<void>
  },
  extractInfo(projectId: string) {
    return request.post(`/novel-writer/projects/${projectId}/extract`, undefined, novelAIRequestConfig) as Promise<NovelProject>
  },
  planOutline(projectId: string) {
    return request.post(`/novel-writer/projects/${projectId}/outline`, undefined, novelAIRequestConfig) as Promise<NovelProject>
  },
  analyzeStyle(projectId: string) {
    return request.post(`/novel-writer/projects/${projectId}/style`, undefined, novelAIRequestConfig) as Promise<NovelProject>
  },
  generateChapter(projectId: string, outlineId?: string) {
    return request.post(`/novel-writer/projects/${projectId}/chapters/generate`, {
      outline_id: outlineId || ''
    }, novelAIRequestConfig) as Promise<NovelProject>
  },
  auditChapter(projectId: string, chapterId: string) {
    return request.post(`/novel-writer/projects/${projectId}/chapters/${chapterId}/audit`, undefined, novelAIRequestConfig) as Promise<NovelProject>
  },
  reviseChapter(projectId: string, chapterId: string) {
    return request.post(`/novel-writer/projects/${projectId}/chapters/${chapterId}/revise`, undefined, novelAIRequestConfig) as Promise<NovelProject>
  },
  approveChapter(projectId: string, chapterId: string) {
    return request.post(`/novel-writer/projects/${projectId}/chapters/${chapterId}/approve`) as Promise<NovelProject>
  }
}
