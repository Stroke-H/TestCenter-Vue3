<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, shallowRef } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import NovelProjectList from '@/components/NovelWriter/NovelProjectList.vue'
import NovelMaterialMap from '@/components/NovelWriter/NovelMaterialMap.vue'
import NovelMaterialPanel from '@/components/NovelWriter/NovelMaterialPanel.vue'
import NovelStructurePanel from '@/components/NovelWriter/NovelStructurePanel.vue'
import NovelChapterPanel from '@/components/NovelWriter/NovelChapterPanel.vue'
import NovelAuditPanel from '@/components/NovelWriter/NovelAuditPanel.vue'
import {
  emptyNovelMaterials,
  novelWriterApi,
  type CreateNovelProjectPayload,
  type NovelExtractedInfo,
  type NovelInfoCard,
  type NovelMaterials,
  type NovelStyleProfile,
  type NovelProject
} from '@/api/novelWriter'

const projects = shallowRef<NovelProject[]>([])
const selectedProject = shallowRef<NovelProject | null>(null)
const selectedOutlineId = shallowRef('')
const selectedChapterId = shallowRef('')
const loading = shallowRef(false)
const saving = shallowRef(false)
const running = shallowRef(false)
const createDialogVisible = shallowRef(false)
const activeStep = shallowRef<'materials' | 'generation'>('materials')
const insightsOpen = shallowRef(false)
const materialPanelRef = shallowRef<InstanceType<typeof NovelMaterialPanel> | null>(null)

const workspaceSwitchItemClass = (step: 'materials' | 'generation') => [
  'workspace-switch__item',
  { 'workspace-switch__item--active': activeStep.value === step }
]

const createForm = reactive<CreateNovelProjectPayload>({
  title: '',
  genre: '',
  target_words: 80000,
  target_chapters: 30,
  materials: emptyNovelMaterials()
})

const selectedAudit = computed(() => {
  const chapter = selectedProject.value?.chapters?.find((item) => item.id === selectedChapterId.value)
  return chapter?.audit || {
    total_score: 0,
    ai_flavor_score: 0,
    character_score: 0,
    logic_score: 0,
    style_score: 0,
    issues: [],
    revision_advice: ''
  }
})

const currentMaterials = computed({
  get() {
    return selectedProject.value?.materials || emptyNovelMaterials()
  },
  set(value: NovelMaterials) {
    if (!selectedProject.value) return
    selectedProject.value = {
      ...selectedProject.value,
      materials: value
    }
  }
})

const splitMaterialBlocks = (text: string) => {
  return String(text || '')
    .split(/\n\s*\n|\n-/)
    .map((item) => item.replace(/^-/, '').trim())
    .filter(Boolean)
}

const splitMaterialLines = (text: string) => {
  return String(text || '')
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
}

const parseMaterialCard = (raw: string, fallbackName: string) => {
  const lines = String(raw || '')
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
  const firstLine = lines[0] || ''
  const matched = firstLine.match(/^(?:新人物|人物|角色|世界观|冲突|灵感|事件)\s*[:：]\s*(.+)$/)
  const name = matched?.[1]?.trim() || firstLine.replace(/[:：]$/, '').trim() || fallbackName
  const description = matched ? lines.slice(1).join('；') : lines.slice(1).join('；') || raw
  return {
    name,
    description: description || raw
  }
}

const buildMaterialExtracted = (materials: NovelMaterials): NovelExtractedInfo => ({
  characters: splitMaterialBlocks(materials.character_raw).map((description, index) => parseMaterialCard(description, `人物 ${index + 1}`)),
  world_rules: splitMaterialBlocks(materials.world_raw).map((description, index) => parseMaterialCard(description, `世界观 ${index + 1}`)),
  conflicts: splitMaterialLines(materials.conflict_raw).map((description, index) => parseMaterialCard(description, `冲突 ${index + 1}`)),
  key_events: splitMaterialLines(materials.raw_text).map((description, index) => parseMaterialCard(description, `灵感 ${index + 1}`)),
  open_questions: []
})

const normalizeInfoCardText = (value: string) => {
  return String(value || '')
    .replace(/^(?:新人物|人物|角色|世界观|冲突|灵感|事件)\s*[:：]/, '')
    .replace(/[，,；;。.\s:：]/g, '')
    .toLowerCase()
}

