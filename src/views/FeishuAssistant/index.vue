<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

// --- Types ---
interface ChatMessage {
  id: string
  sender: 'user' | 'bot'
  senderName: string
  msgType: 'text' | 'post' | 'image' | 'file'
  content: string
  time: string
}

interface SessionRecord {
  id: string
  userName: string
  status: 'active' | 'ended'
  msgCount: number
  startTime: string
  lastActiveTime: string
  summary: string
  messages: ChatMessage[]
}

interface OperationLog {
  id: string
  tool_name: string
  project: string
  env: string
  user_id: string
  user_name: string
  status: string
  detail: string
  timestamp: string
}

type ScheduleType = 'Once' | 'Daily' | 'weekly'

interface ScheduledTask {
  id: string
  name: string
  scheduleType: ScheduleType
  creator: string
  nextRun: string
  testProject: string
  testProjectCode: string
  testEnv: string
  status: 'active' | 'paused' | 'running' | 'completed'
  description: string
}

interface ProjectOption {
  id: string
  project_code: string
  project_name: string
}

interface ScheduledTaskForm {
  taskType: 'episode-playback-test' | ''
  scheduleType: ScheduleType | ''
  creator: string
  startDate: string
  executionTime: string
  testProjectCode: string
  testEnv: string
}

// --- API Service (Direct fetch for simplicity in this dashboard) ---
const API_BASE = 'http://localhost:8080/api'
const authStore = useAuthStore()
const scheduledTasks = ref<ScheduledTask[]>([])

// --- Mock / Init Data ---
const tools = computed(() => [
  {
    id: 'total-sessions',
    name: 'Total Sessions',
    value: '142',
    iconName: 'ChatLineSquare',
    iconColor: '#3b82f6',
    iconBg: 'rgba(59, 130, 246, 0.1)'
  },
  {
    id: 'total-scheduled-tasks',
    name: 'Total Scheduled Tasks',
    value: String(scheduledTasks.value.length),
    iconName: 'AlarmClock',
    iconColor: '#10b981',
    iconBg: 'rgba(16, 185, 129, 0.1)'
  },
  {
    id: 'bot-status',
    name: 'Bot Status',
    value: 'Online',
    iconName: 'Connection',
    iconColor: '#6366f1',
    iconBg: 'rgba(99, 102, 241, 0.1)'
  },
  {
    id: 'bot-config',
    name: 'Configuration',
    value: 'Settings',
    iconName: 'Setting',
    iconColor: '#f59e0b',
    iconBg: 'rgba(245, 158, 11, 0.1)'
  }
])

const sessions = ref<SessionRecord[]>([
  {
    id: 'sess-001',
    userName: 'Minghong Huang',
    status: 'active',
    msgCount: 3,
    startTime: 'Today 10:20 AM',
    lastActiveTime: 'Today 10:25 AM',
    summary: 'Querying streaming media playback API performance.',
    messages: [
      { id: 'm1', sender: 'user', senderName: 'Minghong Huang', msgType: 'text', content: 'Help me check the latest K6 report.', time: '10:20 AM' },
      { id: 'm2', sender: 'bot', senderName: 'Feishu Bot', msgType: 'text', content: 'Sure. Looking up the latest K6 playback test results...', time: '10:20 AM' },
      { id: 'm3', sender: 'user', senderName: 'Minghong Huang', msgType: 'text', content: 'Thanks, what is the P95 latency?', time: '10:25 AM' }
    ]
  }
])

const operationLogs = ref<OperationLog[]>([])
const projects = ref<ProjectOption[]>([])

// --- State ---
const drawerVisible = ref(false)
const selectedSession = ref<SessionRecord | null>(null)
const replyContent = ref('')
const scheduledTaskDialogVisible = ref(false)
const scheduledTaskDialogMode = ref<'create' | 'edit'>('create')
const editingScheduledTaskId = ref('')
const scheduledTaskForm = ref<ScheduledTaskForm>({
  taskType: '',
  scheduleType: '',
  creator: '',
  startDate: '',
  executionTime: '',
  testProjectCode: '',
  testEnv: ''
})

const scheduleTypeOptions: ScheduleType[] = ['Once', 'Daily', 'weekly']
const testEnvOptions = ['测试服务器', '正式服务器']

const currentOperator = computed(() => {
  return authStore.user?.username || authStore.user?.user_name || authStore.user?.name || 'TesterByClaw'
})

