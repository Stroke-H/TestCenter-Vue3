<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  ArrowRight,
  CircleCheck,
  CirclePlus,
  Clock,
  Filter,
  Histogram,
  Operation,
  Refresh,
  Search,
  Tickets,
  TrendCharts,
  WarningFilled
} from '@element-plus/icons-vue'
import DefectIcon from '@/components/icons/DefectIcon.vue'
import { defectApi, type DefectBatchPayload, type DefectListParams, type DefectTransitionPayload } from './api'
import DefectVersionSelect from './components/DefectVersionSelect.vue'
import DefectBatchDialog from './components/DefectBatchDialog.vue'
import DefectDetailDrawer from './components/DefectDetailDrawer.vue'
import DefectFormDrawer from './components/DefectFormDrawer.vue'
import DefectStatsPanel from './components/DefectStatsPanel.vue'
import {
  defectStatusMeta,
  defectTypeOptions,
  priorityLabels,
  severityLabels,
  type Defect,
  type DefectDetail,
  type DefectFieldDefinition,
  type DefectFormValue,
  type DefectMeta,
  type DefectStats,
  type DefectStatus
} from './types'

defineOptions({ name: 'DefectManagement' })

const loading = ref(false)
const acting = ref(false)
const saving = ref(false)
const meta = ref<DefectMeta | null>(null)
const defects = ref<Defect[]>([])
const total = ref(0)
const summary = ref<DefectStats | null>(null)
const formVisible = ref(false)
const formDefect = ref<Defect | null>(null)
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<DefectDetail | null>(null)
const detailRef = ref<InstanceType<typeof DefectDetailDrawer>>()
const tableRef = ref()
const activeSection = ref<'list' | 'stats'>('list')
const selectedDefects = ref<Defect[]>([])
const batchVisible = ref(false)
const batchLoading = ref(false)
const customFilters = reactive<Record<string, string>>({})

const filters = reactive<DefectListParams>({
  search: '',
  project_code: '',
  found_version: '',
  status: '',
  severity: '',
  priority: '',
  assignee_id: '',
  reporter_id: '',
  page: 1,
  page_size: 20
})

const canCreate = computed(() => Object.values(meta.value?.project_actions || {}).some((actions) => actions.create))
const selectableDefect = (row: Defect) => ['process', 'verify', 'reopen', 'archive'].some((action) => row.allowed_actions?.[action] === true)
const canBatch = computed(() => defects.value.some(selectableDefect))
const hasFilters = computed(() => Boolean(filters.search || filters.project_code || filters.found_version || filters.status || filters.severity || filters.priority || filters.assignee_id || filters.reporter_id || Object.values(customFilters).some(Boolean)))
const applicableListFields = computed(() => (meta.value?.fields || []).filter((field) => (
  field.enabled && (!field.project_code || field.project_code === filters.project_code)
)))
const filterableFields = computed(() => applicableListFields.value.filter((field) => field.filterable))
const visibleCustomFields = computed(() => applicableListFields.value.filter((field) => field.list_visible))
const activeFilterCount = computed(() => [
  filters.search,
  filters.project_code,
  filters.found_version,
  filters.severity,
  filters.priority,
  filters.assignee_id,
  filters.reporter_id,
  ...Object.values(customFilters)
].filter(Boolean).length)

const sectionCopy = computed(() => ({
  list: ['缺陷工作台', '集中筛选、跟进并处理项目缺陷'],
  stats: ['质量分析', '观察缺陷趋势、风险分布与处理效率'],
  fields: ['字段配置', '按团队和项目需要扩展缺陷信息结构'],
  permissions: ['项目权限', '控制项目数据范围和成员职责']
}[activeSection.value]))

function errorMessage(error: any, fallback: string) {
  return error?.response?.data?.error || error?.customMessage || fallback
}

function formatTime(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString('zh-CN', { hour12: false })
}

function typeLabel(value: string) {
  return defectTypeOptions.find((item) => item.value === value)?.label || value || '未设置'
}

