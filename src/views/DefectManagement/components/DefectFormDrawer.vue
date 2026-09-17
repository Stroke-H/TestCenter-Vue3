<script setup lang="ts">
import DefectVersionSelect from "./DefectVersionSelect.vue"
import { computed, nextTick, onBeforeUnmount, reactive, ref, watch } from "vue"
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from "element-plus"
import {
  CirclePlus,
  Close,
  Delete,
  Document,
  DocumentChecked,
  Paperclip,
  Plus,
  WarningFilled,
  CircleCheckFilled
} from "@element-plus/icons-vue"
import { v4 as uuidv4 } from "uuid"
import { useAuthStore } from "@/stores/auth"
import { defectApi } from "../api"
import {
  defectToForm,
  defectStatusMeta,
  defectTypeOptions,
  emptyDefectForm,
  priorityLabels,
  severityLabels,
  type Defect,
  type DefectAttachment,
  type DefectFormValue,
  type DefectMeta,
  type DefectStatus
} from "../types"

const props = defineProps<{
  modelValue: boolean
  defect: Defect | null
  attachments?: DefectAttachment[]
  meta: DefectMeta | null
  saving: boolean
  statusChanging?: boolean
  defaultProjectCode?: string
}>()

const emit = defineEmits<{
  "update:modelValue": [value: boolean]
  save: [value: DefectFormValue, titles: string[], files: File[], removedAttachmentIds: string[]]
  "status-change": [status: DefectStatus]
}>()

const authStore = useAuthStore()
const formRef = ref<FormInstance>()
const form = ref<DefectFormValue>(emptyDefectForm())
const additionalTitles = ref<string[]>([])
interface PendingAttachment {
  id: string
  file: File
  objectUrl: string
}
const pendingAttachments = ref<PendingAttachment[]>([])
const removedAttachmentIds = ref<string[]>([])
const fileInputRef = ref<HTMLInputElement>()
const preview = reactive({
  visible: false,
  loading: false,
  title: "",
  type: "file" as "image" | "text" | "pdf" | "file",
  source: "",
  content: "",
  mimeType: "",
  size: 0
})
let ownedPreviewUrl = ""
const isEdit = computed(() => Boolean(props.defect))
const statusOptions: DefectStatus[] = ["new", "active", "resolved", "closed"]
const allowedProject = (code: string) => props.defect?.project_code === code
  ? props.defect.allowed_actions?.edit === true
  : props.meta?.project_actions?.[code]?.create === true
const canSave = computed(() => (!props.defect || props.defect.allowed_actions?.edit === true) && allowedProject(form.value.project_code))
const applicableFields = computed(() => (props.meta?.fields || []).filter((field) => (
  field.enabled && (!field.project_code || field.project_code === form.value.project_code)
)).sort((a, b) => a.sort_order - b.sort_order))
const numberedTitlePrefix = /^\s*\d+\s*[.．、)）]\s*/

function parseTitleBlock(value: string) {
  const lines = value.split(/\r?\n/).map((line) => line.trim()).filter(Boolean)
  if (lines.length <= 1) return lines
  return lines.map((line) => line.replace(numberedTitlePrefix, "").trim()).filter(Boolean)
}

const parsedTitles = computed(() => [form.value.title, ...additionalTitles.value]
  .flatMap(parseTitleBlock)
  .filter(Boolean))
const batchTitleCount = computed(() => parsedTitles.value.length)
const visibleAttachments = computed(() => (props.attachments || []).filter((item) => !removedAttachmentIds.value.includes(item.id)))
const imageExtensions = new Set(["png", "jpg", "jpeg", "gif", "webp"])
const textExtensions = new Set(["txt", "log", "json", "md", "csv", "xml", "yaml", "yml"])
const allowedAttachmentExtensions = new Set([...imageExtensions, ...textExtensions, "zip", "pdf", "mp4", "mov"])

function attachmentExtension(name: string) {
  return name.split(".").pop()?.toLowerCase() || ""
}

function isImageAttachment(name: string, mimeType = "") {
  return mimeType.startsWith("image/") || imageExtensions.has(attachmentExtension(name))
}

function isTextAttachment(name: string, mimeType = "") {
  return mimeType.startsWith("text/") || textExtensions.has(attachmentExtension(name))
}

