<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
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

interface ConfigTagDefinition {
  key: string
  label: string
  pattern: RegExp
}

interface ConfigTag {
  key: string
  label: string
}

interface ConfigTagProject {
  projectCode: string
  projectName: string
  itemCount: number
}

interface ConfigTagGroup extends ConfigTag {
  itemCount: number
  projects: ConfigTagProject[]
}

const CONFIG_TAG_RULES: ConfigTagDefinition[] = [
  {
    key: 'return-interstitial',
    label: '返回插屏',
    pattern: /(返回.{0,12}(插屏|广告)|(插屏|广告).{0,12}返回|return.{0,20}interstitial|interstitial.{0,20}return)/i
  },
  { key: 'interstitial-ad', label: '插屏广告', pattern: /(插屏|interstitial|全屏广告)/i },
  { key: 'rewarded-ad', label: '激励广告', pattern: /(激励|rewarded|incentivevideo)/i },
  {
    key: 'ad-unlock',
    label: '广告解锁',
    pattern: /(解锁.{0,8}(广告|一集)|广告.{0,8}解锁|一集一出|二集一出|adunlock|unlockad)/i
  },
  {
    key: 'free-episodes',
    label: '免费集数',
    pattern: /(前.{0,4}集免费|免费集|免广告|freead|freeepisodes)/i
  },
  {
    key: 'user-segmentation',
    label: '归因与自然量',
    pattern: /(归因|自然量|organic|usersegmentation|adstrategy)/i
  },
  { key: 'preplay', label: '预播配置', pattern: /(预播|preplay)/i },
  { key: 'firebase-ab', label: 'Firebase A/B', pattern: /(firebase|a\/b|ab组|ab功能)/i },
  { key: 'promotion-link', label: '推广链归因', pattern: /推广链/i },
  { key: 'ad-provider', label: '广告平台', pattern: /(adprovider|admob|广告平台|广告商)/i },
  { key: 'payment', label: '支付配置', pattern: /(payment|stripe|支付)/i },
  { key: 'audit-mode', label: '审核模式', pattern: /(auditmode|审核模式)/i },
  { key: 'splash', label: '开屏配置', pattern: /(splash|开屏)/i },
  { key: 'api-endpoint', label: '接口配置', pattern: /(apiendpoint|接口调用|接口地址)/i },
  { key: 'app-icon', label: '图标配置', pattern: /(icon|图标)/i },
  { key: 'widget', label: '小组件', pattern: /(widget|小组件)/i }
]

const props = defineProps<Props>()
const visible = defineModel<boolean>({ required: true })
const selectedTagKey = ref('')
const configContentRef = ref<HTMLElement | null>(null)
const jumpedProjectCode = ref('')
let jumpFeedbackTimer = 0

