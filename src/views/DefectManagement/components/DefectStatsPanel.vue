<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Clock, Refresh, TrendCharts, Warning, CircleCheck } from '@element-plus/icons-vue'
import { defectApi } from '../api'
import type { DefectMeta, DefectStats } from '../types'

const props = defineProps<{ meta: DefectMeta | null }>()
const loading = ref(false)
const stats = ref<DefectStats | null>(null)
const filters = reactive({ project_code: '', dates: [] as string[] })

const maxProject = computed(() => Math.max(1, ...(stats.value?.project_distribution || []).map((item) => item.count)))
const maxAssignee = computed(() => Math.max(1, ...(stats.value?.assignee_distribution || []).map((item) => item.count)))
const maxTrend = computed(() => Math.max(1, ...(stats.value?.trend || []).flatMap((item) => [item.created, item.closed])))

async function load() {
  loading.value = true
  try {
    stats.value = await defectApi.stats({
      project_code: filters.project_code,
      date_from: filters.dates?.[0],
      date_to: filters.dates?.[1]
    })
  } catch (error: any) {
    ElMessage.error(error?.response?.data?.error || error?.customMessage || '统计数据加载失败')
  } finally {
    loading.value = false
  }
}

function compactDate(value: string) {
  return value.slice(5).replace('-', '/')
}

onMounted(load)
defineExpose({ refresh: load })
</script>

<template>
  <section v-loading="loading" class="stats-panel">
    <div class="stats-toolbar">
      <div><strong>质量概览</strong><span>从缺陷生命周期中实时计算</span></div>
      <div>
        <el-select v-model="filters.project_code" clearable filterable placeholder="全部项目" @change="load"><el-option v-for="project in props.meta?.projects || []" :key="project.id" :label="`${project.project_code} · ${project.project_name}`" :value="project.project_code" /></el-select>
        <el-date-picker v-model="filters.dates" type="daterange" value-format="YYYY-MM-DD" range-separator="至" start-placeholder="开始日期" end-placeholder="结束日期" @change="load" />
        <el-button :icon="Refresh" @click="load">刷新</el-button>
      </div>
    </div>

    <template v-if="stats">
      <div class="metric-grid">
        <article><span class="metric-icon is-blue"><el-icon><TrendCharts /></el-icon></span><div><small>全部缺陷</small><strong>{{ stats.total }}</strong><em>当前筛选范围</em></div></article>
        <article><span class="metric-icon is-orange"><el-icon><Warning /></el-icon></span><div><small>处理中</small><strong>{{ stats.active }}</strong><em>{{ stats.new }} 条待确认</em></div></article>
        <article><span class="metric-icon is-amber"><el-icon><Clock /></el-icon></span><div><small>待验证</small><strong>{{ stats.resolved }}</strong><em>{{ stats.overdue }} 条已逾期</em></div></article>
        <article><span class="metric-icon is-green"><el-icon><CircleCheck /></el-icon></span><div><small>关闭率</small><strong>{{ stats.closure_rate.toFixed(1) }}%</strong><em>平均解决 {{ stats.average_resolve_hours.toFixed(1) }}h</em></div></article>
      </div>

      <div class="chart-grid">
        <article class="chart-card distribution-card">
          <header><div><strong>状态分布</strong><span>当前缺陷流转情况</span></div><em>{{ stats.reopened }} 次重新激活</em></header>
          <div class="donut-layout">
            <div class="donut" :style="{ '--closed': `${stats.total ? stats.closed / stats.total * 100 : 0}%`, '--resolved': `${stats.total ? (stats.closed + stats.resolved) / stats.total * 100 : 0}%`, '--active': `${stats.total ? (stats.closed + stats.resolved + stats.active) / stats.total * 100 : 0}%` }"><span><strong>{{ stats.total }}</strong><small>缺陷总数</small></span></div>
            <div class="legend"><div v-for="(item, index) in stats.status_distribution" :key="item.key"><i :class="`tone-${index}`" /><span>{{ item.label }}</span><strong>{{ item.count }}</strong></div></div>
          </div>
        </article>
        <article class="chart-card">
          <header><div><strong>严重程度</strong><span>按影响等级分布</span></div></header>
          <div class="severity-bars"><div v-for="item in stats.severity_distribution" :key="item.key"><div><span>S{{ item.key }} · {{ item.label }}</span><strong>{{ item.count }}</strong></div><i><em :class="`is-s${item.key}`" :style="{ width: `${stats.total ? item.count / stats.total * 100 : 0}%` }" /></i></div></div>
        </article>
      </div>

      <article class="chart-card trend-card">
        <header><div><strong>近 14 天趋势</strong><span>新增与关闭数量对比</span></div><div class="trend-legend"><span><i class="is-created" />新增</span><span><i class="is-closed" />关闭</span></div></header>
        <div class="trend-chart"><div v-for="item in stats.trend" :key="item.date" class="trend-column"><div class="trend-column__bars"><i class="is-created" :style="{ height: `${item.created / maxTrend * 100}%` }" :title="`新增 ${item.created}`" /><i class="is-closed" :style="{ height: `${item.closed / maxTrend * 100}%` }" :title="`关闭 ${item.closed}`" /></div><span>{{ compactDate(item.date) }}</span></div></div>
      </article>

      <div class="chart-grid">
        <article class="chart-card rank-card"><header><div><strong>项目缺陷排行</strong><span>最多显示十个项目</span></div></header><div v-if="stats.project_distribution.length" class="rank-list"><div v-for="(item,index) in stats.project_distribution" :key="item.key"><span>{{ index + 1 }}</span><div><strong>{{ item.key }} · {{ item.label }}</strong><i><em :style="{ width: `${item.count / maxProject * 100}%` }" /></i></div><b>{{ item.count }}</b></div></div><p v-else class="chart-empty">暂无项目数据</p></article>
        <article class="chart-card rank-card"><header><div><strong>处理人负载</strong><span>按当前缺陷数量统计</span></div></header><div v-if="stats.assignee_distribution.length" class="rank-list"><div v-for="(item,index) in stats.assignee_distribution" :key="item.key"><span>{{ index + 1 }}</span><div><strong>{{ item.label }}</strong><i><em class="is-purple" :style="{ width: `${item.count / maxAssignee * 100}%` }" /></i></div><b>{{ item.count }}</b></div></div><p v-else class="chart-empty">暂无人员数据</p></article>
      </div>
    </template>
  </section>