const isSimilarInfoCard = (current: NovelInfoCard, existing: NovelInfoCard) => {
  const currentName = normalizeInfoCardText(current.name)
  const currentDescription = normalizeInfoCardText(current.description)
  const existingName = normalizeInfoCardText(existing.name)
  const existingDescription = normalizeInfoCardText(existing.description)
  const currentFull = `${currentName}${currentDescription}`
  const existingFull = `${existingName}${existingDescription}`

  if (!currentFull || !existingFull) return false
  if (currentName && existingName && currentName === existingName) return true
  return currentFull.includes(existingFull) || existingFull.includes(currentFull)
}

const mergeInfoCards = (source: NovelInfoCard[] = [], preview: NovelInfoCard[] = []) => {
  const result: NovelInfoCard[] = []
  ;[...preview, ...source].forEach((item) => {
    if (!String(item.name || item.description || '').trim()) return
    if (result.some((existing) => isSimilarInfoCard(item, existing))) return
    result.push(item)
  })
  return result
}

const displayExtracted = computed<NovelExtractedInfo>(() => {
  const extracted = selectedProject.value?.extracted
  const materialPreview = buildMaterialExtracted(currentMaterials.value)
  return {
    characters: mergeInfoCards(extracted?.characters, materialPreview.characters),
    world_rules: mergeInfoCards(extracted?.world_rules, materialPreview.world_rules),
    conflicts: mergeInfoCards(extracted?.conflicts, materialPreview.conflicts),
    key_events: mergeInfoCards(extracted?.key_events, materialPreview.key_events),
    open_questions: extracted?.open_questions || []
  }
})

const displayStyleProfile = computed<NovelStyleProfile>(() => {
  const profile = selectedProject.value?.style_profile
  if (
    profile?.summary
    || profile?.narration
    || profile?.sentence
    || profile?.dialogue
    || profile?.rhythm
    || profile?.do_rules?.length
    || profile?.avoid_rules?.length
  ) {
    return profile
  }

  const hasReference = Boolean(currentMaterials.value.reference_raw.trim())
  return {
    summary: hasReference
      ? '已提供文风参考文本，生成文风画像后会提炼抽象写作规则。'
      : '未提供文风参考，可使用模型默认生成原创文风画像。',
    narration: '',
    sentence: '',
    dialogue: '',
    rhythm: '',
    do_rules: hasReference ? ['已提供参考文本', '后续生成将优先使用抽象文风规则'] : ['默认原创文风'],
    avoid_rules: []
  }
})

const refreshProjects = async () => {
  loading.value = true
  try {
    projects.value = await novelWriterApi.listProjects()
    if (selectedProject.value) {
      const latest = projects.value.find((project) => project.id === selectedProject.value?.id)
      if (latest) selectProject(latest)
    }
  } catch (error) {
    ElMessage.error(`加载小说项目失败：${error}`)
  } finally {
    loading.value = false
  }
}

const selectProject = (project: NovelProject) => {
  selectedProject.value = project
  selectedOutlineId.value = project.outline?.chapters?.[0]?.id || ''
  selectedChapterId.value = project.chapters?.[0]?.id || ''
  activeStep.value = 'materials'
}

const backToProjectList = () => {
  selectedProject.value = null
  selectedOutlineId.value = ''
  selectedChapterId.value = ''
  insightsOpen.value = false
}

const createProject = async () => {
  if (!createForm.title.trim()) {
    ElMessage.warning('请先填写小说名称')
    return
  }
  saving.value = true
  try {
    const project = await novelWriterApi.createProject(createForm)
    projects.value = [project, ...projects.value]
    selectProject(project)
    createDialogVisible.value = false
    Object.assign(createForm, {
      title: '',
      genre: '',
      target_words: 80000,
      target_chapters: 30,
      materials: emptyNovelMaterials()
    })
    ElMessage.success('小说项目已创建')
  } catch (error) {
    ElMessage.error(`创建失败：${error}`)
  } finally {
    saving.value = false
  }
}

const syncSelectedProject = (project: NovelProject) => {
  selectedProject.value = project
  projects.value = projects.value.map((item) => item.id === project.id ? project : item)
  const outlineExists = project.outline?.chapters?.some((chapter) => chapter.id === selectedOutlineId.value)
  if (!selectedOutlineId.value || !outlineExists) selectedOutlineId.value = project.outline?.chapters?.[0]?.id || ''
  if (!selectedChapterId.value) selectedChapterId.value = project.chapters?.[0]?.id || ''
}

