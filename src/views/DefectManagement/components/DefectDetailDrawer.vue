<script setup lang="ts">
import DefectVersionSelect from './DefectVersionSelect.vue'
import { computed, reactive, ref, watch } from 'vue'
import type { UploadProps } from 'element-plus'
import {
  ChatDotRound,
  Check,
  CircleCheck,
  Document,
  EditPen,
  Paperclip,
  RefreshLeft,
  Right
} from '@element-plus/icons-vue'
import type { DefectTransitionPayload } from '../api'
import {
  defectStatusMeta,
  defectTypeOptions,
  priorityLabels,
  severityLabels,
  type DefectDetail,
  type DefectMeta
} from '../types'

const props = defineProps<{
  modelValue: boolean
  detail: DefectDetail | null
  meta: DefectMeta | null
  loading: boolean
  acting: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  edit: []
  refresh: []
  transition: [payload: DefectTransitionPayload]
  comment: [content: string]
  upload: [file: File]
}>()

const commentText = ref('')
const transitionVisible = ref(false)
const transition = reactive<DefectTransitionPayload>({ action: 'confirm', comment: '' })

const defect = computed(() => props.detail?.defect || null)
const canEdit = computed(() => defect.value?.allowed_actions?.edit === true)
const canProcess = computed(() => defect.value?.allowed_actions?.process === true)
const canVerify = computed(() => defect.value?.allowed_actions?.verify === true)
const canReopen = computed(() => defect.value?.allowed_actions?.reopen === true)
const canComment = computed(() => defect.value?.allowed_actions?.comment === true)
const defectTypeLabel = computed(() => defectTypeOptions.find((item) => item.value === defect.value?.defect_type)?.label || defect.value?.defect_type || '未设置')
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
    type: 'history' as const,
    action: item.action,
    author: item.operator_name,
    content: item.comment,
    created_at: item.created_at
  }))
  const comments = (props.detail?.comments || []).map((item) => ({
    id: `comment-${item.id}`,
    type: 'comment' as const,
    action: 'comment',
    author: item.author_name,
    content: item.content,
    created_at: item.created_at
  }))
  return [...histories, ...comments].sort((a, b) => b.created_at.localeCompare(a.created_at))
})

const actionLabels: Record<string, string> = {
  create: '提交了缺陷',
  update: '更新了缺陷信息',
  confirm: '确认并开始处理',
  resolve: '标记为已解决，等待验证',
  close: '验证通过并关闭',
  reopen: '重新激活了缺陷',
  comment: '添加了评论'
}

const transitionTitle = computed(() => ({
  confirm: '确认缺陷',
  resolve: '解决缺陷',
  close: '验证并关闭',
  reopen: '重新激活'
}[transition.action] || '状态操作'))

watch(() => props.modelValue, (visible) => {
  if (visible) commentText.value = ''
})

function formatTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

