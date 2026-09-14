<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  Search,
  Timer,
  ArrowRight,
  CaretBottom,
  CaretRight
} from '@element-plus/icons-vue'
import type { ProjectMemoRecord } from '../types'
import {
  deriveActiveConfigs,
  deriveDeprecatedConfigs,
  deriveCategories
} from '../composables/useProjectConfigHelpers'

const props = defineProps<{
  record?: ProjectMemoRecord
  projectCode: string
}>()

const emit = defineEmits<{
  navigateVersion: [version: string]
}>()

const searchQuery = ref('')
const selectedCategory = ref('全部')
const showDeprecated = ref(false)
const expandedEvidences = ref<Record<string, boolean>>({})

const activeConfigs = computed(() => deriveActiveConfigs(props.record))
const deprecatedConfigs = computed(() => deriveDeprecatedConfigs(props.record))
const categories = computed(() => ['全部', ...deriveCategories(activeConfigs.value)])

const filteredActiveConfigs = computed(() => {
  let list = activeConfigs.value

  if (selectedCategory.value !== '全部') {
    list = list.filter((item) => item.category === selectedCategory.value)
  }

  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim()
    list = list.filter((item) =>
      item.feature.toLowerCase().includes(q) ||
      item.value.toLowerCase().includes(q) ||
      item.scope.toLowerCase().includes(q) ||
      item.version.toLowerCase().includes(q)
    )
  }

  return list
})

const currentVersionLabel = computed(() => {
  return props.record?.current_version || '最新生效'
})

const toggleEvidence = (id: string) => {
  expandedEvidences.value[id] = !expandedEvidences.value[id]
}
</script>

