<script setup lang="ts">
import { computed, ref, shallowRef, onBeforeUnmount, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { CollectionTag, Refresh, Search, Plus, Delete, Edit, Check, Close, DocumentCopy } from '@element-plus/icons-vue'
import ProjectTreeBoard from './components/ProjectTreeBoard.vue'
import ProjectConfigRecordsDialog from './components/ProjectConfigRecordsDialog.vue'
import ProjectConfigLedger from './components/ProjectConfigLedger.vue'
import { useAcceptanceProjectTree } from './composables/useAcceptanceProjectTree'
import type { ProjectMemoColor, ProjectMemoItem } from './types'
import {
  ACCEPTANCE_REPORTS_CHANGED_EVENT,
  isAcceptanceReportsChangedStorageKey
} from '@/utils/acceptanceReportEvents'

defineOptions({ name: 'ProjectTree' })

const {
  keyword,
  selectedProjectCode,
  loading,
  projectOptions,
  allProjects,
  projectMemos,
  projectTree,
  selectedProject,
  selectedProjectMemo,
  addProjectMemoItem,
  updateProjectMemoItem,
  reorderProjectMemoItems,
  deleteProjectMemoItem,
  pasteProjectMemoItem,
  fetchReports
} = useAcceptanceProjectTree()

let lastAutoRefreshAt = 0

const refreshProjectTree = () => {
  const now = Date.now()
  if (now - lastAutoRefreshAt < 1000) return
  lastAutoRefreshAt = now
  fetchReports()
}

const handleVisibilityChange = () => {
  if (document.visibilityState === 'visible') {
    refreshProjectTree()
  }
}

const handleStorageChange = (event: StorageEvent) => {
  if (isAcceptanceReportsChangedStorageKey(event.key)) {
    refreshProjectTree()
  }
}

const scheduleAIConfigRefresh = () => {
  refreshProjectTree()
  window.setTimeout(refreshProjectTree, 4000)
  window.setTimeout(refreshProjectTree, 12000)
}

// Keep track of editing notes: record of itemId -> draft state
interface NoteDraft {
  content: string
  color: ProjectMemoColor
}
const editingNotes = ref<Record<string, NoteDraft>>({})
const draggingNoteId = ref('')
const dragOverNoteId = ref('')
const pasteDialogVisible = ref(false)
const pasteSourceNote = ref<ProjectMemoItem | null>(null)
const pasteTargetProjectCodes = ref<string[]>([])
const configRecordsDialogVisible = shallowRef(false)

const pasteTargetProjectOptions = computed(() => {
  return projectOptions.value.filter((project) => project.value !== selectedProjectCode.value)
})

const startEditNote = (note: ProjectMemoItem) => {
  editingNotes.value[note.id] = {
    content: note.content,
    color: note.kind === 'ai' ? 'blue' : note.color
  }
}

const cancelEditNote = (noteId: string) => {
  delete editingNotes.value[noteId]
  // If the note was newly created and is empty, delete it when canceling
  const memo = selectedProjectMemo.value
  const note = memo?.items.find((item) => item.id === noteId)
  if (note && !note.content.trim()) {
    deleteProjectMemoItem(noteId)
  }
}

const saveEditNote = (noteId: string) => {
  const draft = editingNotes.value[noteId]
  if (!draft) return

  const trimmed = draft.content.trim()
  if (!trimmed) {
    // If saving an empty note, we just delete it
    deleteProjectMemoItem(noteId)
  } else {
    updateProjectMemoItem(noteId, {
      content: trimmed,
      color: draft.color
    })
  }
  delete editingNotes.value[noteId]
}

const handleNoteEnter = (event: KeyboardEvent, noteId: string) => {
  if (event.isComposing || event.keyCode === 229) {
    return
  }
  event.preventDefault()
  saveEditNote(noteId)
}

const handleAddNewNote = (color: 'green' | 'red' | 'orange' = 'green') => {
  const newItem = addProjectMemoItem('', color)
  if (newItem) {
    startEditNote(newItem)
  }
}

const setDraftColor = (noteId: string, color: 'green' | 'red' | 'orange') => {
  const draft = editingNotes.value[noteId]
  const note = selectedProjectMemo.value?.items.find((item) => item.id === noteId)
  if (draft && note?.kind !== 'ai') {
    draft.color = color
  }
}

const formatNoteTime = (value: string) => {
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

const getNoteHistoryTooltip = (note: ProjectMemoItem) => {
  const history = note.history || []
  if (!history.length) return '暂无历史修改记录'

  return history
    .slice()
    .reverse()
    .map((item) => `${formatNoteTime(item.modifiedAt)} 修改前：${item.content}`)
    .join('\n')
}

const handleNoteDragStart = (event: DragEvent, noteId: string) => {
  if (editingNotes.value[noteId]) return
  draggingNoteId.value = noteId
  event.dataTransfer?.setData('text/plain', noteId)
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
  }
}

const handleNoteDragEnter = (noteId: string) => {
  if (!draggingNoteId.value || draggingNoteId.value === noteId) return
  dragOverNoteId.value = noteId
}

const handleNoteDrop = (event: DragEvent, targetNoteId: string) => {
  event.preventDefault()
  const sourceNoteId = event.dataTransfer?.getData('text/plain') || draggingNoteId.value
  if (sourceNoteId && sourceNoteId !== targetNoteId) {
    reorderProjectMemoItems(sourceNoteId, targetNoteId)
  }
  draggingNoteId.value = ''
  dragOverNoteId.value = ''
}

const handleNoteDragEnd = () => {
  draggingNoteId.value = ''
  dragOverNoteId.value = ''
}

const openPasteNoteDialog = (note: ProjectMemoItem) => {
  pasteSourceNote.value = note
  pasteTargetProjectCodes.value = []
  pasteDialogVisible.value = true
}

const confirmPasteNote = () => {
  const sourceNote = pasteSourceNote.value
  if (!sourceNote) return
  if (!pasteTargetProjectCodes.value.length) {
    ElMessage.warning('请选择至少一个目标项目')
    return
  }

  const pastedCount = pasteProjectMemoItem(sourceNote, pasteTargetProjectCodes.value)
  if (pastedCount <= 0) {
    ElMessage.warning('没有可粘贴的目标项目')
    return
  }

  ElMessage.success(`已粘贴到 ${pastedCount} 个项目`)
  pasteDialogVisible.value = false
  pasteSourceNote.value = null
  pasteTargetProjectCodes.value = []
}

onMounted(() => {
  fetchReports()
  window.addEventListener(ACCEPTANCE_REPORTS_CHANGED_EVENT, scheduleAIConfigRefresh)
  window.addEventListener('storage', handleStorageChange)
  window.addEventListener('focus', refreshProjectTree)
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener(ACCEPTANCE_REPORTS_CHANGED_EVENT, scheduleAIConfigRefresh)
  window.removeEventListener('storage', handleStorageChange)
  window.removeEventListener('focus', refreshProjectTree)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <div class="project-tree-page">
    <header class="project-workspace-title">
      <span class="project-workspace-title__icon"><el-icon><CollectionTag /></el-icon></span>
      <div><h1>项目树</h1><p>追踪版本演进，查看项目配置与验收记录</p></div>
      <span class="project-workspace-title__count">{{ projectOptions.length }} 个项目</span>
    </header>
    <div class="project-tree-page__header">
      <el-button
        type="primary"
        plain
        :icon="CollectionTag"
        @click="configRecordsDialogVisible = true"
      >
        全部项目配置
      </el-button>

      <div class="project-tree-page__actions">
        <el-select
          v-model="selectedProjectCode"
          filterable
          placeholder="选择项目"
          :disabled="loading || projectOptions.length === 0"
        >
          <el-option
            v-for="project in projectOptions"
            :key="project.value"
            :label="`${project.label}（${project.reportCount}）`"
            :value="project.value"
          />
        </el-select>
        <el-input
          v-model="keyword"
          :prefix-icon="Search"
          clearable
          placeholder="在当前项目内搜索版本号、需求点"
        />
        <el-button :icon="Refresh" :loading="loading" @click="fetchReports">
          刷新
        </el-button>
      </div>
    </div>

    <section
      v-if="selectedProjectCode"
      class="project-memo-card"
    >
      <div class="project-memo-card__header">
        <div class="project-memo-card__title">
          <el-icon><CollectionTag /></el-icon>
          <span>项目配置记录</span>
        </div>
        <div class="project-memo-card__actions">
          <span class="project-memo-card__meta">
            {{ selectedProject?.projectCode }}｜{{ selectedProject?.projectName || '未命名项目' }}
          </span>
          <div class="project-memo-card__buttons">
            <el-button-group>
              <el-button
                type="success"
                size="small"
                plain
                :icon="Plus"
                @click="handleAddNewNote('green')"
              >
                绿色便签
              </el-button>
              <el-button
                type="warning"
                size="small"
                plain
                :icon="Plus"
                @click="handleAddNewNote('orange')"
              >
                橙色便签
              </el-button>
              <el-button
                type="danger"
                size="small"
                plain
                :icon="Plus"
                @click="handleAddNewNote('red')"
              >
                红色便签
              </el-button>
            </el-button-group>
          </div>
        </div>
      </div>

      <ProjectConfigLedger v-if="selectedProjectCode" :project-code="selectedProjectCode" :record="selectedProjectMemo || undefined" @refresh="fetchReports" />
      <div class="memo-section-heading"><strong>当前配置与便签 <span>{{ selectedProjectMemo?.items.filter(item => !item.removed).length || 0 }}</span></strong><small>点击编辑 · 拖动排序 · 悬停查看历史</small></div>
      <div
        v-if="selectedProjectMemo && selectedProjectMemo.items.length > 0"
        class="project-memos-list"
      >
        <div
          v-for="note in selectedProjectMemo.items.filter(item => !item.removed)"
          :key="note.id"
          class="memo-sticky-note"
          :class="[
            `memo-sticky-note--${editingNotes[note.id] ? editingNotes[note.id]!.color : note.color}`,
            {
              'memo-sticky-note--editing': editingNotes[note.id],
              'memo-sticky-note--dragging': draggingNoteId === note.id,
              'memo-sticky-note--drag-over': dragOverNoteId === note.id
            }
          ]"
          :draggable="!editingNotes[note.id]"
          @dragstart="handleNoteDragStart($event, note.id)"
          @dragenter.prevent="handleNoteDragEnter(note.id)"
          @dragover.prevent
          @drop="handleNoteDrop($event, note.id)"
          @dragend="handleNoteDragEnd"
        >
          <!-- Editing state -->
          <template v-if="editingNotes[note.id]">
            <div class="memo-sticky-note__edit">
              <div v-if="note.kind !== 'ai'" class="memo-sticky-note__squares">
                <button
                  type="button"
                  class="color-square color-square--green"
                  :class="{ 'color-square--active': editingNotes[note.id]!.color === 'green' }"
                  title="绿色"
                  @click="setDraftColor(note.id, 'green')"
                />
                <button
                  type="button"
                  class="color-square color-square--orange"
                  :class="{ 'color-square--active': editingNotes[note.id]!.color === 'orange' }"
                  title="橙色"
                  @click="setDraftColor(note.id, 'orange')"
                />
                <button
                  type="button"
                  class="color-square color-square--red"
                  :class="{ 'color-square--active': editingNotes[note.id]!.color === 'red' }"
                  title="红色"
                  @click="setDraftColor(note.id, 'red')"
                />
              </div>
              <el-input
                v-model="editingNotes[note.id]!.content"
                type="text"
                placeholder="输入配置记录内容（按Enter键保存，Esc键取消）"
                class="memo-sticky-note__input"
                @keydown.enter="handleNoteEnter($event, note.id)"
                @keydown.esc="cancelEditNote(note.id)"
              />
              <div class="memo-sticky-note__edit-actions">
                <el-button
                  type="success"
                  link
                  :icon="Check"
                  title="保存"
                  @click="saveEditNote(note.id)"
                />
                <el-button
                  type="info"
                  link
                  :icon="Close"
                  title="取消"
                  @click="cancelEditNote(note.id)"
                />
              </div>
            </div>
          </template>

          <!-- View state -->
          <template v-else>
            <el-tooltip
              effect="dark"
              placement="top-start"
              :content="getNoteHistoryTooltip(note)"
              popper-class="memo-history-tooltip"
            >
              <div
                class="memo-sticky-note__view"
                @click="startEditNote(note)"
              >
                <div class="memo-sticky-note__content">
                  <span class="memo-sticky-note__text">{{ note.content }}</span>
                  <span class="memo-sticky-note__time">
                    {{ formatNoteTime(note.updatedAt) }}
                  </span>
                </div>
                <div class="memo-sticky-note__actions" @click.stop>
                  <el-button
                    type="primary"
                    link
                    :icon="Edit"
                    title="编辑"
                    @click="startEditNote(note)"
                  />
                  <el-button
                    type="success"
                    link
                    :icon="DocumentCopy"
                    title="一键黏贴"
                    @click="openPasteNoteDialog(note)"
                  />
                  <el-button
                    type="danger"
                    link
                    :icon="Delete"
                    title="删除"
                    @click="deleteProjectMemoItem(note.id)"
                  />
                </div>
              </div>
            </el-tooltip>
          </template>
        </div>
      </div>

    </section>

    <div class="version-section-heading"><h2>版本与验收记录</h2><span>按版本追溯需求变化与测试记录</span></div>
    <ProjectTreeBoard
      :projects="projectTree"
      :loading="loading"
    />

    <ProjectConfigRecordsDialog
      v-model="configRecordsDialogVisible"
      :projects="allProjects"
      :project-memos="projectMemos"
    />

    <el-dialog
      v-model="pasteDialogVisible"
      width="520px"
      class="memo-paste-dialog"
      align-center
    >
      <template #header>
        <div class="memo-paste-dialog__header">
          <span class="memo-paste-dialog__pin" />
          <div>
            <h3>一键黏贴</h3>
            <p>把这张便签复制到其他项目</p>
          </div>
        </div>
      </template>
      <div class="memo-paste-dialog__body">
        <div
          class="memo-paste-dialog__preview"
          :class="`memo-paste-dialog__preview--${pasteSourceNote?.color || 'green'}`"
        >
          <span class="memo-paste-dialog__label">将要黏贴的便签</span>
          <p>{{ pasteSourceNote?.content }}</p>
        </div>
        <label class="memo-paste-dialog__field">
          <span>目标项目</span>
          <el-select
            v-model="pasteTargetProjectCodes"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            placeholder="搜索并选择一个或多个项目"
            class="memo-paste-dialog__select"
          >
            <el-option
              v-for="project in pasteTargetProjectOptions"
              :key="project.value"
              :label="`${project.label}（${project.reportCount}）`"
              :value="project.value"
            />
          </el-select>
        </label>
      </div>
      <template #footer>
        <div class="memo-paste-dialog__footer">
          <el-button plain @click="pasteDialogVisible = false">取消</el-button>
          <el-button type="success" :icon="DocumentCopy" @click="confirmPasteNote">
            黏贴到选中项目
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.project-tree-page {
  min-height: 100%;
  padding: 24px;
  background: #f6f8fb;
}

.project-tree-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}

.project-tree-page__actions {
  display: grid;
  grid-template-columns: minmax(260px, 360px) minmax(260px, 360px) auto;
  gap: 10px;
}

.project-memo-card {
  margin-bottom: 20px;
  background: transparent;
  border: none;
  box-shadow: none;
}

.project-memo-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  border-bottom: 1px solid #e2e8f0;
  padding-bottom: 12px;
}

