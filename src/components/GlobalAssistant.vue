<script setup lang="ts">
import { computed, ref, nextTick, onMounted, onBeforeUnmount } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useDramaRunStore } from '@/stores'

// --- Types ---
interface Message {
  role: 'user' | 'assistant'
  text: string
}

interface ReportData {
  project_name: string
  project_code: string
  version: string
  period: string
  environment: string
  bug_links_fixed: string[]
  bug_links_unfixed: string[]
  story_links: string[]
  full_text?: string
}

// --- API Service ---
const API_BASE = 'http://localhost:8080/api'
const authStore = useAuthStore()
const dramaRunStore = useDramaRunStore()

// --- State ---
const assistantVisible = ref(false)
const assistantSessionId = ref(`WEB_${Math.random().toString(36).substring(7)}`)
const assistantInput = ref('')
const assistantLoading = ref(false)
const assistantMessages = ref<Message[]>([
  { role: 'assistant', text: '你好！我是你的智能助手，有什么可以帮你的吗？' }
])
const messageContainer = ref<HTMLElement | null>(null)
const fabPosition = ref({ top: 0, left: 0 })
const isDraggingFab = ref(false)
const assistantButtonLabel = computed(() => dramaRunStore.assistantLabel)
const assistantButtonActive = computed(() => dramaRunStore.isAssistantActive)
let dragOffsetX = 0
let dragOffsetY = 0
let dragMoved = false
let dragStarted = false

// --- Report Dialog State ---
const showReportDialog = ref(false)
const reportForm = ref<ReportData>({
  project_name: '',
  project_code: '',
  version: '',
  period: '',
  environment: '',
  bug_links_fixed: [],
  bug_links_unfixed: [],
  story_links: []
})
const reportSaving = ref(false)

// --- Handlers ---
const toggleAssistant = () => {
  assistantVisible.value = !assistantVisible.value
  if (!assistantVisible.value) {
    finalizeAssistantSession()
  }
}

const sendAssistantMessage = async () => {
  if (!assistantInput.value || assistantLoading.value) return

  const userText = assistantInput.value
  assistantMessages.value.push({ role: 'user', text: userText })
  assistantInput.value = ''
  assistantLoading.value = true

  scrollToBottom()

  try {
    const res = await fetch(`${API_BASE}/ai/web-chat`, {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({
        message: userText,
        session_id: assistantSessionId.value,
        user_id: authStore.user?.id || 'WEB_DASHBOARD_USER'
      })
    })
    const data = await res.json()
    const reply = data.reply || ''
    const toolResult = data.tool_result || ''

    // Add assistant's text response to chat
    assistantMessages.value.push({ role: 'assistant', text: reply || 'AI 暂时无法响应' })

    // Check if tool result contains report data
    if (toolResult && toolResult.includes('_is_report":true')) {
      try {
        const reportObj = JSON.parse(toolResult)
        // Populate form
        reportForm.value = {
          project_name: reportObj.project_name,
          project_code: reportObj.project_code,
          version: reportObj.version,
          period: reportObj.period,
          environment: reportObj.environment,
          bug_links_fixed: reportObj.bug_links_fixed || [],
          bug_links_unfixed: reportObj.bug_links_unfixed || [],
          story_links: reportObj.story_links || [],
          full_text: reportObj.full_text
        }
        showReportDialog.value = true
      } catch (e) {
        console.warn('Failed to parse tool result', e)
      }
    }
  } catch (err) {
    assistantMessages.value.push({ role: 'assistant', text: '❌ 网络异常，请检查后端。' })
  } finally {
    assistantLoading.value = false
    scrollToBottom()
  }
}

const saveReport = async () => {
  reportSaving.value = true
  try {
    const res = await fetch(`${API_BASE}/acceptance-reports/save`, {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({
        ...reportForm.value,
        id: `AR_${Date.now()}`,
        reporter: authStore.user?.username || 'AI Assistant',
        test_owner: authStore.user?.username || 'AI Assistant',
        test_time: reportForm.value.period,
        test_env: reportForm.value.environment,
        test_conclusion: 'Pass',
        bug_submission_status: reportForm.value.bug_links_unfixed.join('\n'),
        bug_fix_status: reportForm.value.bug_links_fixed.join('\n'),
        update_requirements: reportForm.value.story_links.join('\n')
      })
    })
    if (res.ok) {
      ElMessage.success('验收报告已成功入库！')
      showReportDialog.value = false
    } else {
      ElMessage.error('保存失败，请检查后端连接。')
    }
  } catch (err) {
    ElMessage.error('网络错误，无法保存报告。')
  } finally {
    reportSaving.value = false
  }
}