const saveCurrentProject = async () => {
  if (!selectedProject.value) return
  saving.value = true
  try {
    syncSelectedProject(await novelWriterApi.updateProject(selectedProject.value))
    ElMessage.success('素材已保存')
  } catch (error) {
    ElMessage.error(`保存失败：${error}`)
  } finally {
    saving.value = false
  }
}

const runProjectAction = async (label: string, action: () => Promise<NovelProject>) => {
  if (!selectedProject.value) return
  running.value = true
  try {
    const project = await action()
    syncSelectedProject(project)
    ElMessage.success(`${label}完成`)
    return project
  } catch (error) {
    ElMessage.error(`${label}失败：${error}`)
  } finally {
    running.value = false
  }
}

const saveBeforeAIAction = async () => {
  if (!selectedProject.value) return
  syncSelectedProject(await novelWriterApi.updateProject(selectedProject.value))
}

const extractInfo = () => runProjectAction('信息提取', async () => {
  await saveBeforeAIAction()
  return novelWriterApi.extractInfo(selectedProject.value!.id)
})

const planOutline = async () => {
  const project = await runProjectAction('大纲规划', async () => {
    await saveBeforeAIAction()
    return novelWriterApi.planOutline(selectedProject.value!.id)
  })
  if (project?.outline?.chapters?.length) insightsOpen.value = true
}

const analyzeStyle = () => runProjectAction('文风画像', async () => {
  await saveBeforeAIAction()
  return novelWriterApi.analyzeStyle(selectedProject.value!.id)
})

const switchToGeneration = async () => {
  await saveBeforeAIAction()
  activeStep.value = 'generation'
  await nextTick()
}

const handleMaterialMapNext = async () => {
  await switchToGeneration()
  if (currentMaterials.value.reference_raw.trim()) return

  try {
    await ElMessageBox.confirm(
      '未提供文风参考，是否使用模型默认生成？',
      '文风参考确认',
      {
        confirmButtonText: '确认',
        cancelButtonText: '取消',
        type: 'info'
      }
    )
    await analyzeStyle()
  } catch (error) {
    if (error === 'cancel' || error === 'close') {
      await nextTick()
      materialPanelRef.value?.openReferenceDialog()
      return
    }
    ElMessage.error(`文风画像生成失败：${error}`)
  }
}

const ensureOutlineReady = async () => {
  if (!selectedProject.value) throw new Error('请先选择小说项目')
  await saveBeforeAIAction()

  const hasExtractedInfo = Boolean(
    selectedProject.value.extracted?.characters?.length
    || selectedProject.value.extracted?.world_rules?.length
    || selectedProject.value.extracted?.conflicts?.length
    || selectedProject.value.extracted?.key_events?.length
  )
  if (!hasExtractedInfo) {
    ElMessage.info('正在先补充信息提取...')
    syncSelectedProject(await novelWriterApi.extractInfo(selectedProject.value.id))
  }

  if (!selectedProject.value.outline?.chapters?.length) {
    ElMessage.info('正在先生成章节大纲...')
    syncSelectedProject(await novelWriterApi.planOutline(selectedProject.value.id))
  }

  const outlineId = selectedOutlineId.value || selectedProject.value.outline?.chapters?.[0]?.id || ''
  if (!outlineId) throw new Error('大纲生成失败：没有可用章节')
  selectedOutlineId.value = outlineId
  return outlineId
}

const generateChapter = () => runProjectAction('章节生成', async () => {
  const outlineId = await ensureOutlineReady()
  return novelWriterApi.generateChapter(selectedProject.value!.id, outlineId)
})
const auditChapter = (chapterId: string) => runProjectAction('章节审计', () => novelWriterApi.auditChapter(selectedProject.value!.id, chapterId))
const reviseChapter = (chapterId: string) => runProjectAction('智能修订', () => novelWriterApi.reviseChapter(selectedProject.value!.id, chapterId))
const approveChapter = (chapterId: string) => runProjectAction('章节确认', () => novelWriterApi.approveChapter(selectedProject.value!.id, chapterId))

const exportProjectMarkdown = (project: NovelProject) => {
  const content = [
    `# ${project.title}`,
    '',
    ...(project.chapters || []).map((chapter) => `## ${chapter.title}\n\n${chapter.content}`)
  ].join('\n\n')
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `${project.title || 'novel'}.md`
  link.click()
  URL.revokeObjectURL(link.href)
}

