<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import { EditPen, Lock, Plus, Refresh, Switch } from "@element-plus/icons-vue"
import DefectVersionConfig from "./DefectVersionConfig.vue"
import { defectApi } from "../api"
import type { DefectFieldDefinition, DefectMeta } from "../types"

const props = defineProps<{ meta: DefectMeta | null }>()
const emit = defineEmits<{ changed: [] }>()
const loading = ref(false)
const saving = ref(false)
const fields = ref<DefectFieldDefinition[]>([])
const dialogVisible = ref(false)
const editingID = ref("")

const builtins = [
  ["title", "缺陷标题", "单行文本"],
  ["project_code", "所属项目", "项目"],
  ["found_version", "发现版本", "项目版本选择"],
  ["defect_type", "缺陷类型", "单选"],
  ["severity", "严重程度", "单选"],
  ["priority", "优先级", "单选"],
  ["assignee_id", "处理人", "用户"],
  ["steps", "重现步骤", "步骤列表"],
  ["actual_result", "实际结果", "多行文本"],
  ["expected_result", "预期结果", "多行文本"]
]

const fieldTypeOptions = [
  { value: "text", label: "单行文本" },
  { value: "textarea", label: "多行文本" },
  { value: "number", label: "数字" },
  { value: "select", label: "单选" },
  { value: "multi_select", label: "多选" },
  { value: "date", label: "日期" },
  { value: "user", label: "平台用户" },
  { value: "boolean", label: "是/否" },
  { value: "url", label: "链接" }
]

const blankField = (): DefectFieldDefinition => ({
  id: "",
  field_key: "",
  name: "",
  field_type: "text",
  project_code: "",
  required: false,
  enabled: true,
  list_visible: false,
  filterable: false,
  options: [],
  sort_order: fields.value.length + 1
})
const form = reactive<DefectFieldDefinition>(blankField())

function message(error: any, fallback: string) {
  return error?.response?.data?.error || error?.customMessage || fallback
}

async function load() {
  loading.value = true
  try {
    fields.value = await defectApi.fields()
  } catch (error) {
    ElMessage.error(message(error, "字段配置加载失败"))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingID.value = ""
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
    ElMessage.warning("请填写字段名称和字段标识")
    return
  }
  saving.value = true
  try {
    if (editingID.value) await defectApi.updateField(editingID.value, { ...form, options: [...form.options] })
    else await defectApi.createField({ ...form, options: [...form.options] })
    ElMessage.success(editingID.value ? "字段配置已更新" : "自定义字段已创建")
    dialogVisible.value = false
    await load()
    emit("changed")
  } catch (error) {
    ElMessage.error(message(error, "字段保存失败"))
  } finally {
    saving.value = false
  }
}

async function toggle(field: DefectFieldDefinition) {
  try {
    await defectApi.updateField(field.id, { ...field, enabled: !field.enabled, options: [...field.options] })
    ElMessage.success(field.enabled ? "字段已停用，历史数据仍保留" : "字段已启用")
    await load()
    emit("changed")
  } catch (error) {
    ElMessage.error(message(error, "字段状态更新失败"))
  }
}

onMounted(load)
</script>

