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
        v-if="selectedProjectMemo && selectedProjectMemo.items.filter(item => !item.removed).length > 0"
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
      <div v-else class="project-memos-empty">
        <span class="project-memos-empty__hint">暂无配置便签，可点击上方「绿色/橙色/红色便签」按钮添加</span>
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
/* Page Layout */
.project-tree-page {
  min-height: 100%;
  padding: 24px 32px;
  background: #f8fafc;
  box-sizing: border-box;
}

/* Header Banner */
.project-workspace-title {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 22px;
}

.project-workspace-title__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: #ffffff;
  font-size: 22px;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.22);
  flex-shrink: 0;
}

.project-workspace-title h1 {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.02em;
  margin: 0 0 4px;
}

.project-workspace-title p {
  font-size: 13px;
  color: #64748b;
  margin: 0;
}

.project-workspace-title__count {
  margin-left: auto;
  padding: 5px 14px;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  background: #ffffff;
  font-size: 12px;
  font-weight: 600;
  color: #475569;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
}

/* Action Toolbar */
.project-tree-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 20px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.04);
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.project-tree-page__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 1;
  justify-content: flex-end;
}

.project-tree-page__actions .el-select {
  width: 240px;
}

.project-tree-page__actions .el-input {
  width: 280px;
}

/* Project Memo Card */
.project-memo-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  padding: 22px 24px;
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.03);
  margin-bottom: 24px;
}

.project-memo-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
  padding-bottom: 14px;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
}

.project-memo-card__title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
}

.project-memo-card__title .el-icon {
  font-size: 18px;
  color: #3b82f6;
}

.project-memo-card__actions {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
}

.project-memo-card__meta {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: 6px;
  background: #f1f5f9;
  color: #475569;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

/* Section Heading */
.memo-section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 18px 0 14px;
  padding: 0 2px;
}

.memo-section-heading strong {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.memo-section-heading strong span {
  padding: 2px 8px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
}

.memo-section-heading small {
  font-size: 12px;
  color: #94a3b8;
}

/* Sticky Notes Grid */
.project-memos-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  align-items: start;
}

.project-memos-empty {
  padding: 32px 20px;
  text-align: center;
  border: 1.5px dashed #e2e8f0;
  border-radius: 12px;
  background: #fcfcfd;
}

.project-memos-empty__hint {
  font-size: 13px;
  color: #94a3b8;
}

/* Sticky Note Item */
.memo-sticky-note {
  position: relative;
  border-radius: 10px;
  border: 1px solid var(--note-border);
  border-left: 4px solid var(--note-accent);
  background: var(--note-bg);
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  overflow: hidden;
  cursor: grab;
}

.memo-sticky-note:active {
  cursor: grabbing;
}

.memo-sticky-note:hover {
  transform: translateY(-2px);
  border-color: var(--note-accent);
  box-shadow: 0 8px 20px -4px rgba(15, 23, 42, 0.08);
  z-index: 2;
}

.memo-sticky-note--dragging {
  opacity: 0.45;
  transform: scale(0.98);
}

.memo-sticky-note--drag-over {
  box-shadow: 0 0 0 2px #3b82f6, 0 8px 20px rgba(59, 130, 246, 0.2);
}

.memo-sticky-note--editing {
  cursor: default;
  transform: none !important;
}

/* Sticky Note Palette Tokens */
.memo-sticky-note--green {
  --note-bg: #f0fdf4;
  --note-border: #dcfce7;
  --note-accent: #10b981;
  --note-text: #14532d;
  --note-time: #059669;
}

.memo-sticky-note--orange {
  --note-bg: #fffbeb;
  --note-border: #fef3c7;
  --note-accent: #f59e0b;
  --note-text: #78350f;
  --note-time: #d97706;
}

.memo-sticky-note--red {
  --note-bg: #fff1f2;
  --note-border: #ffe4e6;
  --note-accent: #f43f5e;
  --note-text: #881337;
  --note-time: #e11d48;
}

.memo-sticky-note--blue {
  --note-bg: #eff6ff;
  --note-border: #dbeafe;
  --note-accent: #3b82f6;
  --note-text: #1e3a8a;
  --note-time: #2563eb;
}

/* Note View Mode */
.memo-sticky-note__view {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 12px 14px;
  min-height: 74px;
  box-sizing: border-box;
  gap: 8px;
  cursor: pointer;
}

.memo-sticky-note__content {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1;
}

.memo-sticky-note__text {
  font-size: 13px;
  line-height: 1.6;
  color: var(--note-text);
  white-space: pre-wrap;
  word-break: break-word;
  font-weight: 500;
}

