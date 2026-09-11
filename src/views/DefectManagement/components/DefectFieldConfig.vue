<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { EditPen, Lock, Plus, Refresh, Switch } from '@element-plus/icons-vue'
import DefectVersionConfig from './DefectVersionConfig.vue'
import { defectApi } from '../api'
import type { DefectFieldDefinition, DefectMeta } from '../types'

const props = defineProps<{ meta: DefectMeta | null }>()
const emit = defineEmits<{ changed: [] }>()
const loading = ref(false)
const saving = ref(false)
const fields = ref<DefectFieldDefinition[]>([])
const dialogVisible = ref(false)
const editingID = ref('')

const builtins = [
  ['title', '缺陷标题', '单行文本'], ['project_code', '所属项目', '项目'], ['found_version', '发现版本', '项目版本选择'],
  ['defect_type', '缺陷类型', '单选'], ['severity', '严重程度', '单选'], ['priority', '优先级', '单选'],
  ['assignee_id', '处理人', '用户'], ['steps', '重现步骤', '步骤列表'], ['actual_result', '实际结果', '多行文本'],
  ['expected_result', '预期结果', '多行文本']
]

const fieldTypeOptions = [
  { value: 'text', label: '单行文本' }, { value: 'textarea', label: '多行文本' }, { value: 'number', label: '数字' },
  { value: 'select', label: '单选' }, { value: 'multi_select', label: '多选' }, { value: 'date', label: '日期' },
  { value: 'user', label: '平台用户' }, { value: 'boolean', label: '是/否' }, { value: 'url', label: '链接' }
]

const blankField = (): DefectFieldDefinition => ({
  id: '', field_key: '', name: '', field_type: 'text', project_code: '', required: false,
  enabled: true, list_visible: false, filterable: false, options: [], sort_order: fields.value.length + 1
})
const form = reactive<DefectFieldDefinition>(blankField())

function message(error: any, fallback: string) { return error?.response?.data?.error || error?.customMessage || fallback }

async function load() {
  loading.value = true
  try { fields.value = await defectApi.fields() } catch (error) { ElMessage.error(message(error, '字段配置加载失败')) } finally { loading.value = false }
}

function openCreate() {
  editingID.value = ''
  Object.assign(form, blankField())
  dialogVisible.value = true
}

function openEdit(field: DefectFieldDefinition) {
  editingID.value = field.id
  Object.assign(form, { ...field, options: [...field.options] })
  dialogVisible.value = true
}

async function save() {
  if (!form.name.trim() || !form.field_key.trim()) {
    ElMessage.warning('请填写字段名称和字段标识')
    return
  }
  saving.value = true
  try {
    if (editingID.value) await defectApi.updateField(editingID.value, { ...form, options: [...form.options] })
    else await defectApi.createField({ ...form, options: [...form.options] })
    ElMessage.success(editingID.value ? '字段配置已更新' : '自定义字段已创建')
    dialogVisible.value = false
    await load()
    emit('changed')
  } catch (error) { ElMessage.error(message(error, '字段保存失败')) } finally { saving.value = false }
}