const configRecords = computed<ProjectConfigRecord[]>(() => {
  const projectNames = new Map(
    props.projects.map((project) => [project.projectCode, project.projectName])
  )

  return Object.entries(props.projectMemos)
    .map(([projectCode, record]) => ({
      projectCode,
      projectName: projectNames.get(projectCode) || '未命名项目',
      items: [...(record.items || [])].filter(item => !item.removed).sort((left, right) => (
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

const normalizeTagKey = (value: string) => value
  .toLowerCase()
  .replace(/[\s\p{P}\p{S}]+/gu, '')

const summarizeTagLabel = (value: string) => {
  const text = value.replace(/\s+/g, ' ').trim()
  if (!text) return '未命名配置'
  return text.length > 14 ? `${text.slice(0, 14)}…` : text
}

const getItemTags = (item: ProjectMemoItem): ConfigTag[] => {
  if (item.category) return [{ key: `category:${item.category}`, label: item.category }]
  if (item.kind !== 'ai' && !item.feature) return [{ key: 'manual-notes', label: '人工便签（未分类）' }]
  const source = `${item.configKey || ''} ${item.content || ''}`
    .toLowerCase()
    .replace(/\s+/g, '')
  const matchedTags = CONFIG_TAG_RULES
    .filter((rule) => rule.pattern.test(source))
    .map((rule) => ({ key: rule.key, label: rule.label }))

  if (matchedTags.length > 0) return matchedTags

  const configKey = normalizeTagKey(item.configKey || '')
  if (configKey) {
    return [{
      key: `config:${configKey}`,
      label: summarizeTagLabel(item.content || item.configKey || '')
    }]
  }

  const contentKey = normalizeTagKey(item.content || '')
  if (!contentKey) return []
  return [{
    key: `memo:${contentKey}`,
    label: summarizeTagLabel(item.content)
  }]
}

const tagGroups = computed<ConfigTagGroup[]>(() => {
  const groups = new Map<string, {
    key: string
    label: string
    itemCount: number
    projects: Map<string, ConfigTagProject>
  }>()

  configRecords.value.forEach((project) => {
    project.items.forEach((item) => {
      getItemTags(item).forEach((tag) => {
        if (!groups.has(tag.key)) {
          groups.set(tag.key, {
            ...tag,
            itemCount: 0,
            projects: new Map()
          })
        }

        const group = groups.get(tag.key)
        if (!group) return
        group.itemCount += 1
        const currentProject = group.projects.get(project.projectCode)
        group.projects.set(project.projectCode, {
          projectCode: project.projectCode,
          projectName: project.projectName,
          itemCount: (currentProject?.itemCount || 0) + 1
        })
      })
    })
  })

  return Array.from(groups.values())
    .map((group) => ({
      key: group.key,
      label: group.label,
      itemCount: group.itemCount,
      projects: Array.from(group.projects.values()).sort((left, right) => (
        left.projectCode.localeCompare(right.projectCode, undefined, {
          numeric: true,
          sensitivity: 'base'
        })
      ))
    }))
    .sort((left, right) => (
      right.itemCount - left.itemCount || left.label.localeCompare(right.label, 'zh-CN')
    ))
})

const selectedTagGroup = computed(() => (
  tagGroups.value.find((group) => group.key === selectedTagKey.value) || null
))

const displayedConfigRecords = computed(() => {
  if (!selectedTagGroup.value) return configRecords.value
  const projectCodes = new Set(
    selectedTagGroup.value.projects.map((project) => project.projectCode)
  )
  return configRecords.value.filter((project) => projectCodes.has(project.projectCode))
})

const isRecordMatched = (item: ProjectMemoItem) => (
  Boolean(selectedTagKey.value) && getItemTags(item).some((tag) => tag.key === selectedTagKey.value)
)

const selectTag = async (tagKey: string) => {
  selectedTagKey.value = tagKey
  jumpedProjectCode.value = ''
  await nextTick()
  configContentRef.value?.scrollTo({ top: 0, behavior: 'smooth' })
}

const jumpToProject = async (projectCode: string) => {
  await nextTick()
  const content = configContentRef.value
  if (!content) return
  const section = Array.from(
    content.querySelectorAll<HTMLElement>('[data-project-code]')
  ).find((element) => element.dataset.projectCode === projectCode)
  if (!section) return

  content.scrollTo({
    top: Math.max(0, section.offsetTop - content.offsetTop - 6),
    behavior: 'smooth'
  })
  jumpedProjectCode.value = projectCode
  window.clearTimeout(jumpFeedbackTimer)
  jumpFeedbackTimer = window.setTimeout(() => {
    if (jumpedProjectCode.value === projectCode) {
      jumpedProjectCode.value = ''
    }
  }, 1600)
}

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

watch(visible, (isVisible) => {
  if (isVisible) return
  selectedTagKey.value = ''
  jumpedProjectCode.value = ''
  window.clearTimeout(jumpFeedbackTimer)
})

watch(tagGroups, (groups) => {
  if (selectedTagKey.value && !groups.some((group) => group.key === selectedTagKey.value)) {
    selectedTagKey.value = ''
  }
})
</script>

<template>
  <el-dialog
    v-model="visible"
    class="project-config-records-dialog"
    width="min(1080px, calc(100vw - 32px))"
    align-center
    destroy-on-close
  >
    <template #header>
      <div class="config-dialog__header">
        <div class="config-dialog__heading">
          <el-icon><CollectionTag /></el-icon>
          <div class="config-dialog__heading-content">
            <div class="config-dialog__title-row">
              <h2 class="config-dialog__title">全部项目配置</h2>
              <div
                v-if="tagGroups.length > 0"
                class="config-dialog__tags"
                aria-label="项目配置标签"
              >
                <button
                  type="button"
                  class="config-tag"
                  :class="{ 'config-tag--active': !selectedTagKey }"
                  @click="selectTag('')"
                >
                  <span>全部</span>
                  <strong>{{ totalRecordCount }}</strong>
                </button>
                <button
                  v-for="tag in tagGroups"
                  :key="tag.key"
                  type="button"
                  class="config-tag"
                  :class="{ 'config-tag--active': selectedTagKey === tag.key }"
                  :title="`${tag.itemCount} 条配置，涉及 ${tag.projects.length} 个项目`"
                  @click="selectTag(tag.key)"
                >
                  <span>{{ tag.label }}</span>
                  <strong>{{ tag.itemCount }}</strong>
                </button>
              </div>
            </div>
            <p class="config-dialog__summary">
              {{ configRecords.length }} 个项目，共 {{ totalRecordCount }} 条配置记录
            </p>
          </div>
        </div>
      </div>
    </template>

    <div v-if="configRecords.length > 0" class="config-dialog__workspace">
      <section v-if="selectedTagGroup" class="config-filter-overview">
        <div class="config-filter-overview__heading">
          <div>
            <span class="config-filter-overview__eyebrow">当前标签</span>
            <strong>{{ selectedTagGroup.label }}</strong>
            <span>
              {{ selectedTagGroup.projects.length }} 个项目 · {{ selectedTagGroup.itemCount }} 条相关配置
            </span>
          </div>
          <button type="button" class="config-filter-overview__clear" @click="selectTag('')">
            查看全部
          </button>
        </div>

        <div class="config-filter-projects" aria-label="相关项目">
          <button
            v-for="project in selectedTagGroup.projects"
            :key="project.projectCode"
            type="button"
            class="config-filter-project"
            @click="jumpToProject(project.projectCode)"
          >
            <strong>{{ project.projectCode }}</strong>
            <span>{{ project.projectName }}</span>
            <em>{{ project.itemCount }}</em>
          </button>
        </div>
      </section>

      <div ref="configContentRef" class="config-dialog__content">
        <section
          v-for="project in displayedConfigRecords"
          :key="project.projectCode"
          class="config-project"
          :class="{ 'config-project--jumped': jumpedProjectCode === project.projectCode }"
          :data-project-code="project.projectCode"
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
              :class="[
                `config-record--${record.color}`,
                { 'config-record--matched': isRecordMatched(record) }
              ]"
            >
              <p class="config-record__content">{{ record.content }}</p>
              <div class="config-record__meta">
                <span v-if="isRecordMatched(record)" class="config-record__match">匹配</span>
                <time class="config-record__time">{{ formatRecordTime(record.updatedAt) }}</time>
              </div>
            </div>
          </div>
        </section>
      </div>
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
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: #ffffff;
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.16);
}

:global(.project-config-records-dialog .el-dialog__header) {
  margin: 0;
  padding: 20px 24px 14px;
  border-bottom: 1px solid #f1f5f9;
}

:global(.project-config-records-dialog .el-dialog__body) {
  padding: 16px 24px 24px;
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
  min-width: 0;
  color: #0f172a;
}

.config-dialog__heading > .el-icon {
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  color: #2563eb;
  font-size: 22px;
  margin-top: 2px;
}

.config-dialog__heading-content {
  min-width: 0;
  flex: 1;
}

.config-dialog__title-row {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 12px;
}

.config-dialog__title {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  letter-spacing: -0.01em;
}

.config-dialog__tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 10px 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  max-height: 140px;
  overflow-y: auto;
  scrollbar-width: thin;
}

.config-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  padding: 4px 10px;
  border: 1px solid #e2e8f0;
  border-radius: 7px;
  background: #ffffff;
  color: #475569;
  font: inherit;
  font-size: 12px;
  line-height: 1.4;
  cursor: pointer;
  transition: all 0.16s ease;
}

.config-tag:hover {
  border-color: #93c5fd;
  background: #eff6ff;
  color: #2563eb;
}

.config-tag strong {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 10px;
  font-weight: 700;
  flex-shrink: 0;
}

.config-tag--active {
  border-color: #3b82f6;
  background: #eff6ff;
  color: #1d4ed8;
  font-weight: 600;
}

.config-tag--active strong {
  background: rgba(37, 99, 235, 0.15);
  color: #1d4ed8;
}

.config-dialog__summary {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 12px;
}

.config-dialog__workspace {
  display: flex;
  max-height: min(68vh, 680px);
  flex-direction: column;
  gap: 14px;
}

/* Filter Overview Banner */
.config-filter-overview {
  flex: 0 0 auto;
  padding: 12px 16px;
  border: 1px solid #dbeafe;
  border-radius: 10px;
  background: linear-gradient(135deg, #f0fdf4 0%, #eff6ff 100%);
}

.config-filter-overview__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 10px;
}

.config-filter-overview__heading > div {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
}

.config-filter-overview__eyebrow {
  color: #2563eb;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.config-filter-overview__heading strong {
  color: #1e3a8a;
  font-size: 14px;
}

.config-filter-overview__heading span:last-child {
  color: #64748b;
  font-size: 12px;
}

.config-filter-overview__clear {
  padding: 4px 10px;
  border: 1px solid #cbd5e1;
  border-radius: 6px;
  background: #ffffff;
  color: #2563eb;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.config-filter-overview__clear:hover {
  background: #eff6ff;
  border-color: #93c5fd;
}

.config-filter-projects {
  display: flex;
  gap: 7px;
  overflow-x: auto;
  padding: 2px 1px 4px;
  scrollbar-width: thin;
}

.config-filter-project {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  padding: 4px 10px;
  border: 1px solid #e2e8f0;
  border-radius: 7px;
  background: #ffffff;
  color: #475569;
  font: inherit;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.config-filter-project:hover {
  border-color: #93c5fd;
  color: #2563eb;
}

.config-filter-project strong {
  color: #1e3a8a;
  font-weight: 700;
}

.config-filter-project em {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 16px;
  height: 16px;
  border-radius: 999px;
  background: #eff6ff;
  color: #2563eb;
  font-size: 10px;
  font-style: normal;
  font-weight: 700;
}

/* Content List */
.config-dialog__content {
  min-height: 0;
  flex: 1 1 auto;
  max-height: min(68vh, 680px);
  overflow-y: auto;
  padding-right: 6px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.config-project {
  padding: 16px 18px;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.03);
  scroll-margin-top: 6px;
  transition: background 0.2s ease, box-shadow 0.2s ease;
}

.config-project--jumped {
  border-color: #3b82f6;
  background: #eff6ff;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.config-project__header {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #f1f5f9;
}

.config-project__code {
  color: #2563eb;
  font-size: 13px;
  font-weight: 700;
}

.config-project__name {
  color: #0f172a;
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.config-project__count {
  color: #94a3b8;
  font-size: 11px;
  margin-left: auto;
}

.config-project__records {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 10px;
}

/* Individual Config Record Card */
.config-record {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
  border-left: 4px solid #10b981;
  background: #f0fdf4;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.02);
  gap: 8px;
  transition: all 0.15s ease;
}

.config-record:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 10px rgba(15, 23, 42, 0.05);
}

.config-record--orange {
  border-left-color: #f59e0b;
  border-color: #fef3c7;
  background: #fffbeb;
}

.config-record--red {
  border-left-color: #f43f5e;
  border-color: #ffe4e6;
  background: #fff1f2;
}

.config-record--blue {
  border-left-color: #3b82f6;
  border-color: #dbeafe;
  background: #eff6ff;
}

.config-record--matched {
  border-left-color: #6366f1;
  background: #f5f3ff;
  border-color: #ddd6fe;
  box-shadow: 0 0 0 1px rgba(99, 102, 241, 0.2);
}

.config-record__content {
  margin: 0;
  color: #1e293b;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.config-record__meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: auto;
  padding-top: 4px;
}

.config-record__time {
  color: #64748b;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.config-record__match {
  display: inline-flex;
  align-items: center;
  height: 18px;
  padding: 0 6px;
  border-radius: 999px;
  background: #6366f1;
  color: #ffffff;
  font-size: 10px;
  font-weight: 700;
}

/* Responsive */
@media (max-width: 640px) {
  :global(.project-config-records-dialog .el-dialog__header) {
    padding: 16px 18px 12px;
  }
  :global(.project-config-records-dialog .el-dialog__body) {
    padding: 12px 18px 18px;
  }
  .config-project__records {
    grid-template-columns: 1fr;
  }
}
</style>