<template>
  <div class="active-config-view">
    <!-- Top Status Banner -->
    <div class="status-banner">
      <div class="banner-left">
        <div class="version-badge">
          <span class="version-dot"></span>
          <span class="version-label">当前生效版本</span>
          <strong class="version-num">{{ currentVersionLabel }}</strong>
        </div>
        <div class="stat-pill">
          <span>共 <strong>{{ activeConfigs.length }}</strong> 项生效规则</span>
        </div>
      </div>

      <div class="banner-right">
        <el-input
          v-model="searchQuery"
          placeholder="搜索配置名称、参数或适用范围..."
          class="search-input"
          :prefix-icon="Search"
          clearable
        />
      </div>
    </div>

    <!-- Category Filter Pills -->
    <div class="category-filter-row" v-if="categories.length > 2">
      <button
        v-for="cat in categories"
        :key="cat"
        type="button"
        class="category-pill"
        :class="{ active: selectedCategory === cat }"
        @click="selectedCategory = cat"
      >
        <span>{{ cat }}</span>
        <span class="cat-count" v-if="cat !== '全部'">
          {{ activeConfigs.filter(i => i.category === cat).length }}
        </span>
      </button>
    </div>

    <!-- Active Configurations Grid / List -->
    <div v-if="filteredActiveConfigs.length > 0" class="configs-grid">
      <article
        v-for="item in filteredActiveConfigs"
        :key="item.id"
        class="config-card"
        :class="{ 'config-card--modified': item.previousValue }"
      >
        <div class="card-header">
          <div class="title-group">
            <h4 class="feature-title">{{ item.feature }}</h4>
            <div class="badges-row">
              <span class="category-tag">{{ item.category }}</span>
              <span class="scope-tag">{{ item.scope }}</span>
              <span v-if="item.kind === 'ai'" class="ai-tag">AI整理</span>
            </div>
          </div>

          <div class="source-group">
            <button
              type="button"
              class="version-source-btn"
              title="点击查看该版本演进时间轴"
              @click="emit('navigateVersion', item.version)"
            >
              <el-icon><Timer /></el-icon>
              <span>{{ item.version }} 引入</span>
              <el-icon class="arrow-icon"><ArrowRight /></el-icon>
            </button>
          </div>
        </div>

        <div class="card-body">
          <div class="value-display">
            <span class="value-label">生效配置值：</span>
            <span class="value-text">{{ item.value }}</span>
          </div>

          <!-- Diff comparison if modified -->
          <div v-if="item.previousValue && item.previousValue !== item.value" class="diff-banner">
            <span class="diff-tag">参数变更</span>
            <span class="diff-prev">旧值: {{ item.previousValue }}</span>
            <span class="diff-arrow">➔</span>
            <span class="diff-curr">当前: {{ item.value }}</span>
          </div>

          <!-- Evidence or source report section -->
          <div v-if="item.evidence" class="evidence-section">
            <button
              type="button"
              class="evidence-toggle"
              @click="toggleEvidence(item.id)"
            >
              <el-icon>
                <CaretBottom v-if="expandedEvidences[item.id]" />
                <CaretRight v-else />
              </el-icon>
              <span>需求原文与判定依据</span>
            </button>
            <div v-if="expandedEvidences[item.id]" class="evidence-content">
              <blockquote class="evidence-quote">“{{ item.evidence }}”</blockquote>
              <span v-if="item.sourceReportId" class="evidence-source">
                来源报告：#{{ item.sourceReportId }}
              </span>
            </div>
          </div>
        </div>
      </article>
    </div>

    <!-- Empty State -->
    <div v-else class="configs-empty">
      <el-empty
        :description="searchQuery ? '未找到匹配的配置项' : '当前暂无结构化生效配置，可先通过上方「版本与历史」进行整理，或切至经典便签查看'"
      />
    </div>

    <!-- Deprecated Configurations Accordion -->
    <div v-if="deprecatedConfigs.length > 0" class="deprecated-section">
      <button
        type="button"
        class="deprecated-toggle-btn"
        @click="showDeprecated = !showDeprecated"
      >
        <div class="deprecated-toggle-left">
          <el-icon>
            <CaretBottom v-if="showDeprecated" />
            <CaretRight v-else />
          </el-icon>
          <span class="deprecated-title">历史版本已废除配置</span>
          <span class="deprecated-badge">{{ deprecatedConfigs.length }} 项</span>
        </div>
        <span class="deprecated-hint">{{ showDeprecated ? '点击折叠' : '展开查看' }}</span>
      </button>

      <div v-if="showDeprecated" class="deprecated-list">
        <div
          v-for="item in deprecatedConfigs"
          :key="item.id"
          class="deprecated-item"
        >
          <div class="deprecated-item__left">
            <span class="deprecated-name">{{ item.feature }}</span>
            <span class="deprecated-val">最后配置：{{ item.value }}</span>
          </div>
          <div class="deprecated-item__right">
            <span class="deprecated-scope">{{ item.scope }}</span>
            <span class="deprecated-ver">废除版本：{{ item.version }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.active-config-view {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* Status Banner */
.status-banner {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 12px 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  flex-wrap: wrap;
}

.banner-left {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.version-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
}

.version-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.2);
}

.version-label {
  font-size: 12px;
  color: #64748b;
}

.version-num {
  font-size: 14px;
  font-weight: 700;
  color: #1d4ed8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.stat-pill {
  font-size: 13px;
  color: #64748b;
}

.stat-pill strong {
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.search-input {
  width: 280px;
}

/* Category Filter Pills */
.category-filter-row {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 4px;
}

.category-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.category-pill:hover {
  background: #f8fafc;
  border-color: #cbd5e1;
  color: #1e293b;
}

.category-pill.active {
  background: #2563eb;
  border-color: #2563eb;
  color: #ffffff;
  font-weight: 600;
}

.cat-count {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 9999px;
  background: rgba(0, 0, 0, 0.06);
}

.category-pill.active .cat-count {
  background: rgba(255, 255, 255, 0.25);
  color: #ffffff;
}

/* Configs Grid */
.configs-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 16px;
}

.config-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.02);
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
  overflow: hidden;
}

.config-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  bottom: 0;
  width: 3px;
  background: #3b82f6;
}