function formatSize(size: number) {
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

function customValueLabel(value: unknown, fieldType?: string) {
  if (fieldType === 'user') {
    const account = props.meta?.accounts.find((item) => item.id === value)
    return account?.nickname || account?.username || String(value || '—')
  }
  if (typeof value === 'boolean') return value ? '是' : '否'
  if (Array.isArray(value)) return value.join('、') || '—'
  return String(value ?? '—')
}

function openTransition(action: DefectTransitionPayload['action']) {
  Object.assign(transition, {
    action,
    assignee_id: defect.value?.assignee_id || '',
    resolution: action === 'resolve' ? 'fixed' : '',
    resolved_version: '',
    comment: ''
  })
  transitionVisible.value = true
}

function submitTransition() {
  emit('transition', { ...transition })
}

function submitComment() {
  const content = commentText.value.trim()
  if (!content) return
  emit('comment', content)
  commentText.value = ''
}

const interceptUpload: UploadProps['beforeUpload'] = (file) => {
  emit('upload', file)
  return false
}

defineExpose({ closeTransition: () => { transitionVisible.value = false } })
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    size="min(860px, 96vw)"
    class="defect-detail-drawer"
    @close="emit('update:modelValue', false)"
  >
    <template #header>
      <div v-if="defect" class="detail-heading">
        <div>
          <div class="detail-heading__meta">
            <span>{{ defect.defect_no }}</span>
            <span class="status-pill" :class="`is-${defectStatusMeta[defect.status].tone}`">{{ defectStatusMeta[defect.status].label }}</span>
          </div>
          <strong>{{ defect.title }}</strong>
        </div>
        <div class="detail-heading__actions">
          <el-button circle :icon="RefreshLeft" title="刷新" @click="emit('refresh')" />
          <el-button v-if="canEdit" :icon="EditPen" @click="emit('edit')">编辑</el-button>
        </div>
      </div>
    </template>

    <div v-loading="loading" class="detail-body">
      <template v-if="defect">
        <section class="lifecycle-panel">
          <div class="lifecycle-track">
            <div v-for="(item, index) in ['new', 'active', 'resolved', 'closed']" :key="item" class="lifecycle-node" :class="{ 'is-reached': ['new', 'active', 'resolved', 'closed'].indexOf(defect.status) >= index }">
              <span><el-icon><Check /></el-icon></span>
              <em>{{ defectStatusMeta[item as keyof typeof defectStatusMeta].label }}</em>
              <i v-if="index < 3"><el-icon><Right /></el-icon></i>
            </div>
          </div>
          <div class="lifecycle-actions">
            <el-button v-if="defect.status === 'new' && canProcess" type="primary" @click="openTransition('confirm')">确认缺陷</el-button>
            <el-button v-if="['new', 'active'].includes(defect.status) && canProcess" type="success" plain @click="openTransition('resolve')">标记解决</el-button>
            <el-button v-if="defect.status === 'resolved' && canVerify" type="success" @click="openTransition('close')">验证通过</el-button>
            <el-button v-if="['resolved', 'closed'].includes(defect.status) && canReopen" type="warning" plain @click="openTransition('reopen')">重新激活</el-button>
          </div>
        </section>

        <section class="detail-section">
          <h3>基本信息</h3>
          <div class="info-grid">
            <div><span>所属项目</span><strong>{{ defect.project_code }} · {{ defect.project_name }}</strong></div>
            <div><span>发现版本</span><strong>{{ defect.found_version || '未填写' }}</strong></div>
            <div><span>缺陷类型</span><strong>{{ defectTypeLabel }}</strong></div>
            <div><span>严重程度</span><strong class="severity-text" :class="`severity-${defect.severity}`">{{ defect.severity }} · {{ severityLabels[defect.severity] }}</strong></div>
            <div><span>优先级</span><strong>{{ priorityLabels[defect.priority] }}</strong></div>
            <div><span>期望解决</span><strong>{{ defect.due_date || '未设置' }}</strong></div>
            <div><span>提交人</span><strong>{{ defect.reporter_name }}</strong></div>
            <div><span>处理人</span><strong>{{ defect.assignee_name || '暂未指派' }}</strong></div>
            <div><span>验证人</span><strong>{{ defect.verifier_name || '暂未指定' }}</strong></div>
          </div>
          <div v-if="defect.tags?.length" class="tag-row"><el-tag v-for="tag in defect.tags" :key="tag" effect="plain" round>{{ tag }}</el-tag></div>
        </section>

        <section class="detail-section issue-section">
          <h3>问题现场</h3>
          <div v-if="defect.precondition" class="text-block"><span>前置条件</span><p>{{ defect.precondition }}</p></div>
          <div class="text-block"><span>重现步骤</span><ol v-if="defect.steps?.length"><li v-for="step in defect.steps" :key="step.id">{{ step.content }}</li></ol><p v-else class="empty-text">未填写</p></div>
          <div class="result-grid">
            <div class="text-block is-actual"><span>实际结果</span><p>{{ defect.actual_result || '未填写' }}</p></div>
            <div class="text-block is-expected"><span>预期结果</span><p>{{ defect.expected_result || '未填写' }}</p></div>
          </div>
          <div v-if="defect.description" class="text-block"><span>补充说明</span><p>{{ defect.description }}</p></div>
        </section>

        <section class="detail-section">
          <h3>测试环境</h3>
          <div class="environment-row">
            <span v-for="(value, key) in defect.environment" v-show="value" :key="key"><small>{{ ({ platform: '平台', environment: '环境', app_version: 'App版本', os_version: '系统', device_model: '设备', network: '网络' } as Record<string,string>)[key] }}</small>{{ value }}</span>
            <em v-if="!Object.values(defect.environment || {}).some(Boolean)">未记录环境信息</em>
          </div>
        </section>

        <section v-if="customFieldEntries.length" class="detail-section">
          <h3>自定义字段</h3>
          <div class="info-grid custom-field-grid">
            <div v-for="item in customFieldEntries" :key="item.key"><span>{{ item.definition?.name || item.key }}</span><strong>{{ customValueLabel(item.value, item.definition?.field_type) }}</strong></div>
          </div>
        </section>

        <section class="detail-section">
          <div class="section-heading"><h3>附件</h3><span>支持图片、视频、日志、JSON、ZIP 和 PDF，单个不超过 20MB</span></div>
          <div v-if="detail?.attachments.length" class="attachment-list">
            <a v-for="item in detail.attachments" :key="item.id" :href="item.download_url" class="attachment-card" target="_blank">
              <span><el-icon><Document /></el-icon></span><div><strong>{{ item.original_name }}</strong><small>{{ formatSize(item.size) }} · {{ item.uploader_name }}</small></div>
            </a>
          </div>
          <el-upload v-if="canEdit" :show-file-list="false" :before-upload="interceptUpload" :disabled="acting">
            <el-button :icon="Paperclip" plain :loading="acting">上传附件</el-button>
          </el-upload>
        </section>

        <section class="detail-section activity-section">
          <div class="section-heading"><h3>活动记录</h3><span>{{ timeline.length }} 条记录</span></div>
          <div v-if="canComment" class="comment-box">
            <el-input v-model="commentText" type="textarea" :rows="2" maxlength="2000" show-word-limit placeholder="补充处理进展或验证信息" />
            <el-button type="primary" :icon="ChatDotRound" :disabled="!commentText.trim()" :loading="acting" @click="submitComment">发表评论</el-button>
          </div>
          <el-timeline v-if="timeline.length" class="activity-timeline">
            <el-timeline-item v-for="item in timeline" :key="item.id" :timestamp="formatTime(item.created_at)" placement="top" :type="item.type === 'comment' ? 'primary' : 'success'" hollow>
              <div class="activity-card"><div><strong>{{ item.author }}</strong><span>{{ actionLabels[item.action] || item.action }}</span></div><p v-if="item.content">{{ item.content }}</p></div>
            </el-timeline-item>
          </el-timeline>
        </section>
      </template>
    </div>

    <el-dialog v-model="transitionVisible" :title="transitionTitle" width="520px" append-to-body destroy-on-close>
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
          <el-form-item label="解决版本"><DefectVersionSelect v-model="transition.resolved_version" :meta="meta" :projects="defect ? [defect.project_code] : []" /></el-form-item>
        </template>
        <el-form-item :label="transition.action === 'reopen' ? '重新激活原因' : '操作备注'" :required="transition.action === 'reopen'">
          <el-input v-model="transition.comment" type="textarea" :rows="4" maxlength="1000" show-word-limit :placeholder="transition.action === 'reopen' ? '说明验证失败或重新出现的现象' : '可填写处理说明'" />
        </el-form-item>
      </el-form>
      <template #footer><el-button @click="transitionVisible=false">取消</el-button><el-button type="primary" :icon="transition.action === 'reopen' ? RefreshLeft : CircleCheck" :loading="acting" :disabled="transition.action === 'resolve' && !transition.resolution || transition.action === 'reopen' && !transition.comment?.trim()" @click="submitTransition">确认操作</el-button></template>
    </el-dialog>
  </el-drawer>
</template>

<style scoped>
.detail-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:20px;width:100%;padding-right:12px}.detail-heading__meta{display:flex;align-items:center;gap:9px;margin-bottom:7px;color:#64748b;font-size:12px;font-weight:700;letter-spacing:.03em}.detail-heading>div>strong{display:block;max-width:590px;color:#0f172a;font-size:19px;line-height:1.35}.detail-heading__actions{display:flex;gap:8px}.status-pill{padding:4px 9px;border-radius:999px;font-size:11px}.status-pill.is-slate{color:#475569;background:#f1f5f9}.status-pill.is-blue{color:#2563eb;background:#eff6ff}.status-pill.is-amber{color:#b45309;background:#fffbeb}.status-pill.is-green{color:#15803d;background:#f0fdf4}.detail-body{min-height:320px;padding:0 4px 30px}.lifecycle-panel,.detail-section{margin-bottom:16px;padding:20px;border:1px solid #e7edf5;border-radius:16px;background:#fff;box-shadow:0 10px 30px rgba(15,23,42,.035)}.lifecycle-panel{background:linear-gradient(135deg,#f8fbff,#fff)}.lifecycle-track{display:flex;align-items:center}.lifecycle-node{display:flex;align-items:center;flex:1;color:#cbd5e1}.lifecycle-node:last-child{flex:0}.lifecycle-node>span{display:grid;place-items:center;width:28px;height:28px;flex:0 0 28px;border-radius:50%;border:2px solid #dbe4ef;background:#fff}.lifecycle-node em{margin-left:7px;white-space:nowrap;font-size:12px;font-style:normal;font-weight:700}.lifecycle-node i{display:flex;align-items:center;justify-content:center;flex:1;font-style:normal}.lifecycle-node.is-reached{color:#2563eb}.lifecycle-node.is-reached>span{color:#fff;border-color:#3b82f6;background:#3b82f6}.lifecycle-actions{display:flex;justify-content:flex-end;gap:8px;margin-top:18px;padding-top:16px;border-top:1px dashed #dbe4ef}.detail-section h3{margin:0;color:#172033;font-size:15px}.info-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px;margin-top:18px}.info-grid>div{display:flex;flex-direction:column;gap:6px}.info-grid span,.text-block>span{color:#94a3b8;font-size:11px}.info-grid strong{color:#334155;font-size:13px;font-weight:600}.severity-text.severity-1{color:#dc2626}.severity-text.severity-2{color:#ea580c}.tag-row{display:flex;gap:7px;flex-wrap:wrap;margin-top:18px}.issue-section{display:flex;flex-direction:column;gap:15px}.issue-section h3{margin-bottom:3px}.text-block{padding:14px;border-radius:12px;background:#f8fafc}.text-block p{margin:7px 0 0;color:#334155;line-height:1.7;white-space:pre-wrap;font-size:13px}.text-block ol{margin:9px 0 0;padding-left:22px;color:#334155}.text-block li{padding:4px 0;line-height:1.6}.result-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}.text-block.is-actual{border-left:3px solid #f87171;background:#fff7f7}.text-block.is-expected{border-left:3px solid #34d399;background:#f4fdf9}.empty-text{color:#94a3b8!important}.environment-row{display:flex;flex-wrap:wrap;gap:9px;margin-top:16px}.environment-row>span{display:flex;flex-direction:column;gap:3px;padding:9px 12px;border:1px solid #e6ecf4;border-radius:10px;color:#334155;font-size:12px}.environment-row small{color:#94a3b8;font-size:10px}.environment-row em{color:#94a3b8;font-size:12px;font-style:normal}.section-heading{display:flex;align-items:center;justify-content:space-between;margin-bottom:15px}.section-heading>span{color:#94a3b8;font-size:11px}.attachment-list{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin-bottom:12px}.attachment-card{display:flex;align-items:center;gap:10px;padding:11px;border:1px solid #e6ecf4;border-radius:11px;color:inherit;text-decoration:none;transition:.2s}.attachment-card:hover{border-color:#bfdbfe;background:#f8fbff;transform:translateY(-1px)}.attachment-card>span{display:grid;place-items:center;width:34px;height:34px;border-radius:9px;color:#3b82f6;background:#eff6ff}.attachment-card div{min-width:0}.attachment-card strong,.attachment-card small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.attachment-card strong{color:#334155;font-size:12px}.attachment-card small{margin-top:4px;color:#94a3b8;font-size:10px}.comment-box{display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:end;gap:10px;margin-bottom:24px}.activity-timeline{padding-left:4px}.activity-card{padding:2px 0 10px}.activity-card>div{display:flex;gap:7px;align-items:center}.activity-card strong{color:#334155;font-size:13px}.activity-card span{color:#94a3b8;font-size:12px}.activity-card p{margin:7px 0 0;padding:10px 12px;border-radius:9px;background:#f8fafc;color:#475569;line-height:1.6;white-space:pre-wrap;font-size:12px}.full-width{width:100%}@media(max-width:760px){.info-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.result-grid,.attachment-list{grid-template-columns:1fr}.lifecycle-node em{display:none}.detail-heading__actions .el-button:not(.is-circle){padding:8px}.comment-box{grid-template-columns:1fr}.comment-box .el-button{justify-self:end}}
</style>
