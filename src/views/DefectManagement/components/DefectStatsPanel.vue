<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"
import { ElMessage } from "element-plus"
import { Clock, Refresh, TrendCharts, Warning, CircleCheck } from "@element-plus/icons-vue"
import { defectApi } from "../api"
import type { DefectMeta, DefectStats } from "../types"

const props = defineProps<{ meta: DefectMeta | null }>()
const loading = ref(false)
const stats = ref<DefectStats | null>(null)
const filters = reactive({ project_code: "", dates: [] as string[] })

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
    ElMessage.error(error?.response?.data?.error || error?.customMessage || "统计数据加载失败")
  } finally {
    loading.value = false
  }
}

function compactDate(value: string) {
  return value.slice(5).replace("-", "/")
}

onMounted(load)
defineExpose({ refresh: load })
</script>

<template>
  <section v-loading="loading" class="stats-panel">
    <!-- 顶部过滤与刷新栏 -->
    <div class="stats-toolbar">
      <div class="stats-toolbar__info">
        <strong>质量效能大盘</strong>
        <span>全流程缺陷生命周期多维数据指标与趋势透视</span>
      </div>
      <div class="stats-toolbar__filters">
        <el-select
          v-model="filters.project_code"
          clearable
          filterable
          placeholder="全部项目"
          class="project-select"
          @change="load"
        >
          <el-option
            v-for="project in props.meta?.projects || []"
            :key="project.id"
            :label="`${project.project_code} · ${project.project_name}`"
            :value="project.project_code"
          />
        </el-select>
        <el-date-picker
          v-model="filters.dates"
          type="daterange"
          value-format="YYYY-MM-DD"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          class="date-picker"
          @change="load"
        />
        <el-button :icon="Refresh" @click="load">刷新</el-button>
      </div>
    </div>

    <template v-if="stats">
      <!-- 4 大关键效能指标 -->
      <div class="metric-grid">
        <article class="metric-card metric-card--blue">
          <span class="metric-icon is-blue"><el-icon><TrendCharts /></el-icon></span>
          <div class="metric-body">
            <span class="metric-label">全部缺陷</span>
            <strong class="metric-value">{{ stats.total }}</strong>
            <span class="metric-sub">当前统计范围总计</span>
          </div>
        </article>
        <article class="metric-card metric-card--orange">
          <span class="metric-icon is-orange"><el-icon><Warning /></el-icon></span>
          <div class="metric-body">
            <span class="metric-label">处理中</span>
            <strong class="metric-value">{{ stats.active }}</strong>
            <span class="metric-sub">{{ stats.new }} 条待确认</span>
          </div>
        </article>
        <article class="metric-card metric-card--amber">
          <span class="metric-icon is-amber"><el-icon><Clock /></el-icon></span>
          <div class="metric-body">
            <span class="metric-label">待验证</span>
            <strong class="metric-value">{{ stats.resolved }}</strong>
            <span class="metric-sub">{{ stats.overdue }} 条已逾期</span>
          </div>
        </article>
        <article class="metric-card metric-card--green">
          <span class="metric-icon is-green"><el-icon><CircleCheck /></el-icon></span>
          <div class="metric-body">
            <span class="metric-label">修复关闭率</span>
            <strong class="metric-value">{{ stats.closure_rate.toFixed(1) }}%</strong>
            <span class="metric-sub">平均耗时 {{ stats.average_resolve_hours.toFixed(1) }}h</span>
          </div>
        </article>
      </div>

      <!-- 状态分布与严重程度 -->
      <div class="chart-grid">
        <article class="chart-card distribution-card">
          <header class="card-header">
            <div>
              <strong>状态分布</strong>
              <span>生命周期当前流转节点</span>
            </div>
            <span class="header-badge">{{ stats.reopened }} 次重新激活</span>
          </header>
          <div class="donut-layout">
            <div
              class="donut"
              :style="{
                '--closed': `${stats.total ? stats.closed / stats.total * 100 : 0}%`,
                '--resolved': `${stats.total ? (stats.closed + stats.resolved) / stats.total * 100 : 0}%`,
                '--active': `${stats.total ? (stats.closed + stats.resolved + stats.active) / stats.total * 100 : 0}%`
              }"
            >
              <div class="donut-inner">
                <strong>{{ stats.total }}</strong>
                <small>缺陷总数</small>
              </div>
            </div>
            <div class="donut-legend">
              <div v-for="(item, index) in stats.status_distribution" :key="item.key" class="legend-item">
                <i :class="`tone-${index}`" />
                <span class="legend-label">{{ item.label }}</span>
                <strong class="legend-count">{{ item.count }}</strong>
              </div>
            </div>
          </div>
        </article>

        <article class="chart-card">
          <header class="card-header">
            <div>
              <strong>严重程度分布</strong>
              <span>按影响等级 S1-S4 划分</span>
            </div>
          </header>
          <div class="severity-bars">
            <div v-for="item in stats.severity_distribution" :key="item.key" class="severity-bar-item">
              <div class="severity-bar-header">
                <span class="severity-bar-label">S{{ item.key }} · {{ item.label }}</span>
                <strong class="severity-bar-value">{{ item.count }} <em>({{ stats.total ? (item.count / stats.total * 100).toFixed(1) : 0 }}%)</em></strong>
              </div>
              <div class="bar-track">
                <div
                  class="bar-fill"
                  :class="`is-s${item.key}`"
                  :style="{ width: `${stats.total ? item.count / stats.total * 100 : 0}%` }"
                />
              </div>
            </div>
          </div>
        </article>
      </div>

      <!-- 14 天趋势图 -->
      <article class="chart-card trend-card">
        <header class="card-header">
          <div>
            <strong>近 14 天缺陷动态趋势</strong>
            <span>每日新增缺陷与关闭缺陷对比</span>
          </div>
          <div class="trend-legend">
            <span><i class="is-created" />新增</span>
            <span><i class="is-closed" />关闭</span>
          </div>
        </header>
        <div class="trend-chart">
          <div v-for="item in stats.trend" :key="item.date" class="trend-column">
            <div class="trend-column__bars">
              <div
                class="bar is-created"
                :style="{ height: `${maxTrend ? item.created / maxTrend * 100 : 0}%` }"
                :title="`${item.date} 新增 ${item.created} 条`"
              />
              <div
                class="bar is-closed"
                :style="{ height: `${maxTrend ? item.closed / maxTrend * 100 : 0}%` }"
                :title="`${item.date} 关闭 ${item.closed} 条`"
              />
            </div>
            <span class="trend-date">{{ compactDate(item.date) }}</span>
          </div>
        </div>
      </article>

      <!-- 排行榜网格 -->
      <div class="chart-grid">
        <article class="chart-card rank-card">
          <header class="card-header">
            <div>
              <strong>项目缺陷排行</strong>
              <span>缺陷集中度最高的前 10 个项目</span>
            </div>
          </header>
          <div v-if="stats.project_distribution.length" class="rank-list">
            <div v-for="(item, index) in stats.project_distribution" :key="item.key" class="rank-row">
              <span class="rank-badge" :class="{ 'is-top': index < 3 }">{{ index + 1 }}</span>
              <div class="rank-content">
                <div class="rank-text">
                  <strong class="rank-name" :title="`${item.key} · ${item.label}`">{{ item.key }} · {{ item.label }}</strong>
                  <span class="rank-number">{{ item.count }}</span>
                </div>
                <div class="rank-progress">
                  <div class="rank-progress-fill" :style="{ width: `${item.count / maxProject * 100}%` }" />
                </div>
              </div>
            </div>
          </div>
          <p v-else class="chart-empty">暂无项目数据</p>
        </article>

        <article class="chart-card rank-card">
          <header class="card-header">
            <div>
              <strong>处理人当前负载</strong>
              <span>未闭环缺陷承接排名</span>
            </div>
          </header>
          <div v-if="stats.assignee_distribution.length" class="rank-list">
            <div v-for="(item, index) in stats.assignee_distribution" :key="item.key" class="rank-row">
              <span class="rank-badge" :class="{ 'is-top': index < 3 }">{{ index + 1 }}</span>
              <div class="rank-content">
                <div class="rank-text">
                  <strong class="rank-name">{{ item.label }}</strong>
                  <span class="rank-number">{{ item.count }}</span>
                </div>
                <div class="rank-progress">
                  <div class="rank-progress-fill is-purple" :style="{ width: `${item.count / maxAssignee * 100}%` }" />
                </div>
              </div>
            </div>
          </div>
          <p v-else class="chart-empty">暂无人员数据</p>
        </article>
      </div>
    </template>
  </section>