function accountLabel(id: string) {
  const account = meta.value?.accounts.find((item) => item.id === id)
  return account?.nickname || account?.username || ''
}

function customFieldValue(value: any, field: DefectFieldDefinition) {
  if (value === undefined || value === null || value === '') return '—'
  if (field.field_type === 'boolean') return value ? '是' : '否'
  if (field.field_type === 'user') return accountLabel(String(value)) || String(value)
  return Array.isArray(value) ? value.join('、') : String(value)
}

function statusCount(status: DefectStatus | '') {
  if (!summary.value) return 0
  return status ? summary.value[status] : summary.value.total
}

function isOverdue(defect: Defect) {
  if (!defect.due_date || defect.status === 'closed') return false
  const now = new Date()
  const localToday = new Date(now.getTime() - now.getTimezoneOffset() * 60_000).toISOString().slice(0, 10)
  return defect.due_date < localToday
}

function assigneeInitial(defect: Defect) {
  const name = defect.assignee_name || accountLabel(defect.assignee_id) || '待'
  return name.slice(0, 1).toUpperCase()
}

async function loadMeta() {
  try {
    meta.value = await defectApi.meta()
  } catch (error) {
    ElMessage.error(errorMessage(error, '缺陷基础数据加载失败'))
  }
}

async function loadSummary() {
  try {
    summary.value = await defectApi.stats({})
  } catch {
    summary.value = null
  }
}

async function loadDefects() {
  loading.value = true
  try {
    const params: DefectListParams = { ...filters }
    Object.entries(customFilters).forEach(([key, value]) => {
      if (value !== '') params[`cf_${key}`] = value
    })
    const result = await defectApi.list(params)
    defects.value = result.items || []
    total.value = result.total || 0
    selectedDefects.value = []
    tableRef.value?.clearSelection()
  } catch (error) {
    ElMessage.error(errorMessage(error, '缺陷列表加载失败'))
  } finally {
    loading.value = false
  }
}

async function refreshAll() {
  await Promise.all([loadMeta(), loadDefects(), loadSummary()])
}

function search() {
  filters.page = 1
  loadDefects()
}

function selectStatus(status: string) {
  filters.status = status
  search()
}

function clearFilters() {
  Object.assign(filters, {
    search: '', project_code: '', found_version: '', status: '', severity: '', priority: '',
    assignee_id: '', reporter_id: '', page: 1, page_size: filters.page_size
  })
  Object.keys(customFilters).forEach((key) => { customFilters[key] = '' })
  loadDefects()
}

function switchSection(section: typeof activeSection.value) {
  activeSection.value = section
}

function selectionChanged(rows: Defect[]) {
  selectedDefects.value = rows
}

async function executeBatch(payload: Omit<DefectBatchPayload, 'ids'>) {
  batchLoading.value = true
  try {
    const result = await defectApi.batch({ ...payload, ids: selectedDefects.value.map((item) => item.id) })
    const succeeded = result.results.filter((item) => item.success).length
    const failed = result.results.length - succeeded
    if (failed) ElMessage.warning(`已处理 ${succeeded} 条，${failed} 条因状态或权限限制未处理`)
    else ElMessage.success(`已完成 ${succeeded} 条缺陷的批量操作`)
    batchVisible.value = false
    await Promise.all([loadDefects(), loadSummary()])
  } catch (error) {
    ElMessage.error(errorMessage(error, '批量操作失败'))
  } finally {
    batchLoading.value = false
  }
}

function openCreate() {
  formDefect.value = null
  formVisible.value = true
}

function openEdit(defect: Defect) {
  formDefect.value = defect
  detailVisible.value = false
  nextTick(() => { formVisible.value = true })
}

