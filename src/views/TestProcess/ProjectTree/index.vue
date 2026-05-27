<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Refresh, Search } from '@element-plus/icons-vue'
import ProjectTreeBoard from './components/ProjectTreeBoard.vue'
import { useAcceptanceProjectTree } from './composables/useAcceptanceProjectTree'

defineOptions({ name: 'AcceptanceProjectTree' })

const router = useRouter()
const {
  keyword,
  selectedProjectCode,
  loading,
  projectOptions,
  projectTree,
  stats,
  fetchReports
} = useAcceptanceProjectTree()

onMounted(() => {
  fetchReports()
})
</script>

<template>
  <div class="project-tree-page">
    <div class="project-tree-page__header">
      <div>
        <el-button text :icon="ArrowLeft" @click="router.push('/dashboard')">
          返回仪表盘
        </el-button>
        <h1 class="project-tree-page__title">验收项目树</h1>
        <p class="project-tree-page__desc">
          根据验收报告记录，按项目代码聚合项目，以版本号生成节点，并展示测试时间与测试需求点。
        </p>
      </div>

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

    <div class="project-tree-page__stats">
      <div class="stat-tile">
        <span class="stat-tile__label">项目</span>
        <strong>{{ stats.projectCount }}</strong>
      </div>
      <div class="stat-tile">
        <span class="stat-tile__label">版本节点</span>
        <strong>{{ stats.versionCount }}</strong>
      </div>
      <div class="stat-tile">
        <span class="stat-tile__label">验收记录</span>
        <strong>{{ stats.reportCount }}</strong>
      </div>
    </div>

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
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}

.project-tree-page__title {
  margin: 8px 0 8px;
  color: #0f172a;
  font-size: 30px;
  line-height: 1.2;
}

.project-tree-page__desc {
  max-width: 760px;
  margin: 0;
  color: #64748b;
  font-size: 14px;
  line-height: 1.7;
}

.project-tree-page__actions {
  display: grid;
  grid-template-columns: minmax(260px, 360px) minmax(260px, 360px) auto;
  gap: 10px;
  margin-top: 32px;
}

.project-tree-page__stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 20px;
}

.stat-tile {
  min-height: 92px;
  padding: 18px;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  background: #ffffff;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.05);
}

.stat-tile__label {
  display: block;
  margin-bottom: 8px;
  color: #64748b;
  font-size: 13px;
}

.stat-tile strong {
  color: #0f172a;
  font-size: 28px;
  line-height: 1;
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

  .project-tree-page__stats {
    grid-template-columns: 1fr;
  }
}
</style>