</template>

<style scoped>
.stats-panel {
  min-height: 360px;
}

.stats-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 16px;
  padding: 16px 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.stats-toolbar__info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stats-toolbar__info strong {
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
}

.stats-toolbar__info span {
  color: #64748b;
  font-size: 13px;
}

.stats-toolbar__filters {
  display: flex;
  align-items: center;
  gap: 10px;
}

.project-select {
  width: 220px;
}

.date-picker {
  width: 260px;
}

/* Metric Cards */
.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 16px;
}

.metric-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 18px 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
  transition: all 0.2s ease;
}

.metric-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.06);
}

.metric-card--blue {
  border-left: 4px solid #3b82f6;
}

.metric-card--orange {
  border-left: 4px solid #f97316;
}

.metric-card--amber {
  border-left: 4px solid #f59e0b;
}

.metric-card--green {
  border-left: 4px solid #10b981;
}

.metric-icon {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  flex-shrink: 0;
  border-radius: 12px;
  font-size: 22px;
}

.metric-icon.is-blue {
  color: #2563eb;
  background: #eff6ff;
}

.metric-icon.is-orange {
  color: #ea580c;
  background: #fff7ed;
}

.metric-icon.is-amber {
  color: #d97706;
  background: #fffbeb;
}

.metric-icon.is-green {
  color: #16a34a;
  background: #f0fdf4;
}

