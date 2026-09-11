<script setup lang="ts">
import DefectVersionSelect from './DefectVersionSelect.vue'
import { computed, reactive, watch } from 'vue'
import { Operation } from '@element-plus/icons-vue'
import type { DefectBatchPayload } from '../api'
import { priorityLabels, severityLabels, type DefectMeta, type Defect } from '../types'

const props = defineProps<{ modelValue: boolean; count: number; defects: Defect[]; meta: DefectMeta | null; loading: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; execute: [payload: Omit<DefectBatchPayload, 'ids'>] }>()

const form = reactive<Omit<DefectBatchPayload, 'ids'>>({ action: 'assign', assignee_id: '', severity: 3, priority: 3, resolution: 'fixed', resolved_version: '', comment: '' })
const allowed = (action: string) => props.defects.length > 0 && props.defects.every((item) => item.allowed_actions?.[action] === true)
const canProcess = computed(() => allowed('process'))
const canVerify = computed(() => allowed('verify'))
const canReopen = computed(() => allowed('reopen'))
const canArchive = computed(() => allowed('archive'))
const hasCommonAction = computed(() => canProcess.value || canVerify.value || canReopen.value || canArchive.value)
const actionAllowed = () => allowed(({ close: 'verify', reopen: 'reopen', archive: 'archive' } as Record<string, string>)[form.action] || 'process')
const needsComment = computed(() => form.action === 'reopen')
const canSubmit = computed(() => {
  if (!actionAllowed()) return false
  if (form.action === 'assign') return Boolean(form.assignee_id)
  if (form.action === 'resolve') return Boolean(form.resolution)
  if (form.action === 'reopen') return Boolean(form.comment?.trim())
  return true
})

watch(() => props.modelValue, (visible) => {
  if (visible) Object.assign(form, { action: canProcess.value ? 'assign' : canVerify.value ? 'close' : canReopen.value ? 'reopen' : 'archive', assignee_id: '', severity: 3, priority: 3, resolution: 'fixed', resolved_version: '', comment: '' })
})
</script>

<template>
  <el-dialog :model-value="modelValue" title="批量处理缺陷" width="540px" destroy-on-close @close="emit('update:modelValue', false)">
    <div class="batch-hint"><span><el-icon><Operation /></el-icon></span><div><strong>已选择 {{ count }} 条缺陷</strong><p>系统会逐条检查状态和权限，不符合条件的记录不会被修改。</p></div></div>
    <el-empty v-if="!hasCommonAction" description="所选缺陷没有共同可用的操作，请按项目或权限分别选择" :image-size="70" />
    <el-form v-else label-position="top" class="batch-form">
      <el-form-item label="批量操作">
        <el-select v-model="form.action" class="full-width">
          <el-option-group v-if="canProcess" label="属性调整"><el-option label="指派处理人" value="assign" /><el-option label="修改严重程度" value="severity" /><el-option label="修改优先级" value="priority" /></el-option-group>
          <el-option-group label="生命周期"><el-option v-if="canProcess" label="确认缺陷" value="confirm" /><el-option v-if="canProcess" label="标记解决" value="resolve" /><el-option v-if="canVerify" label="验证关闭" value="close" /><el-option v-if="canReopen" label="重新激活" value="reopen" /></el-option-group>
          <el-option-group v-if="canArchive" label="管理操作"><el-option label="归档缺陷" value="archive" /></el-option-group>
        </el-select>
      </el-form-item>
      <el-form-item v-if="form.action === 'assign' || form.action === 'confirm'" label="处理人" :required="form.action === 'assign'"><el-select v-model="form.assignee_id" clearable filterable class="full-width" placeholder="选择处理人"><el-option v-for="account in meta?.accounts || []" :key="account.id" :label="account.nickname ? `${account.nickname} (${account.username})` : account.username" :value="account.id" /></el-select></el-form-item>
      <el-form-item v-if="form.action === 'severity'" label="严重程度"><el-select v-model="form.severity" class="full-width"><el-option v-for="(label,value) in severityLabels" :key="value" :label="`${value} · ${label}`" :value="Number(value)" /></el-select></el-form-item>
      <el-form-item v-if="form.action === 'priority'" label="优先级"><el-select v-model="form.priority" class="full-width"><el-option v-for="(label,value) in priorityLabels" :key="value" :label="label" :value="Number(value)" /></el-select></el-form-item>
      <template v-if="form.action === 'resolve'"><el-form-item label="解决方案" required><el-select v-model="form.resolution" class="full-width"><el-option v-for="item in [{v:'fixed',l:'已修复'},{v:'duplicate',l:'重复缺陷'},{v:'by_design',l:'设计如此'},{v:'cannot_reproduce',l:'无法复现'},{v:'external',l:'外部原因'},{v:'postponed',l:'延期处理'},{v:'wont_fix',l:'不予修复'}]" :key="item.v" :label="item.l" :value="item.v" /></el-select></el-form-item><el-form-item label="解决版本"><DefectVersionSelect v-model="form.resolved_version" :meta="meta" :projects="[...new Set(defects.map(item => item.project_code))]" /></el-form-item></template>
      <el-form-item label="操作备注" :required="needsComment"><el-input v-model="form.comment" type="textarea" :rows="3" maxlength="1000" show-word-limit :placeholder="needsComment ? '重新激活必须填写原因' : '可填写本次批量操作说明'" /></el-form-item>
      <div v-if="form.action === 'archive'" class="archive-warning">归档后缺陷将不再出现在普通列表中，但操作历史和数据仍会保留。</div>
    </el-form>
    <template #footer><el-button @click="emit('update:modelValue', false)">取消</el-button><el-button :type="form.action === 'archive' ? 'danger' : 'primary'" :disabled="!canSubmit" :loading="loading" @click="emit('execute', { ...form })">执行批量操作</el-button></template>
  </el-dialog>
</template>

<style scoped>
.batch-hint{display:flex;align-items:center;gap:12px;margin-bottom:18px;padding:13px;border-radius:12px;background:#f8fbff}.batch-hint>span{display:grid;place-items:center;width:38px;height:38px;border-radius:11px;color:#2563eb;background:#dbeafe}.batch-hint strong{color:#334155;font-size:12px}.batch-hint p{margin:4px 0 0;color:#94a3b8;font-size:10px}.batch-form{padding:2px 3px}.full-width{width:100%}.archive-warning{padding:10px 12px;border:1px solid #fecaca;border-radius:9px;color:#b91c1c;background:#fef2f2;font-size:10px;line-height:1.5}
</style>
