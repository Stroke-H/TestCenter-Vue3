<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, CollectionTag, Refresh, Search } from '@element-plus/icons-vue'
import ProjectTreeBoard from './components/ProjectTreeBoard.vue'
import { useAcceptanceProjectTree } from './composables/useAcceptanceProjectTree'
import {
  ACCEPTANCE_REPORTS_CHANGED_EVENT,
  isAcceptanceReportsChangedStorageKey
} from '@/utils/acceptanceReportEvents'

defineOptions({ name: 'ProjectTree' })

const router = useRouter()
const {
  keyword,
  selectedProjectCode,
  loading,
  projectOptions,
  projectTree,
  selectedProject,
  selectedProjectMemo,
  updateSelectedProjectMemo,
  fetchReports
} = useAcceptanceProjectTree()

let lastAutoRefreshAt = 0

const refreshProjectTree = () => {
  const now = Date.now()
  if (now - lastAutoRefreshAt < 1000) return
  lastAutoRefreshAt = now
  fetchReports()
}

const handleVisibilityChange = () => {
  if (document.visibilityState === 'visible') {
    refreshProjectTree()
  }
}

const handleStorageChange = (event: StorageEvent) => {
  if (isAcceptanceReportsChangedStorageKey(event.key)) {
    refreshProjectTree()
  }
}

const projectMemoContent = computed({
  get: () => selectedProjectMemo.value?.content || '',
  set: (value: string) => updateSelectedProjectMemo(value)
})

const projectMemoUpdatedAt = computed(() => {
  const updatedAt = selectedProjectMemo.value?.updatedAt
  if (!updatedAt) return '未记录'

  const date = new Date(updatedAt)
  if (Number.isNaN(date.getTime())) return '未记录'

  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
})

onMounted(() => {
  fetchReports()
  window.addEventListener(ACCEPTANCE_REPORTS_CHANGED_EVENT, refreshProjectTree)
  window.addEventListener('storage', handleStorageChange)
  window.addEventListener('focus', refreshProjectTree)
  document.addEventListener('visibilitychange', handleVisibilityChange)
})

onBeforeUnmount(() => {
  window.removeEventListener(ACCEPTANCE_REPORTS_CHANGED_EVENT, refreshProjectTree)
  window.removeEventListener('storage', handleStorageChange)
  window.removeEventListener('focus', refreshProjectTree)
  document.removeEventListener('visibilitychange', handleVisibilityChange)
})
</script>

<template>
  <div class="project-tree-page">
    <div class="project-tree-page__header">
      <el-button text :icon="ArrowLeft" @click="router.push('/dashboard')">
        返回仪表盘
      </el-button>

      <div class="project-tree-page__actions">
        <el-select
          v-model="selectedProjectCode"
          filterable
          placeholder="选择项目"
          :disabled="loading || projectOptions.length === 0"
        >
          <el-option
            v-for="project in projectOptions"
            :key="project.value"
            :label="`${project.label}（${project.reportCount}）`"
            :value="project.value"
          />
        </el-select>
        <el-input
          v-model="keyword"
          :prefix-icon="Search"
          clearable
          placeholder="在当前项目内搜索版本号、需求点"
        />
        <el-button :icon="Refresh" :loading="loading" @click="fetchReports">
          刷新
        </el-button>
      </div>
    </div>

    <section
      v-if="selectedProjectCode"
      class="project-memo-card"
    >
      <div class="project-memo-card__header">
        <div class="project-memo-card__title">
          <el-icon><CollectionTag /></el-icon>
          <span>项目配置记录</span>
        </div>
        <span class="project-memo-card__meta">
          {{ selectedProject?.projectCode }}｜{{ selectedProject?.projectName || '未命名项目' }} · {{ projectMemoUpdatedAt }}
        </span>
      </div>
      <el-input
        v-model="projectMemoContent"
        type="textarea"
        :autosize="{ minRows: 2, maxRows: 6 }"
        resize="none"
        placeholder="记录这个项目的回溯信息、特殊配置、环境注意事项、书签链接等"
      />
    </section>

    <ProjectTreeBoard
      :projects="projectTree"
      :loading="loading"
    />
  </div>
</template>

<style scoped>
.project-tree-page {
  min-height: 100%;
  padding: 24px;
  background: #f6f8fb;
}

.project-tree-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}

.project-tree-page__actions {
  display: grid;
  grid-template-columns: minmax(260px, 360px) minmax(260px, 360px) auto;
  gap: 10px;
}

.project-memo-card {
  margin-bottom: 18px;
  padding: 16px 18px;
  border: 1px solid #dbeafe;
  border-radius: 12px;
  background: linear-gradient(135deg, #ffffff 0%, #f8fbff 100%);
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.05);
}

.project-memo-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
}

.project-memo-card__title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.project-memo-card__meta {
  min-width: 0;
  overflow: hidden;
  color: #64748b;
  font-size: 13px;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .project-tree-page {
    padding: 16px;
  }

  .project-tree-page__header {
    flex-direction: column;
  }

  .project-tree-page__actions {
    width: 100%;
    grid-template-columns: 1fr;
    margin-top: 0;
  }

  .project-memo-card__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .project-memo-card__meta {
    width: 100%;
    text-align: left;
  }
}
</style>