.metric-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.metric-label {
  color: #64748b;
  font-size: 13px;
  font-weight: 500;
}

.metric-value {
  margin-top: 4px;
  color: #0f172a;
  font-size: 26px;
  font-weight: 700;
  line-height: 1.15;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.metric-sub {
  margin-top: 4px;
  color: #94a3b8;
  font-size: 12px;
}

/* Charts Layout */
.chart-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-bottom: 16px;
}

.chart-card {
  padding: 20px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 4px 16px rgba(15, 23, 42, 0.03);
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 20px;
}

.card-header > div:first-child {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.card-header strong {
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.card-header span {
  color: #64748b;
  font-size: 12px;
}

.header-badge {
  font-size: 12px;
  color: #b45309;
  background: #fffbeb;
  border: 1px solid #fde68a;
  padding: 3px 9px;
  border-radius: 6px;
  font-weight: 600;
}

/* Donut Chart */
.donut-layout {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 40px;
  min-height: 180px;
}

.donut {
  --closed: 0%;
  --resolved: 0%;
  --active: 0%;
  display: grid;
  place-items: center;
  width: 140px;
  height: 140px;
  border-radius: 50%;
  background: conic-gradient(
    #10b981 0 var(--closed),
    #f59e0b var(--closed) var(--resolved),
    #3b82f6 var(--resolved) var(--active),
    #cbd5e1 var(--active) 100%
  );
  position: relative;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.08);
}

.donut:after {
  content: "";
  position: absolute;
  inset: 18px;
  border-radius: 50%;
  background: #ffffff;
}

.donut-inner {
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.donut-inner strong {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.1;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.donut-inner small {
  color: #64748b;
  font-size: 12px;
  font-weight: 500;
  margin-top: 2px;
}

.donut-legend {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 140px;
}

.legend-item {
  display: grid;
  grid-template-columns: 10px 1fr auto;
  align-items: center;
  gap: 10px;
}

.legend-item i {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.tone-0 {
  background: #cbd5e1;
}

.tone-1 {
  background: #3b82f6;
}

.tone-2 {
  background: #f59e0b;
}

.tone-3 {
  background: #10b981;
}

.legend-label {
  color: #475569;
  font-size: 13px;
  font-weight: 500;
}

.legend-count {
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Severity Bars */
.severity-bars {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.severity-bar-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.severity-bar-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.severity-bar-label {
  color: #475569;
  font-size: 13px;
  font-weight: 600;
}

.severity-bar-value {
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.severity-bar-value em {
  font-style: normal;
  color: #94a3b8;
  font-weight: 400;
  font-size: 12px;
  margin-left: 4px;
}

.bar-track {
  display: block;
  height: 8px;
  overflow: hidden;
  border-radius: 9999px;
  background: #f1f5f9;
}

.bar-fill {
  display: block;
  height: 100%;
  min-width: 4px;
  border-radius: inherit;
  transition: width 0.3s ease;
}

.bar-fill.is-s1 {
  background: linear-gradient(90deg, #ef4444, #dc2626);
}

.bar-fill.is-s2 {
  background: linear-gradient(90deg, #f97316, #ea580c);
}

.bar-fill.is-s3 {
  background: linear-gradient(90deg, #60a5fa, #3b82f6);
}

.bar-fill.is-s4 {
  background: linear-gradient(90deg, #cbd5e1, #94a3b8);
}

/* Trend Card */
.trend-card {
  margin-bottom: 16px;
}

.trend-legend {
  display: flex;
  align-items: center;
  gap: 16px;
}

.trend-legend span {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #64748b;
  font-weight: 500;
}

.trend-legend i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.trend-legend .is-created {
  background: #3b82f6;
}

.trend-legend .is-closed {
  background: #10b981;
}

.trend-chart {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 200px;
  margin-top: 16px;
  padding-top: 10px;
  border-bottom: 1px solid #e2e8f0;
}

.trend-column {
  display: flex;
  flex: 1;
  min-width: 0;
  height: 100%;
  flex-direction: column;
  justify-content: flex-end;
  align-items: center;
}

.trend-column__bars {
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 4px;
  width: 100%;
  height: 160px;
}

.trend-column__bars .bar {
  width: 35%;
  min-width: 4px;
  max-width: 14px;
  min-height: 2px;
  border-radius: 4px 4px 0 0;
  transition: opacity 0.2s ease;
  cursor: pointer;
}

.trend-column__bars .bar:hover {
  opacity: 0.8;
}

.trend-column__bars .is-created {
  background: #3b82f6;
}

.trend-column__bars .is-closed {
  background: #10b981;
}

.trend-date {
  margin-top: 8px;
  color: #64748b;
  font-size: 11px;
  white-space: nowrap;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Rank List */
.rank-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rank-row {
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr);
  align-items: center;
  gap: 12px;
}

.rank-badge {
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  color: #64748b;
  background: #f1f5f9;
  font-size: 12px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.rank-badge.is-top {
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
}

.rank-content {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.rank-text {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.rank-name {
  overflow: hidden;
  color: #334155;
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rank-number {
  color: #0f172a;
  font-size: 13px;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.rank-progress {
  display: block;
  height: 6px;
  overflow: hidden;
  border-radius: 9999px;
  background: #f1f5f9;
}

.rank-progress-fill {
  display: block;
  height: 100%;
  min-width: 3px;
  border-radius: inherit;
  background: linear-gradient(90deg, #60a5fa, #3b82f6);
  transition: width 0.3s ease;
}

.rank-progress-fill.is-purple {
  background: linear-gradient(90deg, #a78bfa, #8b5cf6);
}

.chart-empty {
  padding: 40px 0;
  text-align: center;
  color: #94a3b8;
  font-size: 13px;
}

@media (max-width: 1000px) {
  .metric-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 760px) {
  .stats-toolbar,
  .stats-toolbar__filters {
    align-items: stretch;
    flex-direction: column;
  }
  .project-select,
  .date-picker {
    width: 100%;
  }
  .metric-grid,
  .chart-grid {
    grid-template-columns: 1fr;
  }
  .donut-layout {
    gap: 20px;
  }
}
</style>
