<script setup lang="ts">
import DefectVersionSelect from "./DefectVersionSelect.vue"
import { computed, onBeforeUnmount, reactive, ref, watch } from "vue"
import { ElMessage, type UploadProps } from "element-plus"
import {
  ChatDotRound,
  Check,
  CircleCheck,
  Close,
  Document,
  EditPen,
  Paperclip,
  RefreshLeft
} from "@element-plus/icons-vue"
import type { DefectTransitionPayload } from "../api"
import { defectApi } from "../api"
import {
  defectStatusMeta,
  defectTypeOptions,
  priorityLabels,
  severityLabels,
  type DefectAttachment,
  type DefectDetail,
  type DefectMeta
} from "../types"

const props = defineProps<{
  modelValue: boolean
  detail: DefectDetail | null
  meta: DefectMeta | null
  loading: boolean
  acting: boolean
}>()

const emit = defineEmits<{
  "update:modelValue": [value: boolean]
  edit: []
  refresh: []
  transition: [payload: DefectTransitionPayload]
  comment: [content: string]
  upload: [file: File]
}>()

const commentText = ref("")
const transitionVisible = ref(false)
const transition = reactive<DefectTransitionPayload>({ action: "confirm", comment: "" })
const attachmentPreview = reactive({
  visible: false,
  loading: false,
  title: "",
  type: "file" as "image" | "text" | "pdf" | "file",
  source: "",
  content: "",
  mimeType: "",
  size: 0
})
let attachmentPreviewUrl = ""

const defect = computed(() => props.detail?.defect || null)
const canEdit = computed(() => defect.value?.allowed_actions?.edit === true)
const canProcess = computed(() => defect.value?.allowed_actions?.process === true)
const canVerify = computed(() => defect.value?.allowed_actions?.verify === true)
const canReopen = computed(() => defect.value?.allowed_actions?.reopen === true)
const canComment = computed(() => defect.value?.allowed_actions?.comment === true)
const defectTypeLabel = computed(() => defectTypeOptions.find((item) => item.value === defect.value?.defect_type)?.label || defect.value?.defect_type || "未设置")
const customFieldEntries = computed(() => {
  const values = defect.value?.custom_fields || {}
  return Object.entries(values).map(([key, value]) => ({
    key,
    value,
    definition: props.meta?.fields.find((field) => field.field_key === key && (!field.project_code || field.project_code === defect.value?.project_code))
  }))
})

const timeline = computed(() => {
  const histories = (props.detail?.history || []).map((item) => ({
    id: `history-${item.id}`,
    type: "history" as const,
    action: item.action,
    author: item.operator_name,
    content: item.comment,
    created_at: item.created_at
  }))
  const comments = (props.detail?.comments || []).map((item) => ({
    id: `comment-${item.id}`,
    type: "comment" as const,
    action: "comment",
    author: item.author_name,
    content: item.content,
    created_at: item.created_at
  }))
  return [...histories, ...comments].sort((a, b) => b.created_at.localeCompare(a.created_at))
})

const actionLabels: Record<string, string> = {
  create: "提交了缺陷",
  update: "更新了缺陷信息",
  confirm: "确认并开始处理",
  resolve: "标记为已解决，等待验证",
  close: "验证通过并关闭",
  reopen: "重新激活了缺陷",
  status_change: "直接调整了缺陷状态",
  comment: "添加了评论"
}

const transitionTitle = computed(() => ({
  confirm: "确认缺陷",
  resolve: "解决缺陷",
  close: "验证并关闭",
  reopen: "重新激活"
}[transition.action] || "状态操作"))

watch(() => props.modelValue, (visible) => {
  if (visible) commentText.value = ""
})

