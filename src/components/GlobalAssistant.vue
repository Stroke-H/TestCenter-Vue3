<script setup lang="ts">
import { computed, ref, nextTick, onMounted, onBeforeUnmount } from 'vue'
import * as Icons from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { notifyAcceptanceReportsChanged } from '@/utils/acceptanceReportEvents'
import { useDramaRunStore, usePermissionStore, useTestcaseGenerationRunStore } from '@/stores'
import { useRouter } from 'vue-router'
import {
  ASSISTANT_QUICK_ENTRY_CHANGED_EVENT,
  type AssistantQuickEntry,
  getAssistantQuickEntriesByIds,
  getAssistantQuickEntryIds
} from '@/config/assistantQuickEntries'

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
const API_BASE = '/api'
const authStore = useAuthStore()
const dramaRunStore = useDramaRunStore()
const testcaseGenerationRunStore = useTestcaseGenerationRunStore()
const permissionStore = usePermissionStore()
const router = useRouter()

// --- State ---
const assistantVisible = ref(false)
const assistantSessionId = ref(`WEB_${Math.random().toString(36).substring(7)}`)
const assistantInput = ref('')
const assistantLoading = ref(false)
const assistantMode = ref<'work' | 'casual'>('work')
const isComposingInput = ref(false)
const assistantMessages = ref<Message[]>([
  { role: 'assistant', text: '你好！我是你的智能助手，有什么可以帮你的吗？' }
])
const assistantQuickEntryIds = ref<string[]>(getAssistantQuickEntryIds())
const messageContainer = ref<HTMLElement | null>(null)
const fabContainer = ref<HTMLElement | null>(null)
const fabPosition = ref({ top: 0, left: 0 })
const isDraggingFab = ref(false)
const isFabFanMenuVisible = ref(false)
const primaryRunningTask = computed<'testcase' | 'drama' | null>(() => {
  if (testcaseGenerationRunStore.hasRecoverableRun) return 'testcase'
  if (dramaRunStore.status === 'running') return 'drama'
  return null
})
const assistantButtonLabel = computed(() => {
  if (primaryRunningTask.value === 'testcase') return testcaseGenerationRunStore.assistantLabel
  if (primaryRunningTask.value === 'drama') return dramaRunStore.assistantLabel
  return '智能助手'
})
const assistantButtonActive = computed(() => primaryRunningTask.value !== null || dramaRunStore.isAssistantActive)
const assistantButtonIcon = computed(() => {
  if (primaryRunningTask.value === 'testcase') return Icons.Notebook
  if (assistantButtonActive.value) return Icons.VideoPlay
  return Icons.ChatLineRound
})
const assistantModeLabel = computed(() => assistantMode.value === 'work' ? '工作' : '轻聊')
const assistantModeTip = computed(() => assistantMode.value === 'work'
  ? '当前使用平台工具、权限和安全约束'
  : '当前绕过工具和系统约束，仅使用纯模型回复'
)
const hasRunningTaskShortcut = computed(() => primaryRunningTask.value !== null)
const testcaseTaskPhaseLabel = computed(() => {
  switch (testcaseGenerationRunStore.taskPhase) {
    case 'generating':
      return '用例生成'
    case 'reviewing':
      return '用例评审'
    case 'optimizing':
      return '用例优化'
    case 'result':
      return '用例结果'
    default:
      return '当前任务'
  }
})
const runningTaskButtonLabel = computed(() => {
  if (primaryRunningTask.value === 'testcase') {
    switch (testcaseGenerationRunStore.taskPhase) {
      case 'generating':
        return '返回用例生成'
      case 'reviewing':
        return '返回用例评审'
      case 'optimizing':
        return '返回用例优化'
      case 'result':
        return '查看用例结果'
      default:
        return '返回用例任务'
    }
  }
  if (primaryRunningTask.value === 'drama') return '返回接口测试'
  return '返回当前任务'
})
const runningTaskButtonIcon = computed(() => {
  if (primaryRunningTask.value === 'testcase') {
    switch (testcaseGenerationRunStore.taskPhase) {
      case 'reviewing':
        return Icons.DocumentChecked
      case 'optimizing':
        return Icons.MagicStick
      case 'result':
        return Icons.View
      default:
        return Icons.Notebook
    }
  }
  if (primaryRunningTask.value === 'drama') return Icons.VideoPlay
  return Icons.ChatLineRound
})
const testcaseStatusBubbleText = computed(() => {
  if (testcaseGenerationRunStore.taskPhase === 'reviewing') {
    return `当前用例评审正在后台执行（${testcaseGenerationRunStore.reviewedRoleDoneCount}/${testcaseGenerationRunStore.reviewingRoleCount}）`
  }
  if (testcaseGenerationRunStore.taskPhase === 'optimizing') {
    return '当前正在根据评审意见优化用例'
  }
  if (testcaseGenerationRunStore.generationInProgress) {
    return `当前用例生成正在后台执行（${testcaseGenerationRunStore.progressPercent}%）`
  }
  if (testcaseGenerationRunStore.hasRecoverableRun) {
    return '当前有可恢复的用例生成结果'
  }
  return ''
})
const assistantQuickEntries = computed(() => {
  return getAssistantQuickEntriesByIds(assistantQuickEntryIds.value)
    .filter((entry) => permissionStore.canAccess(entry.permissionKey))
})
let dragOffsetX = 0
let dragOffsetY = 0
let dragMoved = false
let dragStarted = false
let fabHoverTimer: ReturnType<typeof window.setTimeout> | null = null

