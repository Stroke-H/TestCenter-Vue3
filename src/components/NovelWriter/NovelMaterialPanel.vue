<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { NovelMaterials } from '@/api/novelWriter'

const props = defineProps<{
  modelValue: NovelMaterials
  saving: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: NovelMaterials]
  save: []
  outline: []
  style: []
}>()

const form = reactive<NovelMaterials>({ ...props.modelValue })
const referenceDialogVisible = shallowRef(false)
const referenceDraft = shallowRef('')
const hasReference = computed(() => Boolean(form.reference_raw.trim()))

watch(
  () => props.modelValue,
  (value) => Object.assign(form, value),
  { deep: true }
)

watch(
  form,
  () => emit('update:modelValue', { ...form }),
  { deep: true }
)

const openReferenceDialog = () => {
  referenceDraft.value = form.reference_raw || ''
  referenceDialogVisible.value = true
}

const applyReferenceText = () => {
  form.reference_raw = referenceDraft.value
  referenceDialogVisible.value = false
  ElMessage.success(form.reference_raw.trim() ? '文风参考文本已添加' : '已清空文风参考文本')
}

const handleReferenceFile = (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!file.name.toLowerCase().endsWith('.txt')) {
    ElMessage.warning('目前仅支持上传 txt 文本文件')
    return
  }

  const reader = new FileReader()
  reader.onload = () => {
    referenceDraft.value = String(reader.result || '')
    form.reference_raw = referenceDraft.value
    ElMessage.success('已读取 txt 文件内容作为文风参考')
  }
  reader.onerror = () => ElMessage.error('读取文件失败，请重试')
  reader.readAsText(file)
}

defineExpose({
  openReferenceDialog
})
</script>

<template>
  <section class="material-panel">
    <div class="panel-header">
      <div>
        <p class="panel-kicker">Step 2</p>
        <div class="title-line">
          <h3 class="panel-title">文风生成准备</h3>
          <span :class="['reference-status', { 'reference-status--ready': hasReference }]">
            {{ hasReference ? '已提供参考' : '无参考' }}
          </span>
        </div>
        <p class="panel-desc">文风参考不是必填项；不添加时，系统会让大模型自由选择适合题材的文风。</p>
      </div>
      <div class="panel-actions">
        <el-button :loading="saving" @click="emit('save')">保存素材</el-button>
        <el-button @click="openReferenceDialog">
          {{ hasReference ? '编辑文风参考文本' : '添加文风参考文本' }}
        </el-button>
        <el-button type="primary" @click="emit('style')">生成文风画像</el-button>
      </div>
    </div>

    <div class="flow-actions">
      <el-button @click="emit('outline')">生成大纲/章节结构</el-button>
    </div>

    <el-dialog v-model="referenceDialogVisible" title="添加文风参考文本" width="720px">
      <div class="reference-dialog">
        <div class="upload-line">
          <el-button>
            <label class="file-label">
              上传 txt 文件
              <input type="file" accept=".txt,text/plain" @change="handleReferenceFile">
            </label>
          </el-button>
          <span>上传后会读取文件全部内容作为参考文本。</span>
        </div>
        <el-input
          v-model="referenceDraft"
          type="textarea"
          :rows="12"
          placeholder="也可以直接粘贴参考小说片段。系统只提炼抽象文风规则，不复刻原文句子、人物、设定或情节。"
        />
      </div>
      <template #footer>
        <el-button @click="referenceDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="applyReferenceText">确认使用</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped>
.material-panel {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 24px;
  padding: 22px;
}

.panel-header,
.flow-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.panel-header {
  margin-bottom: 18px;
}

.panel-kicker {
  margin: 0 0 4px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 800;
}

.panel-title {
  margin: 0;
  color: #0f172a;
  font-size: 20px;
}

.title-line {
  display: flex;
  align-items: center;
  gap: 10px;
}

.reference-status {
  padding: 4px 9px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
}

.reference-status--ready {
  background: #dcfce7;
  color: #15803d;
}

.panel-desc {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 13px;
}

.panel-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.upload-line span {
  color: #64748b;
  font-size: 13px;
}

.flow-actions {
  justify-content: flex-end;
  margin-top: 14px;
}

.reference-dialog {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.upload-line {
  display: flex;
  align-items: center;
  gap: 12px;
}

.file-label {
  display: inline-flex;
  cursor: pointer;
}

.file-label input {
  display: none;
}

@media (max-width: 900px) {
  .panel-header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