function formatTime(value?: string) {
  if (!value) return "—"
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("zh-CN", { hour12: false })
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

const imageExtensions = new Set(["png", "jpg", "jpeg", "gif", "webp"])
const textExtensions = new Set(["txt", "log", "json", "md", "csv", "xml", "yaml", "yml"])

function attachmentExtension(name: string) {
  return name.split(".").pop()?.toLowerCase() || ""
}

function isImageAttachment(item: DefectAttachment) {
  return item.mime_type.startsWith("image/") || imageExtensions.has(attachmentExtension(item.original_name))
}

function isTextAttachment(item: DefectAttachment) {
  return item.mime_type.startsWith("text/") || textExtensions.has(attachmentExtension(item.original_name))
}

function clearAttachmentPreview() {
  if (attachmentPreviewUrl) URL.revokeObjectURL(attachmentPreviewUrl)
  attachmentPreviewUrl = ""
  attachmentPreview.source = ""
  attachmentPreview.content = ""
}

async function previewAttachment(item: DefectAttachment) {
  if (!defect.value) return
  clearAttachmentPreview()
  attachmentPreview.visible = true
  attachmentPreview.loading = true
  attachmentPreview.title = item.original_name
  attachmentPreview.mimeType = item.mime_type
  attachmentPreview.size = item.size
  try {
    const blob = await defectApi.attachmentBlob(defect.value.id, item.id)
    if (isImageAttachment(item)) {
      attachmentPreview.type = "image"
      attachmentPreviewUrl = URL.createObjectURL(blob)
      attachmentPreview.source = attachmentPreviewUrl
    } else if (isTextAttachment(item)) {
      attachmentPreview.type = "text"
      const previewLimit = 2 * 1024 * 1024
      attachmentPreview.content = await blob.slice(0, previewLimit).text()
      if (blob.size > previewLimit) attachmentPreview.content += "\n\n……文件内容较大，仅预览前 2MB……"
    } else if (attachmentExtension(item.original_name) === "pdf") {
      attachmentPreview.type = "pdf"
      attachmentPreviewUrl = URL.createObjectURL(blob)
      attachmentPreview.source = attachmentPreviewUrl
    } else {
      attachmentPreview.type = "file"
    }
  } catch {
    attachmentPreview.visible = false
    ElMessage.error("附件内容加载失败")
  } finally {
    attachmentPreview.loading = false
  }
}

onBeforeUnmount(clearAttachmentPreview)

function customValueLabel(value: unknown, fieldType?: string) {
  if (fieldType === "user") {
    const account = props.meta?.accounts.find((item) => item.id === value)
    return account?.nickname || account?.username || String(value || "—")
  }
  if (typeof value === "boolean") return value ? "是" : "否"
  if (Array.isArray(value)) return value.join("、") || "—"
  return String(value ?? "—")
}

function openTransition(action: DefectTransitionPayload["action"]) {
  Object.assign(transition, {
    action,
    assignee_id: defect.value?.assignee_id || "",
    resolution: action === "resolve" ? "fixed" : "",
    resolved_version: "",
    comment: ""
  })
  transitionVisible.value = true
}

function submitTransition() {
  emit("transition", { ...transition })
}

function submitComment() {
  const content = commentText.value.trim()
  if (!content) return
  emit("comment", content)
  commentText.value = ""
}

const interceptUpload: UploadProps["beforeUpload"] = (file) => {
  emit("upload", file)
  return false
}

defineExpose({ closeTransition: () => { transitionVisible.value = false } })
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    size="min(880px, 96vw)"
    :show-close="false"
    class="defect-detail-drawer"
    @close="emit('update:modelValue', false)"
  >
    <template #header>
      <div class="detail-heading">
        <div class="detail-heading__main">
          <template v-if="defect">
            <div class="detail-heading__meta">
              <span class="defect-no-badge">{{ defect.defect_no }}</span>
              <span class="status-pill" :class="`is-${defectStatusMeta[defect.status].tone}`">
                {{ defectStatusMeta[defect.status].label }}
              </span>
            </div>
            <h2 class="defect-title">{{ defect.title }}</h2>
          </template>
          <template v-else>
            <div class="detail-heading__meta">
              <span class="defect-no-badge">DEFECT</span>
            </div>
            <h2 class="defect-title">缺陷详情</h2>
          </template>
        </div>
        <div class="detail-heading__actions">
          <button
            type="button"
            class="header-btn header-btn--refresh"
            :class="{ 'is-loading': loading }"
            title="刷新数据"
            aria-label="刷新"
            @click="emit('refresh')"
          >
            <el-icon><RefreshLeft /></el-icon>
          </button>
          <button
            v-if="canEdit"
            type="button"
            class="header-btn header-btn--edit"
            title="编辑缺陷"
            aria-label="编辑"
            @click="emit('edit')"
          >
            <el-icon><EditPen /></el-icon>
            <span>编辑</span>
          </button>
          <button
            type="button"
            class="header-btn header-btn--close"
            title="关闭 (Esc)"
            aria-label="关闭"
            @click="emit('update:modelValue', false)"
          >
            <el-icon><Close /></el-icon>
          </button>
        </div>
      </div>
    </template>

    <div v-loading="loading" class="detail-body">
      <template v-if="defect">
        <!-- 缺陷生命周期流转状态条 -->
        <section class="lifecycle-panel">
          <div class="lifecycle-track">
            <div
              v-for="(item, index) in ['new', 'active', 'resolved', 'closed']"
              :key="item"
              class="lifecycle-node"
              :class="{
                'is-reached': ['new', 'active', 'resolved', 'closed'].indexOf(defect.status) >= index,
                'is-current': defect.status === item
              }"
            >
              <div class="node-circle">
                <el-icon><Check /></el-icon>
              </div>
              <span class="node-label">{{ defectStatusMeta[item as keyof typeof defectStatusMeta].label }}</span>
              <div v-if="index < 3" class="node-line" />
            </div>
          </div>
          <div class="lifecycle-actions">
            <el-button v-if="defect.status === 'new' && canProcess" type="primary" @click="openTransition('confirm')">确认缺陷</el-button>
            <el-button v-if="['new', 'active'].includes(defect.status) && canProcess" type="success" plain @click="openTransition('resolve')">标记解决</el-button>
            <el-button v-if="defect.status === 'resolved' && canVerify" type="success" @click="openTransition('close')">验证通过</el-button>
            <el-button v-if="['resolved', 'closed'].includes(defect.status) && canReopen" type="warning" plain @click="openTransition('reopen')">重新激活</el-button>
          </div>
        </section>

        <!-- 基本信息卡片 -->
        <section class="detail-section">
          <div class="section-title">
            <span class="section-badge">基本</span>
            <h3>基本信息</h3>
          </div>
          <div class="info-grid">
            <div class="info-item">
              <span class="info-label">所属项目</span>
              <strong class="info-value" :title="`${defect.project_code} · ${defect.project_name}`">
                {{ defect.project_code }} · {{ defect.project_name }}
              </strong>
            </div>
            <div class="info-item">
              <span class="info-label">发现版本</span>
              <strong class="info-value">{{ defect.found_version || "未填写" }}</strong>
            </div>
            <div class="info-item">
              <span class="info-label">缺陷类型</span>
              <strong class="info-value">{{ defectTypeLabel }}</strong>
            </div>
            <div class="info-item">
              <span class="info-label">严重程度</span>
              <strong class="info-value severity-text" :class="`severity-${defect.severity}`">
                S{{ defect.severity }} · {{ severityLabels[defect.severity] }}
              </strong>
            </div>
            <div class="info-item">
              <span class="info-label">优先级</span>
              <strong class="info-value">P{{ defect.priority }} · {{ priorityLabels[defect.priority] }}</strong>
            </div>
            <div class="info-item">
              <span class="info-label">期望解决</span>
              <strong class="info-value">{{ defect.due_date || "未设置" }}</strong>
            </div>
            <div class="info-item">
              <span class="info-label">提交人</span>
              <strong class="info-value">{{ defect.reporter_name }}</strong>
            </div>
            <div class="info-item">
              <span class="info-label">处理人</span>
              <strong class="info-value">{{ defect.assignee_name || "暂未指派" }}</strong>
            </div>
            <div class="info-item">
              <span class="info-label">验证人</span>
              <strong class="info-value">{{ defect.verifier_name || "暂未指定" }}</strong>
            </div>
          </div>
          <div v-if="defect.tags?.length" class="tag-row">
            <el-tag v-for="tag in defect.tags" :key="tag" effect="plain" round class="custom-tag">{{ tag }}</el-tag>
          </div>
        </section>

        <!-- 问题现场卡片 -->
        <section class="detail-section issue-section">
          <div class="section-title">
            <span class="section-badge">现场</span>
            <h3>问题现场</h3>
          </div>
          <div v-if="defect.precondition" class="text-block">
            <span class="block-label">前置条件</span>
            <p>{{ defect.precondition }}</p>
          </div>
          <div class="text-block">
            <span class="block-label">重现步骤</span>
            <ol v-if="defect.steps?.length" class="step-ol">
              <li v-for="(step, idx) in defect.steps" :key="step.id" class="step-li">
                <span class="step-idx">{{ idx + 1 }}</span>
                <div class="step-text">{{ step.content }}</div>
              </li>
            </ol>
            <p v-else class="empty-text">未填写</p>
          </div>
          <div class="result-grid">
            <div class="result-card is-actual">
              <div class="result-header">
                <span class="result-tag actual-tag">实际结果</span>
              </div>
              <p>{{ defect.actual_result || "未填写" }}</p>
            </div>
            <div class="result-card is-expected">
              <div class="result-header">
                <span class="result-tag expected-tag">预期结果</span>
              </div>
              <p>{{ defect.expected_result || "未填写" }}</p>
            </div>
          </div>
          <div v-if="defect.description || detail?.attachments.length || canEdit" class="text-block supplement-detail-block">
            <div class="supplement-detail-heading">
              <span class="block-label">补充说明</span>
              <el-upload v-if="canEdit" :show-file-list="false" :before-upload="interceptUpload" :disabled="acting" multiple>
                <el-button size="small" :icon="Paperclip" plain :loading="acting">添加附件</el-button>
              </el-upload>
            </div>
            <p v-if="defect.description">{{ defect.description }}</p>
            <p v-else-if="!detail?.attachments.length" class="empty-text">暂无补充说明或附件</p>
            <div v-if="detail?.attachments.length" class="attachment-list supplement-attachment-list">
              <button
                v-for="item in detail.attachments"
                :key="item.id"
                type="button"
                class="attachment-card"
                :class="{ 'is-image': isImageAttachment(item) }"
                @click="previewAttachment(item)"
              >
                <img v-if="isImageAttachment(item)" :src="item.download_url" :alt="item.original_name" class="attachment-thumbnail" />
                <span v-else class="attachment-icon"><el-icon><Document /></el-icon></span>
                <div class="attachment-meta">
                  <strong class="attachment-name" :title="item.original_name">{{ item.original_name }}</strong>
                  <small class="attachment-sub">{{ formatSize(item.size) }} · {{ item.uploader_name }}</small>
                </div>
              </button>
            </div>
          </div>
        </section>

        <!-- 测试环境卡片 -->
        <section class="detail-section">
          <div class="section-title">
            <span class="section-badge">环境</span>
            <h3>测试环境</h3>
          </div>
          <div class="environment-row">
            <div v-for="(value, key) in defect.environment" v-show="value" :key="key" class="env-chip">
              <span class="env-label">{{ ({ platform: "平台", environment: "环境", app_version: "App版本", os_version: "系统", device_model: "设备", network: "网络" } as Record<string,string>)[key] }}</span>
              <span class="env-value">{{ value }}</span>
            </div>
            <span v-if="!Object.values(defect.environment || {}).some(Boolean)" class="env-empty">未记录环境信息</span>
          </div>
        </section>

        <!-- 自定义字段卡片 -->
        <section v-if="customFieldEntries.length" class="detail-section">
          <div class="section-title">
            <span class="section-badge">扩展</span>
            <h3>自定义字段</h3>
          </div>
          <div class="info-grid custom-field-grid">
            <div v-for="item in customFieldEntries" :key="item.key" class="info-item">
              <span class="info-label">{{ item.definition?.name || item.key }}</span>
              <strong class="info-value">{{ customValueLabel(item.value, item.definition?.field_type) }}</strong>
            </div>
          </div>
        </section>

        <!-- 活动记录 / 评论 -->
        <section class="detail-section activity-section">
          <div class="section-heading">
            <div class="section-title">
              <span class="section-badge">动态</span>
              <h3>活动记录</h3>
            </div>
            <span class="sub-hint">{{ timeline.length }} 条记录</span>
          </div>
          <div v-if="canComment" class="comment-box">
            <el-input v-model="commentText" type="textarea" :rows="3" maxlength="2000" show-word-limit placeholder="补充处理进展或验证信息..." />
            <el-button type="primary" :icon="ChatDotRound" :disabled="!commentText.trim()" :loading="acting" @click="submitComment">发表评论</el-button>
          </div>
          <el-timeline v-if="timeline.length" class="activity-timeline">
            <el-timeline-item v-for="item in timeline" :key="item.id" :timestamp="formatTime(item.created_at)" placement="top" :type="item.type === 'comment' ? 'primary' : 'success'" hollow>
              <div class="activity-card">
                <div class="activity-author">
                  <span class="author-name">{{ item.author }}</span>
                  <span class="action-tag">{{ actionLabels[item.action] || item.action }}</span>
                </div>
                <p v-if="item.content" class="activity-content">{{ item.content }}</p>
              </div>
            </el-timeline-item>
          </el-timeline>
        </section>
      </template>
    </div>

    <!-- 状态流转弹窗 -->
    <el-dialog v-model="transitionVisible" :title="transitionTitle" width="540px" append-to-body destroy-on-close class="defect-transition-dialog">
      <el-form label-position="top">
        <el-form-item v-if="transition.action === 'confirm'" label="处理人">
          <el-select v-model="transition.assignee_id" filterable clearable class="full-width" placeholder="选择处理人">
            <el-option v-for="account in meta?.accounts || []" :key="account.id" :label="account.nickname ? `${account.nickname} (${account.username})` : account.username" :value="account.id" />
          </el-select>
        </el-form-item>
        <template v-if="transition.action === 'resolve'">
          <el-form-item label="解决方案" required>
            <el-select v-model="transition.resolution" class="full-width">
              <el-option v-for="item in [{v:'fixed',l:'已修复'},{v:'duplicate',l:'重复缺陷'},{v:'by_design',l:'设计如此'},{v:'cannot_reproduce',l:'无法复现'},{v:'external',l:'外部原因'},{v:'postponed',l:'延期处理'},{v:'wont_fix',l:'不予修复'}]" :key="item.v" :label="item.l" :value="item.v" />
            </el-select>
          </el-form-item>
          <el-form-item label="解决版本">
            <DefectVersionSelect v-model="transition.resolved_version" :meta="meta" :projects="defect ? [defect.project_code] : []" />
          </el-form-item>
        </template>
        <el-form-item :label="transition.action === 'reopen' ? '重新激活原因' : '操作备注'" :required="transition.action === 'reopen'">
          <el-input v-model="transition.comment" type="textarea" :rows="4" maxlength="1000" show-word-limit :placeholder="transition.action === 'reopen' ? '说明验证失败或重新出现的现象' : '可填写处理说明'" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="transitionVisible=false">取消</el-button>
        <el-button type="primary" :icon="transition.action === 'reopen' ? RefreshLeft : CircleCheck" :loading="acting" :disabled="transition.action === 'resolve' && !transition.resolution || transition.action === 'reopen' && !transition.comment?.trim()" @click="submitTransition">确认操作</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="attachmentPreview.visible"
      :title="attachmentPreview.title"
      width="min(860px, 92vw)"
      append-to-body
      align-center
      destroy-on-close
      class="defect-detail-attachment-preview"
      @closed="clearAttachmentPreview"
    >
      <div v-loading="attachmentPreview.loading" class="detail-attachment-preview-body">
        <img v-if="attachmentPreview.type === 'image' && attachmentPreview.source" :src="attachmentPreview.source" :alt="attachmentPreview.title" class="detail-attachment-preview-image" />
        <pre v-else-if="attachmentPreview.type === 'text'" class="detail-attachment-preview-text">{{ attachmentPreview.content }}</pre>
        <iframe v-else-if="attachmentPreview.type === 'pdf' && attachmentPreview.source" :src="attachmentPreview.source" class="detail-attachment-preview-pdf" title="PDF 附件预览" />
        <div v-else-if="!attachmentPreview.loading" class="detail-attachment-preview-file">
          <span><el-icon><Document /></el-icon></span>
          <strong>{{ attachmentPreview.title }}</strong>
          <p>{{ attachmentPreview.mimeType || '未知文件类型' }} · {{ formatSize(attachmentPreview.size) }}</p>
          <small>该文件类型暂不支持直接展开内容，可在附件地址中下载查看。</small>
        </div>
      </div>
    </el-dialog>
  </el-drawer>