async function saveDefect(value: DefectFormValue) {
  saving.value = true
  try {
    const saved = formDefect.value
      ? await defectApi.update(formDefect.value.id, value)
      : await defectApi.create(value)
    formVisible.value = false
    ElMessage.success(formDefect.value ? '缺陷已更新' : `缺陷 ${saved.defect_no} 已提交`)
    await Promise.all([loadDefects(), loadSummary()])
    await openDetail(saved)
  } catch (error) {
    ElMessage.error(errorMessage(error, '保存缺陷失败'))
  } finally {
    saving.value = false
  }
}

async function openDetail(row: Defect) {
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await defectApi.detail(row.id)
  } catch (error) {
    ElMessage.error(errorMessage(error, '缺陷详情加载失败'))
  } finally {
    detailLoading.value = false
  }
}

async function refreshDetail() {
  if (!detail.value?.defect.id) return
  await openDetail(detail.value.defect)
}

async function transitionDefect(payload: DefectTransitionPayload) {
  if (!detail.value?.defect.id) return
  acting.value = true
  try {
    await defectApi.transition(detail.value.defect.id, payload)
    detailRef.value?.closeTransition()
    ElMessage.success('缺陷状态已更新')
    await Promise.all([refreshDetail(), loadDefects(), loadSummary()])
  } catch (error) {
    ElMessage.error(errorMessage(error, '状态更新失败'))
  } finally {
    acting.value = false
  }
}

async function addComment(content: string) {
  if (!detail.value?.defect.id) return
  acting.value = true
  try {
    await defectApi.comment(detail.value.defect.id, content)
    ElMessage.success('评论已发布')
    await refreshDetail()
  } catch (error) {
    ElMessage.error(errorMessage(error, '发表评论失败'))
  } finally {
    acting.value = false
  }
}

async function uploadAttachment(file: File) {
  if (!detail.value?.defect.id) return
  if (file.size > 20 * 1024 * 1024) {
    ElMessage.warning('单个附件不能超过 20MB')
    return
  }
  acting.value = true
  try {
    await defectApi.upload(detail.value.defect.id, file)
    ElMessage.success('附件上传成功')
    await refreshDetail()
  } catch (error) {
    ElMessage.error(errorMessage(error, '附件上传失败'))
  } finally {
    acting.value = false
  }
}

onMounted(refreshAll)
</script>