const finalizeAssistantSession = async () => {
  try {
    await fetch(`${API_BASE}/ai/web-chat/end`, {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({
        session_id: assistantSessionId.value,
        user_id: authStore.user?.id || 'WEB_DASHBOARD_USER'
      })
    })
    // Reset session for next time
    assistantSessionId.value = `WEB_${Math.random().toString(36).substring(7)}`
    assistantMessages.value = [{ role: 'assistant', text: '你好！我是你的智能助手，有什么可以帮你的吗？' }]
  } catch (e) {
    console.warn('Finalize failed', e)
  }
}

const scrollToBottom = () => {
  nextTick(() => {
    if (messageContainer.value) {
      messageContainer.value.scrollTop = messageContainer.value.scrollHeight
    }
  })
}

const handleEnter = (e: KeyboardEvent) => {
  if (e.shiftKey) return
  e.preventDefault()
  sendAssistantMessage()
}

const syncFabPositionWithinViewport = () => {
  const buttonWidth = 140
  const buttonHeight = 52
  const margin = 24
  const maxLeft = Math.max(margin, window.innerWidth - buttonWidth - margin)
  const maxTop = Math.max(margin, window.innerHeight - buttonHeight - margin)

  if (fabPosition.value.top === 0 && fabPosition.value.left === 0) {
    fabPosition.value = {
      top: Math.max(margin, window.innerHeight - buttonHeight - 100),
      left: maxLeft
    }
    return
  }

  fabPosition.value = {
    top: Math.min(Math.max(fabPosition.value.top, margin), maxTop),
    left: Math.min(Math.max(fabPosition.value.left, margin), maxLeft)
  }
}

const handleFabPointerMove = (event: PointerEvent) => {
  if (!dragStarted) return

  const nextLeft = event.clientX - dragOffsetX
  const nextTop = event.clientY - dragOffsetY
  const distance = Math.abs(nextLeft - fabPosition.value.left) + Math.abs(nextTop - fabPosition.value.top)

  if (distance > 3) {
    dragMoved = true
    isDraggingFab.value = true
  }

  fabPosition.value = { top: nextTop, left: nextLeft }
  syncFabPositionWithinViewport()
}

const stopFabDrag = () => {
  if (!dragStarted) return

  window.removeEventListener('pointermove', handleFabPointerMove)
  window.removeEventListener('pointerup', stopFabDrag)

  const shouldToggle = !dragMoved
  dragStarted = false
  isDraggingFab.value = false
  dragOffsetX = 0
  dragOffsetY = 0

  window.setTimeout(() => {
    dragMoved = false
  }, 0)

  if (shouldToggle) {
    toggleAssistant()
  }
}

const startFabDrag = (event: PointerEvent) => {
  dragStarted = true
  dragMoved = false
  dragOffsetX = event.clientX - fabPosition.value.left
  dragOffsetY = event.clientY - fabPosition.value.top

  window.addEventListener('pointermove', handleFabPointerMove)
  window.addEventListener('pointerup', stopFabDrag)
}

onMounted(() => {
  syncFabPositionWithinViewport()
  dramaRunStore.recoverCurrentRun()
  window.addEventListener('resize', syncFabPositionWithinViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncFabPositionWithinViewport)
  window.removeEventListener('pointermove', handleFabPointerMove)
  window.removeEventListener('pointerup', stopFabDrag)
})
</script>

