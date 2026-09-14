<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  Timer,
  Document,
  Plus,
  Edit,
  Delete
} from '@element-plus/icons-vue'
import type { ProjectMemoRecord, ProjectVersionNode } from '../types'
import {
  deriveVersionTimeline,
  type VersionChangeSummary
} from '../composables/useProjectConfigHelpers'

const props = defineProps<{
  record?: ProjectMemoRecord
  projectVersionNodes?: ProjectVersionNode[]
  projectCode: string
  initialVersion?: string
}>()

const timelineList = computed(() => {
  return deriveVersionTimeline(props.record, props.projectVersionNodes)
})

const selectedVersion = ref<string>('')

// Initialize or update selected version
watch(
  [timelineList, () => props.initialVersion],
  ([list, initVer]) => {
    if (initVer && list.some(item => item.version === initVer)) {
      selectedVersion.value = initVer
    } else if (list.length > 0 && (!selectedVersion.value || !list.some(item => item.version === selectedVersion.value))) {
      selectedVersion.value = list[0]?.version || ''
    }
  },
  { immediate: true }
)

const activeVersionData = computed<VersionChangeSummary | null>(() => {
  if (!selectedVersion.value) return timelineList.value[0] || null
  return timelineList.value.find(item => item.version === selectedVersion.value) || timelineList.value[0] || null
})

const selectVersion = (ver: string) => {
  selectedVersion.value = ver
}
</script>

