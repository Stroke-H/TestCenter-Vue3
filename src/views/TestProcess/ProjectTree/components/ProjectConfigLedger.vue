<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { buildBackendUrl } from '@/utils/runtimeUrl'
import type { ProjectMemoRecord, ProjectMemoItem } from '../types'

const props = defineProps<{ projectCode: string; record?: ProjectMemoRecord }>()
const emit = defineEmits<{ refresh: [] }>()
const auth = useAuthStore()
const visible = ref(false)
const busy = ref(false)
const revision = ref(-1)
const draft = ref<{token: string; before: ProjectMemoItem[]; after: ProjectMemoItem[]; record: ProjectMemoRecord} | null>(null)
const error = ref('')
const revisions = computed(() => props.record?.versions || [])
const rows = computed(() => revision.value < 0 ? props.record?.items || [] : revisions.value[revision.value]?.items || [])
const activeConfigs = computed(() => rows.value.filter(item => !item.removed && item.feature))
const formatTime = (value?: string) => value ? new Date(value).toLocaleString('zh-CN') : '未标注时间'
const scope = (item: ProjectMemoItem) => [item.audience, item.platform, item.variant].filter(Boolean).join(' / ') || '未限定群体'

async function analyze(mode: 'preview' | 'apply' | 'sync') {
  if (busy.value) return
  busy.value = true; error.value = ''
  const code = props.projectCode
  try {
    const response = await fetch(buildBackendUrl('/api/acceptance-reports/project-configs/analyze-project'), {
      method: 'POST', credentials: 'include', headers: {'Content-Type': 'application/json', Authorization: auth.token},
      body: JSON.stringify({project_code: code, mode, token: draft.value?.token}),
    })
    const result = await response.json()
    if (!response.ok) throw new Error(result.error || '处理失败')
    if (code !== props.projectCode) return
    if (mode === 'preview') draft.value = result
    else { draft.value = null; emit('refresh'); ElMessage.success(mode === 'apply' ? '整理结果已应用，旧记录已归档' : '项目配置同步完成') }
  } catch (e) { if (code === props.projectCode) error.value = e instanceof Error ? e.message : '处理失败' }
  finally { busy.value = false }
}
watch(() => props.projectCode, () => { visible.value = false; draft.value = null; revision.value = -1; error.value = '' })
</script>

<template>
  <div class="ledger-entry">
    <div><strong>功能与配置台账</strong><span>当前版本 {{ record?.current_version || '待整理' }}</span></div>
    <el-button size="small" type="primary" plain @click="visible = true">版本与历史</el-button>
  </div>
  <el-dialog v-model="visible" :title="`${projectCode} · 功能与配置台账`" width="min(960px, calc(100vw - 32px))" align-center append-to-body>
    <div class="ledger">
      <div class="ledger-toolbar">
        <el-select v-model="revision" aria-label="查看版本">
          <el-option :value="-1" :label="`当前版本 ${record?.current_version || '待整理'}`" />
          <el-option v-for="(entry, index) in revisions" :key="index" :value="index" :label="`${entry.version} · ${formatTime(entry.submittedAt)} · 提交 ${index + 1}`" />
        </el-select>
        <el-button :disabled="busy" @click="analyze('sync')">同步报告变更</el-button>
        <el-button :loading="busy" type="primary" plain @click="analyze('preview')">历史整理预览</el-button>
      </div>
      <p class="ledger-hint">功能默认继承；相同版本补充修正；只在明确废除时移出当前配置。Bug 不纳入台账。历史整理会调用 AI，完成后先预览再应用。</p>
      <el-alert v-if="error" :title="error" type="error" :closable="false" />
      <el-alert v-for="warning in record?.warnings || []" :key="warning" :title="warning" type="warning" :closable="false" />
      <div v-if="draft" class="ledger-preview">
        <h3>整理预览</h3>
        <p>原记录 {{ draft.before.length }} 条 → 整理后 {{ draft.after.filter(item => !item.removed).length }} 条有效记录。应用前可以逐项对照；原记录保存在历史归档。</p>
        <div class="ledger-comparison">
          <section><h4>原记录</h4><p v-for="item in draft.before" :key="item.id">{{ item.content }}</p></section>
          <section><h4>整理后</h4><p v-for="item in draft.after" :key="item.id">{{ item.removed ? '［已废除］' : '' }}{{ item.content }}<small>{{ scope(item) }} · {{ item.version }}</small></p></section>
        </div>
        <el-alert v-for="warning in draft.record.warnings || []" :key="warning" :title="warning" type="warning" :closable="false" />
        <el-button type="primary" :disabled="busy" @click="analyze('apply')">应用本次整理</el-button>
        <el-button :disabled="busy" @click="draft = null">取消预览</el-button>
      </div>
      <div class="ledger-list">
        <article v-for="item in activeConfigs" :key="item.id" class="ledger-item">
          <header><strong>{{ item.feature || '原始便签（待确认分类）' }}</strong><el-tag size="small" effect="plain">{{ item.category || '未归类' }}</el-tag></header>
          <p>{{ item.content }}</p>
          <div v-if="item.value || item.previousValue" class="ledger-values"><span>历史值：{{ item.previousValue || '未明确' }}</span><b>当前值：{{ item.value || '未明确' }}</b></div>
          <small>{{ scope(item) }} · 来源版本 {{ item.version || '未标注' }} · {{ formatTime(item.updatedAt) }} · {{ item.kind === 'ai' ? 'AI记录' : '人工记录' }}</small>
          <details v-if="item.evidence || item.history?.length"><summary>原文与变更记录</summary><blockquote v-if="item.evidence">{{ item.evidence }}<small>来源报告：{{ item.sourceReportId || '人工便签' }}</small></blockquote><p v-for="(entry, index) in item.history || []" :key="index">{{ formatTime(entry.modifiedAt) }} · {{ entry.content }}</p></details>
        </article>
        <el-empty v-if="!activeConfigs.length" description="暂无已整理配置，可先生成历史整理预览；原始便签仍保留在项目树" />
        <details v-if="rows.some(item => item.removed)"><summary>查看本版本已废除的功能</summary><p v-for="item in rows.filter(item => item.removed)" :key="item.id">{{ item.content }} · {{ formatTime(item.updatedAt) }}</p></details>
        <details v-if="record?.legacy_items?.length"><summary>整理前的原始记录归档（{{ record.legacy_items.length }}）</summary><p v-for="(item, index) in record.legacy_items" :key="index">{{ item.content }} · {{ formatTime(item.updatedAt) }}</p></details>
      </div>
    </div>
    <template #footer><el-button @click="visible = false">关闭</el-button></template>
  </el-dialog>
