<script setup lang="ts">
import { computed } from 'vue'
import { CollectionTag } from '@element-plus/icons-vue'
import type {
  ProjectMemoItem,
  ProjectMemoRecord,
  ProjectTreeNode
} from '../types'

interface Props {
  projects: ProjectTreeNode[]
  projectMemos: Record<string, ProjectMemoRecord>
}

interface ProjectConfigRecord {
  projectCode: string
  projectName: string
  items: ProjectMemoItem[]
}

const props = defineProps<Props>()
const visible = defineModel<boolean>({ required: true })

const configRecords = computed<ProjectConfigRecord[]>(() => {
  const projectNames = new Map(
    props.projects.map((project) => [project.projectCode, project.projectName])
  )

  return Object.entries(props.projectMemos)
    .map(([projectCode, record]) => ({
      projectCode,
      projectName: projectNames.get(projectCode) || '未命名项目',
      items: [...(record.items || [])].sort((left, right) => (
        Date.parse(right.updatedAt) - Date.parse(left.updatedAt)
      ))
    }))
    .filter((record) => record.items.length > 0)
    .sort((left, right) => left.projectCode.localeCompare(right.projectCode, undefined, {
      numeric: true,
      sensitivity: 'base'
    }))
})

const totalRecordCount = computed(() => (
  configRecords.value.reduce((total, project) => total + project.items.length, 0)
))

const formatRecordTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>

<template>
  <el-dialog
    v-model="visible"
    class="project-config-records-dialog"
    width="min(960px, calc(100vw - 32px))"
    align-center
    destroy-on-close
  >
    <template #header>
      <div class="config-dialog__header">
        <div class="config-dialog__heading">
          <el-icon><CollectionTag /></el-icon>
          <div>
            <h2 class="config-dialog__title">全部项目配置</h2>
            <p class="config-dialog__summary">
              {{ configRecords.length }} 个项目，共 {{ totalRecordCount }} 条配置记录
            </p>
          </div>
        </div>
      </div>
    </template>

    <div v-if="configRecords.length > 0" class="config-dialog__content">
      <section
        v-for="project in configRecords"
        :key="project.projectCode"
        class="config-project"
      >
        <div class="config-project__header">
          <strong class="config-project__code">{{ project.projectCode }}</strong>
          <span class="config-project__name">{{ project.projectName }}</span>
          <span class="config-project__count">{{ project.items.length }} 条</span>
        </div>

        <div class="config-project__records">
          <div
            v-for="record in project.items"
            :key="record.id"
            class="config-record"
            :class="`config-record--${record.color}`"
          >
            <p class="config-record__content">{{ record.content }}</p>
            <time class="config-record__time">{{ formatRecordTime(record.updatedAt) }}</time>
          </div>
        </div>
      </section>
    </div>

    <el-empty
      v-else
      description="暂无项目配置记录"
      :image-size="88"
    />
  </el-dialog>
</template>

<style scoped>
:global(.project-config-records-dialog.el-dialog) {
  overflow: hidden;
  border: none;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.18);
}

:global(.project-config-records-dialog .el-dialog__header) {
  margin: 0;
  padding: 20px 24px 16px;
}

:global(.project-config-records-dialog .el-dialog__body) {
  padding: 0 24px 24px;
}

:global(.project-config-records-dialog .el-dialog__headerbtn) {
  top: 18px;
  right: 18px;
}

.config-dialog__header {
  padding-right: 36px;
}

.config-dialog__heading {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  color: #0f172a;
}

.config-dialog__heading > .el-icon {
  width: 30px;
  height: 30px;
  margin-top: 1px;
  color: #2563eb;
  font-size: 22px;
}

.config-dialog__title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0;
}

.config-dialog__summary {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 12px;
}

.config-dialog__content {
  max-height: min(68vh, 680px);
  overflow-y: auto;
  padding-right: 6px;
}

.config-project {
  padding: 18px 0;
  border-top: 1px solid #e2e8f0;
}

.config-project:first-child {
  border-top: none;
  padding-top: 4px;
}

.config-project__header {
  display: grid;
  grid-template-columns: minmax(80px, auto) minmax(0, 1fr) auto;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 10px;
}

.config-project__code {
  color: #0f172a;
  font-size: 14px;
}

.config-project__name {
  min-width: 0;
  overflow: hidden;
  color: #475569;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.config-project__count {
  color: #94a3b8;
  font-size: 12px;
}

.config-project__records {
  display: grid;
  gap: 8px;
}

.config-record {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
  gap: 16px;
  min-height: 38px;
  padding: 9px 12px;
  border-left: 4px solid #10b981;
  background: #f0fdf4;
}

.config-record--orange {
  border-left-color: #f97316;
  background: #fff7ed;
}

.config-record--red {
  border-left-color: #ef4444;
  background: #fef2f2;
}

.config-record--blue {
  border-left-color: #2563eb;
  background: #eff6ff;
}

.config-record__content {
  margin: 0;
  color: #1e293b;
  font-size: 13px;
  line-height: 1.55;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.config-record__time {
  color: #64748b;
  font-size: 11px;
  line-height: 1.55;
  white-space: nowrap;
}

@media (max-width: 640px) {
  :global(.project-config-records-dialog .el-dialog__header) {
    padding: 16px 16px 12px;
  }

  :global(.project-config-records-dialog .el-dialog__body) {
    padding: 0 16px 16px;
  }

  .config-project__header {
    grid-template-columns: auto 1fr;
  }

  .config-project__count {
    grid-column: 2;
  }

  .config-record {
    grid-template-columns: 1fr;
    gap: 4px;
  }
}
</style>
