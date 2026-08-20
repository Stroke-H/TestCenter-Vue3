<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check, Clock, DocumentChecked, Plus, Refresh } from '@element-plus/icons-vue'
import request from '@/api/request'

interface TodoItem {
  id: string
  todo_type: 'acceptance' | 'manual'
  report_id: string
  project_code: string
  project_name: string
  reporter: string
  feature_items: string[]
  due_at: string
  status: string
  last_notified_at?: string
  notification_count: number
  created_at: string
  updated_at: string
}

interface TodoResponse {
  items: TodoItem[]
  count: number
}

interface Project {
  id: string
  project_code: string
  project_name: string
}

const loading = ref(false)
const projectLoading = ref(false)
const creating = ref(false)
const completingID = ref('')
const items = ref<TodoItem[]>([])
const projects = ref<Project[]>([])
const detailVisible = ref(false)
const createVisible = ref(false)
const selectedTodo = ref<TodoItem | null>(null)
const manualForm = ref({ project_code: '', due_at: '', event_content: '' })
const remainingCount = computed(() => items.value.length)

function featurePreview(features: string[]) {
  return features.map((feature) => `• ${feature}`).join('\n')
}

function openTodoDetail(item: TodoItem) {
  selectedTodo.value = item
  detailVisible.value = true
}