function formatAttachmentSize(size: number) {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function revokePendingAttachments() {
  pendingAttachments.value.forEach((item) => URL.revokeObjectURL(item.objectUrl))
  pendingAttachments.value = []
}

function closeAttachmentPreview() {
  if (ownedPreviewUrl) URL.revokeObjectURL(ownedPreviewUrl)
  ownedPreviewUrl = ""
  preview.source = ""
  preview.content = ""
}

function localToday() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, "0")
  const day = String(now.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

function currentUserID() {
  const user = authStore.user
  if (!user) return ""
  return props.meta?.accounts.find((account) => account.id === user.id)?.id
    || props.meta?.accounts.find((account) => account.username.toLowerCase() === user.username.toLowerCase())?.id
    || user.id
}

function projectPlatform(projectCode: string) {
  const project = props.meta?.projects.find((item) => item.project_code === projectCode)
  const source = `${project?.project_name || ""} ${project?.short_code || ""}`.toLowerCase()
  if (source.includes("ttmins") || source.includes("tt mins")) return "TTmins"
  if (/\bios\b|iphone|ipad|苹果/.test(source)) return "iOS"
  if (/android|安卓/.test(source)) return "Android"
  if (/\bweb\b|\bh5\b|网页/.test(source)) return "Web"
  if (/server|backend|\bapi\b|服务端|后端/.test(source)) return "服务端"
  return ""
}

function projectDeviceNames(projectCode: string) {
  const names = (props.meta?.devices || [])
    .filter((device) => device.allowed_app.split(",").some((code) => code.trim() === projectCode))
    .map((device) => device.device_name.trim() || device.model.trim())
    .filter(Boolean)
  return [...new Set(names)].join("、")
}

function applyFoundVersionDefaults() {
  const version = form.value.found_version.trim()
  const stages = props.meta?.project_version_stages?.[form.value.project_code]
  form.value.environment.app_version = version
  form.value.environment.environment = version && stages?.online === version && stages.testing !== version
    ? "正式环境"
    : "测试环境"
}

function applyProjectDefaults(projectCode: string) {
  const stages = props.meta?.project_version_stages?.[projectCode]
  form.value.found_version = stages?.testing || stages?.online || ""
  form.value.environment.platform = projectPlatform(projectCode)
  form.value.environment.device_model = projectDeviceNames(projectCode)
  form.value.environment.network = "WiFi"
  applyFoundVersionDefaults()
}

function applyNewDefectDefaults() {
  form.value.defect_type = "function"
  form.value.due_date = localToday()
  form.value.verifier_id = currentUserID()
  form.value.environment.environment = "测试环境"
  form.value.environment.network = "WiFi"
}

const rules: FormRules<DefectFormValue> = {
  title: [
    { required: true, message: "请输入缺陷标题", trigger: "blur" }
  ],
  project_code: [{ required: true, message: "请选择所属项目", trigger: "change" }],
  defect_type: [{ required: true, message: "请选择缺陷类型", trigger: "change" }]
}

watch(
  () => props.modelValue,
  async (visible) => {
    if (!visible) {
      revokePendingAttachments()
      closeAttachmentPreview()
      return
    }
    revokePendingAttachments()
    form.value = props.defect ? defectToForm(props.defect) : emptyDefectForm()
    additionalTitles.value = []
    removedAttachmentIds.value = []
    if (!props.defect) {
      applyNewDefectDefaults()
      if (props.defaultProjectCode) {
        form.value.project_code = props.defaultProjectCode
        applyProjectDefaults(props.defaultProjectCode)
      }
    }
    await nextTick()
    formRef.value?.clearValidate()
  }
)

watch(
  () => props.defect?.row_version,
  (rowVersion) => {
    if (props.modelValue && props.defect && rowVersion !== undefined) {
      form.value.row_version = rowVersion
    }
  }
)

function close() {
  revokePendingAttachments()
  closeAttachmentPreview()
  emit("update:modelValue", false)
}

function canChangeToStatus(status: DefectStatus) {
  const defect = props.defect
  if (!defect || defect.status === status) return false
  if (status === "closed") return defect.allowed_actions?.verify === true
  if (status === "new") return defect.allowed_actions?.reopen === true
  if (status === "active" && (defect.status === "resolved" || defect.status === "closed")) {
    return defect.allowed_actions?.reopen === true
  }
  return defect.allowed_actions?.process === true
}

async function requestStatusChange(status: DefectStatus) {
  if (!props.defect || !canChangeToStatus(status) || props.statusChanging) return
  const currentLabel = defectStatusMeta[props.defect.status].label
  const targetLabel = defectStatusMeta[status].label
  try {
    await ElMessageBox.confirm(
      `确认将缺陷 ${props.defect.defect_no} 从“${currentLabel}”直接修改为“${targetLabel}”吗？`,
      "确认修改缺陷状态",
      {
        confirmButtonText: `修改为${targetLabel}`,
        cancelButtonText: "取消",
        type: status === "closed" ? "warning" : "info",
        customClass: "defect-status-confirm"
      }
    )
    emit("status-change", status)
  } catch {
    // 用户取消确认时保持当前状态。
  }
}

function addStep() {
  form.value.steps.push({ id: uuidv4(), content: "" })
}

function addTitleInput() {
  if (batchTitleCount.value >= 50 || additionalTitles.value.length >= 49) {
    ElMessage.warning("单次最多批量提交 50 个缺陷")
    return
  }
  additionalTitles.value.push("")
}

function removeTitleInput(index: number) {
  additionalTitles.value.splice(index, 1)
}

function normalizeClipboardFile(file: File, index: number) {
  if (attachmentExtension(file.name)) return file
  const extension = file.type.startsWith("image/") ? (file.type.split("/")[1] || "png") : "txt"
  return new File([file], `clipboard-${Date.now()}-${index + 1}.${extension}`, { type: file.type })
}

function addAttachmentFiles(files: File[]) {
  files.forEach((original, index) => {
    const file = normalizeClipboardFile(original, index)
    if (!allowedAttachmentExtensions.has(attachmentExtension(file.name))) {
      ElMessage.warning(`不支持附件“${file.name}”的文件类型`)
      return
    }
    if (file.size <= 0 || file.size > 20 * 1024 * 1024) {
      ElMessage.warning(`附件“${file.name}”必须小于 20MB`)
      return
    }
    pendingAttachments.value.push({ id: uuidv4(), file, objectUrl: URL.createObjectURL(file) })
  })
}

function handleAttachmentPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || [])
  if (!files.length) return
  event.preventDefault()
  addAttachmentFiles(files)
}

function chooseAttachmentFiles() {
  fileInputRef.value?.click()
}

function handleAttachmentSelection(event: Event) {
  const input = event.target as HTMLInputElement
  addAttachmentFiles(Array.from(input.files || []))
  input.value = ""
}

function removePendingAttachment(id: string) {
  const index = pendingAttachments.value.findIndex((item) => item.id === id)
  if (index < 0) return
  URL.revokeObjectURL(pendingAttachments.value[index]!.objectUrl)
  pendingAttachments.value.splice(index, 1)
}

function removeExistingAttachment(id: string) {
  if (!removedAttachmentIds.value.includes(id)) removedAttachmentIds.value.push(id)
  ElMessage.info("附件将在保存修改后删除")
}

async function showAttachmentPreview(options: { name: string; mimeType: string; size: number; blob: Blob; source?: string }) {
  closeAttachmentPreview()
  preview.visible = true
  preview.loading = true
  preview.title = options.name
  preview.mimeType = options.mimeType
  preview.size = options.size
  try {
    if (isImageAttachment(options.name, options.mimeType)) {
      preview.type = "image"
      preview.source = options.source || URL.createObjectURL(options.blob)
      if (!options.source) ownedPreviewUrl = preview.source
    } else if (isTextAttachment(options.name, options.mimeType)) {
      preview.type = "text"
      const previewLimit = 2 * 1024 * 1024
      preview.content = await options.blob.slice(0, previewLimit).text()
      if (options.blob.size > previewLimit) preview.content += "\n\n……文件内容较大，仅预览前 2MB……"
    } else if (attachmentExtension(options.name) === "pdf") {
      preview.type = "pdf"
      preview.source = URL.createObjectURL(options.blob)
      ownedPreviewUrl = preview.source
    } else {
      preview.type = "file"
    }
  } finally {
    preview.loading = false
  }
}

async function previewPendingAttachment(item: PendingAttachment) {
  await showAttachmentPreview({
    name: item.file.name,
    mimeType: item.file.type,
    size: item.file.size,
    blob: item.file,
    source: isImageAttachment(item.file.name, item.file.type) ? item.objectUrl : undefined
  })
}

async function previewExistingAttachment(item: DefectAttachment) {
  if (!props.defect) return
  preview.visible = true
  preview.loading = true
  preview.title = item.original_name
  try {
    const blob = await defectApi.attachmentBlob(props.defect.id, item.id)
    await showAttachmentPreview({ name: item.original_name, mimeType: item.mime_type, size: item.size, blob })
  } catch {
    preview.visible = false
    ElMessage.error("附件内容加载失败")
  }
}