async function toggle(field: DefectFieldDefinition) {
  try {
    await defectApi.updateField(field.id, { ...field, enabled: !field.enabled, options: [...field.options] })
    ElMessage.success(field.enabled ? '字段已停用，历史数据仍保留' : '字段已启用')
    await load()
    emit('changed')
  } catch (error) { ElMessage.error(message(error, '字段状态更新失败')) }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="field-config">
    <header class="config-header"><div><span class="config-header__icon"><el-icon><Switch /></el-icon></span><div><strong>字段配置</strong><p>核心字段保持稳定，自定义字段可按全部项目或指定项目生效</p></div></div><div><el-button :icon="Refresh" @click="load">刷新</el-button><el-button type="primary" :icon="Plus" @click="openCreate">新增字段</el-button></div></header>

    <article class="config-card">
      <div class="card-title"><div><strong>核心字段</strong><span>系统生命周期依赖，不能停用或删除</span></div><el-tag type="info" effect="plain" round>系统保护</el-tag></div>
      <div class="builtin-grid"><div v-for="item in builtins" :key="item[0]"><span><el-icon><Lock /></el-icon></span><div><strong>{{ item[1] }}</strong><small>{{ item[0] }} · {{ item[2] }}</small></div></div></div>
    </article>

    <article class="config-card">
      <div class="card-title"><div><strong>自定义字段</strong><span>{{ fields.length }} 个字段配置</span></div></div>
      <div v-if="fields.length" class="custom-list">
        <div v-for="field in fields" :key="field.id" class="field-row" :class="{ 'is-disabled': !field.enabled }">
          <span class="field-row__type">{{ fieldTypeOptions.find(item => item.value === field.field_type)?.label }}</span>
          <div class="field-row__main"><div><strong>{{ field.name }}</strong><code>{{ field.field_key }}</code></div><p>{{ field.project_code ? `仅 ${field.project_code} 生效` : '全部项目生效' }}<template v-if="field.options.length"> · {{ field.options.join(' / ') }}</template></p></div>
          <div class="field-row__flags"><el-tag v-if="field.required" size="small" type="danger" effect="plain">必填</el-tag><el-tag v-if="field.list_visible" size="small" effect="plain">列表显示</el-tag><el-tag v-if="field.filterable" size="small" type="success" effect="plain">可筛选</el-tag><el-tag v-if="!field.enabled" size="small" type="info">已停用</el-tag></div>
          <div class="field-row__actions"><el-button text :icon="EditPen" @click="openEdit(field)">编辑</el-button><el-button text :type="field.enabled ? 'warning' : 'success'" @click="toggle(field)">{{ field.enabled ? '停用' : '启用' }}</el-button></div>
        </div>
      </div>
      <div v-else class="config-empty"><span><el-icon><Switch /></el-icon></span><strong>还没有自定义字段</strong><p>按项目需要增加发布渠道、业务模块、复现概率等字段</p><el-button type="primary" plain :icon="Plus" @click="openCreate">创建第一个字段</el-button></div>
    </article>

    <DefectVersionConfig :meta="meta" @changed="emit('changed')" />

    <el-dialog v-model="dialogVisible" :title="editingID ? '编辑自定义字段' : '新增自定义字段'" width="600px" destroy-on-close>
      <el-form label-position="top">
        <div class="form-grid"><el-form-item label="字段名称" required><el-input v-model="form.name" placeholder="例如 复现概率" /></el-form-item><el-form-item label="字段标识" required><el-input v-model="form.field_key" :disabled="Boolean(editingID)" placeholder="例如 reproduce_rate" /></el-form-item></div>
        <div class="form-grid"><el-form-item label="字段类型" required><el-select v-model="form.field_type" class="full-width"><el-option v-for="item in fieldTypeOptions" :key="item.value" :label="item.label" :value="item.value" /></el-select></el-form-item><el-form-item label="生效项目"><el-select v-model="form.project_code" clearable filterable class="full-width" placeholder="全部项目"><el-option v-for="project in props.meta?.projects || []" :key="project.id" :label="`${project.project_code} · ${project.project_name}`" :value="project.project_code" /></el-select></el-form-item></div>
        <el-form-item v-if="['select','multi_select'].includes(form.field_type)" label="字段选项" required><el-select v-model="form.options" multiple filterable allow-create default-first-option class="full-width" placeholder="输入选项后按回车" /></el-form-item>
        <el-form-item label="显示顺序"><el-input-number v-model="form.sort_order" :min="0" :max="999" controls-position="right" /></el-form-item>
        <div class="switch-grid"><label><el-switch v-model="form.required" /><span><strong>提交时必填</strong><small>未填写时禁止保存</small></span></label><label><el-switch v-model="form.list_visible" /><span><strong>列表显示</strong><small>作为缺陷列表动态列</small></span></label><label><el-switch v-model="form.filterable" /><span><strong>支持筛选</strong><small>出现在精细筛选区域</small></span></label><label><el-switch v-model="form.enabled" /><span><strong>立即启用</strong><small>停用不会删除历史值</small></span></label></div>
      </el-form>
      <template #footer><el-button @click="dialogVisible=false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存字段</el-button></template>
    </el-dialog>
  </section>
</template>

<style scoped>
.field-config{display:flex;flex-direction:column;gap:14px}.config-header,.config-card{padding:19px;border:1px solid #e8edf5;border-radius:15px;background:#fff}.config-header{display:flex;align-items:center;justify-content:space-between}.config-header>div{display:flex;align-items:center;gap:11px}.config-header__icon{display:grid;place-items:center;width:40px;height:40px;border-radius:12px;color:#7c3aed;background:#f5f3ff}.config-header strong,.card-title strong{color:#172033;font-size:15px}.config-header p,.card-title span{margin:4px 0 0;color:#94a3b8;font-size:11px}.card-title{display:flex;align-items:center;justify-content:space-between;margin-bottom:17px}.card-title>div{display:flex;flex-direction:column}.builtin-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:9px}.builtin-grid>div{display:flex;align-items:center;gap:8px;padding:11px;border-radius:11px;background:#f8fafc}.builtin-grid>div>span{color:#94a3b8}.builtin-grid strong,.builtin-grid small{display:block}.builtin-grid strong{color:#475569;font-size:11px}.builtin-grid small{margin-top:3px;color:#94a3b8;font-size:8px}.custom-list{display:flex;flex-direction:column}.field-row{display:grid;grid-template-columns:88px minmax(180px,1fr) minmax(200px,auto) 130px;align-items:center;gap:14px;padding:13px 4px;border-top:1px solid #eef2f7}.field-row:first-child{border-top:0}.field-row.is-disabled{opacity:.55}.field-row__type{display:inline-flex;justify-content:center;padding:6px 8px;border-radius:8px;color:#7c3aed;background:#f5f3ff;font-size:10px}.field-row__main>div{display:flex;align-items:center;gap:8px}.field-row__main strong{color:#334155;font-size:12px}.field-row__main code{padding:2px 5px;border-radius:5px;color:#64748b;background:#f1f5f9;font-size:9px}.field-row__main p{margin:5px 0 0;color:#94a3b8;font-size:10px}.field-row__flags{display:flex;flex-wrap:wrap;gap:5px}.field-row__actions{display:flex;justify-content:flex-end}.config-empty{display:flex;align-items:center;flex-direction:column;padding:42px}.config-empty>span{display:grid;place-items:center;width:48px;height:48px;border-radius:14px;color:#8b5cf6;background:#f5f3ff;font-size:20px}.config-empty strong{margin-top:12px;color:#475569}.config-empty p{margin:5px 0 14px;color:#94a3b8;font-size:11px}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.full-width{width:100%}.switch-grid{display:grid;grid-template-columns:1fr 1fr;gap:10px}.switch-grid label{display:flex;align-items:center;gap:10px;padding:12px;border:1px solid #e8edf5;border-radius:11px}.switch-grid span{display:flex;flex-direction:column}.switch-grid strong{color:#475569;font-size:11px}.switch-grid small{margin-top:2px;color:#94a3b8;font-size:9px}@media(max-width:1000px){.builtin-grid{grid-template-columns:repeat(3,1fr)}.field-row{grid-template-columns:80px 1fr 130px}.field-row__flags{display:none}}@media(max-width:680px){.builtin-grid{grid-template-columns:repeat(2,1fr)}.form-grid,.switch-grid{grid-template-columns:1fr}.field-row{grid-template-columns:1fr}.field-row__actions{justify-content:flex-start}}
</style>