<template>
  <div class="timeline-view">
    <div v-if="timelineList.length > 0" class="timeline-layout">
      <!-- Left: Version Stepper / Timeline Nav -->
      <aside class="version-stepper">
        <div class="stepper-header">
          <span class="stepper-title">版本演化序列</span>
          <span class="stepper-count">{{ timelineList.length }} 个版本</span>
        </div>

        <div class="stepper-list">
          <div
            v-for="item in timelineList"
            :key="item.version"
            class="stepper-item"
            :class="{
              active: selectedVersion === item.version,
              'has-changes': item.totalChanges > 0
            }"
            @click="selectVersion(item.version)"
          >
            <div class="stepper-node">
              <div class="stepper-dot"></div>
              <div class="stepper-line"></div>
            </div>

            <div class="stepper-content">
              <div class="stepper-version-row">
                <span class="stepper-version">{{ item.version }}</span>
                <div class="stepper-badges" v-if="item.totalChanges > 0">
                  <span v-if="item.added.length" class="badge-add">+{{ item.added.length }}</span>
                  <span v-if="item.modified.length" class="badge-mod">~{{ item.modified.length }}</span>
                  <span v-if="item.deprecated.length" class="badge-del">-{{ item.deprecated.length }}</span>
                </div>
                <span v-else class="badge-empty">无配置变动</span>
              </div>

              <div class="stepper-time-row">
                <span class="stepper-time">{{ item.testTime || item.submittedAt || '无时间记录' }}</span>
              </div>
            </div>
          </div>
        </div>
      </aside>

      <!-- Right: Version Changelog Details -->
      <section class="version-detail-card" v-if="activeVersionData">
        <div class="detail-header">
          <div class="detail-title-row">
            <h3 class="detail-version-title">{{ activeVersionData.version }}</h3>
            <div class="detail-meta">
              <span v-if="activeVersionData.testTime || activeVersionData.submittedAt" class="meta-time">
                <el-icon><Timer /></el-icon>
                {{ activeVersionData.testTime || activeVersionData.submittedAt }}
              </span>
              <span v-if="activeVersionData.reportId" class="meta-report">
                关联验收报告 #{{ activeVersionData.reportId }}
              </span>
            </div>
          </div>

          <div class="detail-summary-badge" :class="{ 'has-diff': activeVersionData.totalChanges > 0 }">
            <span v-if="activeVersionData.totalChanges > 0">
              本版本共 <strong>{{ activeVersionData.totalChanges }}</strong> 项配置调整
            </span>
            <span v-else>
              本版本未涉及核心配置变动
            </span>
          </div>
        </div>

        <div class="detail-body">
          <!-- 1. Added Configurations -->
          <div v-if="activeVersionData.added.length > 0" class="change-group change-group--added">
            <div class="group-header">
              <span class="group-icon group-icon--added"><el-icon><Plus /></el-icon></span>
              <span class="group-title">本版本新引入配置（{{ activeVersionData.added.length }}）</span>
            </div>
            <div class="items-list">
              <div v-for="item in activeVersionData.added" :key="item.id" class="change-item change-item--added">
                <div class="item-header">
                  <strong class="item-name">{{ item.feature }}</strong>
                  <div class="item-tags">
                    <span class="category-tag">{{ item.category }}</span>
                    <span class="scope-tag">{{ item.scope }}</span>
                  </div>
                </div>
                <div class="item-val">
                  <span class="val-label">初始配置值：</span>
                  <span class="val-text">{{ item.value }}</span>
                </div>
                <p v-if="item.evidence" class="item-evidence">依据：“{{ item.evidence }}”</p>
              </div>
            </div>
          </div>

          <!-- 2. Modified Configurations -->
          <div v-if="activeVersionData.modified.length > 0" class="change-group change-group--modified">
            <div class="group-header">
              <span class="group-icon group-icon--modified"><el-icon><Edit /></el-icon></span>
              <span class="group-title">本版本参数变更调整（{{ activeVersionData.modified.length }}）</span>
            </div>
            <div class="items-list">
              <div v-for="item in activeVersionData.modified" :key="item.id" class="change-item change-item--modified">
                <div class="item-header">
                  <strong class="item-name">{{ item.feature }}</strong>
                  <div class="item-tags">
                    <span class="category-tag">{{ item.category }}</span>
                    <span class="scope-tag">{{ item.scope }}</span>
                  </div>
                </div>
                <div class="item-diff">
                  <span class="diff-prev">历史值：{{ item.previousValue }}</span>
                  <span class="diff-arrow">➔</span>
                  <span class="diff-curr">变更后：{{ item.value }}</span>
                </div>
                <p v-if="item.evidence" class="item-evidence">依据：“{{ item.evidence }}”</p>
              </div>
            </div>
          </div>

          <!-- 3. Deprecated Configurations -->
          <div v-if="activeVersionData.deprecated.length > 0" class="change-group change-group--deprecated">
            <div class="group-header">
              <span class="group-icon group-icon--deprecated"><el-icon><Delete /></el-icon></span>
              <span class="group-title">本版本宣布废除功能（{{ activeVersionData.deprecated.length }}）</span>
            </div>
            <div class="items-list">
              <div v-for="item in activeVersionData.deprecated" :key="item.id" class="change-item change-item--deprecated">
                <div class="item-header">
                  <strong class="item-name item-name--deprecated">{{ item.feature }}</strong>
                  <div class="item-tags">
                    <span class="category-tag">{{ item.category }}</span>
                    <span class="scope-tag">{{ item.scope }}</span>
                  </div>
                </div>
                <div class="item-val">
                  <span class="val-label">废除前最后配置：</span>
                  <span class="val-text">{{ item.value }}</span>
                </div>
                <p v-if="item.evidence" class="item-evidence">依据：“{{ item.evidence }}”</p>
              </div>
            </div>
          </div>

          <!-- No changes empty state for selected version -->
          <div v-if="activeVersionData.totalChanges === 0" class="no-changes-box">
            <div class="no-changes-icon">
              <el-icon><Document /></el-icon>
            </div>
            <h4>常规业务提测版本</h4>
            <p>该版本主要包含业务功能与缺陷修复，未在报告或便签中检测到核心开关或配置项的变更调整。</p>
          </div>
        </div>
      </section>
    </div>

    <div v-else class="empty-state">
      <el-empty description="暂无版本演进记录" />
    </div>
  </div>
</template>

<style scoped>
.timeline-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.timeline-layout {
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}