onBeforeUnmount(() => {
  revokePendingAttachments()
  closeAttachmentPreview()
})

function removeStep(index: number) {
  if (form.value.steps.length === 1) {
    const firstStep = form.value.steps[0]
    if (firstStep) firstStep.content = ""
    return
  }
  form.value.steps.splice(index, 1)
}

async function submit() {
  if (!canSave.value) return
  const titles = isEdit.value ? [form.value.title.trim()] : parsedTitles.value
  if (!titles.length) {
    ElMessage.warning("请至少填写一个缺陷标题")
    return
  }
  if (titles.length > 50) {
    ElMessage.warning("单次最多批量提交 50 个缺陷")
    return
  }
  const oversizedIndex = titles.findIndex((title) => title.length > 255)
  if (oversizedIndex >= 0) {
    ElMessage.warning(`第 ${oversizedIndex + 1} 个缺陷标题超过 255 个字符`)
    return
  }
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  const missingField = applicableFields.value.find((field) => {
    if (!field.required) return false
    const value = form.value.custom_fields[field.field_key]
    return value === undefined || value === null || value === "" || (Array.isArray(value) && value.length === 0)
  })
  if (missingField) {
    ElMessage.warning(`请填写自定义字段“${missingField.name}”`)
    return
  }
  const customFields = Object.fromEntries(applicableFields.value
    .filter((field) => form.value.custom_fields[field.field_key] !== undefined)
    .map((field) => [field.field_key, form.value.custom_fields[field.field_key]]))
  emit("save", {
    ...form.value,
    title: titles[0] || "",
    steps: form.value.steps.map((item) => ({ ...item })),
    environment: { ...form.value.environment },
    tags: [...form.value.tags],
    custom_fields: customFields
  }, titles, pendingAttachments.value.map((item) => item.file), [...removedAttachmentIds.value])
}
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    size="min(880px, 96vw)"
    :close-on-click-modal="false"
    :show-close="false"
    class="defect-form-drawer"
    @close="close"
  >
    <template #header>
      <div class="drawer-heading">
        <div class="drawer-heading__icon">
          <el-icon><DocumentChecked /></el-icon>
        </div>
        <div class="drawer-heading__content">
          <div class="drawer-heading__title-row">
            <strong>{{ isEdit ? "编辑缺陷信息" : "新建缺陷记录" }}</strong>
            <span class="drawer-heading__tag">{{ isEdit ? defect?.defect_no : "创建中" }}</span>
          </div>
          <p>{{ isEdit ? "修改缺陷的关键属性、重现现场或责任指派" : "完整详细地记录问题现场，便于测试与开发快速定位并验证" }}</p>
        </div>
        <button
          type="button"
          class="drawer-close-btn"
          title="关闭 (Esc)"
          aria-label="关闭"
          @click="close"
        >
          <el-icon><Close /></el-icon>
        </button>
      </div>
    </template>

    <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="defect-form">
      <section v-if="isEdit && defect" class="status-direct-panel">
        <div class="status-direct-panel__copy">
          <span class="status-direct-panel__eyebrow">缺陷生命周期</span>
          <strong>点击状态可直接调整</strong>
          <p>状态修改将在确认后立即生效，并自动记录到缺陷操作历史。</p>
        </div>
        <div class="status-direct-track" :class="{ 'is-loading': statusChanging }">
          <template v-for="(status, index) in statusOptions" :key="status">
            <button
              type="button"
              class="status-direct-node"
              :class="[
                `is-${defectStatusMeta[status].tone}`,
                { 'is-current': defect.status === status, 'is-disabled': defect.status !== status && !canChangeToStatus(status) }
              ]"
              :disabled="statusChanging || defect.status === status || !canChangeToStatus(status)"
              :title="defect.status === status ? '当前状态' : (canChangeToStatus(status) ? `修改为${defectStatusMeta[status].label}` : '当前账号无权切换到此状态')"
              @click="requestStatusChange(status)"
            >
              <span class="status-direct-node__dot">{{ index + 1 }}</span>
              <span class="status-direct-node__label">{{ defectStatusMeta[status].label }}</span>
              <small>{{ defect.status === status ? "当前" : "点击切换" }}</small>
            </button>
            <span v-if="index < statusOptions.length - 1" class="status-direct-track__line" />
          </template>
        </div>
      </section>

      <!-- 01 基本属性 -->
      <section class="form-section form-section--primary">
        <div class="form-section__header">
          <div class="form-section__badge">
            <span>01</span>
          </div>
          <div class="form-section__text">
            <strong>基本归属与等级</strong>
            <p>明确缺陷所属项目、类型与严重程度评定</p>
          </div>
        </div>

        <el-form-item label="缺陷标题" prop="title" class="title-item">
          <div class="title-editor">
            <div class="title-list">
              <div class="title-row" :class="{ 'has-index': !isEdit && additionalTitles.length > 0 }">
                <span v-if="!isEdit && additionalTitles.length > 0" class="title-row__badge">1</span>
                <el-input
                  v-model="form.title"
                  :type="isEdit ? 'text' : 'textarea'"
                  :autosize="isEdit ? undefined : { minRows: 1, maxRows: 6 }"
                  :maxlength="isEdit ? 255 : 13000"
                  :resize="isEdit ? undefined : 'none'"
                  size="large"
                  :placeholder="!isEdit && additionalTitles.length > 0 ? '用一句话清晰概述缺陷现象 1（也可粘贴多行编号列表）' : '用一句话清晰概述缺陷现象（也可以粘贴多行编号列表）'"
                  class="title-input"
                />
                <button
                  v-if="!isEdit"
                  type="button"
                  class="title-action-btn title-action-btn--add"
                  title="添加一个缺陷标题"
                  aria-label="添加一个缺陷标题"
                  @click="addTitleInput"
                >
                  <el-icon><Plus /></el-icon>
                </button>
              </div>

              <div
                v-for="(_, index) in additionalTitles"
                :key="index"
                class="title-row has-index"
              >
                <span class="title-row__badge">{{ index + 2 }}</span>
                <el-input
                  v-model="additionalTitles[index]"
                  size="large"
                  maxlength="255"
                  :placeholder="`用一句话清晰概述缺陷现象 ${index + 2}`"
                  class="title-input"
                />
                <button
                  type="button"
                  class="title-action-btn title-action-btn--delete"
                  title="删除此缺陷标题"
                  aria-label="删除此缺陷标题"
                  @click="removeTitleInput(index)"
                >
                  <el-icon><Delete /></el-icon>
                </button>
              </div>
            </div>

            <div v-if="!isEdit" class="title-editor__hint">
              <span>支持逐条添加，也可粘贴按行编号的标题列表</span>
              <strong v-if="batchTitleCount > 1">已识别 {{ batchTitleCount }} 个缺陷</strong>
            </div>
          </div>
        </el-form-item>

        <div class="form-grid form-grid--three">
          <el-form-item label="所属项目" prop="project_code">
            <el-select
              v-model="form.project_code"
              filterable
              placeholder="选择归属项目"
              class="full-width"
              :disabled="!isEdit && Boolean(defaultProjectCode)"
              @change="applyProjectDefaults"
            >
              <el-option
                v-for="project in meta?.projects || []"
                :key="project.id"
                :label="`${project.project_code} · ${project.project_name}`"
                :value="project.project_code"
                :disabled="!allowedProject(project.project_code)"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="发现版本">
            <DefectVersionSelect
              v-model="form.found_version"
              edit-testing
              :meta="meta"
              :projects="form.project_code ? [form.project_code] : []"
              :historical="defect?.project_code === form.project_code ? defect?.found_version : undefined"
              @change="applyFoundVersionDefaults"
            />
          </el-form-item>

          <el-form-item label="缺陷类型" prop="defect_type">
            <el-select v-model="form.defect_type" class="full-width">
              <el-option v-for="item in defectTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
        </div>

        <div class="form-grid form-grid--three">
          <el-form-item label="严重程度">
            <el-select v-model="form.severity" class="full-width">
              <el-option
                v-for="(label, value) in severityLabels"
                :key="value"
                :label="`S${value} · ${label}`"
                :value="Number(value)"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="优先级">
            <el-select v-model="form.priority" class="full-width">
              <el-option
                v-for="(label, value) in priorityLabels"
                :key="value"
                :label="`P${value} · ${label}`"
                :value="Number(value)"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="期望解决日期">
            <el-date-picker
              v-model="form.due_date"
              type="date"
              value-format="YYYY-MM-DD"
              placeholder="选择期望解决日期"
              class="full-width"
            />
          </el-form-item>
        </div>

        <div class="form-grid form-grid--two">
          <el-form-item label="处理人">
            <el-select v-model="form.assignee_id" filterable clearable placeholder="指派处理人（可留空后续分配）" class="full-width">
              <el-option
                v-for="account in meta?.accounts || []"
                :key="account.id"
                :label="account.nickname ? `${account.nickname} (${account.username})` : account.username"
                :value="account.id"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="验证人">
            <el-select v-model="form.verifier_id" filterable clearable placeholder="指派验证人（解决后指定亦可）" class="full-width">
              <el-option
                v-for="account in meta?.accounts || []"
                :key="account.id"
                :label="account.nickname ? `${account.nickname} (${account.username})` : account.username"
                :value="account.id"
              />
            </el-select>
          </el-form-item>
        </div>

        <el-form-item label="标签标识" class="tag-form-item">
          <el-select
            v-model="form.tags"
            multiple
            filterable
            allow-create
            default-first-option
            placeholder="输入标签名称后按回车新增标签（如：高频、线上客诉、回归）"
            class="full-width"
          />
        </el-form-item>
      </section>

      <!-- 02 问题现场 -->
      <section class="form-section form-section--issue">
        <div class="form-section__header">
          <div class="form-section__badge is-issue">
            <span>02</span>
          </div>
          <div class="form-section__text">
            <strong>问题现场还原</strong>
            <p>详述重现步骤与关键对比，提供充足复现线索</p>
          </div>
        </div>

        <el-form-item label="前置条件">
          <el-input
            v-model="form.precondition"
            type="textarea"
            :rows="2"
            placeholder="例如：特定账号权限、初始配置、依赖数据状态或前置准备..."
            class="soft-textarea"
          />
        </el-form-item>

        <!-- 重现步骤卡片列表 -->
        <el-form-item label="重现步骤" class="steps-form-item">
          <div class="step-card-container">
            <div
              v-for="(step, index) in form.steps"
              :key="step.id"
              class="step-card"
            >
              <div class="step-card__index">{{ index + 1 }}</div>
              <div class="step-card__input-wrap">
                <el-input
                  v-model="step.content"
                  type="textarea"
                  :autosize="{ minRows: 1, maxRows: 5 }"
                  :placeholder="`第 ${index + 1} 步操作及页面路径...`"
                  class="step-input"
                />
              </div>
              <el-button
                text
                :icon="Delete"
                class="step-card__del-btn"
                title="删除此步骤"
                @click="removeStep(index)"
              />
            </div>
            <el-button
              class="step-add-btn"
              :icon="CirclePlus"
              @click="addStep"
            >
              添加操作步骤
            </el-button>
          </div>
        </el-form-item>

        <!-- 实际结果 vs 预期结果 双栏对比卡片 -->
        <div class="result-split-grid">
          <div class="result-card-box result-card-box--actual">
            <div class="result-card-box__title">
              <span class="result-badge is-actual">
                <el-icon><WarningFilled /></el-icon>
                实际结果 (异常表现)
              </span>
            </div>
            <el-input
              v-model="form.actual_result"
              type="textarea"
              :rows="4"
              placeholder="实际发生了什么异常现象？错误提示、页面崩溃或计算错误详情..."
              class="result-textarea is-actual"
            />
          </div>

          <div class="result-card-box result-card-box--expected">
            <div class="result-card-box__title">
              <span class="result-badge is-expected">
                <el-icon><CircleCheckFilled /></el-icon>
                预期结果 (正确行为)
              </span>
            </div>
            <el-input
              v-model="form.expected_result"
              type="textarea"
              :rows="4"
              placeholder="按照需求设计，正常且符合预期的正确操作与业务流程表现应该是什么..."
              class="result-textarea is-expected"
            />
          </div>
        </div>

        <el-form-item label="补充说明" class="desc-item">
          <div class="supplement-editor" @paste="handleAttachmentPaste">
            <el-input
              v-model="form.description"
              type="textarea"
              :rows="3"
              placeholder="发生频率、影响范围或日志补充；可直接按 Command/Ctrl + V 粘贴图片或文件..."
              class="soft-textarea"
            />
            <div class="supplement-toolbar">
              <span><el-icon><Paperclip /></el-icon> 支持粘贴图片、TXT、日志、JSON、PDF 等文件，单个不超过 20MB</span>
              <el-button plain :icon="Paperclip" @click="chooseAttachmentFiles">选择文件</el-button>
              <input
                ref="fileInputRef"
                class="attachment-file-input"
                type="file"
                multiple
                accept=".png,.jpg,.jpeg,.gif,.webp,.txt,.log,.json,.md,.csv,.xml,.yaml,.yml,.pdf,.zip,.mp4,.mov"
                @change="handleAttachmentSelection"
              />
            </div>

            <div v-if="visibleAttachments.length || pendingAttachments.length" class="supplement-attachments">
              <article
                v-for="item in visibleAttachments"
                :key="item.id"
                class="supplement-attachment"
                :class="{ 'is-image': isImageAttachment(item.original_name, item.mime_type) }"
                @click="previewExistingAttachment(item)"
              >
                <img
                  v-if="isImageAttachment(item.original_name, item.mime_type)"
                  :src="item.download_url"
                  :alt="item.original_name"
                />
                <span v-else class="supplement-attachment__icon"><el-icon><Document /></el-icon></span>
                <div class="supplement-attachment__meta">
                  <strong :title="item.original_name">{{ item.original_name }}</strong>
                  <small>{{ formatAttachmentSize(item.size) }} · 已上传</small>
                </div>
                <el-button
                  text
                  :icon="Delete"
                  class="supplement-attachment__delete"
                  title="删除附件"
                  @click.stop="removeExistingAttachment(item.id)"
                />
              </article>

              <article
                v-for="item in pendingAttachments"
                :key="item.id"
                class="supplement-attachment is-pending"
                :class="{ 'is-image': isImageAttachment(item.file.name, item.file.type) }"
                @click="previewPendingAttachment(item)"
              >
                <img
                  v-if="isImageAttachment(item.file.name, item.file.type)"
                  :src="item.objectUrl"
                  :alt="item.file.name"
                />
                <span v-else class="supplement-attachment__icon"><el-icon><Document /></el-icon></span>
                <div class="supplement-attachment__meta">
                  <strong :title="item.file.name">{{ item.file.name }}</strong>
                  <small>{{ formatAttachmentSize(item.file.size) }} · 待上传</small>
                </div>
                <el-button
                  text
                  :icon="Delete"
                  class="supplement-attachment__delete"
                  title="移除附件"
                  @click.stop="removePendingAttachment(item.id)"
                />
              </article>
            </div>
          </div>
        </el-form-item>
      </section>

      <!-- 03 环境信息 -->
      <section class="form-section form-section--env">
        <div class="form-section__header">
          <div class="form-section__badge is-env">
            <span>03</span>
          </div>
          <div class="form-section__text">
            <strong>运行与测试环境</strong>
            <p>锁定缺陷发生的终端环境，避免跨环境无法复现</p>
          </div>
        </div>

        <div class="form-grid form-grid--three">
          <el-form-item label="所属平台">
            <el-select v-model="form.environment.platform" clearable placeholder="选择运行端" class="full-width">
              <el-option v-for="item in ['iOS', 'Android', 'TTmins', 'Web', '服务端', '其他']" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
          <el-form-item label="测试环境">
            <el-input v-model="form.environment.environment" placeholder="测试 / 预发 / 生产环境" />
          </el-form-item>
          <el-form-item label="客户端 / App 版本">
            <el-input v-model="form.environment.app_version" placeholder="例如 v2.65.0 (Build 2818)" />
          </el-form-item>
          <el-form-item label="操作系统版本">
            <el-input v-model="form.environment.os_version" placeholder="例如 iOS 17.5 / macOS 14.5" />
          </el-form-item>
          <el-form-item label="终端设备型号">
            <el-input v-model="form.environment.device_model" placeholder="例如 iPhone 15 Pro / Chrome 125" />
          </el-form-item>
          <el-form-item label="网络环境">
            <el-input v-model="form.environment.network" placeholder="Wi-Fi / 5G / 弱网" />
          </el-form-item>
        </div>
      </section>

      <!-- 04 自定义字段 -->
      <section v-if="applicableFields.length" class="form-section form-section--custom">
        <div class="form-section__header">
          <div class="form-section__badge is-custom">
            <span>04</span>
          </div>
          <div class="form-section__text">
            <strong>项目扩展字段</strong>
            <p>根据当前选定项目所配置的个性化字段模板补充数据</p>
          </div>
        </div>

        <div class="form-grid form-grid--two">
          <el-form-item
            v-for="field in applicableFields"
            :key="field.id"
            :label="field.name"
            :required="field.required"
            :class="{ 'form-field--wide': field.field_type === 'textarea' }"
          >
            <el-input
              v-if="field.field_type === 'text' || field.field_type === 'url'"
              v-model="form.custom_fields[field.field_key]"
              :placeholder="field.field_type === 'url' ? 'https://...' : `请输入${field.name}`"
            />
            <el-input
              v-else-if="field.field_type === 'textarea'"
              v-model="form.custom_fields[field.field_key]"
              type="textarea"
              :rows="3"
              :placeholder="`请输入${field.name}`"
            />
            <el-input-number
              v-else-if="field.field_type === 'number'"
              v-model="form.custom_fields[field.field_key]"
              class="full-width"
              controls-position="right"
            />
            <el-select
              v-else-if="field.field_type === 'select'"
              v-model="form.custom_fields[field.field_key]"
              clearable
              class="full-width"
              :placeholder="`选择${field.name}`"
            >
              <el-option v-for="option in field.options" :key="option" :label="option" :value="option" />
            </el-select>
            <el-select
              v-else-if="field.field_type === 'multi_select'"
              v-model="form.custom_fields[field.field_key]"
              multiple
              clearable
              class="full-width"
              :placeholder="`选择${field.name}`"
            >
              <el-option v-for="option in field.options" :key="option" :label="option" :value="option" />
            </el-select>
            <el-date-picker
              v-else-if="field.field_type === 'date'"
              v-model="form.custom_fields[field.field_key]"
              type="date"
              value-format="YYYY-MM-DD"
              class="full-width"
            />
            <el-select
              v-else-if="field.field_type === 'user'"
              v-model="form.custom_fields[field.field_key]"
              filterable
              clearable
              class="full-width"
              :placeholder="`选择${field.name}`"
            >
              <el-option
                v-for="account in meta?.accounts || []"
                :key="account.id"
                :label="account.nickname || account.username"
                :value="account.id"
              />
            </el-select>
            <div v-else-if="field.field_type === 'boolean'" class="custom-switch-wrap">
              <el-switch v-model="form.custom_fields[field.field_key]" inline-prompt active-text="是" inactive-text="否" />
            </div>
          </el-form-item>
        </div>
      </section>
    </el-form>

    <template #footer>
      <div class="drawer-footer">
        <div class="drawer-footer__hint">
          <span class="hint-dot" />
          <span>粘贴或选择的附件将随缺陷内容一并保存</span>
        </div>
        <div class="drawer-footer__actions">
          <el-button class="cancel-btn" @click="close">取消</el-button>
          <el-button
            type="primary"
            class="submit-btn"
            :disabled="!canSave"
            :loading="saving"
            @click="submit"
          >
            {{ isEdit ? "保存修改" : (batchTitleCount > 1 ? `批量提交 ${batchTitleCount} 个缺陷` : "提交缺陷") }}
          </el-button>
        </div>
      </div>
    </template>
  </el-drawer>

  <el-dialog
    v-model="preview.visible"
    :title="preview.title"
    width="min(860px, 92vw)"
    append-to-body
    align-center
    destroy-on-close
    class="defect-attachment-preview"
    @closed="closeAttachmentPreview"
  >
    <div v-loading="preview.loading" class="attachment-preview-body">
      <img v-if="preview.type === 'image' && preview.source" :src="preview.source" :alt="preview.title" class="attachment-preview-image" />
      <pre v-else-if="preview.type === 'text'" class="attachment-preview-text">{{ preview.content }}</pre>
      <iframe v-else-if="preview.type === 'pdf' && preview.source" :src="preview.source" class="attachment-preview-pdf" title="PDF 附件预览" />
      <div v-else-if="!preview.loading" class="attachment-preview-file">
        <span><el-icon><Document /></el-icon></span>
        <strong>{{ preview.title }}</strong>
        <p>{{ preview.mimeType || '未知文件类型' }} · {{ formatAttachmentSize(preview.size) }}</p>
        <small>该文件类型暂不支持直接展开内容，可保存缺陷后在详情页下载查看。</small>
      </div>
    </div>
  </el-dialog>
