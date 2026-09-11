<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { defectApi } from '../api'
import type { DefectMeta } from '../types'
const props = defineProps<{ modelValue?: string; projects: string[]; meta: DefectMeta | null; historical?: string; filter?: boolean; editTesting?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; change: [] }>()
const configured = computed(() => props.projects.filter((project) => Object.prototype.hasOwnProperty.call(props.meta?.project_versions || {}, project)))
const options = computed(() => {
  const lists = props.meta?.project_versions || {}
  if (props.filter) return [...new Set((props.projects.length ? props.projects : Object.keys(lists)).flatMap((project) => lists[project] || []))]
  const first = configured.value[0]
  return first ? (lists[first] || []).filter((version) => configured.value.every((project) => lists[project]?.includes(version))) : []
})
const allowCreate = computed(() => props.filter || !configured.value.length)
function update(value: string) { emit('update:modelValue', value || ''); emit('change') }
const editing = ref(false)
const draft = ref('')
const saving = ref(false)
const editProject = ref('')
const original = ref({ online: '', testing: '' })
const project = computed(() => props.projects[0] || '')
const stages = computed(() => props.meta?.project_version_stages?.[project.value])
const canEdit = computed(() => props.editTesting && props.projects.length === 1 && props.meta?.project_actions?.[project.value]?.create && stages.value)
function openEdit() {
  if (!canEdit.value || !stages.value) return
  editProject.value = project.value
  original.value = { online: stages.value.online, testing: stages.value.testing }
  draft.value = stages.value.testing
  editing.value = true
}
async function saveTesting() {
  if (!draft.value.trim()) return
  saving.value = true
  try {
    const result = await defectApi.updateTestingVersion(editProject.value, { testing: draft.value.trim(), expected_online: original.value.online, expected_testing: original.value.testing })
    if (props.meta) {
      props.meta.project_version_stages ||= {}
      props.meta.project_versions ||= {}
      props.meta.project_version_stages[editProject.value] = result
      props.meta.project_versions[editProject.value] = result.versions
    }
    if (project.value === editProject.value) update(result.testing)
    editing.value = false
    ElMessage.success('提测版本已同步到项目配置')
  } catch (error: any) { ElMessage.error(error?.response?.data?.error || '提测版本更新失败') }
  finally { saving.value = false }
}
</script>

<template>
  <el-select :model-value="modelValue" filterable clearable :allow-create="allowCreate" default-first-option style="width:100%" :placeholder="allowCreate ? '选择或输入版本号' : '选择项目版本号'" @update:model-value="update">
    <el-option v-for="version in options" :key="version" :label="version" :value="version">
      <span>{{ version }}{{ version === stages?.testing ? '（提测）' : version === stages?.online ? '（线上）' : '' }}</span>
      <el-button v-if="canEdit && version === stages?.testing" text type="primary" size="small" style="float:right" @click.stop="openEdit">编辑</el-button>
    </el-option>
    <el-option v-if="historical && !options.includes(historical)" :value="historical" :label="`${historical}（历史版本）`" />
    <template #empty><div style="padding:14px;color:#94a3b8;text-align:center">暂无可选版本，请在字段配置中维护</div></template>
    <template v-if="canEdit && !stages?.testing" #footer><el-button text type="primary" @click="openEdit">设置提测版本</el-button></template>
  </el-select>
  <el-dialog v-model="editing" title="编辑项目提测版本" width="420px" append-to-body :close-on-click-modal="!saving" :show-close="!saving">
    <p style="color:#64748b;margin-top:0">{{ editProject }} · 确认后将更新该项目的提测版本，当前线上版本不变。</p>
    <el-input v-model="draft" maxlength="128" :disabled="saving" placeholder="输入提测版本" @keyup.enter="!saving && saveTesting()" />
    <template #footer><el-button :disabled="saving" @click="editing = false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!draft.trim()" @click="saveTesting">确认更新</el-button></template>
  </el-dialog>
</template>
