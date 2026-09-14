<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Check, CircleClose, Clock, DocumentChecked, Plus, Refresh } from '@element-plus/icons-vue'
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
  previous_version_not_approved: boolean
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
const rejectingID = ref('')
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
    items.value.forEach((todo) => {
      if (todo.project_code.toLowerCase() === item.project_code.toLowerCase()) {
        todo.previous_version_not_approved = false
      }
    })
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

async function markNotApproved(item: TodoItem) {
  try {
    await ElMessageBox.confirm(
      `确认「${item.project_name || item.project_code}」当前版本未过审吗？当前待办将结束，下次该项目待办会显示“上版未过审”角标。`,
      '版本未过审',
      {
        confirmButtonText: '确认未过审',
        cancelButtonText: '暂不处理',
        type: 'warning'
      }
    )
  } catch {
    return
  }

  rejectingID.value = item.id
  try {
    await request.post(`/acceptance-todos/${encodeURIComponent(item.id)}/not-approved`, {
      todo_type: item.todo_type
    })
    items.value = items.value.filter((todo) => todo.id !== item.id)
    items.value.forEach((todo) => {
      if (todo.project_code.toLowerCase() === item.project_code.toLowerCase()) {
        todo.previous_version_not_approved = true
      }
    })
    if (selectedTodo.value?.id === item.id) {
      detailVisible.value = false
      selectedTodo.value = null
    }
    ElMessage.success('已标记为版本未过审，下次该项目待办将显示提示角标')
  } catch (error) {
    console.error('Failed to mark todo as not approved', error)
    ElMessage.error('标记版本未过审失败，请稍后重试')
  } finally {
    rejectingID.value = ''
  }
}

onMounted(() => {
  loadTodos()
  loadProjects()
})
</script>

<template>
  <div class="todo-page" v-loading="loading">
    <!-- Standard Page Header -->
    <div class="page-header">
      <div class="header-left">
        <div class="title-row">
          <h1 class="page-title">我的待办</h1>
          <span class="page-badge">Personal Workspace</span>
        </div>
        <p class="page-desc">集中管理 TTmins 验收跟进和个人手动待办，按计划时间接收飞书提醒</p>
      </div>
      <div class="header-right">
        <div class="todo-count-badge">
          <span class="count-num">{{ remainingCount }}</span>
          <span class="count-label">项待跟进</span>
        </div>
        <el-button type="primary" :icon="Plus" class="add-btn" @click="openCreateDialog">新建待办</el-button>
        <el-button :icon="Refresh" class="refresh-btn" plain aria-label="刷新待办" @click="loadTodos">刷新</el-button>
      </div>
    </div>

    <!-- Todo List Grid -->
    <section v-if="items.length" class="todo-list">
      <article
        v-for="item in items"
        :key="item.id"
        class="todo-card"
        :class="{
          'todo-card--manual': item.todo_type === 'manual',
          'todo-card--has-history': item.previous_version_not_approved
        }"
        role="button"
        tabindex="0"
        :aria-label="`查看 ${item.project_name || item.project_code} 待办详情`"
        @click="openTodoDetail(item)"
        @keydown.enter="openTodoDetail(item)"
        @keydown.space.prevent="openTodoDetail(item)"
      >
        <div v-if="item.previous_version_not_approved" class="previous-version-ribbon">上版未过审</div>
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
            <div class="todo-card__actions">
              <el-button
                type="warning"
                plain
                :icon="CircleClose"
                size="small"
                :loading="rejectingID === item.id"
                :disabled="completingID === item.id"
                @click.stop="markNotApproved(item)"
                @keydown.stop
              >
                未过审
              </el-button>
              <el-button
                type="success"
                :icon="Check"
                size="small"
                :loading="completingID === item.id"
                :disabled="rejectingID === item.id"
                @click.stop="completeTodo(item)"
                @keydown.stop
              >
                完成
              </el-button>
            </div>
          </div>
        </div>
      </article>
    </section>

    <!-- Empty State -->
    <section v-else-if="!loading" class="todo-empty">
      <div class="todo-empty__icon"><el-icon><Check /></el-icon></div>
      <h2>当前没有剩余待办</h2>
      <p>所有验收与跟进事件均已完成。可以新建一条待办，在指定时间通过飞书提醒自己。</p>
      <el-button class="todo-empty__create add-btn" type="primary" :icon="Plus" @click="openCreateDialog">
        新建待办
      </el-button>
    </section>

    <!-- Detail Dialog -->
    <el-dialog
      v-model="detailVisible"
      width="480px"
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
            <span v-if="selectedTodo.previous_version_not_approved" class="previous-version-tag">上版未过审</span>
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
            type="warning"
            plain
            :icon="CircleClose"
            :loading="rejectingID === selectedTodo.id"
            :disabled="completingID === selectedTodo.id"
            @click="markNotApproved(selectedTodo)"
          >
            版本未过审
          </el-button>
          <el-button
            v-if="selectedTodo"
            type="success"
            :icon="Check"
            :loading="completingID === selectedTodo.id"
            :disabled="rejectingID === selectedTodo.id"
            @click="completeTodo(selectedTodo)"
          >
            标记完成
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- Create Dialog -->
    <el-dialog
      v-model="createVisible"
      width="480px"
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
  padding: 24px;
  max-width: 1680px;
  margin: 0 auto;
  color: #1e293b;
}