/* Version Stepper */
.version-stepper {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.stepper-header {
  padding: 14px 16px;
  background: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.stepper-title {
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
}

.stepper-count {
  font-size: 11px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.stepper-list {
  display: flex;
  flex-direction: column;
  max-height: 580px;
  overflow-y: auto;
  padding: 8px 0;
}

.stepper-item {
  display: flex;
  padding: 10px 14px;
  gap: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
}

.stepper-item:hover {
  background: #f8fafc;
}

.stepper-item.active {
  background: #eff6ff;
}

.stepper-node {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 16px;
  flex-shrink: 0;
  padding-top: 4px;
}

.stepper-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #cbd5e1;
  transition: all 0.2s ease;
}

.stepper-item.has-changes .stepper-dot {
  background: #3b82f6;
}

.stepper-item.active .stepper-dot {
  background: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.25);
  transform: scale(1.2);
}

.stepper-line {
  flex: 1;
  width: 2px;
  background: #f1f5f9;
  margin-top: 4px;
}

.stepper-item:last-child .stepper-line {
  display: none;
}

.stepper-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.stepper-version-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.stepper-version {
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.stepper-item.active .stepper-version {
  color: #2563eb;
}

.stepper-badges {
  display: flex;
  align-items: center;
  gap: 4px;
}

.badge-add {
  font-size: 10px;
  font-weight: 700;
  color: #059669;
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  padding: 1px 4px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.badge-mod {
  font-size: 10px;
  font-weight: 700;
  color: #d97706;
  background: #fffbeb;
  border: 1px solid #fde68a;
  padding: 1px 4px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.badge-del {
  font-size: 10px;
  font-weight: 700;
  color: #dc2626;
  background: #fef2f2;
  border: 1px solid #fecaca;
  padding: 1px 4px;
  border-radius: 4px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.badge-empty {
  font-size: 10px;
  color: #94a3b8;
}

.stepper-time-row {
  display: flex;
}

.stepper-time {
  font-size: 11px;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* Detail Card */
.version-detail-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding-bottom: 16px;
  border-bottom: 1px solid #f1f5f9;
  flex-wrap: wrap;
}

.detail-title-row {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.detail-version-title {
  margin: 0;
  font-size: 20px;
  font-weight: 800;
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.detail-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: #64748b;
}

.meta-time {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.meta-report {
  color: #2563eb;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.detail-summary-badge {
  font-size: 12px;
  color: #64748b;
  background: #f8fafc;
  padding: 6px 12px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
}

.detail-summary-badge.has-diff {
  background: #eff6ff;
  border-color: #bfdbfe;
  color: #1d4ed8;
}

.detail-summary-badge strong {
  font-size: 14px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Detail Body Groups */
.detail-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.change-group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.group-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.group-icon {
  width: 22px;
  height: 22px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
}

.group-icon--added {
  background: #ecfdf5;
  color: #059669;
}

.group-icon--modified {
  background: #fffbeb;
  color: #d97706;
}

.group-icon--deprecated {
  background: #fef2f2;
  color: #dc2626;
}

.group-title {
  font-size: 13px;
  font-weight: 700;
  color: #1e293b;
}

.items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.change-item {
  padding: 12px 14px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  transition: all 0.2s ease;
}

.change-item--added {
  border-left: 3px solid #10b981;
}

.change-item--modified {
  border-left: 3px solid #f59e0b;
}

.change-item--deprecated {
  border-left: 3px solid #ef4444;
  opacity: 0.85;
}

.change-item:hover {
  background: #ffffff;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.05);
}

.item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.item-name {
  font-size: 14px;
  color: #0f172a;
}

.item-name--deprecated {
  text-decoration: line-through;
  color: #94a3b8;
}

.item-tags {
  display: flex;
  align-items: center;
  gap: 6px;
}

.category-tag {
  font-size: 11px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  padding: 1px 6px;
  border-radius: 4px;
}

.scope-tag {
  font-size: 11px;
  color: #475569;
  background: #e2e8f0;
  padding: 1px 6px;
  border-radius: 4px;
}

.item-val {
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.val-label {
  color: #64748b;
}

.val-text {
  font-weight: 600;
  color: #0f172a;
}

.item-diff {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  padding: 4px 8px;
  background: #fffbeb;
  border-radius: 6px;
  width: fit-content;
}

.diff-prev {
  color: #92400e;
  text-decoration: line-through;
}

.diff-arrow {
  color: #d97706;
}

.diff-curr {
  color: #b45309;
  font-weight: 700;
}

.item-evidence {
  margin: 0;
  font-size: 11px;
  color: #64748b;
  font-style: italic;
  line-height: 1.5;
}

/* No Changes Box */
.no-changes-box {
  padding: 40px 20px;
  text-align: center;
  background: #f8fafc;
  border: 1px dashed #cbd5e1;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.no-changes-icon {
  font-size: 28px;
  color: #94a3b8;
}

.no-changes-box h4 {
  margin: 0;
  font-size: 15px;
  color: #334155;
}

.no-changes-box p {
  margin: 0;
  font-size: 12px;
  color: #64748b;
  max-width: 420px;
  line-height: 1.6;
}

@media (max-width: 860px) {
  .timeline-layout {
    grid-template-columns: 1fr;
  }
}
</style>