</template>

<style scoped>
/* 整个 Drawer 的基底与滚动区域 */
.defect-form-drawer :deep(.el-drawer__header) {
  margin-bottom: 0;
  padding: 18px 24px;
  border-bottom: 1px solid #edf2f7;
  background: #ffffff;
}

.defect-form-drawer :deep(.el-drawer__body) {
  padding: 20px 24px;
  background: #f8fafc;
}

.defect-form-drawer :deep(.el-drawer__footer) {
  padding: 16px 24px;
  border-top: 1px solid #edf2f7;
  background: #ffffff;
}

/* 抽屉头部 */
.drawer-heading {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
}

.drawer-heading__icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  border-radius: 12px;
  color: #2563eb;
  background: linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%);
  border: 1px solid #bfdbfe;
  font-size: 22px;
  box-shadow: 0 2px 6px rgba(37, 99, 235, 0.08);
}

.drawer-heading__content {
  min-width: 0;
  flex: 1;
}

.drawer-heading__title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.drawer-heading strong {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.01em;
}

.drawer-heading__tag {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  padding: 2px 8px;
  border-radius: 6px;
}

.drawer-heading p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.4;
}

.drawer-close-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  color: #64748b;
  font-size: 16px;
  cursor: pointer;
  outline: none;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.drawer-close-btn:hover {
  background: #fee2e2;
  border-color: #fecaca;
  color: #ef4444;
  transform: scale(1.05);
}