.project-memo-card__title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.project-memo-card__actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.project-memo-card__meta {
  color: #64748b;
  font-size: 13px;
}

/* Memos List Layout - thin long items */
.project-memos-list {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 14px;
}

/* Sticky Note Styles */
.memo-sticky-note {
  position: relative;
  width: fit-content;
  max-width: 100%;
  border-radius: 6px;
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
  overflow: hidden;
  cursor: grab;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.08), 0 2px 4px -1px rgba(0, 0, 0, 0.04);
  border: 1px solid rgba(0, 0, 0, 0.06);
}

.memo-sticky-note:active {
  cursor: grabbing;
}

.memo-sticky-note:hover {
  transform: translateY(-4px) scale(1.01);
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05);
  z-index: 10;
}

.memo-sticky-note--dragging {
  opacity: 0.45;
  transform: scale(0.98);
}

.memo-sticky-note--drag-over {
  box-shadow: 0 0 0 2px rgba(37, 99, 235, 0.35), 0 10px 18px rgba(37, 99, 235, 0.16);
}

.memo-sticky-note--editing {
  cursor: default;
}

/* Color Themes for Sticky Notes (Slightly more saturated to look like colored paper) */
.memo-sticky-note--green {
  background: #f0fdf4;
  border-left: 6px solid #10b981;
}