<template>
  <main class="defect-page">
    <header class="page-header">
      <div class="page-title">
        <span class="page-title__icon"><el-icon><DefectIcon /></el-icon></span>
        <div>
          <p>QUALITY CENTER / DEFECTS</p>
          <h1>缺陷管理</h1>
          <span>从发现、处理到验证关闭，持续追踪产品质量</span>
        </div>
      </div>
      <div class="page-actions">
        <el-button class="refresh-button" :icon="Refresh" :loading="loading" @click="refreshAll">刷新数据</el-button>
        <el-button v-if="canCreate" class="create-button" type="primary" :icon="CirclePlus" @click="openCreate">提交缺陷</el-button>
      </div>
    </header>

    <section class="quality-overview">
      <article class="overview-card is-total"><span><el-icon><Histogram /></el-icon></span><div><small>缺陷总数</small><strong>{{ summary?.total ?? '—' }}</strong><em>累计记录</em></div></article>
      <article class="overview-card is-progress"><span><el-icon><Clock /></el-icon></span><div><small>当前待处理</small><strong>{{ (summary?.new || 0) + (summary?.active || 0) }}</strong><em>{{ summary?.new || 0 }} 条待确认</em></div></article>
      <article class="overview-card is-verify"><span><el-icon><WarningFilled /></el-icon></span><div><small>等待验证</small><strong>{{ summary?.resolved ?? '—' }}</strong><em>{{ summary?.overdue || 0 }} 条已逾期</em></div></article>
      <article class="overview-card is-closed"><span><el-icon><CircleCheck /></el-icon></span><div><small>关闭率</small><strong>{{ summary ? `${summary.closure_rate.toFixed(1)}%` : '—' }}</strong><em>{{ summary?.closed || 0 }} 条已关闭</em></div></article>
    </section>

    <section class="module-bar">
      <nav class="module-navigation">
        <button :class="{ 'is-active': activeSection === 'list' }" @click="switchSection('list')"><span class="module-icon"><el-icon><Tickets /></el-icon></span><span>缺陷列表</span></button>
        <button :class="{ 'is-active': activeSection === 'stats' }" @click="switchSection('stats')"><span class="module-icon"><el-icon><TrendCharts /></el-icon></span><span>统计分析</span></button>
      </nav>
      <div class="module-context"><strong>{{ sectionCopy[0] }}</strong><span>{{ sectionCopy[1] }}</span></div>
    </section>

    <template v-if="activeSection === 'list'">
      <section class="status-navigation">
        <button :class="{ 'is-active': filters.status === '' }" @click="selectStatus('')"><span class="status-dot is-all" /><span>全部缺陷</span><b>{{ statusCount('') }}</b></button>
        <button v-for="status in (['new','active','resolved','closed'] as DefectStatus[])" :key="status" :class="{ 'is-active': filters.status === status }" @click="selectStatus(status)"><span class="status-dot" :class="`is-${defectStatusMeta[status].tone}`" /><span>{{ defectStatusMeta[status].label }}</span><b>{{ statusCount(status) }}</b></button>
      </section>

      <section class="filter-panel">
        <div class="filter-panel__header">
          <div><span class="filter-icon"><el-icon><Filter /></el-icon></span><div><strong>筛选条件</strong><small>快速定位需要跟进的缺陷</small></div><em v-if="activeFilterCount">已启用 {{ activeFilterCount }} 项</em></div>
          <div><el-button v-if="hasFilters" text @click="clearFilters">重置</el-button><el-button type="primary" :icon="Search" @click="search">查询</el-button></div>
        </div>
        <div class="filter-panel__main">
          <el-input v-model="filters.search" clearable placeholder="搜索缺陷编号、标题或描述" class="search-input" @keyup.enter="search" @clear="search"><template #prefix><el-icon><Search /></el-icon></template></el-input>
          <el-select v-model="filters.project_code" filterable clearable placeholder="全部项目" @change="search"><el-option v-for="project in meta?.projects || []" :key="project.id" :label="`${project.project_code} · ${project.project_name}`" :value="project.project_code" /></el-select>
          <DefectVersionSelect v-model="filters.found_version" :meta="meta" :projects="filters.project_code ? [filters.project_code] : []" filter @change="search" />
          <el-select v-model="filters.assignee_id" filterable clearable placeholder="全部处理人" @change="search"><el-option v-for="account in meta?.accounts || []" :key="account.id" :label="account.nickname || account.username" :value="account.id" /></el-select>
        </div>
        <div class="filter-panel__more">
          <span>更多条件</span>
          <el-select v-model="filters.severity" clearable placeholder="严重程度" @change="search"><el-option v-for="(label, value) in severityLabels" :key="value" :label="`${value} · ${label}`" :value="String(value)" /></el-select>
          <el-select v-model="filters.priority" clearable placeholder="优先级" @change="search"><el-option v-for="(label, value) in priorityLabels" :key="value" :label="label" :value="String(value)" /></el-select>
          <template v-for="field in filterableFields" :key="field.id">
            <el-select v-if="field.field_type === 'select' || field.field_type === 'multi_select'" v-model="customFilters[field.field_key]" clearable :placeholder="field.name" @change="search"><el-option v-for="option in field.options" :key="option" :label="option" :value="option" /></el-select>
            <el-select v-else-if="field.field_type === 'user'" v-model="customFilters[field.field_key]" clearable filterable :placeholder="field.name" @change="search"><el-option v-for="account in meta?.accounts || []" :key="account.id" :label="account.nickname || account.username" :value="account.id" /></el-select>
            <el-select v-else-if="field.field_type === 'boolean'" v-model="customFilters[field.field_key]" clearable :placeholder="field.name" @change="search"><el-option label="是" value="true" /><el-option label="否" value="false" /></el-select>
            <el-input v-else v-model="customFilters[field.field_key]" clearable :placeholder="field.name" class="custom-filter-input" @keyup.enter="search" @clear="search" />
          </template>
        </div>
      </section>

      <section class="defect-table-card">
        <div v-if="selectedDefects.length" class="batch-toolbar"><div><span><el-icon><Operation /></el-icon></span><strong>已选择 {{ selectedDefects.length }} 条缺陷</strong><small>最多可一次处理 100 条</small></div><el-button type="primary" plain :icon="Operation" :disabled="selectedDefects.length > 100" @click="batchVisible = true">批量处理</el-button></div>
        <div class="table-heading"><div><strong>缺陷列表</strong><span>{{ total }}</span></div><small>单击任意记录查看详情与完整流转历史</small></div>
        <el-table ref="tableRef" v-loading="loading" :data="defects" row-key="id" row-class-name="defect-row" class="defect-table" @selection-change="selectionChanged" @row-click="openDetail">
          <el-table-column v-if="canBatch" type="selection" width="46" :selectable="selectableDefect" />
          <el-table-column label="缺陷" min-width="330"><template #default="{ row }"><div class="defect-cell"><div class="defect-cell__meta"><span>{{ row.defect_no }}</span><em>{{ typeLabel(row.defect_type) }}</em></div><strong>{{ row.title }}</strong><div><el-tag v-for="tag in row.tags?.slice(0,2)" :key="tag" size="small" effect="plain">{{ tag }}</el-tag></div></div></template></el-table-column>
          <el-table-column label="所属项目" min-width="190"><template #default="{ row }"><div class="project-cell"><strong>{{ row.project_code }}</strong><span>{{ row.project_name }}</span><small>发现于 {{ row.found_version || '未填写版本' }}</small></div></template></el-table-column>
          <el-table-column label="严重 / 优先" width="140"><template #default="{ row }"><div class="level-cell"><span :class="`severity-${row.severity}`"><i />S{{ row.severity }} · {{ severityLabels[row.severity] }}</span><small>{{ priorityLabels[row.priority] }}</small></div></template></el-table-column>
          <el-table-column label="状态" width="110"><template #default="{ row }"><span class="table-status" :class="`is-${defectStatusMeta[row.status as DefectStatus].tone}`">{{ defectStatusMeta[row.status as DefectStatus].label }}</span></template></el-table-column>
          <el-table-column label="处理人" width="140"><template #default="{ row }"><div class="assignee-cell"><span>{{ assigneeInitial(row) }}</span><div><strong>{{ row.assignee_name || accountLabel(row.assignee_id) || '未指派' }}</strong><small>{{ row.assignee_id ? '当前负责人' : '等待指派' }}</small></div></div></template></el-table-column>
          <el-table-column label="截止日期" width="130"><template #default="{ row }"><span class="due-date" :class="{ 'is-overdue': isOverdue(row) }">{{ row.due_date || '未设置' }}<small v-if="isOverdue(row)">已逾期</small></span></template></el-table-column>
          <el-table-column v-for="field in visibleCustomFields" :key="field.id" :label="field.name" width="140" show-overflow-tooltip><template #default="{ row }"><span class="custom-value">{{ customFieldValue(row.custom_fields?.[field.field_key], field) }}</span></template></el-table-column>
          <el-table-column label="最后更新" width="168"><template #default="{ row }"><span class="time-text">{{ formatTime(row.updated_at) }}</span></template></el-table-column>
          <el-table-column width="54" fixed="right"><template #default="{ row }"><el-button class="row-entry" text circle :icon="ArrowRight" title="查看详情" @click.stop="openDetail(row)" /></template></el-table-column>
          <template #empty><div class="empty-state"><span><el-icon><WarningFilled /></el-icon></span><strong>暂无匹配的缺陷</strong><p>{{ hasFilters ? '尝试调整筛选条件' : '点击右上角“提交缺陷”记录第一个问题' }}</p></div></template>
        </el-table>
        <div class="pagination-row"><el-pagination v-model:current-page="filters.page" v-model:page-size="filters.page_size" :total="total" :page-sizes="[20,50,100]" layout="total, sizes, prev, pager, next" @change="loadDefects" /></div>
      </section>
    </template>

    <DefectStatsPanel v-else-if="activeSection === 'stats'" :meta="meta" />

    <DefectFormDrawer v-model="formVisible" :defect="formDefect" :meta="meta" :saving="saving" @save="saveDefect" />
    <DefectDetailDrawer ref="detailRef" v-model="detailVisible" :detail="detail" :meta="meta" :loading="detailLoading" :acting="acting" @edit="detail && openEdit(detail.defect)" @refresh="refreshDetail" @transition="transitionDefect" @comment="addComment" @upload="uploadAttachment" />
    <DefectBatchDialog v-model="batchVisible" :count="selectedDefects.length" :defects="selectedDefects" :meta="meta" :loading="batchLoading" @execute="executeBatch" />
  </main>