.drawer-close-btn:active {
  transform: scale(0.95);
}

.defect-form-drawer :deep(.el-drawer__close-btn) {
  display: none !important;
}

/* 表单主体与分段卡片 */
.defect-form {
  padding-bottom: 8px;
}

.status-direct-panel {
  display: grid;
  grid-template-columns: minmax(180px, 0.8fr) minmax(420px, 1.5fr);
  align-items: center;
  gap: 24px;
  padding: 18px 22px;
  margin-bottom: 18px;
  overflow: hidden;
  border: 1px solid #dbeafe;
  border-radius: 16px;
  background: linear-gradient(135deg, #ffffff 0%, #f4f8ff 100%);
  box-shadow: 0 5px 20px rgba(37, 99, 235, 0.07);
}

.status-direct-panel__copy {
  min-width: 0;
}

.status-direct-panel__eyebrow {
  display: block;
  margin-bottom: 4px;
  color: #2563eb;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.08em;
}

.status-direct-panel__copy strong {
  color: #0f172a;
  font-size: 15px;
}

.status-direct-panel__copy p {
  margin: 5px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.55;
}

.status-direct-track {
  display: flex;
  align-items: center;
  min-width: 0;
  transition: opacity 0.2s ease;
}

.status-direct-track.is-loading {
  opacity: 0.6;
}

.status-direct-track__line {
  min-width: 12px;
  height: 1px;
  flex: 1;
  background: #dbe3ef;
}

.status-direct-node {
  display: grid;
  justify-items: center;
  gap: 4px;
  min-width: 70px;
  padding: 7px 5px;
  border: 0;
  border-radius: 11px;
  color: #475569;
  background: transparent;
  font-family: inherit;
  cursor: pointer;
  transition: transform 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}

.status-direct-node:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 5px 14px rgba(15, 23, 42, 0.08);
  transform: translateY(-2px);
}