</template>

<style scoped>
.stats-panel{min-height:360px}.stats-toolbar{display:flex;align-items:center;justify-content:space-between;gap:20px;margin-bottom:14px;padding:17px 19px;border:1px solid #e8edf5;border-radius:15px;background:#fff}.stats-toolbar>div:first-child{display:flex;flex-direction:column;gap:4px}.stats-toolbar strong{color:#172033;font-size:15px}.stats-toolbar span{color:#94a3b8;font-size:11px}.stats-toolbar>div:last-child{display:flex;gap:9px}.stats-toolbar .el-select{width:210px}.metric-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:13px;margin-bottom:13px}.metric-grid article{display:flex;align-items:center;gap:14px;padding:18px;border:1px solid #e8edf5;border-radius:15px;background:#fff}.metric-icon{display:grid;place-items:center;width:42px;height:42px;border-radius:13px;font-size:19px}.metric-icon.is-blue{color:#2563eb;background:#eff6ff}.metric-icon.is-orange{color:#ea580c;background:#fff7ed}.metric-icon.is-amber{color:#d97706;background:#fffbeb}.metric-icon.is-green{color:#16a34a;background:#f0fdf4}.metric-grid article>div{display:flex;flex-direction:column}.metric-grid small{color:#64748b;font-size:11px}.metric-grid strong{margin-top:2px;color:#0f172a;font-size:24px;line-height:1.2}.metric-grid em{margin-top:3px;color:#94a3b8;font-size:10px;font-style:normal}.chart-grid{display:grid;grid-template-columns:1fr 1fr;gap:13px;margin-bottom:13px}.chart-card{padding:19px;border:1px solid #e8edf5;border-radius:15px;background:#fff}.chart-card header{display:flex;align-items:flex-start;justify-content:space-between}.chart-card header>div:first-child{display:flex;flex-direction:column;gap:4px}.chart-card header strong{color:#273449;font-size:14px}.chart-card header span,.chart-card header em{color:#94a3b8;font-size:10px;font-style:normal}.donut-layout{display:flex;align-items:center;justify-content:center;gap:46px;min-height:190px}.donut{--closed:0%;--resolved:0%;--active:0%;display:grid;place-items:center;width:132px;height:132px;border-radius:50%;background:conic-gradient(#22c55e 0 var(--closed),#f59e0b var(--closed) var(--resolved),#3b82f6 var(--resolved) var(--active),#cbd5e1 var(--active) 100%);position:relative}.donut:after{content:"";position:absolute;inset:17px;border-radius:50%;background:#fff}.donut>span{z-index:1;display:flex;flex-direction:column;align-items:center}.donut strong{font-size:23px}.donut small{color:#94a3b8;font-size:9px}.legend{display:flex;flex-direction:column;gap:12px;min-width:120px}.legend>div{display:grid;grid-template-columns:8px 1fr auto;align-items:center;gap:8px}.legend i{width:8px;height:8px;border-radius:50%}.tone-0{background:#cbd5e1}.tone-1{background:#3b82f6}.tone-2{background:#f59e0b}.tone-3{background:#22c55e}.legend span{color:#64748b;font-size:11px}.legend strong{font-size:12px}.severity-bars{display:flex;flex-direction:column;gap:18px;margin-top:27px}.severity-bars>div>div{display:flex;justify-content:space-between;margin-bottom:7px}.severity-bars span{color:#64748b;font-size:11px}.severity-bars strong{font-size:11px}.severity-bars i,.rank-list i{display:block;height:7px;overflow:hidden;border-radius:999px;background:#f1f5f9}.severity-bars em,.rank-list em{display:block;height:100%;min-width:3px;border-radius:inherit;background:#3b82f6}.severity-bars .is-s1{background:#ef4444}.severity-bars .is-s2{background:#f97316}.severity-bars .is-s3{background:#3b82f6}.severity-bars .is-s4{background:#94a3b8}.trend-card{margin-bottom:13px}.trend-legend{display:flex;gap:12px}.trend-legend span{display:flex;align-items:center;gap:5px}.trend-legend i{width:7px;height:7px;border-radius:50%}.trend-legend .is-created{background:#3b82f6}.trend-legend .is-closed{background:#22c55e}.trend-chart{display:flex;align-items:end;gap:7px;height:190px;margin-top:20px;padding-top:8px;border-bottom:1px solid #e8edf5}.trend-column{display:flex;flex:1;min-width:0;height:100%;flex-direction:column;justify-content:flex-end;align-items:center}.trend-column__bars{display:flex;align-items:end;justify-content:center;gap:2px;width:100%;height:150px}.trend-column__bars i{width:26%;min-width:3px;max-width:10px;min-height:1px;border-radius:4px 4px 0 0}.trend-column__bars .is-created{background:#60a5fa}.trend-column__bars .is-closed{background:#4ade80}.trend-column>span{margin-top:7px;color:#94a3b8;font-size:8px;white-space:nowrap}.rank-list{display:flex;flex-direction:column;gap:13px;margin-top:20px}.rank-list>div{display:grid;grid-template-columns:24px minmax(0,1fr) 30px;align-items:center;gap:9px}.rank-list>div>span{display:grid;place-items:center;width:20px;height:20px;border-radius:7px;color:#64748b;background:#f1f5f9;font-size:9px}.rank-list>div>div{min-width:0}.rank-list strong{display:block;overflow:hidden;margin-bottom:6px;color:#475569;font-size:10px;text-overflow:ellipsis;white-space:nowrap}.rank-list b{color:#334155;font-size:11px;text-align:right}.rank-list .is-purple{background:#8b5cf6}.chart-empty{padding:50px 0;text-align:center;color:#94a3b8;font-size:11px}@media(max-width:1000px){.metric-grid{grid-template-columns:repeat(2,1fr)}}@media(max-width:760px){.stats-toolbar,.stats-toolbar>div:last-child{align-items:stretch;flex-direction:column}.stats-toolbar .el-select{width:100%}.metric-grid,.chart-grid{grid-template-columns:1fr}.donut-layout{gap:24px}.trend-column>span{display:none}}
</style>

