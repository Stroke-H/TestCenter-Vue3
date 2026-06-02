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
  margin-bottom: 20px;
  padding: 22px 24px 26px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 12px 30px rgba(15, 23, 42, 0.06);
}

.project-card__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 22px;
}

.project-card__code {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  background: #eef6ff;
  color: #2563eb;
  font-size: 13px;
  font-weight: 700;
}

.project-card__name {
  margin: 10px 0 0;
  color: #0f172a;
  font-size: 22px;
  line-height: 1.25;
}

.project-card__meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.version-tree {
  position: relative;
  display: grid;
  gap: 6px;
  padding: 8px 0 2px;
}

.version-tree::before {
  position: absolute;
  top: 20px;
  bottom: 12px;
  left: 50%;
  width: 2px;
  transform: translateX(-50%);
  border-radius: 999px;
  background: linear-gradient(180deg, #60a5fa, #22c55e);
  content: "";
}

.version-node {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 56px minmax(0, 1fr);
  align-items: start;
  column-gap: 18px;
  min-height: 110px;
}

.version-node + .version-node {
  margin-top: 8px;
}

.version-node__marker {
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: 3px solid #ffffff;
  border-radius: 50%;
  background: #0f766e;
  color: #ffffff;
  box-shadow: 0 8px 20px rgba(15, 118, 110, 0.24);
  grid-column: 2;
  grid-row: 1;
  justify-self: center;
  margin-top: 18px;
}

.version-node__body {
  position: relative;
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 10px;
  background: linear-gradient(135deg, #f8fbff 0%, #f7fdf9 100%);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.05);
}

.version-node__body::before {
  position: absolute;
  top: 28px;
  width: 18px;
  height: 2px;
  background: #93c5fd;
  content: "";
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
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.version-node__label {
  display: block;
  margin-bottom: 5px;
  color: #64748b;
  font-size: 12px;
}

.version-node__title {
  color: #111827;
  font-size: 20px;
}

.version-node__summary {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px 14px;
  color: #64748b;
  font-size: 13px;
}

.version-node__summary :deep(.el-icon) {
  margin-right: -8px;
}

.report-list {
  display: grid;
  gap: 10px;
}

.report-row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  align-items: start;
  gap: 14px;
  width: 100%;
  min-height: 56px;
  padding: 10px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.82);
  color: #334155;
}

.report-row--toggleable {
  grid-template-columns: 120px minmax(0, 1fr) 24px;
}

.report-row__time {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.report-row__time {
  color: #475569;
  font-size: 13px;
}

.report-row__requirements {
  overflow: hidden;
  color: #0f172a;
  line-height: 1.55;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  white-space: pre-wrap;
  word-break: break-word;
}

.report-row__requirements--expanded {
  display: block;
  -webkit-line-clamp: unset;
}

.report-row__toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  width: 24px;
  height: 24px;
  padding: 0;
  border: 1px solid #dbeafe;
  border-radius: 6px;
  background: #eff6ff;
  color: #2563eb;
  cursor: pointer;
  transition: background-color 0.2s ease, border-color 0.2s ease;
}

.report-row__toggle-icon {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 7px;
  height: 7px;
  border-right: 2px solid currentColor;
  border-bottom: 2px solid currentColor;
  transform: translate(-50%, -62%) rotate(45deg);
  transform-origin: center;
}

.report-row__toggle--expanded .report-row__toggle-icon {
  transform: translate(-50%, -38%) rotate(225deg);
}

.report-row__toggle:hover {
  border-color: #93c5fd;
  background: #dbeafe;
}

@media (max-width: 900px) {
  .project-card__header,
  .version-node__header {
    flex-direction: column;
  }

  .project-card__meta,
  .version-node__summary {
    justify-content: flex-start;
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
    grid-template-columns: 1fr 24px;
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
  }
}
</style>
