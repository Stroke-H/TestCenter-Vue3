<script setup lang="ts">
import type { NovelProject } from '@/api/novelWriter'

defineProps<{
  projects: NovelProject[]
  selectedId: string
  loading: boolean
}>()

const emit = defineEmits<{
  select: [project: NovelProject]
  create: []
  export: [project: NovelProject]
  delete: [project: NovelProject]
}>()

const stageLabelMap: Record<string, string> = {
  material_ready: '素材整理中',
  info_extracted: '事实库已生成',
  outline_ready: '大纲已生成',
  style_ready: '文风画像已生成',
  chapter_drafted: '章节创作中',
  chapter_audited: '审计修订中',
  chapter_approved: '章节已确认'
}

const formatStage = (stage: string) => stageLabelMap[stage] || '编辑中'
</script>

<template>
  <section class="project-list">
    <div class="project-list__header">
      <div>
        <p class="project-list__eyebrow">Novel Studio</p>
        <h2 class="project-list__title">小说创作入口</h2>
        <p class="project-list__desc">选择一本正在编辑或已保存的小说，继续进入素材图谱与文风生成流程。</p>
      </div>
      <el-button type="primary" @click="emit('create')">新建小说</el-button>
    </div>

    <el-skeleton v-if="loading" :rows="4" animated />
    <el-empty v-else-if="projects.length === 0" description="暂无小说项目">
      <el-button type="primary" @click="emit('create')">创建第一本小说</el-button>
    </el-empty>
    <div v-else class="project-list__items">
      <button
        v-for="project in projects"
        :key="project.id"
        :class="['project-card', { 'project-card--active': project.id === selectedId }]"
        @click="emit('select', project)"
      >
        <div class="project-card__top">
          <div class="project-card__actions">
            <el-button size="small" type="primary" plain @click.stop="emit('select', project)">编辑</el-button>
            <el-button size="small" text @click.stop="emit('export', project)">导出 Markdown</el-button>
            <el-button size="small" text type="danger" @click.stop="emit('delete', project)">删除</el-button>
          </div>
        </div>
        <strong class="project-card__title">{{ project.title }}</strong>
        <div class="project-card__meta">
          <span class="project-card__stage">
            <i class="project-card__stage-dot" />
            {{ formatStage(project.current_stage || 'material_ready') }}
          </span>
          <span>{{ project.genre || '未设置题材' }}</span>
          <span>{{ project.chapters?.length || 0 }} 章正文</span>
          <span>{{ project.outline?.chapters?.length || 0 }} 个大纲章节</span>
          <span>更新于 {{ project.updated_at || project.created_at }}</span>
        </div>
      </button>
    </div>
  </section>
</template>

<style scoped>
.project-list {
  width: 100%;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 28px;
  padding: 26px;
  min-height: 620px;
  box-shadow: 0 20px 60px rgba(15, 23, 42, 0.06);
}

.project-list__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 18px;
}

.project-list__eyebrow {
  margin: 0 0 4px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.project-list__title {
  margin: 0;
  color: #0f172a;
  font-size: 28px;
}

.project-list__desc {
  margin: 8px 0 0;
  color: #64748b;
  line-height: 1.7;
}

.project-list__items {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 16px;
}

.project-card {
  position: relative;
  width: 100%;
  min-height: 220px;
  padding: 18px;
  text-align: left;
  border: 1px solid #e2e8f0;
  border-radius: 22px;
  background: linear-gradient(145deg, #f8fafc 0%, #ffffff 100%);
  cursor: pointer;
  transition: 0.2s ease;
}

.project-card:hover,
.project-card--active {
  border-color: #14b8a6;
  background: linear-gradient(135deg, #ecfeff 0%, #f8fafc 100%);
  transform: translateY(-1px);
}

.project-card__stage,
.project-card__meta {
  color: #64748b;
  font-size: 12px;
}

.project-card__top {
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
  gap: 10px;
}

.project-card__actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.project-card__stage {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: #eefdfa;
  color: #0f766e;
  font-weight: 700;
}

.project-card__stage-dot {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: #14b8a6;
  box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.12);
}

.project-card__title {
  display: block;
  margin: 60px 0 24px;
  color: #0f172a;
  font-size: 30px;
  line-height: 1.2;
}

.project-card__meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.project-card__meta span {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 5px 10px;
  border-radius: 999px;
  background: #f1f5f9;
  white-space: nowrap;
}

@media (max-width: 760px) {
  .project-list__header {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