<template>
  <section v-loading="loading" class="field-config">
    <!-- 头部工具栏 -->
    <header class="config-header">
      <div class="config-header__title">
        <span class="config-header__icon"><el-icon><Switch /></el-icon></span>
        <div>
          <strong>字段与属性配置</strong>
          <p>核心字段规范基础数据，自定义字段可按全局或指定项目按需扩展</p>
        </div>
      </div>
      <div class="config-header__actions">
        <el-button :icon="Refresh" @click="load">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate">新增字段</el-button>
      </div>
    </header>

    <!-- 系统内置核心字段 -->
    <article class="config-card">
      <div class="card-title">
        <div>
          <strong>系统核心字段</strong>
          <span>生命周期与统计强依赖字段，由系统底层托管</span>
        </div>
        <el-tag type="info" effect="plain" round>系统预置</el-tag>
      </div>
      <div class="builtin-grid">
        <div v-for="item in builtins" :key="item[0]" class="builtin-item">
          <span class="builtin-icon"><el-icon><Lock /></el-icon></span>
          <div class="builtin-info">
            <strong>{{ item[1] }}</strong>
            <small>{{ item[0] }} · {{ item[2] }}</small>
          </div>
        </div>
      </div>
    </article>

    <!-- 自定义扩展字段 -->
    <article class="config-card">
      <div class="card-title">
        <div>
          <strong>项目自定义字段</strong>
          <span>共 {{ fields.length }} 个已配置扩展字段</span>
        </div>
      </div>
      <div v-if="fields.length" class="custom-list">
        <div
          v-for="field in fields"
          :key="field.id"
          class="field-row"
          :class="{ 'is-disabled': !field.enabled }"
        >
          <span class="field-row__type">
            {{ fieldTypeOptions.find(item => item.value === field.field_type)?.label }}
          </span>
          <div class="field-row__main">
            <div class="field-name-wrap">
              <strong>{{ field.name }}</strong>
              <code class="field-key">{{ field.field_key }}</code>
            </div>
            <p class="field-scope">
              {{ field.project_code ? `仅限项目 ${field.project_code} 生效` : '全部项目通用' }}
              <template v-if="field.options.length"> · 候选值: {{ field.options.join(' / ') }}</template>
            </p>
          </div>
          <div class="field-row__flags">
            <el-tag v-if="field.required" size="small" type="danger" effect="plain">必填</el-tag>
            <el-tag v-if="field.list_visible" size="small" effect="plain">表格列显示</el-tag>
            <el-tag v-if="field.filterable" size="small" type="success" effect="plain">支持筛选</el-tag>
            <el-tag v-if="!field.enabled" size="small" type="info">已停用</el-tag>
          </div>
          <div class="field-row__actions">
            <el-button text :icon="EditPen" @click="openEdit(field)">编辑</el-button>
            <el-button text :type="field.enabled ? 'warning' : 'success'" @click="toggle(field)">
              {{ field.enabled ? '停用' : '启用' }}
            </el-button>
          </div>
        </div>
      </div>
      <div v-else class="config-empty">
        <span class="empty-icon"><el-icon><Switch /></el-icon></span>
        <strong>暂无自定义字段</strong>
        <p>可按业务需求增加发布渠道、复现概率、设备环境等个性化字段</p>
        <el-button type="primary" plain :icon="Plus" @click="openCreate">创建第一个自定义字段</el-button>
      </div>
    </article>

    <!-- 版本号配置 -->
    <DefectVersionConfig :meta="meta" @changed="emit('changed')" />

    <!-- 字段编辑弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="editingID ? '编辑自定义字段' : '新增自定义字段'"
      width="620px"
      destroy-on-close
      class="defect-field-dialog"
    >
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item label="字段名称" required>
            <el-input v-model="form.name" placeholder="例如 复现概率" />
          </el-form-item>
          <el-form-item label="字段标识 (Key)" required>
            <el-input v-model="form.field_key" :disabled="Boolean(editingID)" placeholder="例如 reproduce_rate" />
          </el-form-item>
        </div>
        <div class="form-grid">
          <el-form-item label="字段类型" required>
            <el-select v-model="form.field_type" class="full-width">
              <el-option v-for="item in fieldTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="生效项目范围">
            <el-select v-model="form.project_code" clearable filterable class="full-width" placeholder="全部项目通用">
              <el-option
                v-for="project in props.meta?.projects || []"
                :key="project.id"
                :label="`${project.project_code} · ${project.project_name}`"
                :value="project.project_code"
              />
            </el-select>
          </el-form-item>
        </div>
        <el-form-item v-if="['select', 'multi_select'].includes(form.field_type)" label="下拉候选值" required>
          <el-select v-model="form.options" multiple filterable allow-create default-first-option class="full-width" placeholder="输入选项值后回车添加" />
        </el-form-item>
        <el-form-item label="显示排序">
          <el-input-number v-model="form.sort_order" :min="0" :max="999" controls-position="right" />
        </el-form-item>
        <div class="switch-grid">
          <label class="switch-card">
            <el-switch v-model="form.required" />
            <div class="switch-text">
              <strong>提交时必填</strong>
              <small>未填写时禁止保存缺陷记录</small>
            </div>
          </label>
          <label class="switch-card">
            <el-switch v-model="form.list_visible" />
            <div class="switch-text">
              <strong>列表动态列显示</strong>
              <small>在主表格中作为可展开或展示列</small>
            </div>
          </label>
          <label class="switch-card">
            <el-switch v-model="form.filterable" />
            <div class="switch-text">
              <strong>支持筛选项</strong>
              <small>出现在高级筛选区域供精准检索</small>
            </div>
          </label>
          <label class="switch-card">
            <el-switch v-model="form.enabled" />
            <div class="switch-text">
              <strong>立即启用</strong>
              <small>停用后仅隐藏，保留历史缺陷数据</small>
            </div>
          </label>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="save">保存配置</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped>
