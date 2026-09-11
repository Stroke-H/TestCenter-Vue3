<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, shallowRef, watch } from 'vue'
import { CollectionTag, Document, Timer } from '@element-plus/icons-vue'
import type { ProjectTreeNode } from '../types'

const props = defineProps<{
  projects: ProjectTreeNode[]
  loading: boolean
}>()

const formatRequirements = (value?: string) => {
  const text = (value || '').trim()
  return text || '未填写测试需求点'
}

const expandedReports = shallowRef<Set<string>>(new Set())
const overflowingReports = shallowRef<Set<string>>(new Set())
const requirementElements = new Map<string, HTMLElement>()
let measureFrame = 0
let resizeObserver: ResizeObserver | null = null

const normalizeDateText = (value: string) => value.replace(/\./g, '-').replace(/\//g, '-')

const getEndTime = (value?: string) => {
  const text = (value || '').trim()
  if (!text) return ''

  const parts = text
    .split(/(?:～|~|至|到|—|–| - )/)
    .map((item) => item.trim())
    .filter(Boolean)

  return normalizeDateText(parts[parts.length - 1] || text)
}

const getDisplayTime = (testTime?: string, createdAt?: string) => {
  return getEndTime(testTime) || getEndTime(createdAt) || '-'
}

const isReportExpanded = (id: string) => expandedReports.value.has(id)
const isReportOverflowing = (id: string) => overflowingReports.value.has(id)

const measureRequirementOverflow = () => {
  const next = new Set<string>()

  requirementElements.forEach((element, id) => {
    const style = window.getComputedStyle(element)
    const lineHeight = Number.parseFloat(style.lineHeight)
    const fallbackLineHeight = Number.parseFloat(style.fontSize) * 1.55
    const collapsedHeight = (Number.isFinite(lineHeight) ? lineHeight : fallbackLineHeight) * 2

    if (element.scrollHeight > collapsedHeight + 2) {
      next.add(id)
    }
  })

  overflowingReports.value = next
}

const scheduleMeasureRequirementOverflow = () => {
  if (measureFrame) {
    window.cancelAnimationFrame(measureFrame)
  }

  measureFrame = window.requestAnimationFrame(() => {
    measureFrame = 0
    measureRequirementOverflow()
  })
}

const setRequirementRef = (id: string, element: unknown) => {
  if (element instanceof HTMLElement) {
    requirementElements.set(id, element)
    resizeObserver?.observe(element)
  } else {
    const previous = requirementElements.get(id)
    if (previous) {
      resizeObserver?.unobserve(previous)
    }
    requirementElements.delete(id)
  }

  scheduleMeasureRequirementOverflow()
}

const toggleReportExpanded = (id: string) => {
  const next = new Set(expandedReports.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  expandedReports.value = next
}

const getReportRowClass = (reportId: string) => ({
  'report-row--toggleable': isReportOverflowing(reportId)
})

const getReportRequirementClass = (reportId: string) => ({
  'report-row__requirements--expanded': isReportExpanded(reportId)
})

onMounted(() => {
  resizeObserver = new ResizeObserver(scheduleMeasureRequirementOverflow)
  requirementElements.forEach((element) => resizeObserver?.observe(element))
  nextTick(scheduleMeasureRequirementOverflow)
})

onBeforeUnmount(() => {
  if (measureFrame) {
    window.cancelAnimationFrame(measureFrame)
  }
  resizeObserver?.disconnect()
  requirementElements.clear()
})

watch(
  () => props.projects,
  () => nextTick(scheduleMeasureRequirementOverflow),
  { deep: true }
)
</script>

<template>
  <div v-loading="loading" class="project-tree-board">
    <el-empty
      v-if="!loading && !projects.length"
      description="暂无匹配的验收报告记录"
    />

    <section
      v-for="project in projects"
      :key="project.projectCode"
      class="project-card"
    >
      <div class="project-card__header">
        <div>
          <div class="project-card__code">{{ project.projectCode }}</div>
          <h3 class="project-card__name">{{ project.projectName }}</h3>
        </div>
        <div class="project-card__meta">
          <el-tag type="info" round>{{ project.versionCount }} 个版本</el-tag>
          <el-tag type="success" round>{{ project.reportCount }} 条报告</el-tag>
        </div>
      </div>

      <div class="version-tree">
        <article
          v-for="version in project.versions"
          :key="`${project.projectCode}-${version.version}`"
          class="version-node"
        >
          <div class="version-node__marker">
            <el-icon><CollectionTag /></el-icon>
          </div>

          <div class="version-node__body">
            <div class="version-node__header">
              <div>
                <span class="version-node__label">版本号</span>
                <strong class="version-node__title">{{ version.version }}</strong>
              </div>
              <div class="version-node__summary">
                <el-icon><Timer /></el-icon>
                <span>最近测试 {{ version.latestTestTime }}</span>
                <span>{{ version.reportCount }} 条记录</span>
              </div>
            </div>

            <div class="report-list">
              <div
                v-for="report in version.reports"
                :key="report.id"
                class="report-row"
                :class="getReportRowClass(report.id)"
              >
                <span class="report-row__time">
                  <el-icon><Document /></el-icon>
                  {{ getDisplayTime(report.test_time, report.created_at) }}
                </span>
                <span
                  :ref="(element) => setRequirementRef(report.id, element)"
                  class="report-row__requirements"
                  :class="getReportRequirementClass(report.id)"
                >
                  {{ formatRequirements(report.update_requirements) }}
                </span>
                <button
                  v-if="isReportOverflowing(report.id)"
                  type="button"
                  class="report-row__toggle"
                  :class="{ 'report-row__toggle--expanded': isReportExpanded(report.id) }"
                  :aria-label="isReportExpanded(report.id) ? '收起内容' : '展开内容'"
                  @click="toggleReportExpanded(report.id)"
                >
                  <span>{{ isReportExpanded(report.id) ? '收起' : '展开' }}</span>
                  <span class="report-row__toggle-icon" />
                </button>
              </div>
            </div>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.project-tree-board {
  min-height: 280px;
}

/* Project Card */
.project-card {
  margin-bottom: 24px;
  padding: 24px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
  transition: box-shadow 0.25s ease, border-color 0.25s ease;
}

.project-card:hover {
  box-shadow: 0 8px 24px -4px rgba(15, 23, 42, 0.06);
  border-color: #cbd5e1;
}

.project-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
}

.project-card__header > div:first-child {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.project-card__code {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 6px;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  color: #2563eb;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.4px;
}

.project-card__name {
  margin: 0;
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
  overflow-wrap: anywhere;
}

.project-card__meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.project-card__meta :deep(.el-tag) {
  border-radius: 6px;
  font-weight: 600;
  font-size: 11px;
}

/* Single-rail Vertical Version Tree */
.version-tree {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: 10px 0 4px;
}

.version-tree::before {
  position: absolute;
  top: 24px;
  bottom: 24px;
  left: 13px;
  width: 2px;
  border-radius: 999px;
  background: #e2e8f0;
  content: "";
}

.version-node {
  position: relative;
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 18px;
  align-items: start;
  padding-bottom: 20px;
}

.version-node:last-child {
  padding-bottom: 0;
}

/* Luminous Marker */
.version-node__marker {
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: 2px solid #3b82f6;
  border-radius: 50%;
  background: #ffffff;
  color: #3b82f6;
  box-shadow: 0 0 0 4px #eff6ff;
  margin-top: 14px;
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
  flex-shrink: 0;
}

.version-node:hover .version-node__marker {
  background: #3b82f6;
  color: #ffffff;
  box-shadow: 0 0 0 5px rgba(59, 130, 246, 0.2);
  transform: scale(1.05);
}

.version-node__marker :deep(.el-icon) {
  font-size: 12px;
}

/* Version Card Body */
.version-node__body {
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  overflow: hidden;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.03);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.version-node:hover .version-node__body {
  border-color: #cbd5e1;
  box-shadow: 0 6px 18px -3px rgba(15, 23, 42, 0.06);
}

/* Version Header */
.version-node__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 13px 18px;
  background: #f8fafc;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
}

.version-node__header > div:first-child {
  display: flex;
  align-items: center;
  gap: 10px;
}

.version-node__label {
  display: inline-flex;
  align-items: center;
  padding: 2px 7px;
  border: 1px solid #e2e8f0;
  border-radius: 5px;
  background: #ffffff;
  color: #64748b;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.version-node__title {
  color: #0f172a;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.version-node__summary {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #64748b;
  font-size: 12px;
  flex-wrap: wrap;
}

.version-node__summary :deep(.el-icon) {
  font-size: 13px;
  color: #94a3b8;
}

.version-node__summary > span:first-of-type {
  font-variant-numeric: tabular-nums;
}

.version-node__summary > span:last-child {
  padding-left: 10px;
  border-left: 1px solid #e2e8f0;
  color: #475569;
  font-weight: 600;
}

/* Reports List */
.report-list {
  padding: 0 18px;
}

.report-row {
  display: grid;
  grid-template-columns: 140px minmax(0, 1fr);
  align-items: start;
  gap: 18px;
  padding: 14px 0;
  border-bottom: 1px solid #f1f5f9;
  color: #334155;
  transition: background-color 0.15s ease;
}

.report-row:last-child {
  border-bottom: none;
}

.report-row--toggleable {
  grid-template-columns: 140px minmax(0, 1fr) 60px;
}

.report-row__time {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #64748b;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  margin-top: 1px;
  font-variant-numeric: tabular-nums;
}

.report-row__time :deep(.el-icon) {
  color: #94a3b8;
  font-size: 13px;
  flex-shrink: 0;
}

.report-row__requirements {
  overflow: hidden;
  color: #334155;
  font-size: 13px;
  line-height: 1.7;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  white-space: pre-wrap;
  word-break: break-word;
}

.report-row:hover .report-row__requirements {
  color: #0f172a;
}

.report-row__requirements--expanded {
  display: block;
  -webkit-line-clamp: unset;
}

/* Toggle Expand Button */
.report-row__toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  width: 58px;
  height: 26px;
  padding: 0 8px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #f8fafc;
  color: #64748b;
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.18s ease;
  align-self: start;
  margin-top: 1px;
}

