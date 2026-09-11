<script setup lang="ts">
import DefectVersionSelect from './DefectVersionSelect.vue'
import { computed, nextTick, ref, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { CirclePlus, Delete, DocumentChecked } from '@element-plus/icons-vue'
import { v4 as uuidv4 } from 'uuid'
import {
  defectToForm,
  defectTypeOptions,
  emptyDefectForm,
  priorityLabels,
  severityLabels,
  type Defect,
  type DefectFormValue,
  type DefectMeta
} from '../types'

const props = defineProps<{
  modelValue: boolean
  defect: Defect | null
  meta: DefectMeta | null
  saving: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  save: [value: DefectFormValue]
}>()

const formRef = ref<FormInstance>()
const form = ref<DefectFormValue>(emptyDefectForm())
const isEdit = computed(() => Boolean(props.defect))
const allowedProject = (code: string) => props.defect?.project_code === code
  ? props.defect.allowed_actions?.edit === true
  : props.meta?.project_actions?.[code]?.create === true
const canSave = computed(() => (!props.defect || props.defect.allowed_actions?.edit === true) && allowedProject(form.value.project_code))
const applicableFields = computed(() => (props.meta?.fields || []).filter((field) => (
  field.enabled && (!field.project_code || field.project_code === form.value.project_code)
)).sort((a, b) => a.sort_order - b.sort_order))

const rules: FormRules<DefectFormValue> = {
  title: [
    { required: true, message: '请输入缺陷标题', trigger: 'blur' },
    { max: 255, message: '标题不能超过 255 个字符', trigger: 'blur' }
  ],
  project_code: [{ required: true, message: '请选择所属项目', trigger: 'change' }],
  defect_type: [{ required: true, message: '请选择缺陷类型', trigger: 'change' }]
}

watch(
  () => props.modelValue,
  async (visible) => {
    if (!visible) return
    form.value = props.defect ? defectToForm(props.defect) : emptyDefectForm()
    await nextTick()
    formRef.value?.clearValidate()
  }
)

function close() {
  emit('update:modelValue', false)
}

function addStep() {
  form.value.steps.push({ id: uuidv4(), content: '' })
}

function removeStep(index: number) {
  if (form.value.steps.length === 1) {
    const firstStep = form.value.steps[0]
    if (firstStep) firstStep.content = ''
    return
  }
  form.value.steps.splice(index, 1)
}

async function submit() {
  if (!canSave.value) return
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  const missingField = applicableFields.value.find((field) => {
    if (!field.required) return false
    const value = form.value.custom_fields[field.field_key]
    return value === undefined || value === null || value === '' || (Array.isArray(value) && value.length === 0)
  })
  if (missingField) {
    ElMessage.warning(`请填写自定义字段“${missingField.name}”`)
    return
  }
  const customFields = Object.fromEntries(applicableFields.value
    .filter((field) => form.value.custom_fields[field.field_key] !== undefined)
    .map((field) => [field.field_key, form.value.custom_fields[field.field_key]]))
  emit('save', {
    ...form.value,
    steps: form.value.steps.map((item) => ({ ...item })),
    environment: { ...form.value.environment },
    tags: [...form.value.tags],
    custom_fields: customFields
  })
}
</script>

<template>
  <el-drawer
    :model-value="modelValue"
    size="min(820px, 94vw)"
    :close-on-click-modal="false"
    class="defect-form-drawer"
    @close="close"
  >
    <template #header>
      <div class="drawer-heading">
        <span class="drawer-heading__icon"><el-icon><DocumentChecked /></el-icon></span>
        <div>
          <strong>{{ isEdit ? '编辑缺陷' : '提交缺陷' }}</strong>
          <p>{{ isEdit ? defect?.defect_no : '完整记录问题现场，帮助处理人快速复现' }}</p>
        </div>
      </div>
    </template>

    <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="defect-form">
      <section class="form-section">
        <div class="form-section__title"><span>01</span><div><strong>基本信息</strong><p>确定缺陷归属和处理优先级</p></div></div>
        <el-form-item label="缺陷标题" prop="title">
          <el-input v-model="form.title" maxlength="255" show-word-limit placeholder="用一句话描述问题现象" />
        </el-form-item>
        <div class="form-grid form-grid--three">
          <el-form-item label="所属项目" prop="project_code">
            <el-select v-model="form.project_code" @change="form.found_version = ''" filterable placeholder="选择项目" class="full-width">
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
            <DefectVersionSelect v-model="form.found_version" edit-testing :meta="meta" :projects="form.project_code ? [form.project_code] : []" :historical="defect?.project_code === form.project_code ? defect?.found_version : undefined" />
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
              <el-option v-for="(label, value) in severityLabels" :key="value" :label="`${value} · ${label}`" :value="Number(value)" />
            </el-select>
          </el-form-item>
          <el-form-item label="优先级">
            <el-select v-model="form.priority" class="full-width">
              <el-option v-for="(label, value) in priorityLabels" :key="value" :label="label" :value="Number(value)" />
            </el-select>
          </el-form-item>
          <el-form-item label="期望解决日期">
            <el-date-picker v-model="form.due_date" type="date" value-format="YYYY-MM-DD" placeholder="选择日期" class="full-width" />
          </el-form-item>
        </div>
        <div class="form-grid form-grid--two">
          <el-form-item label="处理人">
            <el-select v-model="form.assignee_id" filterable clearable placeholder="暂不指派" class="full-width">
              <el-option
                v-for="account in meta?.accounts || []"
                :key="account.id"
                :label="account.nickname ? `${account.nickname} (${account.username})` : account.username"
                :value="account.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="验证人">
            <el-select v-model="form.verifier_id" filterable clearable placeholder="解决后再指定也可以" class="full-width">
              <el-option
                v-for="account in meta?.accounts || []"
                :key="account.id"
                :label="account.nickname ? `${account.nickname} (${account.username})` : account.username"
                :value="account.id"
              />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item label="标签">
          <el-select v-model="form.tags" multiple filterable allow-create default-first-option placeholder="输入后回车添加标签" class="full-width" />
        </el-form-item>
      </section>

      <section class="form-section">
        <div class="form-section__title"><span>02</span><div><strong>问题现场</strong><p>记录复现路径、实际结果与预期结果</p></div></div>
        <el-form-item label="前置条件">
          <el-input v-model="form.precondition" type="textarea" :rows="2" placeholder="账号状态、配置条件或依赖数据" />
        </el-form-item>
        <el-form-item label="重现步骤">
          <div class="step-list">
            <div v-for="(step, index) in form.steps" :key="step.id" class="step-row">
              <span class="step-row__number">{{ index + 1 }}</span>
              <el-input v-model="step.content" type="textarea" :autosize="{ minRows: 1, maxRows: 4 }" :placeholder="`第 ${index + 1} 步`" />
              <el-button text :icon="Delete" class="step-row__delete" @click="removeStep(index)" />
            </div>
            <el-button plain :icon="CirclePlus" class="add-step" @click="addStep">增加步骤</el-button>
          </div>
        </el-form-item>
        <div class="form-grid form-grid--two">
          <el-form-item label="实际结果">
            <el-input v-model="form.actual_result" type="textarea" :rows="4" placeholder="实际发生了什么" />
          </el-form-item>
          <el-form-item label="预期结果">
            <el-input v-model="form.expected_result" type="textarea" :rows="4" placeholder="正确行为应该是什么" />
          </el-form-item>
        </div>
        <el-form-item label="补充说明">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="影响范围、发生频率或其他说明" />
        </el-form-item>
      </section>

      <section class="form-section">
        <div class="form-section__title"><span>03</span><div><strong>环境信息</strong><p>帮助处理人还原相同测试环境</p></div></div>
        <div class="form-grid form-grid--three">
          <el-form-item label="平台">
            <el-select v-model="form.environment.platform" clearable placeholder="选择平台" class="full-width">
              <el-option v-for="item in ['iOS', 'Android', 'Web', '服务端', '其他']" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
          <el-form-item label="测试环境"><el-input v-model="form.environment.environment" placeholder="测试/预发/生产" /></el-form-item>
          <el-form-item label="App 版本"><el-input v-model="form.environment.app_version" placeholder="例如 2.65.0 (2818)" /></el-form-item>
          <el-form-item label="系统版本"><el-input v-model="form.environment.os_version" placeholder="例如 iOS 27.0" /></el-form-item>
          <el-form-item label="设备型号"><el-input v-model="form.environment.device_model" placeholder="例如 iPhone16,1" /></el-form-item>
          <el-form-item label="网络环境"><el-input v-model="form.environment.network" placeholder="Wi-Fi / 5G" /></el-form-item>
        </div>
      </section>

      <section v-if="applicableFields.length" class="form-section">
        <div class="form-section__title"><span>04</span><div><strong>自定义字段</strong><p>按当前项目的字段模板补充信息</p></div></div>
        <div class="form-grid form-grid--two">
          <el-form-item v-for="field in applicableFields" :key="field.id" :label="field.name" :required="field.required" :class="{ 'form-field--wide': field.field_type === 'textarea' }">
            <el-input v-if="field.field_type === 'text' || field.field_type === 'url'" v-model="form.custom_fields[field.field_key]" :placeholder="field.field_type === 'url' ? 'https://...' : `请输入${field.name}`" />
            <el-input v-else-if="field.field_type === 'textarea'" v-model="form.custom_fields[field.field_key]" type="textarea" :rows="3" :placeholder="`请输入${field.name}`" />
            <el-input-number v-else-if="field.field_type === 'number'" v-model="form.custom_fields[field.field_key]" class="full-width" controls-position="right" />
            <el-select v-else-if="field.field_type === 'select'" v-model="form.custom_fields[field.field_key]" clearable class="full-width" :placeholder="`选择${field.name}`"><el-option v-for="option in field.options" :key="option" :label="option" :value="option" /></el-select>
            <el-select v-else-if="field.field_type === 'multi_select'" v-model="form.custom_fields[field.field_key]" multiple clearable class="full-width" :placeholder="`选择${field.name}`"><el-option v-for="option in field.options" :key="option" :label="option" :value="option" /></el-select>
            <el-date-picker v-else-if="field.field_type === 'date'" v-model="form.custom_fields[field.field_key]" type="date" value-format="YYYY-MM-DD" class="full-width" />
            <el-select v-else-if="field.field_type === 'user'" v-model="form.custom_fields[field.field_key]" filterable clearable class="full-width" :placeholder="`选择${field.name}`"><el-option v-for="account in meta?.accounts || []" :key="account.id" :label="account.nickname || account.username" :value="account.id" /></el-select>
            <el-switch v-else-if="field.field_type === 'boolean'" v-model="form.custom_fields[field.field_key]" inline-prompt active-text="是" inactive-text="否" />
          </el-form-item>
        </div>
      </section>
    </el-form>

    <template #footer>
      <div class="drawer-footer">
        <span>保存后可在详情中补充附件和评论</span>
        <div><el-button @click="close">取消</el-button><el-button type="primary" :disabled="!canSave" :loading="saving" @click="submit">{{ isEdit ? '保存修改' : '提交缺陷' }}</el-button></div>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.drawer-heading{display:flex;align-items:center;gap:12px}.drawer-heading__icon{display:grid;place-items:center;width:42px;height:42px;border-radius:13px;color:#2563eb;background:#eff6ff}.drawer-heading strong{font-size:18px;color:#0f172a}.drawer-heading p{margin:4px 0 0;color:#94a3b8;font-size:12px}.defect-form{padding:0 4px 18px}.form-section{padding:22px;margin-bottom:18px;border:1px solid #e8edf5;border-radius:16px;background:#fff;box-shadow:0 10px 30px rgba(15,23,42,.035)}.form-section__title{display:flex;align-items:center;gap:12px;margin-bottom:22px}.form-section__title>span{display:grid;place-items:center;width:34px;height:34px;border-radius:10px;background:#f1f5f9;color:#3b82f6;font-weight:800;font-size:12px}.form-section__title strong{display:block;color:#172033;font-size:15px}.form-section__title p{margin:3px 0 0;color:#94a3b8;font-size:12px}.form-grid{display:grid;gap:14px}.form-grid--three{grid-template-columns:repeat(3,minmax(0,1fr))}.form-grid--two{grid-template-columns:repeat(2,minmax(0,1fr))}.full-width{width:100%}.step-list{display:flex;flex-direction:column;gap:10px;width:100%}.step-row{display:grid;grid-template-columns:30px minmax(0,1fr) 32px;align-items:start;gap:9px}.step-row__number{display:grid;place-items:center;height:32px;border-radius:9px;background:#eff6ff;color:#2563eb;font-size:12px;font-weight:800}.step-row__delete{color:#94a3b8}.step-row__delete:hover{color:#ef4444}.add-step{align-self:flex-start;border-style:dashed;color:#2563eb}.drawer-footer{display:flex;align-items:center;justify-content:space-between;gap:20px;width:100%}.drawer-footer>span{color:#94a3b8;font-size:12px}@media(max-width:760px){.form-grid--three,.form-grid--two{grid-template-columns:1fr}.form-section{padding:16px}.drawer-footer>span{display:none}}
.form-field--wide{grid-column:1/-1}
</style>
