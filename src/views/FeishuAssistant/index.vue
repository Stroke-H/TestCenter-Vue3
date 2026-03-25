<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
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

// --- API Service (Direct fetch for simplicity in this dashboard) ---
const API_BASE = 'http://localhost:8080/api'
const authStore = useAuthStore()

// --- Mock / Init Data ---
const tools = ref([
  {
    id: 'total-sessions',
    name: 'Total Sessions',
    value: '142',
    iconName: 'ChatLineSquare',
    iconColor: '#3b82f6',
    iconBg: 'rgba(59, 130, 246, 0.1)'
  },
  {
    id: 'active-users',
    name: 'Active Users',
    value: '45',
    iconName: 'User',
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

// --- State ---
const drawerVisible = ref(false)
const selectedSession = ref<SessionRecord | null>(null)
const replyContent = ref('')

// --- Lifecycle ---
onMounted(() => {
  fetchOperationLogs()
  // Refresh every 10s
  const timer = setInterval(fetchOperationLogs, 10000)
  onUnmounted(() => clearInterval(timer))
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

const handleToolClick = (toolId: string) => {
  if (toolId === 'bot-config') {
    ElMessage.info('Opening Bot Configuration...')
  }
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