.memo-sticky-note__time {
  font-size: 11px;
  color: var(--note-time);
  opacity: 0.85;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.memo-sticky-note__actions {
  display: flex;
  align-items: center;
  gap: 4px;
  opacity: 0.35;
  transition: opacity 0.15s ease;
  margin-left: auto;
}

.memo-sticky-note:hover .memo-sticky-note__actions,
.memo-sticky-note:focus-within .memo-sticky-note__actions {
  opacity: 1;
}

.memo-sticky-note__actions :deep(.el-button) {
  padding: 4px !important;
  height: 24px !important;
  width: 24px !important;
  border-radius: 6px;
}

/* Note Edit Mode */
.memo-sticky-note__edit {
  display: flex;
  flex-direction: column;
  padding: 12px 14px;
  gap: 10px;
  min-height: 74px;
  box-sizing: border-box;
}

.memo-sticky-note__squares {
  display: flex;
  gap: 6px;
  align-items: center;
}

.color-square {
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 1px solid rgba(0, 0, 0, 0.12);
  cursor: pointer;
  padding: 0;
  outline: none;
  transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.color-square:hover {
  transform: scale(1.15);
}

.color-square--green {
  background-color: #10b981;
}

.color-square--orange {
  background-color: #f59e0b;
}

.color-square--red {
  background-color: #f43f5e;
}

.color-square--active {
  box-shadow: 0 0 0 2px #ffffff, 0 0 0 4px #3b82f6;
  transform: scale(1.1);
}

.memo-sticky-note__input {
  width: 100%;
}

.memo-sticky-note__input :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.75) !important;
  border: 1px solid rgba(0, 0, 0, 0.08) !important;
  border-radius: 6px !important;
  box-shadow: none !important;
  padding: 4px 8px !important;
}

.memo-sticky-note__input :deep(.el-input__inner) {
  color: var(--note-text);
  font-size: 13px;
  line-height: 1.5;
}

.memo-sticky-note__edit-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.memo-sticky-note__edit-actions :deep(.el-button) {
  padding: 4px 8px !important;
  height: 26px !important;
  border-radius: 6px;
}

/* Tooltip */
:global(.memo-history-tooltip) {
  max-width: min(520px, calc(100vw - 48px));
  white-space: pre-line;
  line-height: 1.6;
  border-radius: 8px;
  font-size: 12px;
}

/* Version Tree Heading */
.version-section-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin: 32px 0 16px;
  padding: 0 4px;
}

.version-section-heading h2 {
  font-size: 17px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.01em;
  margin: 0;
}

.version-section-heading span {
  font-size: 13px;
  color: #64748b;
}

/* Paste Dialog */
:deep(.memo-paste-dialog.el-dialog) {
  border-radius: 14px;
  overflow: hidden;
  box-shadow: 0 20px 50px rgba(15, 23, 42, 0.15);
  border: 1px solid #e2e8f0;
}

:deep(.memo-paste-dialog .el-dialog__header) {
  margin: 0;
  padding: 20px 24px 14px;
  border-bottom: 1px solid #f1f5f9;
}

:deep(.memo-paste-dialog .el-dialog__body) {
  padding: 20px 24px;
}

:deep(.memo-paste-dialog .el-dialog__footer) {
  padding: 14px 24px 20px;
  border-top: 1px solid #f1f5f9;
}

.memo-paste-dialog__header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.memo-paste-dialog__pin {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: #10b981;
  box-shadow: 0 0 0 4px rgba(16, 185, 129, 0.2);
}

.memo-paste-dialog__header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
}

.memo-paste-dialog__header p {
  margin: 2px 0 0;
  font-size: 12px;
  color: #64748b;
}

.memo-paste-dialog__body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.memo-paste-dialog__preview {
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid #e2e8f0;
  border-left: 4px solid #10b981;
  background: #f0fdf4;
}

.memo-paste-dialog__preview--orange {
  border-left-color: #f59e0b;
  background: #fffbeb;
}

.memo-paste-dialog__preview--red {
  border-left-color: #f43f5e;
  background: #fff1f2;
}

.memo-paste-dialog__label {
  display: block;
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  margin-bottom: 6px;
}

.memo-paste-dialog__preview p {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: #0f172a;
  white-space: pre-wrap;
  word-break: break-word;
}

.memo-paste-dialog__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.memo-paste-dialog__field > span {
  font-size: 12px;
  font-weight: 600;
  color: #334155;
}

.memo-paste-dialog__select {
  width: 100%;
}

.memo-paste-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

/* Responsive */
@media (max-width: 900px) {
  .project-tree-page {
    padding: 16px;
  }
  .project-tree-page__header {
    flex-direction: column;
    align-items: stretch;
  }
  .project-tree-page__actions {
    flex-direction: column;
    align-items: stretch;
  }
  .project-tree-page__actions .el-select,
  .project-tree-page__actions .el-input {
    width: 100%;
  }
  .project-memo-card {
    padding: 16px;
  }
  .project-memo-card__header {
    flex-direction: column;
    align-items: flex-start;
  }
  .project-memo-card__actions {
    width: 100%;
    justify-content: space-between;
  }
}

@media (max-width: 600px) {
  .project-workspace-title h1 {
    font-size: 20px;
  }
  .project-workspace-title__count {
    display: none;
  }
  .project-memos-list {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .memo-sticky-note {
    transition: none;
  }
}
</style>