/* Standard Page Header */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.page-title {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  letter-spacing: -0.02em;
}

.page-badge {
  font-size: 12px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  padding: 3px 10px;
  border-radius: 9999px;
  letter-spacing: 0.02em;
}

.page-desc {
  margin: 6px 0 0;
  font-size: 13px;
  color: #64748b;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.todo-count-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}

.todo-count-badge .count-num {
  font-size: 18px;
  font-weight: 800;
  color: #2563eb;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.todo-count-badge .count-label {
  font-size: 12px;
  color: #64748b;
  font-weight: 500;
}

.add-btn {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  border: none;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.25);
  transition: all 0.2s ease;
}

.add-btn:hover {
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.35);
  transform: translateY(-1px);
}

.refresh-btn {
  border-radius: 8px;
  font-weight: 500;
}

/* Grid & Cards */
.todo-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 16px;
}

.todo-card {
  position: relative;
  display: flex;
  height: 254px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
  cursor: pointer;
  transition: transform 0.2s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.2s ease, border-color 0.2s ease;
}

.todo-card:hover {
  transform: translateY(-3px);
  box-shadow: 0 12px 24px rgba(15, 23, 42, 0.08);
  border-color: #93c5fd;
}

.todo-card:active {
  transform: translateY(0) scale(0.99);
}

.previous-version-ribbon {
  position: absolute;
  top: 13px;
  right: -34px;
  z-index: 2;
  width: 126px;
  padding: 4px 0;
  transform: rotate(39deg);
  color: #ffffff;
  background: linear-gradient(90deg, #f97316, #ef4444);
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.24);
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-align: center;
  pointer-events: none;
}

.todo-card__accent {
  width: 4px;
  background: linear-gradient(180deg, #3b82f6, #60a5fa);
}

.todo-card--manual .todo-card__accent {
  background: linear-gradient(180deg, #0ea5e9, #14b8a6);
}

.todo-card__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  padding: 16px 18px;
}

.todo-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.todo-card--has-history .todo-card__header {
  padding-right: 36px;
}

.todo-card__meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.project-code,
.todo-status {
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
}

.project-code {
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.todo-status {
  color: #b45309;
  background: #fff7ed;
  border: 1px solid #ffedd5;
}

.todo-status--manual {
  color: #0369a1;
  background: #f0f9ff;
  border: 1px solid #e0f2fe;
}

.previous-version-tag {
  padding: 3px 8px;
  border-radius: 6px;
  color: #c2410c;
  background: #fff1e8;
  font-size: 11px;
  font-weight: 700;
}

.todo-card h2 {
  margin: 10px 0 0;
  overflow: hidden;
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.due-chip {
  display: flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
  padding: 4px 8px;
  border-radius: 6px;
  color: #64748b;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  font-size: 11px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.feature-block {
  box-sizing: border-box;
  height: 96px;
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px solid #f1f5f9;
  border-radius: 10px;
  background: #f8fafc;
}

.feature-block__title {
  display: flex;
  align-items: center;
  gap: 5px;
  color: #475569;
  font-size: 11px;
  font-weight: 700;
}

.feature-preview {
  display: -webkit-box;
  height: 54px;
  margin-top: 6px;
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
  padding-top: 10px;
}

.notify-meta {
  display: grid;
  gap: 2px;
  min-width: 0;
  overflow: hidden;
  color: #94a3b8;
  font-size: 11px;
}

.notify-meta span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.todo-card__actions {
  display: flex;
  flex-shrink: 0;
  gap: 6px;
}

.todo-card__actions :deep(.el-button) {
  border-radius: 6px;
}

.todo-empty {
  margin-top: 30px;
  padding: 60px 24px;
  text-align: center;
  border: 1px dashed #cbd5e1;
  border-radius: 16px;
  background: #ffffff;
}

.todo-empty__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 60px;
  height: 60px;
  border-radius: 16px;
  color: #10b981;
  background: #ecfdf5;
  font-size: 28px;
}

.todo-empty h2 {
  margin: 16px 0 6px;
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
}

.todo-empty p {
  margin: 0;
  color: #64748b;
  font-size: 13px;
}

.todo-empty__create {
  margin-top: 18px;
}

.detail-header__tags {
  display: flex;
  gap: 7px;
  margin-bottom: 10px;
}

.detail-header h2 {
  margin: 0;
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
  line-height: 1.45;
}

.detail-body {
  padding-top: 2px;
}

.detail-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.detail-meta-item {
  display: grid;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #f8fafc;
}

.detail-meta-item--wide {
  grid-column: 1 / -1;
}

.detail-meta-item span {
  color: #64748b;
  font-size: 11px;
}

.detail-meta-item strong {
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 13px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.detail-features {
  margin-top: 14px;
  padding: 14px 16px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #f8fafc;
}

.detail-features__title {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #2563eb;
  font-size: 13px;
  font-weight: 700;
}

.detail-features ul {
  display: grid;
  gap: 8px;
  max-height: 34vh;
  margin: 12px 0 0;
  padding: 0 0 0 18px;
  overflow-y: auto;
}

.detail-features li {
  color: #334155;
  font-size: 13px;
  line-height: 1.6;
}

.detail-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.create-dialog__hint {
  margin: 0 0 16px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 16px;
  }
  .todo-list {
    grid-template-columns: 1fr;
  }
}
</style>
