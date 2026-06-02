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
          v-for="(version, index) in project.versions"
          :key="`${project.projectCode}-${version.version}`"
          class="version-node"
          :class="index % 2 === 0 ? 'version-node--left' : 'version-node--right'"
        >
          <div class="version-node__spacer" />

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
  min-height: 320px;
}

.project-card {
  margin-bottom: 24px;
  padding: 24px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 4px 20px rgba(15, 23, 42, 0.03);
  transition: box-shadow 0.3s ease;
}

.project-card:hover {
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
}

.project-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
  border-bottom: 1px solid #f1f5f9;
  padding-bottom: 16px;
}

.project-card__code {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0 10px;
  border-radius: 6px;
  background: #eff6ff;
  color: #3b82f6;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.5px;
}

.project-card__name {
  margin: 8px 0 0;
  color: #0f172a;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.3;
}

.project-card__meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.project-card__meta :deep(.el-tag) {
  border: none;
  font-weight: 500;
}

.version-tree {
  position: relative;
  display: grid;
  gap: 16px;
  padding: 16px 0 8px;
}

/* Beautiful central timeline line */
.version-tree::before {
  position: absolute;
  top: 20px;
  bottom: 20px;
  left: 50%;
  width: 2px;
  transform: translateX(-50%);
  border-radius: 999px;
  background: #e2e8f0;
  content: "";
}

.version-node {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 56px minmax(0, 1fr);
  align-items: start;
  column-gap: 18px;
}

.version-node + .version-node {
  margin-top: 12px;
}

/* Clean, glowing timeline marker */
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
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.1);
  grid-column: 2;
  grid-row: 1;
  justify-self: center;
  margin-top: 20px;
  transition: all 0.3s ease;
}

.version-node:hover .version-node__marker {
  background: #3b82f6;
  color: #ffffff;
  box-shadow: 0 0 0 6px rgba(59, 130, 246, 0.15);
}

.version-node__marker :deep(.el-icon) {
  font-size: 13px;
}

/* Glassmorphism body layout */
.version-node__body {
  position: relative;
  padding: 18px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.02);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.version-node__body:hover {
  transform: translateY(-2px);
  border-color: #cbd5e1;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.06);
}

/* Connector line */
.version-node__body::before {
  position: absolute;
  top: 32px;
  width: 18px;
  height: 2px;
  background: #e2e8f0;
  content: "";
  transition: background-color 0.3s ease;
}

.version-node:hover .version-node__body::before {
  background: #3b82f6;
}

.version-node--left .version-node__body {
  grid-column: 1;
  grid-row: 1;
}

.version-node--left .version-node__spacer {
  grid-column: 3;
  grid-row: 1;
}

.version-node--left .version-node__body::before {
  right: -19px;
}

.version-node--right .version-node__spacer {
  grid-column: 1;
  grid-row: 1;
}

.version-node--right .version-node__body {
  grid-column: 3;
  grid-row: 1;
}

.version-node--right .version-node__body::before {
  left: -19px;
}

.version-node__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  border-bottom: 1px solid #f8fafc;
  padding-bottom: 10px;
}

.version-node__label {
  display: inline-block;
  margin-right: 8px;
  color: #64748b;
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.version-node__title {
  color: #0f172a;
  font-size: 18px;
  font-weight: 700;
}

.version-node__summary {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #64748b;
  font-size: 12px;
}

.version-node__summary :deep(.el-icon) {
  font-size: 13px;
  color: #94a3b8;
}

.report-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* Report row optimization - Slate aesthetic */
.report-row {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  align-items: start;
  gap: 16px;
  width: 100%;
  min-height: 48px;
  padding: 10px 14px;
  border: 1px solid #f1f5f9;
  border-radius: 8px;
  background: #f8fafc;
  color: #334155;
  transition: all 0.2s ease;
}

.report-row:hover {
  background: #f1f5f9;
  border-color: #e2e8f0;
  color: #0f172a;
}

.report-row--toggleable {
  grid-template-columns: 110px minmax(0, 1fr) 24px;
}

.report-row__time {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #64748b;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  margin-top: 2px;
}

.report-row__time :deep(.el-icon) {
  color: #94a3b8;
  font-size: 13px;
}

.report-row__requirements {
  overflow: hidden;
  color: #334155;
  font-size: 13px;
  line-height: 1.6;
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

/* Beautiful custom toggle arrow button */
.report-row__toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  width: 20px;
  height: 20px;
  padding: 0;
  border: 1px solid #e2e8f0;
  border-radius: 4px;
  background: #ffffff;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s ease;
  outline: none;
  align-self: center;
}

.report-row__toggle-icon {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 6px;
  height: 6px;
  border-right: 1.5px solid currentColor;
  border-bottom: 1.5px solid currentColor;
  transform: translate(-50%, -65%) rotate(45deg);
  transform-origin: center;
  transition: transform 0.2s ease;
}

.report-row__toggle--expanded .report-row__toggle-icon {
  transform: translate(-50%, -35%) rotate(225deg);
}

.report-row__toggle:hover {
  border-color: #3b82f6;
  background: #eff6ff;
  color: #3b82f6;
}

@media (max-width: 900px) {
  .project-card__header,
  .version-node__header {
    flex-direction: column;
    align-items: flex-start;
  }

  .project-card__meta,
  .version-node__summary {
    justify-content: flex-start;
    margin-top: 8px;
  }

  .version-tree::before {
    left: 18px;
  }

  .version-node {
    grid-template-columns: 36px minmax(0, 1fr);
    column-gap: 16px;
    min-height: 0;
  }

  .version-node__spacer {
    display: none;
  }

  .version-node__marker {
    grid-column: 1;
    margin-top: 18px;
    width: 24px;
    height: 24px;
  }

  .version-node__marker :deep(.el-icon) {
    font-size: 11px;
  }

  .version-node--left .version-node__body,
  .version-node--right .version-node__body {
    grid-column: 2;
  }

  .version-node__body::before {
    left: -17px;
    right: auto;
    width: 16px;
  }

  .report-row {
    grid-template-columns: 1fr;
    gap: 6px;
  }

  .report-row--toggleable {
    grid-template-columns: 1fr 20px;
  }

  .report-row__time {
    grid-column: 1 / -1;
  }

  .report-row__requirements {
    grid-column: 1;
    white-space: normal;
  }

  .report-row__toggle {
    grid-column: 2;
    grid-row: 2;
    align-self: flex-start;
    margin-top: 2px;
  }
}
</style>