</template>

<style scoped>
/* 抽屉基底与头部规范 */
.defect-detail-drawer :deep(.el-drawer__header) {
  margin-bottom: 0;
  padding: 16px 20px 16px 24px;
  border-bottom: 1px solid #edf2f7;
  background: #ffffff;
}

.defect-detail-drawer :deep(.el-drawer__body) {
  padding: 20px 24px;
  background: #f8fafc;
}

.defect-detail-drawer :deep(.el-drawer__close-btn) {
  display: none !important;
}

.detail-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  width: 100%;
}

.detail-heading__main {
  min-width: 0;
  flex: 1;
}

.detail-heading__meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 8px;
}

.defect-no-badge {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  font-weight: 700;
  color: #3b82f6;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  padding: 3px 8px;
  border-radius: 6px;
  letter-spacing: 0.02em;
}

.defect-title {
  margin: 0;
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
  line-height: 1.4;
  word-break: break-word;
}

.detail-heading__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  margin-top: 2px;
}

.header-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  color: #475569;
  cursor: pointer;
  outline: none;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.header-btn:active {
  transform: scale(0.95);
}

.header-btn--refresh {
  width: 32px;
  padding: 0;
  font-size: 15px;
}

.header-btn--refresh .el-icon {
  display: inline-flex;
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.header-btn--refresh:hover {
  background: #f8fafc;
  border-color: #cbd5e1;
  color: #2563eb;
}

.header-btn--refresh:hover .el-icon {
  transform: rotate(-60deg);
}

.header-btn--refresh.is-loading .el-icon {
  animation: refresh-spin 0.8s linear infinite;
}

@keyframes refresh-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(-360deg);
  }
}