.config-card--modified::before {
  background: #f59e0b;
}

.config-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 16px rgba(15, 23, 42, 0.06);
  border-color: #93c5fd;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.title-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.feature-title {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.badges-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.category-tag {
  font-size: 11px;
  font-weight: 600;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #dbeafe;
  padding: 2px 7px;
  border-radius: 6px;
}

.scope-tag {
  font-size: 11px;
  font-weight: 500;
  color: #475569;
  background: #f1f5f9;
  padding: 2px 7px;
  border-radius: 6px;
}

.ai-tag {
  font-size: 10px;
  color: #059669;
  background: #ecfdf5;
  border: 1px solid #a7f3d0;
  padding: 1px 5px;
  border-radius: 4px;
}

.source-group {
  flex-shrink: 0;
}

.version-source-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  font-size: 11px;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  cursor: pointer;
  transition: all 0.2s ease;
}

.version-source-btn:hover {
  background: #eff6ff;
  color: #2563eb;
  border-color: #bfdbfe;
}

.arrow-icon {
  font-size: 10px;
}

.card-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.value-display {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  background: #f8fafc;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid #eef2f6;
}

.value-label {
  font-size: 12px;
  color: #64748b;
  flex-shrink: 0;
}

.value-text {
  font-size: 13px;
  font-weight: 700;
  color: #0f172a;
  word-break: break-all;
  line-height: 1.4;
}

/* Diff Banner */
.diff-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: #fffbeb;
  border: 1px solid #fef3c7;
  border-radius: 6px;
  font-size: 11px;
}

.diff-tag {
  font-weight: 700;
  color: #b45309;
  background: #fde68a;
  padding: 1px 5px;
  border-radius: 4px;
}

.diff-prev {
  color: #92400e;
  text-decoration: line-through;
}

.diff-arrow {
  color: #d97706;
}

.diff-curr {
  font-weight: 700;
  color: #b45309;
}

/* Evidence Section */
.evidence-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.evidence-toggle {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: transparent;
  border: none;
  padding: 0;
  font-size: 11px;
  color: #64748b;
  cursor: pointer;
  transition: color 0.2s ease;
}

.evidence-toggle:hover {
  color: #2563eb;
}

.evidence-content {
  padding: 8px 10px;
  background: #f8fafc;
  border-left: 2px solid #cbd5e1;
  border-radius: 0 6px 6px 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.evidence-quote {
  margin: 0;
  font-size: 12px;
  color: #475569;
  line-height: 1.5;
  font-style: italic;
}

.evidence-source {
  font-size: 10px;
  color: #94a3b8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

/* Empty State */
.configs-empty {
  padding: 40px 0;
}

/* Deprecated Section */
.deprecated-section {
  border: 1px dashed #cbd5e1;
  border-radius: 10px;
  background: #f8fafc;
  overflow: hidden;
}

.deprecated-toggle-btn {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: transparent;
  border: none;
  cursor: pointer;
  transition: background 0.2s ease;
}

.deprecated-toggle-btn:hover {
  background: #f1f5f9;
}

.deprecated-toggle-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.deprecated-title {
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
}

.deprecated-badge {
  font-size: 11px;
  color: #94a3b8;
  background: #e2e8f0;
  padding: 1px 6px;
  border-radius: 9999px;
}

.deprecated-hint {
  font-size: 11px;
  color: #94a3b8;
}

.deprecated-list {
  padding: 0 14px 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-top: 1px solid #e2e8f0;
  margin-top: 4px;
  padding-top: 8px;
}

.deprecated-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  padding: 6px 8px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
}

.deprecated-item__left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.deprecated-name {
  color: #94a3b8;
  text-decoration: line-through;
}

.deprecated-val {
  color: #64748b;
  font-size: 11px;
}

.deprecated-item__right {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 11px;
}

.deprecated-scope {
  color: #94a3b8;
}

.deprecated-ver {
  color: #ef4444;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