.memo-sticky-note--orange {
  background: #fff7ed;
  border-left: 6px solid #f97316;
}

.memo-sticky-note--red {
  background: #fef2f2;
  border-left: 6px solid #ef4444;
}

.memo-sticky-note--blue {
  background: #eff6ff;
  border-left: 6px solid #2563eb;
}

/* View Mode Styling - Compact Row */
.memo-sticky-note__view {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 3px 10px;
  cursor: pointer;
  min-height: 26px;
  width: fit-content;
  max-width: 100%;
}

.memo-sticky-note__content {
  flex: 0 1 auto;
  min-width: 0;
  max-width: min(860px, calc(100vw - 260px));
  display: flex;
  align-items: center;
  gap: 12px;
}

.memo-sticky-note__text {
  flex: 0 1 auto;
  font-size: 12px;
  line-height: 1.4;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.memo-sticky-note--green .memo-sticky-note__text {
  color: #166534;
}
.memo-sticky-note--orange .memo-sticky-note__text {
  color: #9a3412;
}
.memo-sticky-note--red .memo-sticky-note__text {
  color: #991b1b;
}
.memo-sticky-note--blue .memo-sticky-note__text {
  color: #1e40af;
}

.memo-sticky-note__time {
  font-size: 10px;
  white-space: nowrap;
  flex-shrink: 0;
}

.memo-sticky-note--green .memo-sticky-note__time {
  color: #15803d;
  opacity: 0.7;
}
.memo-sticky-note--orange .memo-sticky-note__time {
  color: #c2410c;
  opacity: 0.7;
}
.memo-sticky-note--red .memo-sticky-note__time {
  color: #b91c1c;
  opacity: 0.7;
}
.memo-sticky-note--blue .memo-sticky-note__time {
  color: #1d4ed8;
  opacity: 0.7;
}

.memo-sticky-note__actions {
  display: flex;
  flex-shrink: 0;
  gap: 2px;
  margin-left: 10px;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.memo-sticky-note:hover .memo-sticky-note__actions {
  opacity: 1;
}

.memo-sticky-note__actions :deep(.el-button) {
  padding: 2px !important;
  height: 20px !important;
  width: 20px !important;
}

/* Edit Mode Styling - Compact Row */
.memo-sticky-note__edit {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 3px 10px;
  min-height: 26px;
  min-width: 320px;
  width: min(640px, calc(100vw - 80px));
}

.memo-sticky-note__squares {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

/* Color selection squares as requested */
.color-square {
  width: 12px;
  height: 12px;
  border-radius: 2px;
  border: 1px solid rgba(0, 0, 0, 0.15);
  cursor: pointer;
  padding: 0;
  outline: none;
  transition: transform 0.2s ease;
}

.color-square:hover {
  transform: scale(1.2);
}

.color-square--green {
  background-color: #10b981;
}

.color-square--orange {
  background-color: #f97316;
}

.color-square--red {
  background-color: #ef4444;
}

.color-square--active {
  box-shadow: 0 0 0 1px #fff, 0 0 0 3px #4f46e5;
  transform: scale(1.1);
}

.memo-sticky-note__input {
  flex: 1;
  min-width: 0;
}

/* Deep overwrite of text input background to match sticky note colors */
.memo-sticky-note__input :deep(.el-input__inner) {
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  padding: 0 !important;
  height: 20px !important;
  line-height: 20px !important;
  font-size: 12px;
}

.memo-sticky-note__input :deep(.el-input__wrapper) {
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  padding: 0 !important;
}

.memo-sticky-note--green .memo-sticky-note__input :deep(.el-input__inner) {
  color: #166534;
}
.memo-sticky-note--green .memo-sticky-note__input :deep(.el-input__inner::placeholder) {
  color: #86efac;
}

.memo-sticky-note--orange .memo-sticky-note__input :deep(.el-input__inner) {
  color: #9a3412;
}
.memo-sticky-note--orange .memo-sticky-note__input :deep(.el-input__inner::placeholder) {
  color: #fdba74;
}

.memo-sticky-note--red .memo-sticky-note__input :deep(.el-input__inner) {
  color: #991b1b;
}
.memo-sticky-note--red .memo-sticky-note__input :deep(.el-input__inner::placeholder) {
  color: #fca5a5;
}

.memo-sticky-note--blue .memo-sticky-note__input :deep(.el-input__inner) {
  color: #1e40af;
}
.memo-sticky-note--blue .memo-sticky-note__input :deep(.el-input__inner::placeholder) {
  color: #93c5fd;
}

.memo-sticky-note__edit-actions {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.memo-sticky-note__edit-actions :deep(.el-button) {
  padding: 2px !important;
  height: 20px !important;
  width: 20px !important;
}

:global(.memo-history-tooltip) {
  max-width: min(520px, calc(100vw - 48px));
  white-space: pre-line;
  line-height: 1.55;
}

:deep(.memo-paste-dialog.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(15, 23, 42, 0.08);
  border-radius: 14px;
  background: linear-gradient(145deg, #fffdf7 0%, #f8fafc 100%);
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.18), 0 8px 24px rgba(15, 23, 42, 0.1);
}

:deep(.memo-paste-dialog .el-dialog__header) {
  padding: 18px 22px 12px;
  margin: 0;
}

:deep(.memo-paste-dialog .el-dialog__body) {
  padding: 0 22px 18px;
}

:deep(.memo-paste-dialog .el-dialog__footer) {
  padding: 0 22px 20px;
}

:deep(.memo-paste-dialog .el-dialog__headerbtn) {
  top: 16px;
  right: 16px;
}

.memo-paste-dialog__header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding-right: 30px;
}

.memo-paste-dialog__pin {
  width: 12px;
  height: 12px;
  margin-top: 5px;
  border-radius: 999px;
  background: #10b981;
  box-shadow: 0 0 0 5px rgba(16, 185, 129, 0.12), 0 7px 14px rgba(16, 185, 129, 0.25);
}

.memo-paste-dialog__header h3 {
  margin: 0;
  color: #0f172a;
  font-size: 18px;
  font-weight: 800;
  letter-spacing: 0;
}

.memo-paste-dialog__header p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 12px;
}

.memo-paste-dialog__body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.memo-paste-dialog__preview {
  position: relative;
  padding: 13px 16px 13px 18px;
  border: 1px solid rgba(15, 23, 42, 0.06);
  border-left: 6px solid #10b981;
  border-radius: 8px;
  background: #f0fdf4;
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.08);
}

.memo-paste-dialog__preview--orange {
  border-left-color: #f97316;
  background: #fff7ed;
}

.memo-paste-dialog__preview--red {
  border-left-color: #ef4444;
  background: #fef2f2;
}

.memo-paste-dialog__label {
  display: block;
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 700;
  color: #64748b;
}

.memo-paste-dialog__preview p {
  margin: 0;
  color: #0f172a;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.memo-paste-dialog__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.memo-paste-dialog__field > span {
  color: #334155;
  font-size: 12px;
  font-weight: 800;
}

.memo-paste-dialog__select {
  width: 100%;
}

:deep(.memo-paste-dialog__select .el-select__wrapper) {
  min-height: 42px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 0 0 1px rgba(148, 163, 184, 0.26) inset;
}

.memo-paste-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.memo-paste-dialog__footer :deep(.el-button) {
  min-width: 104px;
  border-radius: 10px;
}

@media (max-width: 900px) {
  .project-tree-page {
    padding: 16px;
  }

  .project-tree-page__header {
    flex-direction: column;
  }

  .project-tree-page__actions {
    width: 100%;
    grid-template-columns: 1fr;
    margin-top: 0;
  }

  .project-memo-card__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .project-memo-card__actions {
    width: 100%;
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .project-memo-card__meta {
    width: 100%;
  }
}
.project-tree-page{padding:28px;background:#f7f8fa}
.project-workspace-title{display:flex;align-items:center;gap:14px;margin-bottom:24px;color:#172033}
.project-workspace-title__icon{display:grid;place-items:center;width:44px;height:44px;border:1px solid #dbe6f5;border-radius:13px;background:#fff;color:#4263b8;font-size:22px}
.project-workspace-title h1{font-size:24px;letter-spacing:-.5px;margin:0 0 6px}
.project-workspace-title p{font-size:13px;color:#7b8595;margin:0}
.project-workspace-title__count{margin-left:auto;padding:6px 11px;border:1px solid #e3e8ef;border-radius:7px;font-size:12px;background:#fff;color:#64748b}
.project-tree-page__header{padding:14px 16px;gap:16px;background:#fff;border:1px solid #e5e9f0;border-radius:12px;margin-bottom:20px;flex-wrap:wrap}
.project-tree-page__actions{flex:1;grid-template-columns:minmax(160px,240px) minmax(180px,1fr) auto}
.project-memo-card{padding:22px;background:#fff;border:1px solid #e5e9f0;border-radius:14px;box-shadow:0 2px 6px #17203303}
.project-memo-card__header{align-items:flex-start;flex-wrap:wrap;border-bottom:0;margin:0;padding:0}
.project-memo-card__title{font-size:15px}
.project-memo-card__actions{flex-wrap:wrap;gap:12px}
.project-memo-card__meta{font-size:12px}
.memo-section-heading{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;padding:6px 0 10px}
.memo-section-heading strong{font-size:12px;color:#475569;font-weight:600}
.memo-section-heading strong span{margin-left:6px;padding:2px 7px;border-radius:5px;background:#f1f5f9;color:#64748b}
.memo-section-heading small{font-size:11px;color:#94a3b8}
.project-memos-list{display:grid;grid-template-columns:repeat(auto-fill,minmax(min(240px,100%),1fr));gap:8px;align-items:start}
.memo-sticky-note{width:auto;min-width:0;border-radius:9px;box-shadow:none;border:1px solid #e5e9f0;border-left:3px solid var(--memo-accent);background:#fff;transition:border-color .18s,box-shadow .18s}
.memo-sticky-note--green{--memo-accent:#34a77d}
.memo-sticky-note--orange{--memo-accent:#d99b43}
.memo-sticky-note--red{--memo-accent:#df7373}
.memo-sticky-note--blue{--memo-accent:#7294d5}
.memo-sticky-note:hover{transform:none;border-color:var(--memo-accent);box-shadow:0 3px 12px #17203308}
.memo-sticky-note__view{display:grid;grid-template-columns:minmax(0,1fr) auto;width:auto;min-height:64px;padding:8px 10px;box-sizing:border-box;align-items:center;gap:4px 8px}
.memo-sticky-note__content{display:contents}
.memo-sticky-note .memo-sticky-note__text{grid-column:1/-1;min-width:0;font-size:12px;line-height:1.6;color:#334155;white-space:pre-wrap}
.memo-sticky-note .memo-sticky-note__time{grid-column:1;grid-row:2;min-width:0;font-size:10px;color:#94a3b8;opacity:1}
.memo-sticky-note__actions{grid-column:2;grid-row:2;margin:0;align-self:center;opacity:.55}
.memo-sticky-note:focus-within .memo-sticky-note__actions,.memo-sticky-note:hover .memo-sticky-note__actions{opacity:1}
.memo-sticky-note__edit{width:100%;box-sizing:border-box;min-width:0;flex-wrap:wrap;padding:8px 10px;gap:6px}
.memo-sticky-note__input{min-width:0;flex:1 1 180px}
.version-section-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin:28px 0 14px}
.version-section-heading h2{font-size:15px;color:#334155;margin:0}
.version-section-heading span{font-size:12px;color:#94a3b8}
@media(max-width:900px){.project-tree-page{padding:16px}.project-tree-page__actions{width:100%;flex-basis:100%;grid-template-columns:minmax(0,1fr) minmax(0,1fr) auto}.project-memo-card{padding:16px}}
@media(max-width:600px){.project-tree-page__actions{grid-template-columns:minmax(0,1fr) auto}.project-tree-page__actions>.el-select{grid-column:1/-1}.project-workspace-title__count{display:none}.project-memo-card__actions{align-items:flex-start}.project-workspace-title h1{font-size:21px}}
@media(prefers-reduced-motion:reduce){.memo-sticky-note{transition:none}}
</style>