.header-btn--edit {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #2563eb;
  font-weight: 600;
}

.header-btn--edit:hover {
  background: #dbeafe;
  border-color: #93c5fd;
  color: #1d4ed8;
}

.header-btn--close {
  width: 32px;
  padding: 0;
  font-size: 15px;
  color: #64748b;
}

.header-btn--close:hover {
  background: #fee2e2;
  border-color: #fecaca;
  color: #ef4444;
  transform: scale(1.05);
}

.status-pill {
  padding: 4px 12px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.2;
}

.status-pill.is-slate {
  color: #475569;
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
}

.status-pill.is-blue {
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
}

.status-pill.is-amber {
  color: #b45309;
  background: #fffbeb;
  border: 1px solid #fde68a;
}

.status-pill.is-green {
  color: #15803d;
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
}

.detail-body {
  min-height: 320px;
  padding: 0 4px 30px;
}

.lifecycle-panel,
.detail-section {
  margin-bottom: 16px;
  padding: 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.lifecycle-panel {
  background: linear-gradient(135deg, #f8fafc 0%, #ffffff 100%);
}

.lifecycle-track {
  display: flex;
  align-items: center;
}

.lifecycle-node {
  display: flex;
  align-items: center;
  flex: 1;
}

.lifecycle-node:last-child {
  flex: 0;
}

.node-circle {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  border-radius: 50%;
  border: 2px solid #cbd5e1;
  background: #ffffff;
  color: transparent;
  font-size: 13px;
  transition: all 0.2s ease;
}

.node-label {
  margin: 0 8px;
  white-space: nowrap;
  font-size: 13px;
  font-weight: 600;
  color: #94a3b8;
  transition: color 0.2s ease;
}

.node-line {
  flex: 1;
  height: 2px;
  background: #e2e8f0;
  margin-right: 8px;
  transition: background 0.2s ease;
}

.lifecycle-node.is-reached .node-circle {
  border-color: #3b82f6;
  background: #3b82f6;
  color: #ffffff;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.25);
}

.lifecycle-node.is-reached .node-label {
  color: #334155;
}

.lifecycle-node.is-reached .node-line {
  background: #93c5fd;
}

.lifecycle-node.is-current .node-label {
  color: #2563eb;
  font-weight: 700;
}

.lifecycle-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px dashed #e2e8f0;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.section-badge {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 6px;
  background: #eff6ff;
  color: #2563eb;
  border: 1px solid #dbeafe;
}

.section-title h3 {
  margin: 0;
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.section-heading .section-title {
  margin-bottom: 0;
}

.sub-hint {
  color: #94a3b8;
  font-size: 12px;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border-radius: 9px;
  background: #f8fafc;
  border: 1px solid #eef2f6;
}

.info-label {
  color: #64748b;
  font-size: 12px;
  font-weight: 500;
}

.info-value {
  color: #1e293b;
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.severity-text.severity-1 {
  color: #dc2626;
}

.severity-text.severity-2 {
  color: #ea580c;
}

.tag-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 14px;
}

.custom-tag {
  font-size: 12px;
}

.issue-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.text-block {
  padding: 14px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #eef2f6;
}

.block-label {
  display: block;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  margin-bottom: 6px;
}

.text-block p {
  margin: 0;
  color: #334155;
  line-height: 1.7;
  white-space: pre-wrap;
  font-size: 13px;
}

.supplement-detail-block {
  border-color: #dbeafe;
  background: linear-gradient(135deg, #f8fafc 0%, #f5f9ff 100%);
}

.supplement-detail-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.supplement-detail-heading .block-label { margin-bottom: 0; }

.supplement-detail-heading .el-button {
  border-color: #bfdbfe;
  border-radius: 8px;
  color: #2563eb;
  background: #ffffff;
}

.step-ol {
  margin: 0;
  padding: 0;
  list-style: none;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.step-li {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.step-idx {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  flex: 0 0 22px;
  border-radius: 50%;
  background: #eff6ff;
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
}

.step-text {
  color: #334155;
  font-size: 13px;
  line-height: 1.6;
  padding-top: 1px;
}

.result-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.result-card {
  padding: 14px;
  border-radius: 10px;
  border: 1px solid;
}

.result-card.is-actual {
  border-color: #fecaca;
  background: #fff8f8;
}

.result-card.is-expected {
  border-color: #bbf7d0;
  background: #f4fdf7;
}

.result-header {
  margin-bottom: 8px;
}

.result-tag {
  font-size: 12px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 5px;
}

.actual-tag {
  background: #fee2e2;
  color: #dc2626;
  border: 1px solid #fca5a5;
}

.expected-tag {
  background: #dcfce7;
  color: #15803d;
  border: 1px solid #86efac;
}

.result-card p {
  margin: 0;
  color: #334155;
  line-height: 1.65;
  white-space: pre-wrap;
  font-size: 13px;
}

.empty-text {
  color: #94a3b8 !important;
}

.environment-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.env-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.env-label {
  color: #64748b;
  font-size: 12px;
  font-weight: 500;
}

.env-value {
  color: #1e293b;
  font-size: 13px;
  font-weight: 600;
}

.env-empty {
  color: #94a3b8;
  font-size: 13px;
}

.attachment-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.supplement-attachment-list {
  margin-top: 12px;
  margin-bottom: 0;
}

.attachment-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  width: 100%;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  color: inherit;
  text-decoration: none;
  background: #ffffff;
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: all 0.2s ease;
}

.attachment-thumbnail {
  display: block;
  width: 54px;
  height: 54px;
  flex: 0 0 54px;
  border-radius: 9px;
  object-fit: cover;
  background: #e2e8f0;
}

.attachment-card:hover {
  border-color: #93c5fd;
  background: #f8fbff;
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.08);
}

.attachment-icon {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  border-radius: 8px;
  color: #2563eb;
  background: #eff6ff;
  font-size: 18px;
}

.attachment-meta {
  min-width: 0;
  flex: 1;
}

.attachment-name,
.attachment-sub {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.defect-detail-attachment-preview) {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.22);
}

:global(.defect-detail-attachment-preview .el-dialog__header) {
  padding: 18px 22px 14px;
  border-bottom: 1px solid #edf2f7;
}

:global(.defect-detail-attachment-preview .el-dialog__body) { padding: 0; }

.detail-attachment-preview-body {
  display: grid;
  place-items: center;
  min-height: 240px;
  max-height: 76vh;
  overflow: auto;
  background: #f8fafc;
}

.detail-attachment-preview-image {
  display: block;
  max-width: 100%;
  max-height: 74vh;
  margin: auto;
  object-fit: contain;
}

.detail-attachment-preview-text {
  align-self: stretch;
  justify-self: stretch;
  min-height: 300px;
  margin: 0;
  padding: 22px 24px;
  overflow: auto;
  color: #1e293b;
  background: #f8fafc;
  font: 12px/1.65 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

.detail-attachment-preview-pdf {
  width: 100%;
  height: 72vh;
  border: 0;
  background: #ffffff;
}

.detail-attachment-preview-file {
  display: grid;
  justify-items: center;
  max-width: 480px;
  padding: 38px;
  text-align: center;
}

.detail-attachment-preview-file > span {
  display: grid;
  place-items: center;
  width: 68px;
  height: 68px;
  margin-bottom: 14px;
  border-radius: 18px;
  color: #2563eb;
  background: #dbeafe;
  font-size: 30px;
}

.detail-attachment-preview-file strong { color: #0f172a; font-size: 15px; }

.detail-attachment-preview-file p,
.detail-attachment-preview-file small {
  margin: 7px 0 0;
  color: #64748b;
  font-size: 12px;
}

.attachment-name {
  color: #1e293b;
  font-size: 13px;
  font-weight: 600;
}

.attachment-sub {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
}

.comment-box {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  margin-bottom: 24px;
}

.activity-timeline {
  padding-left: 4px;
}

.activity-card {
  padding: 2px 0 10px;
}

.activity-author {
  display: flex;
  gap: 8px;
  align-items: center;
}

.author-name {
  color: #1e293b;
  font-size: 13px;
  font-weight: 600;
}

.action-tag {
  color: #64748b;
  font-size: 12px;
  background: #f1f5f9;
  padding: 2px 8px;
  border-radius: 4px;
}

.activity-content {
  margin: 8px 0 0;
  padding: 10px 14px;
  border-radius: 9px;
  background: #f8fafc;
  border: 1px solid #eef2f6;
  color: #334155;
  line-height: 1.65;
  white-space: pre-wrap;
  font-size: 13px;
}

.full-width {
  width: 100%;
}

@media (max-width: 760px) {
  .info-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .result-grid,
  .attachment-list {
    grid-template-columns: 1fr;
  }
  .node-label {
    display: none;
  }
}
</style>