.report-row__toggle:hover {
  border-color: #bfdbfe;
  background: #eff6ff;
  color: #2563eb;
}

.report-row__toggle-icon {
  display: inline-block;
  width: 5px;
  height: 5px;
  border-right: 1.5px solid currentColor;
  border-bottom: 1.5px solid currentColor;
  transform: rotate(45deg);
  margin-top: -2px;
  transition: transform 0.2s ease;
}

.report-row__toggle--expanded .report-row__toggle-icon {
  transform: rotate(225deg);
  margin-top: 2px;
}

/* Responsive */
@media (max-width: 900px) {
  .project-card {
    padding: 16px;
  }
  .version-node__header {
    flex-direction: column;
    align-items: flex-start;
  }
  .report-row,
  .report-row--toggleable {
    grid-template-columns: 120px minmax(0, 1fr) 56px;
    gap: 12px;
  }
}

@media (max-width: 600px) {
  .project-card__header > div:first-child {
    flex-wrap: wrap;
  }
  .version-node {
    grid-template-columns: 22px minmax(0, 1fr);
    gap: 12px;
  }
  .version-tree::before {
    left: 10px;
  }
  .version-node__marker {
    width: 22px;
    height: 22px;
    margin-top: 16px;
  }
  .version-node__marker :deep(.el-icon) {
    font-size: 10px;
  }
  .report-list {
    padding: 0 12px;
  }
  .report-row,
  .report-row--toggleable {
    grid-template-columns: minmax(0, 1fr) 56px;
    gap: 8px;
  }
  .report-row__time {
    grid-column: 1;
    grid-row: 1;
  }
  .report-row__requirements {
    grid-column: 1 / -1;
    grid-row: 2;
  }
  .report-row__toggle {
    grid-column: 2;
    grid-row: 1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .version-node__body,
  .version-node__marker,
  .report-row__toggle {
    transition: none;
  }
}
</style>