const deleteProject = async (project: NovelProject) => {
  try {
    await ElMessageBox.confirm(
      `确认删除《${project.title}》吗？删除后无法在平台内恢复。`,
      '删除小说项目',
      {
        confirmButtonText: '删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }
    )
    try {
      await novelWriterApi.deleteProject(project.id)
    } catch (error: any) {
      if (error?.response?.status !== 404) throw error
      await novelWriterApi.deleteProjectFallback(project.id)
    }
    projects.value = projects.value.filter((item) => item.id !== project.id)
    if (selectedProject.value?.id === project.id) backToProjectList()
    ElMessage.success('小说项目已删除')
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(`删除失败：${error}`)
  }
}

onMounted(refreshProjects)
</script>

<template>
  <div class="novel-page">
    <main class="novel-workspace">
      <NovelProjectList
        v-if="!selectedProject"
        :projects="projects"
        selected-id=""
        :loading="loading"
        @select="selectProject"
        @create="createDialogVisible = true"
        @export="exportProjectMarkdown"
        @delete="deleteProject"
      />

      <template v-else>
        <NovelMaterialMap
          v-if="activeStep === 'materials'"
          v-model="currentMaterials"
          :saving="saving"
          @save="saveCurrentProject"
          @extract="extractInfo"
          @next="handleMaterialMapNext"
        >
          <template #workspace-switch>
            <div class="workspace-switch">
              <button
                :class="workspaceSwitchItemClass('materials')"
                @click="activeStep = 'materials'"
              >
                素材图谱
              </button>
              <button
                :class="workspaceSwitchItemClass('generation')"
                @click="switchToGeneration"
              >
                文风生成
              </button>
            </div>
          </template>
        </NovelMaterialMap>

        <div v-else class="generation-workspace">
          <div class="generation-switch-row">
            <div class="workspace-switch">
              <button
                :class="workspaceSwitchItemClass('materials')"
                @click="activeStep = 'materials'"
              >
                素材图谱
              </button>
              <button
                :class="workspaceSwitchItemClass('generation')"
                @click="switchToGeneration"
              >
                文风生成
              </button>
            </div>
          </div>

          <NovelMaterialPanel
            ref="materialPanelRef"
            v-model="currentMaterials"
            :saving="saving"
            @save="saveCurrentProject"
            @outline="planOutline"
            @style="analyzeStyle"
          />

          <NovelStructurePanel
            :extracted="displayExtracted"
            :outline="selectedProject.outline"
            :style-profile="displayStyleProfile"
            :open="insightsOpen"
            @update:open="insightsOpen = $event"
          />

          <NovelChapterPanel
            :outline="selectedProject.outline"
            :selected-outline-id="selectedOutlineId"
            :chapters="selectedProject.chapters || []"
            :selected-chapter-id="selectedChapterId"
            :running="running"
            @select-outline="selectedOutlineId = $event"
            @select-chapter="selectedChapterId = $event"
            @generate="generateChapter"
            @audit="auditChapter"
            @revise="reviseChapter"
            @approve="approveChapter"
          />

          <NovelAuditPanel :audit="selectedAudit" />
        </div>
      </template>
    </main>

    <el-dialog v-model="createDialogVisible" title="新建小说项目" width="560px">
      <el-form label-position="top">
        <el-form-item label="小说名称">
          <el-input v-model="createForm.title" placeholder="例如：风暴之前" />
        </el-form-item>
        <el-form-item label="题材">
          <el-input v-model="createForm.genre" placeholder="都市 / 玄幻 / 科幻 / 短剧改编..." />
        </el-form-item>
        <div class="dialog-grid">
          <el-form-item label="目标字数">
            <el-input-number v-model="createForm.target_words" :min="1000" :step="10000" />
          </el-form-item>
          <el-form-item label="目标章节数">
            <el-input-number v-model="createForm.target_chapters" :min="1" :step="1" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="createProject">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.novel-page {
  padding: 0 0 32px;
}

.novel-workspace {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

.generation-workspace {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.workspace-switch {
  display: inline-flex;
  gap: 6px;
  padding: 7px;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 18px 45px rgba(15, 23, 42, 0.13);
  backdrop-filter: blur(14px);
}

.workspace-switch__item {
  padding: 9px 18px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: #64748b;
  font-weight: 800;
  cursor: pointer;
}

.workspace-switch__item--active {
  background: #0f766e;
  color: #ffffff;
}

.generation-switch-row {
  display: flex;
  justify-content: center;
  min-height: 48px;
}

.dialog-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

</style>