<template>
  <div class="global-assistant">
    <!-- FAB Button -->
    <div
      class="fab-container"
      :class="{ 'is-dragging': isDraggingFab, 'is-test-active': assistantButtonActive }"
      :style="{ top: `${fabPosition.top}px`, left: `${fabPosition.left}px` }"
      @pointerdown.prevent="startFabDrag"
    >
      <transition name="assistant-bubble">
        <div v-if="dramaRunStore.bubbleVisible" class="fab-status-bubble">
          {{ dramaRunStore.bubbleText }}
        </div>
      </transition>
      <el-button
        type="primary"
        size="large"
        class="fab-btn"
        :class="{ 'fab-btn--test-active': assistantButtonActive }"
      >
        <el-icon class="mr-2"><component :is="assistantButtonActive ? Icons.VideoPlay : Icons.ChatLineRound" /></el-icon>
        {{ assistantButtonLabel }}
      </el-button>
    </div>

    <!-- Assistant Chat Window -->
    <div 
      v-if="assistantVisible" 
      :class="['assistant-window-fixed', 'scale-up', { 'is-minimized': showReportDialog }]"
    >
      <div class="assistant-header">
        <div class="header-left">
          <el-avatar :size="28" style="background:#fff; color:#6366f1">🤖</el-avatar>
          <span class="header-title">智能助手 (DeepSeek AI)</span>
        </div>
        <el-icon class="close-btn" @click="toggleAssistant"><component :is="Icons.Close" /></el-icon>
      </div>
      <div class="assistant-body" ref="messageContainer">
        <div
          v-for="(msg, idx) in assistantMessages"
          :key="idx"
          :class="['msg-bubble-row', msg.role === 'user' ? 'user-row' : 'bot-row']"
        >
          <div class="msg-bubble shadow-sm">{{ msg.text }}</div>
        </div>
        <div v-if="assistantLoading" class="msg-bubble-row bot-row">
          <div class="msg-bubble loading-dots">
            <el-icon class="is-loading mr-1"><component :is="Icons.Loading" /></el-icon> AI 正在思考中...
          </div>
        </div>
      </div>
      <div class="assistant-footer">
        <div class="input-container">
          <el-input
            v-model="assistantInput"
            type="textarea"
            :autosize="{ minRows: 1, maxRows: 6 }"
            placeholder="输入指令 (Enter 发送, Shift+Enter 换行)..."
            @keydown.enter="handleEnter"
            class="assistant-textarea"
          />
          <el-button 
            type="primary" 
            circle 
            class="send-btn-circle" 
            @click="sendAssistantMessage"
            :disabled="!assistantInput || assistantLoading"
          >
            <el-icon :size="20"><component :is="Icons.Promotion" /></el-icon>
          </el-button>
        </div>
      </div>
    </div>

    <!-- Editable Report Dialog -->
    <el-dialog
      v-model="showReportDialog"
      title="核对并发送验收报告"
      width="90%"
      style="max-width: 600px"
      append-to-body
      destroy-on-close
    >
      <el-form :model="reportForm" label-width="100px" size="default">
        <el-form-item label="项目名称">
          <el-input v-model="reportForm.project_name" placeholder="请输入项目全称" />
        </el-form-item>
        <el-form-item label="项目代码">
          <el-input v-model="reportForm.project_code" disabled />
        </el-form-item>
        <el-form-item label="版本号">
          <el-input v-model="reportForm.version" placeholder="vX.X.X" />
        </el-form-item>
        <el-form-item label="测试周期">
          <el-input v-model="reportForm.period" placeholder="MM.DD-MM.DD" />
        </el-form-item>
        <el-form-item label="测试环境">
          <el-input v-model="reportForm.environment" type="textarea" :rows="2" />
        </el-form-item>
        <el-divider content-position="left">测试内容链接</el-divider>
        <el-form-item label="已修复 Bug">
          <el-input 
            v-model="reportForm.bug_links_fixed[i]" 
            v-for="(_, i) in reportForm.bug_links_fixed" 
            :key="'fixed-'+i"
            placeholder="链接 URL"
            class="mb-2"
          />
        </el-form-item>
        <el-form-item label="未修复 Bug">
          <el-input 
            v-model="reportForm.bug_links_unfixed[i]" 
            v-for="(_, i) in reportForm.bug_links_unfixed" 
            :key="'unfixed-'+i"
            placeholder="链接 URL"
            class="mb-2"
          />
        </el-form-item>
        <el-form-item label="需求点">
          <el-input 
            v-model="reportForm.story_links[i]" 
            v-for="(_, i) in reportForm.story_links" 
            :key="'story-'+i"
            placeholder="链接 URL"
            class="mb-2"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="showReportDialog = false">取消</el-button>
          <el-button type="primary" :loading="reportSaving" @click="saveReport">
            确认入库并保存
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.fab-container {
  position: fixed;
  z-index: 3000;
  touch-action: none;
}

.fab-container.is-dragging {
  cursor: grabbing;
}

.fab-container.is-test-active {
  animation: fabFloat 2.4s ease-in-out infinite;
}

.fab-status-bubble {
  position: absolute;
  right: 0;
  bottom: calc(100% + 12px);
  width: max-content;
  max-width: 260px;
  padding: 10px 14px;
  border-radius: 16px 16px 4px 16px;
  background: rgba(15, 23, 42, 0.94);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  box-shadow: 0 14px 30px rgba(15, 23, 42, 0.22);
  pointer-events: none;
}