</template>

<style scoped>
/* Main Page Container */
.defect-page {
  --defect-blue: #2563eb;
  --defect-ink: #0f172a;
  --defect-muted: #64748b;
  --defect-line: #e2e8f0;
  --defect-surface: #ffffff;
  min-height: 100%;
  padding: 24px 32px;
  background: #f8fafc;
  color: #0f172a;
  box-sizing: border-box;
}

/* Page Header */
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 22px;
  flex-wrap: wrap;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 16px;
}

.page-title__icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 14px;
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: #ffffff;
  font-size: 24px;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.22);
  flex-shrink: 0;
}

.page-title p {
  margin: 0 0 2px;
  color: #3b82f6;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.page-title h1 {
  margin: 0;
  color: #0f172a;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: -0.02em;
  line-height: 1.25;
}

.page-title > div > span {
  display: block;
  margin-top: 3px;
  color: #64748b;
  font-size: 12px;
}

.page-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.page-actions :deep(.el-button) {
  height: 38px;
  padding: 0 16px;
  border-radius: 9px;
  font-weight: 600;
  font-size: 13px;
}

.refresh-button {
  color: #475569;
  border-color: #cbd5e1;
  background: #ffffff;
}

.create-button {
  border-color: transparent;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.2);
}

/* Quality Overview 4 Cards */
.quality-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 20px;
}