function toLocalDateTime(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:00`
}

function openCreateDialog() {
  manualForm.value = {
    project_code: '',
    due_at: toLocalDateTime(new Date(Date.now() + 60 * 60 * 1000)),
    event_content: ''
  }
  createVisible.value = true
  if (!projects.value.length) loadProjects()
}

function disablePastDates(date: Date) {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  return date.getTime() < today.getTime()
}

function formatDateTime(value?: string) {
  if (!value) return '尚未提醒'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  }).format(date)
}

async function loadTodos() {
  loading.value = true
  try {
    const response = await request.get('/acceptance-todos/mine') as TodoResponse
    items.value = Array.isArray(response.items) ? response.items : []
  } catch (error) {
    console.error('Failed to load personal todos', error)
    ElMessage.error('加载我的待办失败')
  } finally {
    loading.value = false
  }
}

async function loadProjects() {
  projectLoading.value = true
  try {
    const response = await request.get('/config/projects') as Project[]
    projects.value = Array.isArray(response)
      ? response.filter((project) => project.project_code || project.id)
      : []
  } catch (error) {
    console.error('Failed to load projects for manual todo', error)
    ElMessage.error('加载项目列表失败')
  } finally {
    projectLoading.value = false
  }
}

async function createManualTodo() {
  const form = manualForm.value
  if (!form.project_code) {
    ElMessage.warning('请选择项目')
    return
  }
  if (!form.due_at) {
    ElMessage.warning('请选择提醒时间')
    return
  }
  if (!form.event_content.trim()) {
    ElMessage.warning('请填写待办事件')
    return
  }
  const dueAt = new Date(form.due_at.replace(' ', 'T'))
  if (Number.isNaN(dueAt.getTime()) || dueAt.getTime() <= Date.now()) {
    ElMessage.warning('提醒时间必须晚于当前时间')
    return
  }

  creating.value = true
  try {
    await request.post('/acceptance-todos/manual', {
      project_code: form.project_code,
      due_at: form.due_at,
      event_content: form.event_content.trim()
    })
    createVisible.value = false
    ElMessage.success('待办创建成功，将在指定时间发送飞书提醒')
    await loadTodos()
  } catch (error) {
    console.error('Failed to create manual todo', error)
    ElMessage.error('创建待办失败，请检查填写内容')
  } finally {
    creating.value = false
  }
}

async function completeTodo(item: TodoItem) {
  try {
    await ElMessageBox.confirm(
      `确认已完成「${item.project_name || item.project_code}」的这项待办吗？`,
      '完成待办',
      {
        confirmButtonText: '确认完成',
        cancelButtonText: '暂不处理',
        type: 'success'
      }
    )
  } catch {
    return
  }

  completingID.value = item.id
  try {
    await request.post(`/acceptance-todos/${encodeURIComponent(item.id)}/complete`, {
      todo_type: item.todo_type
    })
    items.value = items.value.filter((todo) => todo.id !== item.id)
    if (selectedTodo.value?.id === item.id) {
      detailVisible.value = false
      selectedTodo.value = null
    }
    ElMessage.success('待办已完成，后续不再提醒')
  } catch (error) {
    console.error('Failed to complete personal todo', error)
    ElMessage.error('完成待办失败，请稍后重试')
  } finally {
    completingID.value = ''
  }
}

onMounted(() => {
  loadTodos()
  loadProjects()
})
</script>

<template>
  <div class="todo-page" v-loading="loading">
    <section class="todo-hero">
      <div class="todo-hero__copy">
        <div class="todo-hero__eyebrow">PERSONAL WORKSPACE</div>
        <h1>我的待办</h1>
        <p>集中管理 TTmins 验收跟进和个人手动待办，按计划时间接收飞书提醒。</p>
      </div>
      <div class="todo-hero__actions">
        <div class="todo-count">
          <span class="todo-count__number">{{ remainingCount }}</span>
          <span class="todo-count__label">项待跟进</span>
        </div>
        <el-button type="primary" :icon="Plus" @click="openCreateDialog">新建待办</el-button>
        <el-button :icon="Refresh" circle plain aria-label="刷新待办" @click="loadTodos" />
      </div>
    </section>

    <section v-if="items.length" class="todo-list">
      <article
        v-for="item in items"
        :key="item.id"
        class="todo-card"
        :class="{ 'todo-card--manual': item.todo_type === 'manual' }"
        role="button"
        tabindex="0"
        :aria-label="`查看 ${item.project_name || item.project_code} 待办详情`"
        @click="openTodoDetail(item)"
        @keydown.enter="openTodoDetail(item)"
        @keydown.space.prevent="openTodoDetail(item)"
      >
        <div class="todo-card__accent"></div>
        <div class="todo-card__body">
          <div class="todo-card__header">
            <div class="todo-card__meta">
              <span class="project-code">{{ item.project_code }}</span>
              <span class="todo-status" :class="{ 'todo-status--manual': item.todo_type === 'manual' }">
                {{ item.todo_type === 'manual' ? '手动待办' : '验收跟进' }}
              </span>
            </div>
            <div class="due-chip">
              <el-icon><Clock /></el-icon>
              <span>{{ formatDateTime(item.due_at) }}</span>
            </div>
          </div>
          <h2>{{ item.project_name || item.project_code }}</h2>

          <div class="feature-block">
            <div class="feature-block__title">
              <el-icon><DocumentChecked /></el-icon>
              <span>{{ item.todo_type === 'manual' ? '待办事件' : '待验证功能' }}</span>
            </div>
            <div class="feature-preview">{{ featurePreview(item.feature_items) }}</div>
          </div>

          <div class="todo-card__footer">
            <div class="notify-meta">
              <span>{{ item.todo_type === 'manual' ? '手动创建' : `报告 ${item.report_id}` }}</span>
              <span>{{ item.notification_count ? `已提醒 ${item.notification_count} 次` : '尚未提醒' }}</span>
            </div>
            <el-button
              type="success"
              :icon="Check"
              size="small"
              :loading="completingID === item.id"
              @click.stop="completeTodo(item)"
              @keydown.stop
            >
              标记完成
            </el-button>
          </div>
        </div>
      </article>
    </section>

    <section v-else-if="!loading" class="todo-empty">
      <div class="todo-empty__icon"><el-icon><Check /></el-icon></div>
      <h2>当前没有剩余待办</h2>
      <p>可以新建一条待办，在指定时间通过飞书提醒自己。</p>
      <el-button class="todo-empty__create" type="primary" :icon="Plus" @click="openCreateDialog">
        新建待办
      </el-button>
    </section>

    <el-dialog
      v-model="detailVisible"
      width="30%"
      align-center
      append-to-body
      destroy-on-close
      class="todo-detail-dialog"
      transition="todo-dialog-zoom"
    >
      <template #header>
        <div v-if="selectedTodo" class="detail-header">
          <div class="detail-header__tags">
            <span class="project-code">{{ selectedTodo.project_code }}</span>
            <span class="todo-status" :class="{ 'todo-status--manual': selectedTodo.todo_type === 'manual' }">
              {{ selectedTodo.todo_type === 'manual' ? '手动待办' : '验收跟进' }}
            </span>
          </div>
          <h2>{{ selectedTodo.project_name || selectedTodo.project_code }}</h2>
        </div>
      </template>

      <div v-if="selectedTodo" class="detail-body">
        <div class="detail-meta-grid">
          <div class="detail-meta-item">
            <span>计划提醒</span>
            <strong>{{ formatDateTime(selectedTodo.due_at) }}</strong>
          </div>
          <div class="detail-meta-item">
            <span>提醒次数</span>
            <strong>{{ selectedTodo.notification_count }} 次</strong>
          </div>
          <div v-if="selectedTodo.todo_type === 'acceptance'" class="detail-meta-item detail-meta-item--wide">
            <span>验收报告</span>
            <strong>{{ selectedTodo.report_id }}</strong>
          </div>
        </div>

        <div class="detail-features">
          <div class="detail-features__title">
            <el-icon><DocumentChecked /></el-icon>
            <span>{{ selectedTodo.todo_type === 'manual' ? '完整待办事件' : '完整待验证功能' }}</span>
          </div>
          <ul>
            <li v-for="feature in selectedTodo.feature_items" :key="feature">{{ feature }}</li>
          </ul>
        </div>
      </div>

      <template #footer>
        <div class="detail-footer">
          <el-button @click="detailVisible = false">关闭</el-button>
          <el-button
            v-if="selectedTodo"
            type="success"
            :icon="Check"
            :loading="completingID === selectedTodo.id"
            @click="completeTodo(selectedTodo)"
          >
            标记完成
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="createVisible"
      width="440px"
      align-center
      append-to-body
      destroy-on-close
      class="todo-create-dialog"
      title="新建手动待办"
      :close-on-click-modal="!creating"
    >
      <p class="create-dialog__hint">选择项目、提醒时间和需要跟进的事件。提醒只发送给当前账号绑定的飞书用户。</p>
      <el-form label-position="top" @submit.prevent="createManualTodo">
        <el-form-item label="指定项目" required>
          <el-select
            v-model="manualForm.project_code"
            filterable
            :loading="projectLoading"
            placeholder="搜索项目编号或名称"
            style="width: 100%"
          >
            <el-option
              v-for="project in projects"
              :key="project.id || project.project_code"
              :label="`${project.project_code || project.id} · ${project.project_name || '未命名项目'}`"
              :value="project.project_code || project.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="提醒时间" required>
          <el-date-picker
            v-model="manualForm.due_at"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            format="YYYY-MM-DD HH:mm"
            placeholder="选择提醒时间"
            :disabled-date="disablePastDates"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="待办事件" required>
          <el-input
            v-model="manualForm.event_content"
            type="textarea"
            :rows="5"
            maxlength="2000"
            show-word-limit
            resize="none"
            placeholder="例如：复核提审状态，并验证返回插屏和支付流程"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="detail-footer">
          <el-button :disabled="creating" @click="createVisible = false">取消</el-button>
          <el-button type="primary" :loading="creating" @click="createManualTodo">创建待办</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.todo-page {
  width: min(1180px, 100%);
  margin: 0 auto;
  color: #172033;
}

.todo-hero {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 20px 24px;
  overflow: hidden;
  border: 1px solid rgba(99, 102, 241, 0.16);
  border-radius: 16px;
  background:
    radial-gradient(circle at 85% 10%, rgba(99, 102, 241, 0.17), transparent 30%),
    linear-gradient(135deg, #ffffff 0%, #f7f8ff 100%);
  box-shadow: 0 10px 28px rgba(30, 41, 59, 0.055);
}

.todo-hero__eyebrow {
  margin-bottom: 5px;
  color: #6366f1;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.16em;
}

.todo-hero h1 {
  margin: 0;
  font-size: clamp(24px, 3vw, 30px);
  line-height: 1.2;
  letter-spacing: -0.03em;
}

.todo-hero p {
  max-width: 640px;
  margin: 7px 0 0;
  color: #64748b;
  font-size: 14px;
  line-height: 1.65;
}

.todo-hero__actions,
.todo-count {
  display: flex;
  align-items: center;
}

.todo-hero__actions { gap: 14px; }

.todo-count {
  min-width: 102px;
  justify-content: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid rgba(99, 102, 241, 0.14);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.78);
}

.todo-count__number {
  color: #4f46e5;
  font-size: 23px;
  font-weight: 800;
}

.todo-count__label { color: #64748b; font-size: 13px; }

.todo-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: start;
  gap: 14px;
  margin-top: 16px;
}

.todo-card {
  position: relative;
  display: flex;
  height: 254px;
  overflow: hidden;
  border: 1px solid #e8ebf2;
  border-radius: 14px;
  background: #fff;
  box-shadow: 0 7px 22px rgba(15, 23, 42, 0.04);
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.todo-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.075);
}

.todo-card:focus-visible {
  outline: 2px solid #818cf8;
  outline-offset: 3px;
}

.todo-card:active { transform: translateY(0) scale(0.988); }

.todo-card__accent { width: 3px; background: linear-gradient(180deg, #6366f1, #8b5cf6); }
.todo-card--manual .todo-card__accent { background: linear-gradient(180deg, #0ea5e9, #14b8a6); }
.todo-card__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  padding: 15px 16px;
}
.todo-card__header { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.todo-card__meta { display: flex; align-items: center; gap: 6px; }

.project-code,
.todo-status {
  padding: 3px 7px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
}

.project-code { color: #4f46e5; background: #eef2ff; }
.todo-status { color: #b45309; background: #fff7ed; }
.todo-status--manual { color: #0369a1; background: #e0f2fe; }
.todo-card h2 {
  margin: 11px 0 0;
  overflow: hidden;
  color: #1e293b;
  font-size: 16px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.due-chip {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
  padding: 5px 7px;
  border-radius: 8px;
  color: #64748b;
  background: #f8fafc;
  font-size: 11px;
}

.feature-block {
  box-sizing: border-box;
  height: 99px;
  margin-top: 12px;
  padding: 11px 12px;
  border: 1px solid #edf0f5;
  border-radius: 10px;
  background: #fafbfc;
}

.feature-block__title {
  display: flex;
  align-items: center;
  gap: 5px;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.feature-preview {
  display: -webkit-box;
  height: 54px;
  margin-top: 8px;
  overflow: hidden;
  color: #475569;
  font-size: 12px;
  line-height: 18px;
  overflow-wrap: anywhere;
  white-space: pre-line;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
}

.todo-card__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: auto;
  padding-top: 12px;
}

.notify-meta {
  display: grid;
  gap: 2px;
  min-width: 0;
  overflow: hidden;
  color: #94a3b8;
  font-size: 10px;
}

.notify-meta span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.todo-empty {
  margin-top: 20px;
  padding: 78px 24px;
  text-align: center;
  border: 1px dashed #d9deea;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.68);
}

.todo-empty__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: 20px;
  color: #16a34a;
  background: #ecfdf3;
  font-size: 30px;
}

.todo-empty h2 { margin: 18px 0 6px; color: #334155; font-size: 19px; }
.todo-empty p { margin: 0; color: #94a3b8; font-size: 13px; }
.todo-empty__create { margin-top: 20px; }

.detail-header__tags {
  display: flex;
  gap: 7px;
  margin-bottom: 10px;
}

.detail-header h2 {
  margin: 0;
  color: #1e293b;
  font-size: 20px;
  line-height: 1.45;
}

.detail-body { padding-top: 2px; }

.detail-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 9px;
}

.detail-meta-item {
  display: grid;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid #edf0f5;
  border-radius: 10px;
  background: #f8fafc;
}

.detail-meta-item--wide { grid-column: 1 / -1; }
.detail-meta-item span { color: #94a3b8; font-size: 11px; }
.detail-meta-item strong { overflow-wrap: anywhere; color: #475569; font-size: 12px; }

.detail-features {
  margin-top: 14px;
  padding: 14px 15px;
  border: 1px solid #e8ebf2;
  border-radius: 12px;
  background: linear-gradient(145deg, #fafbff, #f8fafc);
}

.detail-features__title {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #4338ca;
  font-size: 13px;
  font-weight: 700;
}

.detail-features ul {
  display: grid;
  gap: 8px;
  max-height: 34vh;
  margin: 12px 0 0;
  padding: 0 0 0 19px;
  overflow-y: auto;
}

.detail-features li { color: #475569; font-size: 13px; line-height: 1.65; }
.detail-footer { display: flex; justify-content: flex-end; gap: 8px; }

:global(.todo-detail-dialog.el-dialog) {
  min-width: 400px;
  max-width: 520px;
  overflow: hidden;
  border: 1px solid rgba(99, 102, 241, 0.13);
  border-radius: 18px;
  box-shadow: 0 28px 80px rgba(15, 23, 42, 0.22);
}

:global(.todo-create-dialog.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(99, 102, 241, 0.13);
  border-radius: 18px;
  box-shadow: 0 28px 80px rgba(15, 23, 42, 0.22);
}

:global(.todo-create-dialog .el-dialog__header) {
  margin: 0;
  padding: 20px 22px 15px;
  border-bottom: 1px solid #f0f2f6;
  background: linear-gradient(145deg, #ffffff, #f7f8ff);
}

:global(.todo-create-dialog .el-dialog__title) { color: #1e293b; font-weight: 750; }
:global(.todo-create-dialog .el-dialog__body) { padding: 18px 22px 4px; }
:global(.todo-create-dialog .el-dialog__footer) { padding: 10px 22px 20px; }

.create-dialog__hint {
  margin: 0 0 18px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.65;
}

:global(.todo-detail-dialog .el-dialog__header) {
  margin: 0;
  padding: 20px 22px 14px;
  border-bottom: 1px solid #f0f2f6;
  background: linear-gradient(145deg, #ffffff, #f7f8ff);
}

:global(.todo-detail-dialog .el-dialog__body) { padding: 18px 22px; }
:global(.todo-detail-dialog .el-dialog__footer) { padding: 0 22px 20px; }

:global(.todo-dialog-zoom-enter-active),
:global(.todo-dialog-zoom-leave-active) {
  transition: opacity 0.24s ease;
}

:global(.todo-dialog-zoom-enter-from),
:global(.todo-dialog-zoom-leave-to) {
  opacity: 0;
}

:global(.todo-dialog-zoom-enter-active .todo-detail-dialog) {
  animation: todo-card-expand 0.3s cubic-bezier(0.2, 0.9, 0.25, 1.08);
}

:global(.todo-dialog-zoom-leave-active .todo-detail-dialog) {
  animation: todo-card-expand 0.2s ease-in reverse;
}

@keyframes todo-card-expand {
  from {
    opacity: 0;
    transform: translateY(28px) scale(0.68);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@media (max-width: 720px) {
  .todo-hero,
  .todo-card__footer { align-items: stretch; flex-direction: column; }
  .todo-hero { padding: 18px; }
  .todo-hero__actions { justify-content: space-between; }
  .due-chip { align-self: flex-start; }
  .todo-card__footer :deep(.el-button) { width: 100%; }
  .todo-list { grid-template-columns: 1fr; }
  .todo-card__header { flex-direction: row; align-items: center; }
  :global(.todo-detail-dialog.el-dialog) {
    width: calc(100vw - 32px) !important;
    min-width: 0;
  }
  :global(.todo-create-dialog.el-dialog) { width: calc(100vw - 32px) !important; }
}

@media (max-width: 1040px) and (min-width: 721px) {
  .todo-list { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

</style>