.fab-status-bubble::after {
  content: '';
  position: absolute;
  right: 20px;
  bottom: -6px;
  width: 12px;
  height: 12px;
  background: rgba(15, 23, 42, 0.94);
  transform: rotate(45deg);
}

.fab-btn {
  box-shadow: 0 4px 12px rgba(99, 102, 241, 0.4);
  height: 52px;
  padding: 0 28px;
  border-radius: 26px;
  font-weight: 600;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  user-select: none;
}

.fab-btn--test-active {
  min-width: 112px;
  color: #0f172a;
  border: none;
  background: linear-gradient(135deg, #fef3c7 0%, #67e8f9 48%, #86efac 100%);
  box-shadow: 0 10px 26px rgba(14, 165, 233, 0.34);
}

.fab-btn:hover {
  transform: scale(1.05) translateY(-2px);
  box-shadow: 0 6px 16px rgba(99, 102, 241, 0.5);
}

.fab-container.is-dragging .fab-btn,
.fab-container.is-dragging .fab-btn:hover {
  transform: none;
  box-shadow: 0 8px 20px rgba(99, 102, 241, 0.35);
}

.assistant-bubble-enter-active,
.assistant-bubble-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.assistant-bubble-enter-from,
.assistant-bubble-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.96);
}

@keyframes fabFloat {
  0%, 100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-5px);
  }
}

.mr-2 { margin-right: 8px; }
.mr-1 { margin-right: 4px; }

.assistant-window-fixed {
  position: fixed;
  bottom: 100px;
  right: 40px;
  width: 480px;
  height: 650px;
  max-width: 90vw;
  max-height: 85vh;
  background: #fff;
  border-radius: 20px;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.2);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(226, 232, 240, 0.8);
  z-index: 3001;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.assistant-window-fixed.is-minimized {
  opacity: 0.15;
  transform: scale(0.6) translate(100px, 100px);
  filter: blur(2px);
  pointer-events: none;
}

.scale-up {
  animation: scaleIn 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes scaleIn {
  from { transform: scale(0.9); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

.assistant-header {
  height: 64px;
  background: linear-gradient(135deg, #6366f1 0%, #4f46e5 100%);
  padding: 0 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #fff;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-title { font-weight: 700; font-size: 16px; letter-spacing: 0.5px; }
.close-btn { cursor: pointer; font-size: 20px; transition: opacity 0.2s; }
.close-btn:hover { opacity: 0.7; }

.assistant-body {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  background: #f1f5f9;
}

.msg-bubble-row { display: flex; width: 100%; }
.user-row { justify-content: flex-end; }
.bot-row { justify-content: flex-start; }

.msg-bubble {
  max-width: 85%;
  padding: 12px 18px;
  border-radius: 16px;
  font-size: 15px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: break-word;
}

.user-row .msg-bubble {
  background: #6366f1;
  color: #fff;
  border-bottom-right-radius: 4px;
  box-shadow: 0 4px 10px rgba(99, 102, 241, 0.2);
}

.bot-row .msg-bubble {
  background: #fff;
  color: #1e293b;
  border: 1px solid #e2e8f0;
  border-bottom-left-radius: 4px;
  box-shadow: 0 4px 6px rgba(0, 0, 0, 0.02);
}

.loading-dots { 
  color: #64748b; 
  font-weight: 500;
  display: flex;
  align-items: center;
}

.assistant-footer {
  padding: 16px 20px;
  border-top: 1px solid #e2e8f0;
  background: #fff;
}

.input-container {
  display: flex;
  align-items: flex-end;
  gap: 12px;
}

.assistant-textarea :deep(.el-textarea__inner) {
  border-radius: 16px;
  padding: 10px 16px;
  resize: none;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
  font-size: 15px;
  line-height: 1.5;
  transition: all 0.2s;
  box-shadow: none !important;
}

.assistant-textarea :deep(.el-textarea__inner:focus) {
  border-color: #6366f1;
  background: #fff;
  box-shadow: 0 0 0 1px #6366f1 inset !important;
}

.send-btn-circle {
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  margin-bottom: 2px;
  transition: all 0.2s;
}

.send-btn-circle:not(:disabled):hover {
  background: #6366f1 !important;
  color: #fff !important;
  transform: scale(1.05);
}
</style>