.overview-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
  overflow: hidden;
}

.overview-card:hover {
  transform: translateY(-2px);
  border-color: #cbd5e1;
  box-shadow: 0 8px 24px -4px rgba(15, 23, 42, 0.08);
}

.overview-card > span {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  font-size: 20px;
  flex-shrink: 0;
}

.overview-card > div {
  display: flex;
  flex-direction: column;
}

.overview-card small {
  font-size: 12px;
  color: #64748b;
  font-weight: 500;
}

.overview-card strong {
  color: #0f172a;
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
  margin: 2px 0;
  font-variant-numeric: tabular-nums;
}

.overview-card em {
  color: #94a3b8;
  font-size: 11px;
  font-style: normal;
}

.overview-card.is-total > span { color: #2563eb; background: #eff6ff; }
.overview-card.is-progress > span { color: #ea580c; background: #fff7ed; }
.overview-card.is-verify > span { color: #d97706; background: #fffbeb; }
.overview-card.is-closed > span { color: #16a34a; background: #f0fdf4; }

/* Module Bar */
.module-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 50px;
  padding: 6px 16px 6px 8px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.03);
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.module-navigation {
  display: flex;
  align-items: center;
  gap: 4px;
}

.module-navigation button {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #64748b;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.16s ease;
}

.module-navigation button:hover {
  color: #2563eb;
  background: #f8fafc;
}

.module-navigation button.is-active {
  color: #1d4ed8;
  background: #eff6ff;
  font-weight: 600;
}

.module-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  font-size: 14px;
}

.module-context {
  display: flex;
  align-items: center;
  gap: 8px;
}

.module-context strong {
  color: #334155;
  font-size: 12px;
  font-weight: 600;
}

.module-context span {
  color: #94a3b8;
  font-size: 12px;
}

/* Status Navigation */
.status-navigation {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.03);
  margin-bottom: 16px;
  overflow-x: auto;
}

.status-navigation button {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #64748b;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.16s ease;
  white-space: nowrap;
}

.status-navigation button:hover {
  color: #0f172a;
  background: #f8fafc;
}

.status-navigation button.is-active {
  color: #1d4ed8;
  background: #eff6ff;
  font-weight: 600;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-dot.is-all { background: #64748b; }
.status-dot.is-blue { background: #3b82f6; }
.status-dot.is-amber { background: #f59e0b; }
.status-dot.is-green { background: #10b981; }

.status-navigation button b {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 20px;
  padding: 0 6px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.status-navigation button.is-active b {
  color: #1d4ed8;
  background: #dbeafe;
}

/* Filter Panel */
.filter-panel {
  padding: 16px 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.03);
  margin-bottom: 18px;
}

.filter-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
}

.filter-panel__header > div:first-child {
  display: flex;
  align-items: center;
  gap: 10px;
}

.filter-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  color: #2563eb;
  background: #eff6ff;
  font-size: 16px;
}

.filter-panel__header strong {
  color: #0f172a;
  font-size: 14px;
  font-weight: 700;
}

.filter-panel__header small {
  color: #94a3b8;
  font-size: 12px;
  margin-left: 6px;
}

.filter-panel__header em {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 999px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 11px;
  font-weight: 600;
  font-style: normal;
  margin-left: 8px;
}

.filter-panel__main {
  display: grid;
  grid-template-columns: minmax(260px, 1.8fr) repeat(3, minmax(160px, 1fr));
  gap: 12px;
}

.filter-panel__more {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid #f1f5f9;
  flex-wrap: wrap;
}

.filter-panel__more > span {
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  min-width: 56px;
}

.filter-panel__more .el-select,
.custom-filter-input {
  width: 160px;
}

/* Defect Table Card */
.defect-table-card {
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 1px 4px rgba(15, 23, 42, 0.04);
  overflow: hidden;
}

.batch-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 18px;
  background: linear-gradient(90deg, #eff6ff, #f8fbff);
  border-bottom: 1px solid #bfdbfe;
}

.batch-toolbar > div {
  display: flex;
  align-items: center;
  gap: 8px;
}

.batch-toolbar > div > span {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: #dbeafe;
  color: #2563eb;
  font-size: 15px;
}

.batch-toolbar strong {
  color: #1e40af;
  font-size: 13px;
  font-weight: 600;
}

.batch-toolbar small {
  color: #60a5fa;
  font-size: 12px;
  margin-left: 6px;
}

.table-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid #f1f5f9;
}

.table-heading > div {
  display: flex;
  align-items: center;
}

.table-heading strong {
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.table-heading span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 8px;
  border-radius: 999px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
  margin-left: 8px;
}

.table-heading small {
  color: #94a3b8;
  font-size: 12px;
}

.defect-table {
  width: 100%;
}

:deep(.defect-row) {
  cursor: pointer;
  transition: background-color 0.15s ease;
}

:deep(.defect-row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

:deep(.el-table th.el-table__cell) {
  height: 46px;
  background: #f8fafc;
  color: #475569;
  font-size: 12px;
  font-weight: 600;
  border-bottom: 1px solid #e2e8f0;
}

:deep(.el-table td.el-table__cell) {
  padding: 13px 0;
  border-bottom: 1px solid #f1f5f9;
}

/* Table Cells */
.defect-cell {
  display: flex;
  flex-direction: column;
}

.defect-cell__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.defect-cell__meta > span {
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.3px;
}

.defect-cell__meta > em {
  padding: 1px 6px;
  border-radius: 4px;
  background: #f1f5f9;
  color: #475569;
  font-size: 11px;
  font-style: normal;
  font-weight: 500;
}

.defect-cell > strong {
  color: #0f172a;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.5;
  word-break: break-word;
}

.defect-cell > div {
  display: flex;
  gap: 6px;
  margin-top: 6px;
  flex-wrap: wrap;
}

.defect-cell :deep(.el-tag) {
  height: 20px;
  padding: 0 6px;
  border-radius: 4px;
  font-size: 11px;
}

.project-cell {
  display: flex;
  flex-direction: column;
}

.project-cell strong {
  display: inline-block;
  padding: 2px 7px;
  border-radius: 5px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
  align-self: flex-start;
  letter-spacing: 0.3px;
}

.project-cell span {
  margin-top: 4px;
  color: #1e293b;
  font-size: 12px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-cell small {
  margin-top: 2px;
  color: #94a3b8;
  font-size: 11px;
}

.level-cell {
  display: flex;
  flex-direction: column;
}

.level-cell span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
}

.level-cell span i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.level-cell small {
  margin-top: 3px;
  color: #64748b;
  font-size: 11px;
}

.severity-1 { color: #dc2626; }
.severity-2 { color: #ea580c; }
.severity-3 { color: #2563eb; }
.severity-4 { color: #64748b; }

.table-status {
  display: inline-flex;
  align-items: center;
  padding: 3px 9px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  border: 1px solid transparent;
}

.table-status.is-slate { color: #475569; background: #f1f5f9; border-color: #e2e8f0; }
.table-status.is-blue { color: #1d4ed8; background: #eff6ff; border-color: #dbeafe; }
.table-status.is-amber { color: #b45309; background: #fffbeb; border-color: #fde68a; }
.table-status.is-green { color: #15803d; background: #f0fdf4; border-color: #bbf7d0; }

.assignee-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.assignee-cell > span {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: #eff6ff;
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
  flex-shrink: 0;
}

.assignee-cell strong {
  display: block;
  color: #0f172a;
  font-size: 12px;
  font-weight: 500;
}

.assignee-cell small {
  display: block;
  color: #94a3b8;
  font-size: 11px;
}

.due-date {
  display: inline-flex;
  flex-direction: column;
  color: #475569;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.due-date.is-overdue {
  color: #dc2626;
  font-weight: 600;
}

.due-date small {
  display: inline-block;
  padding: 1px 5px;
  border-radius: 4px;
  background: #fee2e2;
  color: #dc2626;
  font-size: 10px;
  font-weight: 700;
  margin-top: 2px;
  align-self: flex-start;
}

.time-text,
.custom-value {
  color: #64748b;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.row-entry {
  width: 28px;
  height: 28px;
  color: #94a3b8;
  background: #f8fafc;
  border-radius: 6px;
  transition: all 0.15s ease;
}

.row-entry:hover {
  color: #2563eb;
  background: #eff6ff;
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  padding: 14px 20px;
  border-top: 1px solid #f1f5f9;
  background: #ffffff;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 52px 0;
}

.empty-state > span {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  color: #94a3b8;
  background: #f1f5f9;
  font-size: 22px;
}

.empty-state strong {
  margin-top: 12px;
  color: #334155;
  font-size: 14px;
}

.empty-state p {
  margin: 4px 0 0;
  color: #94a3b8;
  font-size: 12px;
}

/* Responsive */
@media (max-width: 1180px) {
  .quality-overview { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .module-context { display: none; }
  .filter-panel__main { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 720px) {
  .defect-page { padding: 16px 14px 24px; }
  .quality-overview { grid-template-columns: 1fr; }
  .filter-panel__main { grid-template-columns: 1fr; }
  .table-heading small { display: none; }
  .batch-toolbar small { display: none; }
}
</style>