.field-config {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.config-header,
.config-card {
  padding: 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.config-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.config-header__title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.config-header__icon {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  color: #7c3aed;
  background: #f5f3ff;
  border: 1px solid #ddd6fe;
  font-size: 20px;
}

.config-header strong,
.card-title strong {
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
}

.config-header p,
.card-title span {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 13px;
}

.config-header__actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.card-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.card-title > div {
  display: flex;
  flex-direction: column;
}

.builtin-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 10px;
}

.builtin-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid #f1f5f9;
}

.builtin-icon {
  color: #94a3b8;
  display: flex;
  font-size: 16px;
}

.builtin-info strong,
.builtin-info small {
  display: block;
}

.builtin-info strong {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.builtin-info small {
  margin-top: 3px;
  color: #94a3b8;
  font-size: 11px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.custom-list {
  display: flex;
  flex-direction: column;
}

.field-row {
  display: grid;
  grid-template-columns: 90px minmax(180px, 1fr) minmax(200px, auto) 140px;
  align-items: center;
  gap: 16px;
  padding: 14px 6px;
  border-top: 1px solid #f1f5f9;
}

.field-row:first-child {
  border-top: 0;
}

.field-row.is-disabled {
  opacity: 0.55;
}

.field-row__type {
  display: inline-flex;
  justify-content: center;
  padding: 5px 10px;
  border-radius: 6px;
  color: #7c3aed;
  background: #f5f3ff;
  border: 1px solid #ede9fe;
  font-size: 12px;
  font-weight: 600;
}

.field-name-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.field-row__main strong {
  color: #0f172a;
  font-size: 13px;
  font-weight: 600;
}

.field-key {
  padding: 2px 7px;
  border-radius: 5px;
  color: #475569;
  background: #f1f5f9;
  font-size: 11px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.field-scope {
  margin: 5px 0 0;
  color: #64748b;
  font-size: 12px;
}

.field-row__flags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.field-row__actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

.config-empty {
  display: flex;
  align-items: center;
  flex-direction: column;
  padding: 40px 20px;
}

.empty-icon {
  display: grid;
  place-items: center;
  width: 50px;
  height: 50px;
  border-radius: 14px;
  color: #8b5cf6;
  background: #f5f3ff;
  font-size: 24px;
}

.config-empty strong {
  margin-top: 14px;
  color: #334155;
  font-size: 15px;
}

.config-empty p {
  margin: 6px 0 16px;
  color: #94a3b8;
  font-size: 13px;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.full-width {
  width: 100%;
}

.switch-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-top: 6px;
}

.switch-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #f8fafc;
  cursor: pointer;
}

.switch-text {
  display: flex;
  flex-direction: column;
}

.switch-text strong {
  color: #334155;
  font-size: 13px;
  font-weight: 600;
}

.switch-text small {
  margin-top: 2px;
  color: #94a3b8;
  font-size: 11px;
}

@media (max-width: 1000px) {
  .builtin-grid {
    grid-template-columns: repeat(3, 1fr);
  }
  .field-row {
    grid-template-columns: 80px 1fr 140px;
  }
  .field-row__flags {
    display: none;
  }
}

@media (max-width: 680px) {
  .builtin-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .form-grid,
  .switch-grid {
    grid-template-columns: 1fr;
  }
  .field-row {
    grid-template-columns: 1fr;
  }
  .field-row__actions {
    justify-content: flex-start;
  }
}
</style>