const cleanAssistantMarkdown = (text: string) => {
  return text
    .replace(/\*\*(.*?)\*\*/g, '$1')
    .replace(/__(.*?)__/g, '$1')
}

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
  closeFabFanMenu()
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
      credentials: 'include',
      headers: { 
        'Content-Type': 'application/json',
        'Authorization': authStore.token
      },
      body: JSON.stringify({
        message: userText,
        session_id: assistantSessionId.value,
        user_id: authStore.user?.id || 'WEB_DASHBOARD_USER',
        mode: assistantMode.value
      })
    })
    const data = await res.json()
    const reply = data.reply || ''
    const toolResult = data.tool_result || ''

    // Add assistant's text response to chat
    assistantMessages.value.push({ role: 'assistant', text: cleanAssistantMarkdown(reply || 'AI 暂时无法响应') })

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

const toggleAssistantMode = () => {
  assistantMode.value = assistantMode.value === 'work' ? 'casual' : 'work'
}

const saveReport = async () => {
  reportSaving.value = true
  try {
    const res = await fetch(`${API_BASE}/acceptance-reports/save`, {
      method: 'POST',
      credentials: 'include',
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
      notifyAcceptanceReportsChanged()
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
      credentials: 'include',
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
  if (isComposingInput.value || e.isComposing) return
  if (e.shiftKey) return
  e.preventDefault()
  sendAssistantMessage()
}

const openCurrentRunningTask = () => {
  if (primaryRunningTask.value === 'testcase') {
    router.push('/testcase_gen/new')
    return
  }
  if (primaryRunningTask.value === 'drama') {
    router.push('/com_api_commit')
  }
}

const openAssistantQuickEntry = async (entry: AssistantQuickEntry) => {
  closeFabFanMenu()
  if (!permissionStore.canAccess(entry.permissionKey)) {
    ElMessage.warning('当前账号暂无该入口权限')
    return
  }
  if (!entry.path) return
  await router.push({
    path: entry.path,
    query: entry.query
  })
  assistantVisible.value = false
}

const clearFabHoverTimer = () => {
  if (fabHoverTimer) {
    window.clearTimeout(fabHoverTimer)
    fabHoverTimer = null
  }
}

const closeFabFanMenu = () => {
  clearFabHoverTimer()
  isFabFanMenuVisible.value = false
}

const startFabHover = () => {
  if (assistantVisible.value || isDraggingFab.value || dragStarted || assistantQuickEntries.value.length === 0) return
  clearFabHoverTimer()
  fabHoverTimer = window.setTimeout(() => {
    if (!assistantVisible.value && !isDraggingFab.value && !dragStarted && assistantQuickEntries.value.length > 0) {
      isFabFanMenuVisible.value = true
    }
    fabHoverTimer = null
  }, 1500)
}

const handleFabHoverAreaLeave = () => {
  closeFabFanMenu()
}

const handleFabFanBlankClick = () => {
  closeFabFanMenu()
}

const handleFabOutsidePointerDown = (event: PointerEvent) => {
  if (!isFabFanMenuVisible.value) return
  const target = event.target
  if (target instanceof Node && fabContainer.value?.contains(target)) return
  closeFabFanMenu()
}

const getFabFanEntryStyle = (index: number, total: number) => {
  const anglesByCount: Record<number, number[]> = {
    1: [135],
    2: [176, 78],
    3: [176, 127, 78],
    4: [176, 143, 111, 78]
  }
  const fallbackAngles = [176, 143, 111, 78]
  const angles = anglesByCount[Math.min(Math.max(total, 1), 4)] ?? fallbackAngles
  const angle = angles[index] ?? 135
  const radius = 168
  const radian = angle * Math.PI / 180

  return {
    '--fan-x': `${Math.cos(radian) * radius}px`,
    '--fan-y': `${-Math.sin(radian) * radius}px`,
    '--fan-delay': `${index * 45}ms`
  }
}

const syncAssistantQuickEntries = () => {
  assistantQuickEntryIds.value = getAssistantQuickEntryIds()
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
  closeFabFanMenu()
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
  window.addEventListener('storage', syncAssistantQuickEntries)
  window.addEventListener(ASSISTANT_QUICK_ENTRY_CHANGED_EVENT, syncAssistantQuickEntries)
  window.addEventListener('pointerdown', handleFabOutsidePointerDown)
})

onBeforeUnmount(() => {
  clearFabHoverTimer()
  window.removeEventListener('resize', syncFabPositionWithinViewport)
  window.removeEventListener('storage', syncAssistantQuickEntries)
  window.removeEventListener(ASSISTANT_QUICK_ENTRY_CHANGED_EVENT, syncAssistantQuickEntries)
  window.removeEventListener('pointerdown', handleFabOutsidePointerDown)
  window.removeEventListener('pointermove', handleFabPointerMove)
  window.removeEventListener('pointerup', stopFabDrag)
})
</script>

<template>
  <div class="global-assistant">
    <!-- FAB Button -->
    <div
      v-if="!assistantVisible"
      ref="fabContainer"
      class="fab-container"
      :class="{ 'is-dragging': isDraggingFab, 'is-test-active': assistantButtonActive, 'has-fan-menu': isFabFanMenuVisible }"
      :style="{ top: `${fabPosition.top}px`, left: `${fabPosition.left}px` }"
      @pointerenter="startFabHover"
      @pointerleave="handleFabHoverAreaLeave"
      @pointerdown.prevent="startFabDrag"
    >
      <transition name="assistant-bubble">
        <div v-if="dramaRunStore.bubbleVisible" class="fab-status-bubble">
          {{ primaryRunningTask === 'testcase' ? testcaseStatusBubbleText : dramaRunStore.bubbleText }}
        </div>
      </transition>
      <div
        v-if="assistantQuickEntries.length"
        class="fab-fan-menu"
        :class="{ 'is-visible': isFabFanMenuVisible }"
      >
        <div
          v-if="isFabFanMenuVisible"
          class="fab-fan-hit-area"
          @pointerdown.stop
          @click.stop="handleFabFanBlankClick"
        />
        <button
          v-for="(entry, index) in assistantQuickEntries"
          :key="entry.id"
          type="button"
          class="fab-fan-entry"
          :style="getFabFanEntryStyle(index, assistantQuickEntries.length)"
          :title="entry.name"
          @pointerdown.stop
          @click.stop="openAssistantQuickEntry(entry)"
        >
          <span class="fab-fan-entry__icon" :style="{ background: entry.iconBg, color: entry.iconColor }">
            <el-icon><component :is="Icons[entry.iconName as keyof typeof Icons]" /></el-icon>
          </span>
          <span class="fab-fan-entry__label">{{ entry.name }}</span>
        </button>
      </div>
      <el-button
        type="primary"
        size="large"
        class="fab-btn"
        :class="{ 'fab-btn--test-active': assistantButtonActive }"
      >
        <el-icon class="mr-2"><component :is="assistantButtonIcon" /></el-icon>
        {{ assistantButtonLabel }}
      </el-button>
    </div>

    <!-- Assistant Chat Window -->
    <div
      v-if="assistantVisible"
      class="assistant-shell-fixed"
      :class="{ 'is-minimized': showReportDialog }"
    >
      <button
        v-if="hasRunningTaskShortcut"
        type="button"
        class="running-task-float-btn"
        @click="openCurrentRunningTask"
      >
        <span class="running-task-float-btn__icon">
          <el-icon><component :is="runningTaskButtonIcon" /></el-icon>
        </span>
        <span class="running-task-float-btn__copy">
          <span class="running-task-float-btn__eyebrow">{{ primaryRunningTask === 'testcase' ? testcaseTaskPhaseLabel : '接口测试' }}</span>
          <span class="running-task-float-btn__label">{{ runningTaskButtonLabel }}</span>
        </span>
      </button>
      <div
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
          <button
            type="button"
            class="mode-toggle-btn"
            :class="{ 'is-casual': assistantMode === 'casual' }"
            :title="assistantModeTip"
            @click="toggleAssistantMode"
          >
            {{ assistantModeLabel }}
          </button>
          <el-input
            v-model="assistantInput"
            type="textarea"
            :autosize="{ minRows: 1, maxRows: 6 }"
            placeholder="输入指令 (Enter 发送, Shift+Enter 换行)..."
            @keydown.enter="handleEnter"
            @compositionstart="isComposingInput = true"
            @compositionend="isComposingInput = false"
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
      <div v-if="assistantQuickEntries.length" class="assistant-quick-entry-bar">
        <button
          v-for="entry in assistantQuickEntries"
          :key="entry.id"
          type="button"
          class="assistant-quick-entry-btn"
          @click="openAssistantQuickEntry(entry)"
        >
          <span class="assistant-quick-entry-btn__icon" :style="{ background: entry.iconBg, color: entry.iconColor }">
            <el-icon><component :is="Icons[entry.iconName as keyof typeof Icons]" /></el-icon>
          </span>
          <span class="assistant-quick-entry-btn__text">{{ entry.name }}</span>
        </button>
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

.fab-container.has-fan-menu {
  z-index: 3002;
}

.fab-fan-menu {
  position: absolute;
  left: 50%;
  top: 26px;
  width: 0;
  height: 0;
  pointer-events: none;
  z-index: 1;
}

.fab-fan-menu.is-visible {
  pointer-events: auto;
}

.fab-fan-hit-area {
  position: absolute;
  left: 0;
  top: 0;
  width: 440px;
  height: 440px;
  border-radius: 50%;
  background: transparent;
  cursor: default;
  transform: translate(-50%, -50%);
  z-index: 0;
}

.fab-fan-entry {
  position: absolute;
  left: 0;
  top: 0;
  z-index: 2;
  width: 72px;
  min-height: 72px;
  padding: 0;
  color: #334155;
  background: transparent;
  border: 0;
  display: inline-flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  gap: 5px;
  cursor: pointer;
  opacity: 0;
  visibility: hidden;
  transform: translate(-50%, -50%) translate(0, 0) scale(0.72);
  transition: opacity 0.18s ease, visibility 0.18s ease, color 0.18s ease, transform 0.22s cubic-bezier(0.34, 1.56, 0.64, 1);
  transition-delay: 0ms;
}

.fab-fan-menu.is-visible .fab-fan-entry {
  opacity: 1;
  visibility: visible;
  transform: translate(-50%, -50%) translate(var(--fan-x), var(--fan-y));
  transition-delay: var(--fan-delay);
}

.fab-fan-menu.is-visible .fab-fan-entry:hover {
  color: #1d4ed8;
  transform: translate(-50%, -50%) translate(var(--fan-x), var(--fan-y)) translateY(-2px);
}

.fab-fan-entry__icon {
  width: 52px;
  height: 52px;
  border: 1px solid rgba(203, 213, 225, 0.92);
  border-radius: 16px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  box-shadow: 0 12px 26px rgba(15, 23, 42, 0.15);
  backdrop-filter: blur(12px);
  transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.fab-fan-entry:hover .fab-fan-entry__icon {
  border-color: #93c5fd;
  box-shadow: 0 18px 36px rgba(37, 99, 235, 0.20);
  transform: translateY(-1px);
}

.fab-fan-entry__label {
  width: max-content;
  max-width: 86px;
  min-width: 0;
  color: #0f172a;
  font-size: 11px;
  font-weight: 760;
  line-height: 1.2;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  text-shadow: 0 1px 2px rgba(255, 255, 255, 0.92), 0 8px 18px rgba(15, 23, 42, 0.18);
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
  position: relative;
  z-index: 3;
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

.assistant-shell-fixed {
  position: fixed;
  bottom: 156px;
  right: 40px;
  width: 480px;
  max-width: 90vw;
  max-height: 85vh;
  z-index: 3001;
  overflow: visible;
}

.assistant-shell-fixed.is-minimized {
  pointer-events: none;
}

.assistant-window-fixed {
  position: relative;
  width: 480px;
  max-width: 90vw;
  height: 650px;
  max-height: 85vh;
  background: #fff;
  border-radius: 20px;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.2);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(226, 232, 240, 0.8);
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

.assistant-quick-entry-bar {
  position: absolute;
  top: calc(100% + 10px);
  left: 0;
  width: 100%;
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  gap: 8px;
}

.assistant-quick-entry-btn {
  flex: 1 1 0;
  min-width: 0;
  height: 38px;
  padding: 0 8px;
  color: #334155;
  background: rgba(255, 255, 255, 0.98);
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  cursor: pointer;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.10);
  transition: color 0.18s ease, border-color 0.18s ease, background 0.18s ease, transform 0.18s ease;
}

.assistant-quick-entry-btn:hover {
  color: #2563eb;
  border-color: #93c5fd;
  background: #eff6ff;
  transform: translateY(-1px);
}

.assistant-quick-entry-btn__icon {
  width: 22px;
  height: 22px;
  border-radius: 7px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
}

.assistant-quick-entry-btn__text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  font-weight: 800;
  line-height: 1;
}

.input-container {
  display: flex;
  align-items: flex-end;
  gap: 12px;
}

.running-task-float-btn {
  position: absolute;
  left: 0;
  top: calc(100% + 58px);
  z-index: 2;
  min-width: 128px;
  min-height: 52px;
  padding: 8px 12px;
  color: #1e293b;
  background: linear-gradient(180deg, rgba(255,255,255,0.98) 0%, #eef6ff 100%);
  border: 1px solid #d6e4f5;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
  cursor: pointer;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.10);
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease, background 0.2s ease;
}

.running-task-float-btn:hover {
  color: #1d4ed8;
  border-color: #93c5fd;
  background: linear-gradient(180deg, #ffffff 0%, #e0f2fe 100%);
  box-shadow: 0 16px 32px rgba(59, 130, 246, 0.16);
  transform: translateY(-1px);
}

.running-task-float-btn__icon {
  width: 30px;
  height: 30px;
  border-radius: 10px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #dbeafe 0%, #eff6ff 100%);
  color: #2563eb;
  font-size: 15px;
}

.running-task-float-btn__copy {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 0;
}

.running-task-float-btn__eyebrow {
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.04em;
  color: #64748b;
  text-transform: uppercase;
}

.running-task-float-btn__label {
  font-size: 13px;
  line-height: 1.2;
  font-weight: 800;
  color: #0f172a;
}

.mode-toggle-btn {
  flex-shrink: 0;
  height: 38px;
  min-width: 50px;
  margin-bottom: 3px;
  padding: 0 10px;
  color: #334155;
  background: #f8fafc;
  border: 1px solid #cbd5e1;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 800;
  cursor: pointer;
  transition: all 0.2s ease;
}

.mode-toggle-btn:hover {
  color: #4f46e5;
  border-color: #818cf8;
  background: #eef2ff;
}

.mode-toggle-btn.is-casual {
  color: #075985;
  border-color: #7dd3fc;
  background: linear-gradient(135deg, #ecfeff 0%, #eff6ff 100%);
}

.assistant-textarea :deep(.el-textarea__inner) {
  border-radius: 16px;
  min-height: 44px !important;
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
