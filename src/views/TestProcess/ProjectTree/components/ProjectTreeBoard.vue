<script setup lang="ts">
import { CollectionTag, Document, Timer } from '@element-plus/icons-vue'
import type { ProjectTreeNode } from '../types'

defineProps<{
  projects: ProjectTreeNode[]
  loading: boolean
}>()

const formatRequirements = (value?: string) => {
  const text = (value || '').trim()
  return text || '未填写测试需求点'
}
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
              >
                <span class="report-row__time">
                  <el-icon><Document /></el-icon>
                  {{ report.test_time || report.created_at || '-' }}
                </span>
                <span class="report-row__requirements">
                  {{ formatRequirements(report.update_requirements) }}
                </span>
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
}

.version-tree::before {
  position: absolute;
  top: 16px;
  bottom: 8px;
  left: 17px;
  width: 2px;
  border-radius: 999px;
  background: linear-gradient(180deg, #60a5fa, #22c55e);
  content: "";
}

.version-node {
  position: relative;
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  gap: 16px;
}

.version-node + .version-node {
  margin-top: 18px;
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
}

.version-node__body {
  padding: 18px;
  border: 1px solid #dbeafe;
  border-radius: 10px;
  background: linear-gradient(135deg, #f8fbff 0%, #f7fdf9 100%);
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
  grid-template-columns: 180px minmax(0, 1fr);
  align-items: center;
  gap: 14px;
  width: 100%;
  min-height: 56px;
  padding: 10px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.82);
  color: #334155;
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
  text-overflow: ellipsis;
  white-space: nowrap;
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

  .report-row {
    grid-template-columns: 1fr;
    gap: 6px;
  }

  .report-row__requirements {
    white-space: normal;
  }
}
</style>