.status-direct-node__dot {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  border: 2px solid #cbd5e1;
  border-radius: 50%;
  color: #64748b;
  background: #ffffff;
  font-size: 11px;
  font-weight: 800;
}

.status-direct-node__label {
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

.status-direct-node small {
  color: #94a3b8;
  font-size: 10px;
  white-space: nowrap;
}

.status-direct-node.is-current {
  cursor: default;
}

.status-direct-node.is-current .status-direct-node__dot {
  color: #ffffff;
  border-color: currentColor;
  box-shadow: 0 0 0 4px rgba(37, 99, 235, 0.1);
}

.status-direct-node.is-current.is-slate .status-direct-node__dot { background: #64748b; }
.status-direct-node.is-current.is-blue .status-direct-node__dot { background: #2563eb; }
.status-direct-node.is-current.is-amber .status-direct-node__dot { background: #d97706; }
.status-direct-node.is-current.is-green .status-direct-node__dot { background: #16a34a; }
.status-direct-node.is-current.is-slate { color: #475569; }
.status-direct-node.is-current.is-blue { color: #1d4ed8; }
.status-direct-node.is-current.is-amber { color: #b45309; }
.status-direct-node.is-current.is-green { color: #15803d; }

.status-direct-node.is-disabled {
  opacity: 0.42;
  cursor: not-allowed;
}

.form-section {
  padding: 22px 24px 20px;
  margin-bottom: 18px;
  border: 1px solid rgba(226, 232, 240, 0.9);
  border-radius: 16px;
  background: #ffffff;
  box-shadow: 0 4px 18px -2px rgba(15, 23, 42, 0.04), 0 1px 4px -1px rgba(15, 23, 42, 0.02);
  transition: all 0.25s ease;
}

.form-section:hover {
  border-color: #cbd5e1;
  box-shadow: 0 6px 24px -2px rgba(15, 23, 42, 0.07);
}

/* 分段头部 */
.form-section__header {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-bottom: 16px;
  margin-bottom: 18px;
  border-bottom: 1px dashed #e2e8f0;
}

.form-section__badge {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 10px;
  background: linear-gradient(135deg, #eff6ff 0%, #dbeafe 100%);
  color: #2563eb;
  font-weight: 800;
  font-size: 13px;
  border: 1px solid #bfdbfe;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.form-section__badge.is-issue {
  background: linear-gradient(135deg, #fff7ed 0%, #ffedd5 100%);
  color: #ea580c;
  border-color: #fed7aa;
}

.form-section__badge.is-env {
  background: linear-gradient(135deg, #ecfeff 0%, #cffafe 100%);
  color: #0891b2;
  border-color: #a5f3fc;
}

.form-section__badge.is-custom {
  background: linear-gradient(135deg, #f5f3ff 0%, #ede9fe 100%);
  color: #7c3aed;
  border-color: #ddd6fe;
}

.form-section__text strong {
  display: block;
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.form-section__text p {
  margin: 3px 0 0;
  color: #64748b;
  font-size: 12px;
}

/* 标题高亮项 */
.title-item {
  margin-bottom: 18px;
}

.title-editor {
  width: 100%;
}

.title-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
}

.title-row__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  flex-shrink: 0;
  border-radius: 7px;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  font-size: 12px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.title-row .title-input {
  flex: 1;
  min-width: 0;
}

.title-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  min-width: 40px;
  height: 40px;
  padding: 0;
  border-radius: 10px;
  font-size: 16px;
  cursor: pointer;
  outline: none;
  border: 1px solid transparent;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.title-action-btn:active {
  transform: scale(0.95);
}

.title-action-btn--add {
  border-color: #bfdbfe;
  color: #2563eb;
  background: #eff6ff;
}

.title-action-btn--add:hover {
  border-color: #93c5fd;
  color: #1d4ed8;
  background: #dbeafe;
  transform: scale(1.04);
}

.title-action-btn--delete {
  border-color: #e2e8f0;
  color: #94a3b8;
  background: #ffffff;
}

.title-action-btn--delete:hover {
  border-color: #fecaca;
  color: #ef4444;
  background: #fee2e2;
  transform: scale(1.04);
}

.title-editor__hint {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 8px;
  padding: 0 2px;
  color: #94a3b8;
  font-size: 12px;
}

.title-editor__hint strong {
  padding: 2px 8px;
  border-radius: 999px;
  color: #1d4ed8;
  background: #dbeafe;
  font-weight: 700;
  white-space: nowrap;
}

/* 统一输入框与文本域尺寸、圆角与聚焦效果，保证主/次标题 100% 视觉一致 */
.title-input :deep(.el-input__wrapper) {
  height: 40px;
  line-height: 40px;
  padding: 0 14px;
  border-radius: 10px;
  box-shadow: 0 0 0 1px #cbd5e1 inset;
  background-color: #ffffff;
  font-size: 14px;
  font-weight: 500;
  color: #0f172a;
  transition: all 0.2s ease;
}

.title-input :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #94a3b8 inset;
}

.title-input :deep(.el-input__wrapper.is-focus) {
  background-color: #ffffff;
  box-shadow: 0 0 0 1px #3b82f6 inset, 0 0 0 3px rgba(59, 130, 246, 0.12) !important;
}

.title-input :deep(.el-textarea__inner) {
  min-height: 40px !important;
  height: 40px;
  border-radius: 10px;
  box-shadow: 0 0 0 1px #cbd5e1 inset;
  background-color: #ffffff;
  font-size: 14px;
  font-weight: 500;
  line-height: 22px;
  padding: 8px 14px;
  color: #0f172a;
  resize: none;
  transition: all 0.2s ease;
}

.title-input :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px #94a3b8 inset;
}

.title-input :deep(.el-textarea__inner:focus) {
  background-color: #ffffff;
  box-shadow: 0 0 0 1px #3b82f6 inset, 0 0 0 3px rgba(59, 130, 246, 0.12) !important;
  outline: none;
}

/* 统一输入框与文本域圆润视觉 */
:deep(.el-input__wrapper) {
  border-radius: 9px;
  box-shadow: 0 0 0 1px #e2e8f0 inset;
  transition: all 0.2s ease;
}

:deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #cbd5e1 inset;
}

:deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #3b82f6 inset, 0 0 0 3px rgba(59, 130, 246, 0.12) !important;
}

:deep(.el-textarea__inner) {
  border-radius: 10px;
  border: 1px solid #e2e8f0;
  padding: 10px 14px;
  font-size: 13px;
  line-height: 1.6;
  font-family: inherit;
  transition: all 0.2s ease;
}

:deep(.el-textarea__inner:hover) {
  border-color: #cbd5e1;
}

:deep(.el-textarea__inner:focus) {
  border-color: #3b82f6;
  box-shadow: 0 0 0 3px rgba(59, 130, 246, 0.12);
}

:deep(.el-form-item__label) {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
  margin-bottom: 6px;
  line-height: 1.3;
}

/* 栅格布局 */
.form-grid {
  display: grid;
  gap: 14px;
}

.form-grid--three {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.form-grid--two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.form-field--wide {
  grid-column: 1 / -1;
}

.full-width {
  width: 100%;
}

.soft-textarea :deep(.el-textarea__inner) {
  background-color: #fafbfc;
}

.soft-textarea :deep(.el-textarea__inner:focus) {
  background-color: #ffffff;
}

.supplement-editor { width: 100%; }

.supplement-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 8px;
}

.supplement-toolbar > span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  flex: 1;
  color: #94a3b8;
  font-size: 11px;
}

.supplement-toolbar .el-button {
  border-color: #dbeafe;
  border-radius: 9px;
  color: #2563eb;
  background: #f8fbff;
}

.attachment-file-input { display: none; }

.supplement-attachments {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.supplement-attachment {
  position: relative;
  display: grid;
  grid-template-columns: 48px minmax(0, 1fr) 28px;
  align-items: center;
  gap: 10px;
  min-height: 64px;
  padding: 8px 8px 8px 10px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #f8fafc;
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.supplement-attachment:hover {
  border-color: #93c5fd;
  box-shadow: 0 7px 18px rgba(37, 99, 235, 0.1);
  transform: translateY(-1px);
}

.supplement-attachment.is-pending {
  border-style: dashed;
  background: #f0f7ff;
}

.supplement-attachment img,
.supplement-attachment__icon {
  width: 48px;
  height: 48px;
  border-radius: 9px;
}

.supplement-attachment img {
  display: block;
  object-fit: cover;
  background: #e2e8f0;
}

.supplement-attachment__icon {
  display: grid;
  place-items: center;
  color: #2563eb;
  background: #dbeafe;
  font-size: 22px;
}

.supplement-attachment__meta { min-width: 0; }

.supplement-attachment__meta strong,
.supplement-attachment__meta small { display: block; }

.supplement-attachment__meta strong {
  overflow: hidden;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.supplement-attachment__meta small {
  margin-top: 4px;
  color: #94a3b8;
  font-size: 10px;
}

.supplement-attachment__delete { color: #94a3b8; }

.supplement-attachment__delete:hover {
  color: #dc2626;
  background: #fee2e2;
}

:global(.defect-attachment-preview) {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.22);
}

:global(.defect-attachment-preview .el-dialog__header) {
  padding: 18px 22px 14px;
  border-bottom: 1px solid #edf2f7;
}

:global(.defect-attachment-preview .el-dialog__body) { padding: 0; }

.attachment-preview-body {
  display: grid;
  place-items: center;
  min-height: 240px;
  max-height: 76vh;
  overflow: auto;
  background: #f8fafc;
}

.attachment-preview-image {
  display: block;
  max-width: 100%;
  max-height: 74vh;
  margin: auto;
  object-fit: contain;
}

.attachment-preview-text {
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

.attachment-preview-pdf {
  width: 100%;
  height: 72vh;
  border: 0;
  background: #ffffff;
}

.attachment-preview-file {
  display: grid;
  justify-items: center;
  max-width: 480px;
  padding: 38px;
  text-align: center;
}

.attachment-preview-file > span {
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

.attachment-preview-file strong { color: #0f172a; font-size: 15px; }

.attachment-preview-file p,
.attachment-preview-file small {
  margin: 7px 0 0;
  color: #64748b;
  font-size: 12px;
}

/* 重现步骤卡片列表 */
.step-card-container {
  display: flex;
  flex-direction: column;
  gap: 10px;
  width: 100%;
}

.step-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 12px;
  border-radius: 12px;
  background: #f8fafc;
  border: 1px solid #edf2f7;
  transition: all 0.2s ease;
}

.step-card:hover {
  background: #f1f5f9;
  border-color: #e2e8f0;
}

.step-card__index {
  display: grid;
  place-items: center;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  border-radius: 50%;
  background: #3b82f6;
  color: #ffffff;
  font-size: 12px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.step-card__input-wrap {
  flex: 1;
  min-width: 0;
}

.step-card__input-wrap :deep(.el-textarea__inner) {
  background: #ffffff;
  border-color: #e2e8f0;
  box-shadow: none;
}

.step-card__del-btn {
  color: #94a3b8;
  font-size: 16px;
  padding: 6px;
  border-radius: 8px;
}

.step-card__del-btn:hover {
  color: #ef4444;
  background: #fee2e2;
}

.step-add-btn {
  align-self: flex-start;
  border: 1px dashed #93c5fd;
  background: #f0f7ff;
  color: #2563eb;
  border-radius: 10px;
  font-weight: 600;
  padding: 8px 16px;
  font-size: 13px;
  margin-top: 4px;
  transition: all 0.2s ease;
}

.step-add-btn:hover {
  background: #e0f2fe;
  border-color: #3b82f6;
  color: #1d4ed8;
}

/* 实际结果 vs 预期结果 双栏对比卡片 */
.result-split-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin: 10px 0 16px;
}

.result-card-box {
  padding: 14px;
  border-radius: 14px;
  border: 1px solid;
  transition: all 0.2s ease;
}

.result-card-box--actual {
  background: #fff8f8;
  border-color: #fecaca;
}

.result-card-box--expected {
  background: #f4fdf7;
  border-color: #bbf7d0;
}

.result-card-box__title {
  margin-bottom: 10px;
}

.result-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 700;
  padding: 3px 10px;
  border-radius: 6px;
}

.result-badge.is-actual {
  background: #fee2e2;
  color: #dc2626;
  border: 1px solid #fca5a5;
}

.result-badge.is-expected {
  background: #dcfce7;
  color: #15803d;
  border: 1px solid #86efac;
}

.result-textarea.is-actual :deep(.el-textarea__inner) {
  background: #ffffff;
  border-color: #fca5a5;
}

.result-textarea.is-actual :deep(.el-textarea__inner:focus) {
  border-color: #ef4444;
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.12);
}

.result-textarea.is-expected :deep(.el-textarea__inner) {
  background: #ffffff;
  border-color: #86efac;
}

.result-textarea.is-expected :deep(.el-textarea__inner:focus) {
  border-color: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.12);
}

.custom-switch-wrap {
  padding-top: 6px;
}

/* 底部操作条 */
.drawer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  width: 100%;
}

.drawer-footer__hint {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #64748b;
  font-size: 13px;
}

.hint-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #3b82f6;
}

.drawer-footer__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.cancel-btn {
  padding: 10px 18px;
  border-radius: 9px;
  font-weight: 500;
}

.submit-btn {
  padding: 10px 24px;
  border-radius: 9px;
  font-weight: 600;
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  border: none;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.28);
  transition: all 0.2s ease;
}

.submit-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, #2563eb 0%, #1d4ed8 100%);
  box-shadow: 0 6px 18px rgba(37, 99, 235, 0.38);
  transform: translateY(-1px);
}

:global(.defect-status-confirm) {
  overflow: hidden;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.18);
}

:global(.defect-status-confirm .el-message-box__header) {
  padding: 20px 22px 12px;
}

:global(.defect-status-confirm .el-message-box__content) {
  padding: 8px 22px 18px;
  color: #475569;
}

:global(.defect-status-confirm .el-message-box__btns) {
  padding: 12px 22px 20px;
}

:global(.defect-status-confirm .el-button) {
  border-radius: 9px;
}

@media (max-width: 760px) {
  .status-direct-panel {
    grid-template-columns: 1fr;
    gap: 14px;
  }
  .status-direct-node {
    min-width: 58px;
  }
  .title-row {
    align-items: flex-start;
  }
  .supplement-attachments { grid-template-columns: 1fr; }
  .supplement-toolbar > span { display: none; }
  .form-grid--three,
  .form-grid--two,
  .result-split-grid {
    grid-template-columns: 1fr;
  }
  .form-section {
    padding: 16px;
  }
  .drawer-footer__hint {
    display: none;
  }
}
</style>