const nextRunPreview = computed(() => {
  const form = scheduledTaskForm.value
  if (!form.scheduleType) return ''
  if (!form.startDate || !form.executionTime) return ''
  return `${form.startDate} ${form.executionTime}`
})

const scheduledTaskDialogTitle = computed(() => {
  return scheduledTaskDialogMode.value === 'edit' ? 'Edit Scheduled Task' : 'New Scheduled Tasks'
})

const scheduledTaskSubmitText = computed(() => {
  return scheduledTaskDialogMode.value === 'edit' ? 'Save' : 'Create'
})

// --- Lifecycle ---
onMounted(() => {
  fetchOperationLogs()
  fetchProjects()
  fetchScheduledTasks()
  // Refresh every 10s
  const logTimer = setInterval(fetchOperationLogs, 10000)
  const taskTimer = setInterval(fetchScheduledTasks, 10000)
  onUnmounted(() => {
    clearInterval(logTimer)
    clearInterval(taskTimer)
  })
})

// --- Handlers ---
const fetchOperationLogs = async () => {
  try {
    const res = await fetch(`${API_BASE}/ai/logs/operations`, {
      headers: {
        'Authorization': authStore.token
      }
    })
    const data = await res.json()
    operationLogs.value = data || []
  } catch (e) {
    console.error('Failed to fetch logs', e)
  }
}

const fetchProjects = async () => {
  try {
    const res = await fetch(`${API_BASE}/config/projects`)
    const data = await res.json()
    projects.value = Array.isArray(data) ? data : []
  } catch (e) {
    console.error('Failed to fetch projects', e)
  }
}

const parseApiResponse = async (res: Response) => {
  const rawText = await res.text()
  let data: any = null
  try {
    data = rawText ? JSON.parse(rawText) : null
  } catch {
    data = { error: rawText || 'Unexpected empty response' }
  }

  if (!res.ok) {
    throw new Error(data?.error || `Request failed (${res.status})`)
  }

  return data
}

const normalizeScheduledTask = (task: any): ScheduledTask => {
  return {
    id: task.id,
    name: task.name,
    scheduleType: task.scheduleType || task.schedule_type,
    creator: task.creator,
    nextRun: task.nextRun || task.next_run,
    testProject: task.testProject || task.test_project,
    testProjectCode: task.testProjectCode || task.test_project_code,
    testEnv: task.testEnv || task.test_env,
    status: task.status,
    description: task.description
  }
}

const fetchScheduledTasks = async () => {
  try {
    const res = await fetch(`${API_BASE}/scheduled-tasks`)
    const data = await parseApiResponse(res)
    scheduledTasks.value = Array.isArray(data) ? data.map(normalizeScheduledTask) : []
  } catch (e) {
    console.error('Failed to fetch scheduled tasks', e)
  }
}

const handleToolClick = (toolId: string) => {
  if (toolId === 'bot-config') {
    ElMessage.info('Opening Bot Configuration...')
  }
}

const getProjectLabel = (code: string) => {
  const matched = projects.value.find(item => item.project_code === code)
  if (!matched) return code || '-'
  return `${matched.project_name} (${matched.project_code})`
}

const getProjectName = (code: string) => {
  return projects.value.find(item => item.project_code === code)?.project_name || code
}

const openCreateScheduledTask = () => {
  scheduledTaskDialogMode.value = 'create'
  editingScheduledTaskId.value = ''
  scheduledTaskForm.value = {
    taskType: 'episode-playback-test',
    scheduleType: '',
    creator: currentOperator.value,
    startDate: '',
    executionTime: '',
    testProjectCode: '',
    testEnv: ''
  }
  scheduledTaskDialogVisible.value = true
}

const splitNextRun = (nextRun: string) => {
  const [date = '', time = ''] = (nextRun || '').split(' ')
  return { date, time }
}

const openEditScheduledTask = (task: ScheduledTask) => {
  const { date, time } = splitNextRun(task.nextRun)
  scheduledTaskDialogMode.value = 'edit'
  editingScheduledTaskId.value = task.id
  scheduledTaskForm.value = {
    taskType: 'episode-playback-test',
    scheduleType: task.scheduleType,
    creator: task.creator,
    startDate: date,
    executionTime: time,
    testProjectCode: task.testProjectCode,
    testEnv: task.testEnv
  }
  scheduledTaskDialogVisible.value = true
}

const handleScheduleTypeChange = () => {
  scheduledTaskForm.value.startDate = ''
  scheduledTaskForm.value.executionTime = ''
}

