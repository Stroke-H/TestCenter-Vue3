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
.defect-page{min-height:100%;padding:28px;background:#f7f9fc;color:#172033}.page-header{display:flex;align-items:center;justify-content:space-between;gap:24px;margin-bottom:22px}.page-title{display:flex;align-items:center;gap:15px}.page-title__icon{display:grid;place-items:center;width:50px;height:50px;border-radius:16px;color:#2563eb;background:linear-gradient(145deg,#dbeafe,#eff6ff);box-shadow:0 9px 22px rgba(37,99,235,.13)}.page-title__icon .el-icon{font-size:23px}.page-title p{margin:0 0 2px;color:#3b82f6;font-size:10px;font-weight:800;letter-spacing:.16em}.page-title h1{margin:0;color:#0f172a;font-size:25px;line-height:1.2}.page-title>div>span{display:block;margin-top:5px;color:#94a3b8;font-size:12px}.page-actions{display:flex;gap:10px}.status-navigation{display:flex;gap:8px;margin-bottom:14px;padding:7px;border:1px solid #e8edf5;border-radius:14px;background:#fff;box-shadow:0 8px 26px rgba(15,23,42,.035)}.status-navigation button{display:flex;align-items:center;gap:8px;padding:9px 15px;border:0;border-radius:10px;background:transparent;color:#64748b;font:inherit;font-size:13px;cursor:pointer;transition:.18s}.status-navigation button:hover{color:#2563eb;background:#f8fafc}.status-navigation button.is-active{color:#1d4ed8;background:#eff6ff;font-weight:700}.status-dot{width:7px;height:7px;border-radius:50%;background:#cbd5e1}.status-dot.is-all{background:#64748b}.status-dot.is-blue{background:#3b82f6}.status-dot.is-amber{background:#f59e0b}.status-dot.is-green{background:#22c55e}.filter-panel{margin-bottom:14px;padding:16px 18px;border:1px solid #e8edf5;border-radius:15px;background:#fff}.filter-panel__main{display:grid;grid-template-columns:minmax(240px,1.5fr) repeat(3,minmax(150px,1fr));gap:11px}.filter-panel__more{display:flex;align-items:center;gap:10px;margin-top:12px;padding-top:12px;border-top:1px dashed #e2e8f0}.filter-panel__more>span{display:flex;align-items:center;gap:5px;margin-right:2px;color:#94a3b8;font-size:11px}.filter-panel__more .el-select{width:150px}.defect-table-card{overflow:hidden;border:1px solid #e8edf5;border-radius:17px;background:#fff;box-shadow:0 14px 38px rgba(15,23,42,.045)}.table-heading{display:flex;align-items:center;justify-content:space-between;padding:18px 20px;border-bottom:1px solid #edf1f6}.table-heading>div{display:flex;align-items:baseline;gap:10px}.table-heading strong{font-size:15px}.table-heading span,.table-heading small{color:#94a3b8;font-size:11px}.defect-table{width:100%}.defect-cell,.project-cell,.level-cell{display:flex;flex-direction:column}.defect-cell>span{color:#3b82f6;font-size:10px;font-weight:800;letter-spacing:.04em}.defect-cell>strong{overflow:hidden;margin-top:5px;color:#273449;font-size:13px;font-weight:650;text-overflow:ellipsis;white-space:nowrap}.defect-cell>div{display:flex;gap:4px;margin-top:7px}.project-cell strong{color:#334155;font-size:12px}.project-cell span{overflow:hidden;margin-top:3px;color:#64748b;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.project-cell small{margin-top:4px;color:#94a3b8;font-size:10px}.level-cell span{font-size:11px;font-weight:700}.level-cell small{margin-top:4px;color:#94a3b8;font-size:10px}.severity-1{color:#dc2626}.severity-2{color:#ea580c}.severity-3{color:#2563eb}.severity-4{color:#64748b}.table-status{display:inline-flex;padding:5px 9px;border-radius:999px;font-size:11px;font-weight:700}.table-status.is-slate{color:#475569;background:#f1f5f9}.table-status.is-blue{color:#2563eb;background:#eff6ff}.table-status.is-amber{color:#b45309;background:#fffbeb}.table-status.is-green{color:#15803d;background:#f0fdf4}.assignee-name,.time-text{color:#64748b;font-size:11px}.pagination-row{display:flex;justify-content:flex-end;padding:15px 18px;border-top:1px solid #edf1f6}.empty-state{display:flex;flex-direction:column;align-items:center;padding:56px 0}.empty-state>span{display:grid;place-items:center;width:48px;height:48px;border-radius:15px;color:#94a3b8;background:#f1f5f9;font-size:21px}.empty-state strong{margin-top:13px;color:#475569}.empty-state p{margin:5px 0 0;color:#94a3b8;font-size:12px}:deep(.defect-row){cursor:pointer}:deep(.defect-row:hover>td.el-table__cell){background:#f8fbff!important}:deep(.el-table th.el-table__cell){height:44px;background:#fafbfc;color:#64748b;font-size:11px;font-weight:700}:deep(.el-table td.el-table__cell){padding:13px 0}@media(max-width:1100px){.filter-panel__main{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:720px){.defect-page{padding:18px}.page-header{align-items:flex-start}.page-title>div>span{display:none}.page-actions .el-button:first-child{display:none}.status-navigation{overflow-x:auto}.status-navigation button{white-space:nowrap}.filter-panel__main{grid-template-columns:1fr}.filter-panel__more{flex-wrap:wrap}.table-heading small{display:none}}
.module-navigation{display:flex;align-items:center;gap:6px;margin-bottom:14px;padding:6px;border:1px solid #e8edf5;border-radius:14px;background:#fff;box-shadow:0 8px 26px rgba(15,23,42,.035)}
.module-navigation button{display:flex;align-items:center;gap:7px;padding:9px 14px;border:0;border-radius:9px;color:#64748b;background:transparent;font:inherit;font-size:12px;cursor:pointer;transition:.18s}
.module-navigation button:hover{color:#2563eb;background:#f8fafc}.module-navigation button.is-active{color:#1d4ed8;background:#eff6ff;font-weight:700}.module-navigation .el-icon{font-size:14px}
.batch-toolbar{display:flex;align-items:center;justify-content:space-between;padding:10px 16px;border-bottom:1px solid #bfdbfe;background:#eff6ff}.batch-toolbar>div{display:flex;align-items:center;gap:8px}.batch-toolbar>div>span{display:grid;place-items:center;width:28px;height:28px;border-radius:8px;color:#2563eb;background:#dbeafe}.batch-toolbar strong{color:#1e3a8a;font-size:12px}.batch-toolbar small{color:#60a5fa;font-size:10px}.custom-filter-input{width:150px}.custom-value{color:#64748b;font-size:11px}
@media(max-width:720px){.module-navigation{overflow-x:auto}.module-navigation button{white-space:nowrap}.batch-toolbar small{display:none}}

/* Refined defect workspace: dense enough for daily triage, consistent with TestCenter. */
.defect-page {
  --defect-blue: #2563eb;
  --defect-ink: #172033;
  --defect-muted: #7b8ba4;
  --defect-line: #e5ebf3;
  --defect-surface: #ffffff;
  min-height: 100%;
  padding: 24px 26px 34px;
  background:
    radial-gradient(circle at 100% 0, rgba(219, 234, 254, .48), transparent 320px),
    #f5f7fb;
}

.page-header {
  margin-bottom: 16px;
  padding: 3px 2px;
}

.page-title {
  gap: 13px;
}

.page-title__icon {
  width: 46px;
  height: 46px;
  border: 1px solid rgba(147, 197, 253, .55);
  border-radius: 14px;
  color: #fff;
  background: linear-gradient(145deg, #4f8df7, #2563eb);
  box-shadow: 0 10px 24px rgba(37, 99, 235, .22);
}

.page-title p {
  margin-bottom: 3px;
  color: #5781c5;
  font-size: 9px;
  letter-spacing: .14em;
}

.page-title h1 {
  font-size: 23px;
  letter-spacing: -.02em;
}

.page-title > div > span {
  margin-top: 4px;
  color: var(--defect-muted);
  font-size: 11px;
}

.page-actions :deep(.el-button) {
  height: 38px;
  padding: 0 16px;
  border-radius: 10px;
  font-weight: 600;
}

.refresh-button {
  color: #52627a;
  border-color: #dce4ef;
  background: rgba(255, 255, 255, .82);
}

.create-button {
  border-color: transparent;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  box-shadow: 0 8px 18px rgba(37, 99, 235, .2);
}

.quality-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 11px;
  margin-bottom: 14px;
}

.overview-card {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  overflow: hidden;
  min-height: 78px;
  padding: 14px 16px;
  border: 1px solid var(--defect-line);
  border-radius: 14px;
  background: rgba(255, 255, 255, .92);
  box-shadow: 0 7px 20px rgba(30, 48, 78, .04);
}

.overview-card::after {
  position: absolute;
  top: 0;
  right: 0;
  width: 58px;
  height: 58px;
  border-radius: 0 0 0 58px;
  background: currentColor;
  content: '';
  opacity: .035;
}

.overview-card > span {
  display: grid;
  flex: 0 0 38px;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 11px;
  font-size: 17px;
}

.overview-card > div {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: baseline;
  column-gap: 9px;
}

.overview-card small {
  grid-column: 1 / -1;
  margin-bottom: 1px;
  color: #66758c;
  font-size: 10px;
  font-weight: 600;
}

.overview-card strong {
  color: #172033;
  font-size: 22px;
  line-height: 1.25;
  letter-spacing: -.02em;
}

.overview-card em {
  color: #9aa7b8;
  font-size: 9px;
  font-style: normal;
}

.overview-card.is-total { color: #2563eb; }
.overview-card.is-progress { color: #ea580c; }
.overview-card.is-verify { color: #d97706; }
.overview-card.is-closed { color: #16a34a; }
.overview-card.is-total > span { color: #2563eb; background: #eef5ff; }
.overview-card.is-progress > span { color: #ea580c; background: #fff3e9; }
.overview-card.is-verify > span { color: #d97706; background: #fff8df; }
.overview-card.is-closed > span { color: #16a34a; background: #ebfaef; }

.module-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 52px;
  margin-bottom: 12px;
  padding: 0 14px 0 6px;
  border: 1px solid var(--defect-line);
  border-radius: 14px;
  background: rgba(255, 255, 255, .94);
  box-shadow: 0 6px 18px rgba(30, 48, 78, .035);
}

.module-navigation {
  gap: 2px;
  margin: 0;
  padding: 5px;
  border: 0;
  background: transparent;
  box-shadow: none;
}

.module-navigation button {
  position: relative;
  min-height: 39px;
  padding: 0 13px 0 7px;
  border-radius: 9px;
  color: #6b7a90;
  font-size: 11px;
}

.module-icon {
  display: grid;
  flex: 0 0 27px;
  width: 27px;
  height: 27px;
  place-items: center;
  border: 1px solid #e8edf4;
  border-radius: 8px;
  color: #728198;
  background: linear-gradient(145deg, #fff, #f4f7fb);
  box-shadow: 0 2px 5px rgba(30, 48, 78, .04);
  transition: all .18s ease;
}

.module-icon .el-icon {
  font-size: 13px;
}

.module-navigation button:hover .module-icon {
  color: #3b82f6;
  border-color: #dbeafe;
  background: #eff6ff;
}

.module-navigation button.is-active {
  color: #1d4ed8;
  background: #edf5ff;
  box-shadow: inset 0 0 0 1px #dbeafe;
}

.module-navigation button.is-active .module-icon {
  color: #fff;
  border-color: #3b82f6;
  background: linear-gradient(145deg, #60a5fa, #2563eb);
  box-shadow: 0 4px 10px rgba(37, 99, 235, .2);
}

.module-navigation button.is-active::after {
  position: absolute;
  right: 13px;
  bottom: 3px;
  left: 13px;
  height: 2px;
  border-radius: 2px;
  background: #3b82f6;
  content: '';
}

.module-context {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.module-context strong {
  color: #3d4b61;
  font-size: 11px;
}

.module-context span {
  overflow: hidden;
  max-width: 290px;
  color: #9aa7b8;
  font-size: 9px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-navigation {
  gap: 2px;
  margin-bottom: 10px;
  padding: 4px;
  border-color: var(--defect-line);
  border-radius: 12px;
  box-shadow: none;
}

.status-navigation button {
  gap: 7px;
  min-height: 34px;
  padding: 0 12px;
  border-radius: 8px;
  font-size: 11px;
}

.status-navigation button b {
  display: inline-grid;
  min-width: 21px;
  height: 19px;
  place-items: center;
  padding: 0 5px;
  border-radius: 6px;
  color: #8491a5;
  background: #f1f4f8;
  font-size: 9px;
  font-weight: 700;
}

.status-navigation button.is-active b {
  color: #2563eb;
  background: #dbeafe;
}

.filter-panel {
  margin-bottom: 10px;
  padding: 0;
  border-color: var(--defect-line);
  border-radius: 13px;
  box-shadow: 0 6px 18px rgba(30, 48, 78, .03);
}

.filter-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 51px;
  padding: 0 14px;
  border-bottom: 1px solid #edf1f6;
}

.filter-panel__header > div {
  display: flex;
  align-items: center;
  gap: 9px;
}

.filter-panel__header > div:first-child > div {
  display: flex;
  flex-direction: column;
}

.filter-panel__header strong {
  color: #334155;
  font-size: 11px;
}

.filter-panel__header small {
  margin-top: 1px;
  color: #9aa7b8;
  font-size: 9px;
}

.filter-panel__header em {
  margin-left: 5px;
  padding: 3px 7px;
  border-radius: 6px;
  color: #2563eb;
  background: #eff6ff;
  font-size: 9px;
  font-style: normal;
}

.filter-icon {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 9px;
  color: #2563eb;
  background: #eef5ff;
}

.filter-panel__header :deep(.el-button) {
  height: 31px;
  border-radius: 8px;
  font-size: 11px;
}

.filter-panel__main {
  grid-template-columns: minmax(260px, 1.55fr) repeat(3, minmax(145px, 1fr));
  gap: 9px;
  padding: 13px 14px 10px;
}

.filter-panel__more {
  flex-wrap: wrap;
  gap: 8px;
  margin: 0 14px;
  padding: 9px 0 12px;
  border-top-color: #e8edf4;
}

.filter-panel__more > span {
  min-width: 58px;
  color: #8a98aa;
  font-size: 9px;
  font-weight: 600;
  text-transform: uppercase;
}

.filter-panel__more .el-select,
.custom-filter-input {
  width: 145px;
}

.filter-panel :deep(.el-input__wrapper),
.filter-panel :deep(.el-select__wrapper) {
  min-height: 34px;
  border-radius: 8px;
  background: #fafbfd;
  box-shadow: 0 0 0 1px #e5eaf1 inset;
}

.filter-panel :deep(.el-input__wrapper:hover),
.filter-panel :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9c9df inset;
}

.filter-panel :deep(.is-focus) {
  background: #fff;
  box-shadow: 0 0 0 1px #60a5fa inset !important;
}

.defect-table-card {
  border-color: var(--defect-line);
  border-radius: 14px;
  box-shadow: 0 10px 28px rgba(30, 48, 78, .045);
}

.table-heading {
  min-height: 53px;
  padding: 0 16px;
}

.table-heading > div {
  gap: 8px;
}

.table-heading strong {
  color: #283548;
  font-size: 13px;
}

.table-heading span {
  display: inline-grid;
  min-width: 24px;
  height: 20px;
  place-items: center;
  padding: 0 7px;
  border-radius: 7px;
  color: #2563eb;
  background: #edf5ff;
  font-size: 9px;
  font-weight: 700;
}

.table-heading small {
  color: #9aa7b8;
  font-size: 9px;
}

.defect-cell > strong {
  margin-top: 5px;
  color: #253247;
  font-size: 12px;
  line-height: 1.45;
}

.defect-cell > div {
  margin-top: 5px;
}

.defect-cell__meta {
  align-items: center;
  margin-top: 0 !important;
}

.defect-cell__meta > span {
  color: #2563eb;
  font-size: 9px;
  font-weight: 800;
  letter-spacing: .04em;
}

.defect-cell__meta > em {
  padding: 2px 6px;
  border-radius: 5px;
  color: #718096;
  background: #f1f4f8;
  font-size: 8px;
  font-style: normal;
}

.defect-cell :deep(.el-tag) {
  height: 19px;
  border-color: #dbe5f2;
  border-radius: 6px;
  color: #62738a;
  background: #f8fafc;
  font-size: 8px;
}

.project-cell strong {
  align-self: flex-start;
  padding: 3px 7px;
  border-radius: 6px;
  color: #245bb5;
  background: #edf5ff;
  font-size: 9px;
}

.project-cell span {
  margin-top: 5px;
  color: #4a596e;
  font-size: 10px;
  font-weight: 600;
}

.project-cell small {
  margin-top: 3px;
  color: #9aa7b8;
  font-size: 9px;
}

.level-cell span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 10px;
}

.level-cell span i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 0 3px currentColor;
  opacity: .72;
}

.level-cell small {
  margin: 5px 0 0 12px;
  font-size: 9px;
}

.table-status {
  gap: 5px;
  align-items: center;
  padding: 5px 9px;
  border: 1px solid transparent;
  border-radius: 7px;
  font-size: 9px;
}

.table-status::before {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: currentColor;
  content: '';
}

.table-status.is-slate { border-color: #e2e8f0; }
.table-status.is-blue { border-color: #dbeafe; }
.table-status.is-amber { border-color: #fde68a; }
.table-status.is-green { border-color: #bbf7d0; }

.assignee-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.assignee-cell > span {
  display: grid;
  flex: 0 0 28px;
  width: 28px;
  height: 28px;
  place-items: center;
  border: 1px solid #dbeafe;
  border-radius: 9px;
  color: #2563eb;
  background: #eff6ff;
  font-size: 10px;
  font-weight: 800;
}

.assignee-cell > div {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.assignee-cell strong {
  overflow: hidden;
  color: #48566a;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assignee-cell small {
  margin-top: 2px;
  color: #a0abba;
  font-size: 8px;
}

.due-date {
  display: inline-flex;
  flex-direction: column;
  color: #66758a;
  font-size: 10px;
}

.due-date small {
  margin-top: 3px;
  color: #dc2626;
  font-size: 8px;
  font-weight: 700;
}

.due-date.is-overdue {
  color: #b91c1c;
}

.time-text,
.custom-value {
  color: #7b889b;
  font-size: 9px;
}

.row-entry {
  color: #94a3b8;
  background: #f5f7fa;
}

.row-entry:hover {
  color: #2563eb;
  background: #eaf3ff;
}

.pagination-row {
  min-height: 54px;
  align-items: center;
  padding: 0 16px;
  background: #fbfcfe;
}

.batch-toolbar {
  min-height: 48px;
  padding: 0 16px;
  border-bottom-color: #cfe2ff;
  background: linear-gradient(90deg, #eff6ff, #f8fbff);
}

.empty-state {
  padding: 62px 0;
}

:deep(.defect-row > td.el-table__cell) {
  transition: background .16s ease;
}

:deep(.defect-row:hover > td.el-table__cell) {
  background: #f6f9fe !important;
}

:deep(.el-table th.el-table__cell) {
  height: 42px;
  border-bottom-color: #e8edf4;
  color: #6f7e92;
  background: #f7f9fc;
  font-size: 9px;
  letter-spacing: .03em;
}

:deep(.el-table td.el-table__cell) {
  padding: 11px 0;
  border-bottom-color: #edf1f6;
}

:deep(.el-table .cell) {
  padding-right: 11px;
  padding-left: 11px;
}

/* Bring phase-two panels into the same visual surface system. */
:deep(.stats-toolbar),
:deep(.metric-grid article),
:deep(.chart-card),
:deep(.config-header),
:deep(.config-card),
:deep(.permission-header),
:deep(.permission-layout) {
  border-color: var(--defect-line);
  border-radius: 14px;
  box-shadow: 0 8px 24px rgba(30, 48, 78, .04);
}

@media (max-width: 1180px) {
  .quality-overview { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .module-context { display: none; }
  .filter-panel__main { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 720px) {
  .defect-page { padding: 18px 14px 28px; }
  .page-header { align-items: center; }
  .page-title__icon { width: 42px; height: 42px; }
  .page-title h1 { font-size: 20px; }
  .quality-overview { grid-template-columns: 1fr 1fr; gap: 8px; }
  .overview-card { min-height: 70px; padding: 11px; }
  .overview-card > span { width: 34px; height: 34px; flex-basis: 34px; }
  .overview-card strong { font-size: 18px; }
  .overview-card em { display: none; }
  .module-bar { padding-right: 5px; }
  .module-navigation { overflow-x: auto; }
  .module-navigation button { white-space: nowrap; }
  .status-navigation { overflow-x: auto; }
  .status-navigation button { white-space: nowrap; }
  .filter-panel__header small { display: none; }
  .filter-panel__main { grid-template-columns: 1fr; }
  .filter-panel__more { flex-wrap: wrap; }
  .filter-panel__more .el-select,
  .custom-filter-input { width: calc(50% - 5px); }
  .table-heading small { display: none; }
  .batch-toolbar small { display: none; }
}
</style>
