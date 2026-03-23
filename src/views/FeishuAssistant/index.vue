<script setup lang="ts">
import { ref } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'

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

// --- Mock Data ---
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
  },
  {
    id: 'sess-002',
    userName: 'Alice Wong',
    status: 'ended',
    msgCount: 6,
    startTime: 'Yesterday 14:00 PM',
    lastActiveTime: 'Yesterday 14:15 PM',
    summary: 'Troubleshooting test environment login issue.',
    messages: [
      { id: 'm4', sender: 'user', senderName: 'Alice Wong', msgType: 'text', content: 'I cannot login to test env.', time: '14:00 PM' },
      { id: 'm5', sender: 'bot', senderName: 'Feishu Bot', msgType: 'text', content: 'It seems the auth server was restarting. Please try again now.', time: '14:02 PM' },
      { id: 'm6', sender: 'user', senderName: 'Alice Wong', msgType: 'text', content: 'Works now, thanks.', time: '14:10 PM' },
      { id: 'm7', sender: 'user', senderName: 'Alice Wong', msgType: 'text', content: '会话结束', time: '14:15 PM' },
      { id: 'm8', sender: 'bot', senderName: 'Feishu Bot', msgType: 'text', content: '会话已结束。', time: '14:15 PM' }
    ]
  }
])

// --- State ---
const drawerVisible = ref(false)
const selectedSession = ref<SessionRecord | null>(null)
const replyContent = ref('')

// --- Handlers ---
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

    <!-- ========== Chat Drawer ========== -->
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
            <!-- User Avatar -->
            <el-avatar v-if="msg.sender === 'user'" :size="32" :style="getAvatarStyle(msg.senderName)" class="chat-avatar">
              {{ msg.senderName.charAt(0) }}
            </el-avatar>

            <!-- Message Content -->
            <div class="chat-bubble-content">
              <span class="chat-time">{{ msg.time }}</span>
              <div :class="['chat-bubble', msg.sender === 'bot' ? 'bg-bot' : 'bg-user']">
                {{ msg.content }}
              </div>
            </div>
            
            <!-- Bot Avatar -->
            <el-avatar v-if="msg.sender === 'bot'" :size="32" style="background:#6366f1; color:#fff;" class="chat-avatar">
              🤖
            </el-avatar>
          </div>
        </div>

        <!-- Reply Area -->
        <div class="chat-reply-area" v-if="selectedSession?.status === 'active'">
          <el-input 
            v-model="replyContent" 
            type="textarea" 
            :rows="3" 
            placeholder="Type a reply to send to Feishu..." 
            resize="none"
          />
          <div class="reply-actions">
            <el-button type="primary" size="default" @click="sendReply">Send</el-button>
          </div>
        </div>
        <div class="chat-ended-notice" v-else>
          This session has been ended.
        </div>
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

.section-title {
  font-size: 17px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
  letter-spacing: -0.2px;
}

/* ==================== Card Grid (Dashboard Layout) ==================== */
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
  position: relative;
}

.tool-card:hover {
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.06);
  transform: translateY(-2px);
}

.tool-card__top {
  display: flex;
  justify-content: flex-start;
  align-items: flex-start;
}

.tool-card__icon {
  width: 42px;
  height: 42px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.tool-card__name {
  font-size: 14px;
  font-weight: 600;
  color: #64748b;
  margin: 0;
}

.tool-card__footer {
  margin-top: auto;
}

.tool-card__value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
}

/* ==================== Table Container ==================== */
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

.sender-name {
  font-weight: 500;
  color: #333;
}

:deep(.session-row) {
  cursor: pointer;
  transition: background-color 0.2s;
}

:deep(.session-row:hover > td.el-table__cell) {
  background-color: #f8fafc !important;
}

/* ==================== Chat Drawer ==================== */
.chat-drawer-container {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  background-color: #f8fafc;
}

.chat-bubble-wrapper {
  display: flex;
  align-items: flex-end;
  gap: 12px;
}

.chat-bubble-wrapper.left {
  justify-content: flex-start;
}

.chat-bubble-wrapper.right {
  justify-content: flex-end;
}

.chat-bubble-content {
  display: flex;
  flex-direction: column;
  max-width: 70%;
}

.chat-bubble-wrapper.left .chat-bubble-content {
  align-items: flex-start;
}

.chat-bubble-wrapper.right .chat-bubble-content {
  align-items: flex-end;
}

.chat-time {
  font-size: 11px;
  color: #94a3b8;
  margin-bottom: 4px;
}

.chat-bubble {
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
}

.bg-user {
  background-color: #ffffff;
  color: #1e293b;
  border: 1px solid #e2e8f0;
  border-bottom-left-radius: 4px;
}

.bg-bot {
  background-color: #e0e7ff;
  color: #312e81;
  border: 1px solid #c7d2fe;
  border-bottom-right-radius: 4px;
}

.chat-reply-area {
  padding: 16px 20px;
  background: #ffffff;
  border-top: 1px solid #e2e8f0;
}

.reply-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}

.chat-ended-notice {
  padding: 16px;
  text-align: center;
  color: #94a3b8;
  font-size: 13px;
  background: #f1f5f9;
  border-top: 1px solid #e2e8f0;
}
</style>