const createScheduledTask = async () => {
  const form = scheduledTaskForm.value
  if (!form.taskType || !form.scheduleType || !form.creator || !form.testProjectCode || !form.testEnv) {
    ElMessage.warning('请填写 Function、Schedule Type、Creater、Test Project 和 Test Env')
    return
  }
  if (!form.startDate || !form.executionTime) {
    ElMessage.warning('请选择定时任务的执行日期和执行时间')
    return
  }

  try {
    const res = await fetch(`${API_BASE}/scheduled-tasks`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        function: form.taskType,
        schedule_type: form.scheduleType,
        creator: form.creator,
        next_run: nextRunPreview.value,
        test_project: getProjectName(form.testProjectCode),
        test_project_code: form.testProjectCode,
        test_env: form.testEnv
      })
    })
    const data = await parseApiResponse(res)
    scheduledTasks.value.unshift(normalizeScheduledTask(data))
    scheduledTaskDialogVisible.value = false
    ElMessage.success('Scheduled task created')
  } catch (e: any) {
    ElMessage.error(e?.message || 'Scheduled task 创建失败')
  }
}

const updateScheduledTask = async () => {
  const form = scheduledTaskForm.value
  if (!editingScheduledTaskId.value || !form.scheduleType) {
    ElMessage.warning('请选择需要修改的 Scheduled task 和 Schedule Type')
    return
  }
  if (!form.startDate || !form.executionTime) {
    ElMessage.warning('请选择定时任务的执行日期和执行时间')
    return
  }

  try {
    const res = await fetch(`${API_BASE}/scheduled-tasks/${editingScheduledTaskId.value}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        schedule_type: form.scheduleType,
        next_run: nextRunPreview.value
      })
    })
    const data = await parseApiResponse(res)
    const updated = normalizeScheduledTask(data)
    scheduledTasks.value = scheduledTasks.value.map(item => item.id === updated.id ? updated : item)
    scheduledTaskDialogVisible.value = false
    ElMessage.success('Scheduled task updated')
  } catch (e: any) {
    ElMessage.error(e?.message || 'Scheduled task 修改失败')
  }
}

const submitScheduledTask = () => {
  if (scheduledTaskDialogMode.value === 'edit') {
    updateScheduledTask()
    return
  }
  createScheduledTask()
}

const openSessionDetail = (row: SessionRecord) => {
  selectedSession.value = row
  drawerVisible.value = true
}

const sendReply = () => {
  if (!replyContent.value) {
    ElMessage.warning('Empty message.')
    return
  }
  if (selectedSession.value) {
    selectedSession.value.messages.push({
      id: `m${Date.now()}`,
      sender: 'bot',
      senderName: 'Feishu Bot',
      msgType: 'text',
      content: replyContent.value,
      time: 'Just now'
    })
    replyContent.value = ''
    ElMessage.success('Reply sent (Mock)')
  }
}

// --- Helpers ---
const getAvatarStyle = (name: string) => {
  const colors = ['#409EFF', '#67C23A', '#E6A23C', '#F56C6C', '#8E44AD', '#3498DB']
  const charCode = name.charCodeAt(0) || 0
  return {
    backgroundColor: colors[charCode % colors.length],
    color: '#fff'
  }
}

const getStatusType = (status: string) => {
  return status === 'active' ? 'success' : 'info'
}

const formatTime = (ts: string) => {
  if (!ts) return '-'
  return new Date(ts).toLocaleString()
}
</script>

<template>
  <div class="dashboard">
    <div class="page-header">
      <div class="header-left">
        <h2 class="page-title">Feishu Assistant</h2>
        <div class="breadcrumb">Feishu Assistant <span class="divider">/</span> Bot Operations</div>
      </div>
      <div class="header-right">
        <el-button type="primary" class="new-scheduled-task-btn" @click="openCreateScheduledTask">
          <el-icon><component :is="Icons.Plus" /></el-icon>
          New Scheduled Tasks
        </el-button>
      </div>
    </div>

    <!-- ========== Overview Cards ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--indigo">
            <el-icon :size="14"><component :is="Icons.DataBoard" /></el-icon>
          </div>
          <h2 class="section-title">Bot Subsystem Overview</h2>
        </div>
      </div>

      <div class="card-grid card-grid--4">
        <div
          v-for="tool in tools"
          :key="tool.id"
          class="tool-card"
          @click="handleToolClick(tool.id)"
          :style="{ cursor: tool.id === 'bot-config' ? 'pointer' : 'default' }"
        >
          <div class="tool-card__top">
            <div class="tool-card__icon" :style="{ background: tool.iconBg }">
              <el-icon :size="20" :color="tool.iconColor">
                <component :is="Icons[tool.iconName as keyof typeof Icons]" />
              </el-icon>
            </div>
          </div>
          <h3 class="tool-card__name">{{ tool.name }}</h3>
          <div class="tool-card__footer">
            <span class="tool-card__value" :style="{ color: tool.iconColor }">{{ tool.value }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- ========== Session Cycles ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--blue">
            <el-icon :size="14"><component :is="Icons.ChatDotRound" /></el-icon>
          </div>
          <h2 class="section-title">Session Cycles (Recent Messages)</h2>
        </div>
      </div>

      <div class="table-container">
        <el-table 
          :data="sessions" 
          style="width: 100%" 
          row-class-name="session-row"
          @row-click="openSessionDetail"
        >
          <el-table-column label="User" width="200">
            <template #default="scope">
              <div class="sender-info">
                <el-avatar :size="30" :style="getAvatarStyle(scope.row.userName)">
                  {{ scope.row.userName.charAt(0) }}
                </el-avatar>
                <span class="sender-name">{{ scope.row.userName }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="status" label="State" width="100">
            <template #default="scope">
              <el-tag :type="getStatusType(scope.row.status)" size="small" round>
                {{ scope.row.status.toUpperCase() }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column prop="msgCount" label="Msgs" width="80" align="center" />

          <el-table-column prop="summary" label="Latest Activity Summary" min-width="250" show-overflow-tooltip />

          <el-table-column prop="lastActiveTime" label="Last Active Time" width="180" />
        </el-table>
      </div>
    </div>

    <!-- ========== Scheduled Tasks ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--green">
            <el-icon :size="14"><component :is="Icons.Clock" /></el-icon>
          </div>
          <h2 class="section-title">Scheduled tasks</h2>
        </div>
      </div>

      <div class="table-container">
        <el-table :data="scheduledTasks" style="width: 100%" empty-text="No scheduled tasks yet.">
          <el-table-column label="Task" min-width="260">
            <template #default="scope">
              <div class="scheduled-task">
                <span class="scheduled-task__name">{{ scope.row.name }}</span>
                <span class="scheduled-task__desc">{{ scope.row.description }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="scheduleType" label="Schedule Type" width="150" />
          <el-table-column prop="testProject" label="Test Project" width="190">
            <template #default="scope">
              <span>{{ scope.row.testProject }}</span>
              <span class="scheduled-task__code"> / {{ scope.row.testProjectCode }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="testEnv" label="Test Env" min-width="150" />
          <el-table-column prop="creator" label="Creater" width="150" />
          <el-table-column prop="nextRun" label="Next Run" width="170" />

          <el-table-column prop="status" label="Status" width="120" align="center">
            <template #default="scope">
              <el-tag :type="getStatusType(scope.row.status)" size="small" round>
                {{ scope.row.status.toUpperCase() }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="Actions" width="110" align="center" fixed="right">
            <template #default="scope">
              <el-button
                link
                type="primary"
                :disabled="scope.row.status === 'running'"
                @click.stop="openEditScheduledTask(scope.row)"
              >
                Edit
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- ========== Operation History ========== -->
    <div class="section">
      <div class="section-header">
        <div class="section-title-row">
          <div class="section-icon section-icon--orange">
            <el-icon :size="14"><component :is="Icons.List" /></el-icon>
          </div>
          <h2 class="section-title">Audit Log: AI Operations History</h2>
        </div>
      </div>

      <div class="table-container">
        <el-table :data="operationLogs" style="width: 100%" empty-text="No records yet. Perform an operation via AI to see it here.">
          <el-table-column prop="timestamp" label="Time" width="180">
            <template #default="scope">{{ formatTime(scope.row.timestamp) }}</template>
          </el-table-column>
          <el-table-column prop="tool_name" label="Operation" width="140">
            <template #default="scope">
              <el-tag type="danger" size="small">{{ scope.row.tool_name }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="project" label="Project" width="120" />
          <el-table-column prop="env" label="Env" width="100">
             <template #default="scope">
                <el-tag :type="scope.row.env === 'prod' ? 'danger' : 'warning'" size="small">
                  {{ scope.row.env.toUpperCase() }}
                </el-tag>
             </template>
          </el-table-column>
          <el-table-column prop="detail" label="Result Summary" min-width="300" />
          <el-table-column prop="user_id" label="Operator" width="180">
            <template #default="scope">
              <span>{{ scope.row.user_name || scope.row.user_id || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="Status" width="100" align="center">
            <template #default="scope">
              <el-icon color="#67C23A" v-if="scope.row.status === 'success'"><component :is="Icons.CircleCheckFilled" /></el-icon>
              <el-icon color="#F56C6C" v-else><component :is="Icons.CircleCloseFilled" /></el-icon>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>

    <!-- ========== Chat Drawer (Original) ========== -->
    <el-drawer
      v-model="drawerVisible"
      :title="`Session with ${selectedSession?.userName}`"
      size="450px"
      direction="rtl"
      destroy-on-close
    >
      <div class="chat-drawer-container">
        <div class="chat-messages">
          <div 
            v-for="msg in selectedSession?.messages" 
            :key="msg.id" 
            :class="['chat-bubble-wrapper', msg.sender === 'bot' ? 'right' : 'left']"
          >
            <el-avatar v-if="msg.sender === 'user'" :size="32" :style="getAvatarStyle(msg.senderName)" class="chat-avatar">
              {{ msg.senderName.charAt(0) }}
            </el-avatar>
            <div class="chat-bubble-content">
              <span class="chat-time">{{ msg.time }}</span>
              <div :class="['chat-bubble', msg.sender === 'bot' ? 'bg-bot' : 'bg-user']">
                {{ msg.content }}
              </div>
            </div>
            <el-avatar v-if="msg.sender === 'bot'" :size="32" style="background:#6366f1; color:#fff;" class="chat-avatar">
              🤖
            </el-avatar>
          </div>
        </div>
        <div class="chat-reply-area" v-if="selectedSession?.status === 'active'">
          <el-input v-model="replyContent" type="textarea" :rows="3" placeholder="Type a reply to send to Feishu..." resize="none" />
          <div class="reply-actions text-right mt-2"><el-button type="primary" @click="sendReply">Send</el-button></div>
        </div>
        <div class="chat-ended-notice" v-else>This session has been ended.</div>
      </div>
    </el-drawer>

    <el-dialog
      v-model="scheduledTaskDialogVisible"
      :title="scheduledTaskDialogTitle"
      width="560px"
      destroy-on-close
    >
      <el-form :model="scheduledTaskForm" label-position="top" class="scheduled-task-form">
        <el-form-item label="Function">
          <el-select
            v-model="scheduledTaskForm.taskType"
            placeholder="请选择功能"
            style="width: 100%"
            :disabled="scheduledTaskDialogMode === 'edit'"
          >
            <el-option label="剧集播放接口测试" value="episode-playback-test" />
          </el-select>
        </el-form-item>

        <el-form-item label="Schedule Type">
          <el-select
            v-model="scheduledTaskForm.scheduleType"
            placeholder="请选择执行频率"
            style="width: 100%"
            @change="handleScheduleTypeChange"
          >
            <el-option
              v-for="item in scheduleTypeOptions"
              :key="item"
              :label="item"
              :value="item"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="Test Project">
          <el-select
            v-model="scheduledTaskForm.testProjectCode"
            placeholder="请选择测试项目"
            style="width: 100%"
            filterable
            :disabled="scheduledTaskDialogMode === 'edit'"
          >
            <el-option
              v-for="project in projects"
              :key="project.id || project.project_code"
              :label="getProjectLabel(project.project_code)"
              :value="project.project_code"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="Test Env">
          <el-select
            v-model="scheduledTaskForm.testEnv"
            placeholder="请选择测试环境"
            style="width: 100%"
            :disabled="scheduledTaskDialogMode === 'edit'"
          >
            <el-option
              v-for="env in testEnvOptions"
              :key="env"
              :label="env"
              :value="env"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="Creater">
          <el-input v-model="scheduledTaskForm.creator" disabled />
        </el-form-item>

        <el-form-item label="Next Run">
          <div
            v-if="scheduledTaskForm.scheduleType"
            class="next-run-picker"
          >
            <el-date-picker
              v-model="scheduledTaskForm.startDate"
              type="date"
              placeholder="请选择开始日期"
              value-format="YYYY-MM-DD"
              format="YYYY-MM-DD"
              class="next-run-picker__date"
            />
            <el-time-picker
              v-model="scheduledTaskForm.executionTime"
              placeholder="请选择执行时间"
              value-format="HH:mm:ss"
              format="HH:mm"
              class="next-run-picker__time"
            />
          </div>
          <el-input v-else placeholder="请先选择 Schedule Type" disabled />
          <div v-if="scheduledTaskForm.scheduleType === 'weekly'" class="schedule-hint">
            Weekly tasks run every 7 days from the selected start date.
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="scheduledTaskDialogVisible = false">Cancel</el-button>
        <el-button type="primary" @click="submitScheduledTask">{{ scheduledTaskSubmitText }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
/* ==================== Container ==================== */
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 32px;
  padding: 8px 0;
  position: relative;
}

/* ==================== Page Header ==================== */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 4px 0 2px;
}

.page-title {
  color: #111827;
  font-size: 28px;
  font-weight: 800;
  letter-spacing: -0.7px;
  margin: 0 0 6px;
}

.breadcrumb {
  color: #94a3b8;
  font-size: 13px;
}

.divider {
  margin: 0 8px;
  color: #cbd5e1;
}

.new-scheduled-task-btn {
  border-radius: 10px;
  font-weight: 700;
  height: 40px;
}

/* ==================== Sections ==================== */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.section-icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
}

.section-icon--blue { background: #3b82f6; }
.section-icon--indigo { background: #6366f1; }
.section-icon--orange { background: #f59e0b; }
.section-icon--green { background: #10b981; }

.section-title {
  font-size: 17px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
  letter-spacing: -0.2px;
}

/* ==================== Card Grid ==================== */
.card-grid {
  display: grid;
  gap: 16px;
}

.card-grid--4 {
  grid-template-columns: repeat(4, 1fr);
}

.tool-card {
  background: #ffffff;
  border: 1px solid #f0f0f0;
  border-radius: 14px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  transition: box-shadow 0.25s ease, transform 0.2s ease;
}

.tool-card:hover {
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.06);
  transform: translateY(-2px);
}

.tool-card__icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.tool-card__name {
  font-size: 14px;
  font-weight: 600;
  color: #64748b;
  margin: 0;
}

.tool-card__value {
  font-size: 28px;
  font-weight: 700;
}

/* ==================== Table ==================== */
.table-container {
  background: #ffffff;
  border: 1px solid #f0f0f0;
  border-radius: 14px;
  overflow: hidden;
}

.sender-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

:deep(.session-row) {
  cursor: pointer;
}

.scheduled-task {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 4px 0;
}

.scheduled-task__name {
  color: #1e293b;
  font-size: 14px;
  font-weight: 700;
}

.scheduled-task__desc {
  color: #94a3b8;
  font-size: 12px;
}

.scheduled-task__code {
  color: #94a3b8;
  font-size: 12px;
}

.scheduled-task-form {
  padding-top: 4px;
}

.next-run-picker {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 12px;
  width: 100%;
  min-width: 0;
}

.next-run-picker__date,
.next-run-picker__time {
  max-width: 100%;
  min-width: 0;
  width: 100%;
}

:deep(.next-run-picker .el-date-editor.el-input),
:deep(.next-run-picker .el-date-editor.el-input__wrapper) {
  width: 100%;
}

.schedule-hint {
  color: #94a3b8;
  font-size: 12px;
  line-height: 1.4;
  margin-top: 8px;
}

/* ==================== Chat Drawer ==================== */
.chat-drawer-container { display: flex; flex-direction: column; height: 100%; }
.chat-messages { flex: 1; overflow-y: auto; padding: 20px; display: flex; flex-direction: column; gap: 20px; background: #f8fafc; }
.chat-bubble-wrapper { display: flex; align-items: flex-end; gap: 12px; }
.chat-bubble-wrapper.right { justify-content: flex-end; }
.chat-bubble-content { display: flex; flex-direction: column; max-width: 70%; }
.chat-bubble-wrapper.right .chat-bubble-content { align-items: flex-end; }
.chat-bubble { padding: 10px 14px; border-radius: 12px; font-size: 14px; }
.bg-user { background: #fff; border: 1px solid #e2e8f0; }
.bg-bot { background: #e0e7ff; color: #312e81; }
.chat-reply-area { padding: 16px 20px; background: #fff; border-top: 1px solid #e2e8f0; }
.text-right { text-align: right; }
.mt-2 { margin-top: 8px; }
.chat-ended-notice { padding: 16px; text-align: center; color: #94a3b8; background: #f1f5f9; }
</style>