</template>

<style scoped>
/* Ledger Entry Banner in Memo Card */
.ledger-entry {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 18px;
  margin: 14px 0 16px;
  border: 1px solid #dbeafe;
  border-radius: 10px;
  background: linear-gradient(135deg, #eff6ff 0%, #f8fafc 100%);
  box-sizing: border-box;
}

.ledger-entry > div {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.ledger-entry strong {
  font-size: 13px;
  font-weight: 700;
  color: #1e3a8a;
}

.ledger-entry span {
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
  font-size: 11px;
  font-weight: 600;
}

/* Ledger Modal Container */
.ledger {
  max-height: 68vh;
  overflow-y: auto;
  color: #334155;
  padding-right: 4px;
}

.ledger-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 12px;
}

.ledger-toolbar .el-select {
  width: 340px;
  max-width: 100%;
}

.ledger-hint {
  font-size: 12px;
  color: #64748b;
  line-height: 1.6;
  padding: 8px 14px;
  border-radius: 8px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  margin: 0 0 12px;
}

.ledger small {
  font-size: 12px;
  color: #64748b;
  line-height: 1.6;
}

.ledger .el-alert {
  margin-bottom: 10px;
  border-radius: 8px;
}

/* AI Draft Preview */
.ledger-preview {
  margin: 14px 0;
  padding: 18px;
  border: 1px solid #bfdbfe;
  border-radius: 12px;
  background: #f8fafc;
}

.ledger-preview h3 {
  margin: 0 0 6px;
  font-size: 15px;
  font-weight: 700;
  color: #1e3a8a;
}

.ledger-preview > p {
  margin: 0 0 12px;
  font-size: 12px;
  color: #64748b;
}

.ledger-comparison {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  max-height: 300px;
  overflow-y: auto;
  margin-bottom: 14px;
}

.ledger-comparison section {
  min-width: 0;
  padding: 12px 14px;
  border-radius: 8px;
  overflow-wrap: anywhere;
}

.ledger-comparison section:first-child {
  background: #fff1f2;
  border: 1px solid #fecdd3;
}

.ledger-comparison section:last-child {
  background: #f0fdf4;
  border: 1px solid #bbf7d0;
}

.ledger-comparison h4 {
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 700;
}

.ledger-comparison section:first-child h4 {
  color: #9f1239;
}

.ledger-comparison section:last-child h4 {
  color: #14532d;
}

.ledger-comparison p {
  margin: 0 0 8px;
  font-size: 12px;
  line-height: 1.6;
}

.ledger-comparison small {
  display: block;
  font-size: 11px;
  color: #64748b;
  margin-top: 2px;
}

/* Ledger Items List */
.ledger-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 14px;
}

.ledger-item {
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.03);
  overflow-wrap: anywhere;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.ledger-item:hover {
  border-color: #cbd5e1;
  box-shadow: 0 4px 12px -2px rgba(15, 23, 42, 0.05);
}

.ledger-item header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.ledger-item header strong {
  font-size: 14px;
  font-weight: 700;
  color: #0f172a;
}

.ledger p {
  font-size: 13px;
  line-height: 1.65;
  color: #334155;
  margin: 0 0 10px;
}

.ledger-values {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  padding: 8px 12px;
  border-radius: 8px;
  background: #f8fafc;
  border: 1px solid #f1f5f9;
  font-size: 12px;
  margin-bottom: 10px;
}

.ledger-values span {
  color: #64748b;
}

.ledger-values b {
  color: #2563eb;
  font-weight: 700;
}

.ledger summary {
  cursor: pointer;
  color: #2563eb;
  font-size: 12px;
  font-weight: 500;
  padding-top: 6px;
  user-select: none;
}

.ledger summary:hover {
  text-decoration: underline;
}

.ledger blockquote {
  margin: 10px 0;
  padding: 10px 14px;
  border-left: 3px solid #3b82f6;
  border-radius: 0 8px 8px 0;
  background: #f8fafc;
  white-space: pre-wrap;
  font-size: 12px;
  color: #475569;
  line-height: 1.6;
}

.ledger blockquote small {
  display: block;
  margin-top: 4px;
  color: #94a3b8;
}

@media (max-width: 600px) {
  .ledger-comparison {
    grid-template-columns: 1fr;
  }
  .ledger-entry {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>