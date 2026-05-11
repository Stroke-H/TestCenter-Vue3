<script setup lang="ts">
import type { NovelAuditReport } from '@/api/novelWriter'

defineProps<{
  audit: NovelAuditReport
}>()
</script>

<template>
  <section class="audit-panel">
    <div class="audit-panel__head">
      <div>
        <p class="audit-panel__kicker">Step 4</p>
        <h3 class="audit-panel__title">自动审计/评分</h3>
      </div>
      <div class="score-orb">{{ audit?.total_score || 0 }}</div>
    </div>

    <div class="score-grid">
      <div class="score-card">
        <span>AI 味</span>
        <strong>{{ audit?.ai_flavor_score || 0 }}</strong>
      </div>
      <div class="score-card">
        <span>人物一致性</span>
        <strong>{{ audit?.character_score || 0 }}</strong>
      </div>
      <div class="score-card">
        <span>剧情逻辑</span>
        <strong>{{ audit?.logic_score || 0 }}</strong>
      </div>
      <div class="score-card">
        <span>文风贴合</span>
        <strong>{{ audit?.style_score || 0 }}</strong>
      </div>
    </div>

    <div class="issue-list">
      <div v-for="issue in audit?.issues || []" :key="`${issue.title}-${issue.detail}`" class="issue-card">
        <span :class="['issue-card__severity', `issue-card__severity--${issue.severity}`]">{{ issue.severity }}</span>
        <strong>{{ issue.title }}</strong>
        <p>{{ issue.detail }}</p>
        <em>{{ issue.suggestion }}</em>
      </div>
      <el-empty v-if="!audit?.issues?.length" description="暂无审计结果" />
    </div>

    <div v-if="audit?.revision_advice" class="advice-box">
      {{ audit.revision_advice }}
    </div>
  </section>
</template>

<style scoped>
.audit-panel {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 24px;
  padding: 22px;
}

.audit-panel__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.audit-panel__kicker {
  margin: 0 0 4px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 800;
}

.audit-panel__title {
  margin: 0;
  color: #0f172a;
  font-size: 20px;
}

.score-orb {
  display: grid;
  place-items: center;
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: linear-gradient(135deg, #14b8a6, #0f766e);
  color: #ffffff;
  font-size: 24px;
  font-weight: 900;
}

.score-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 16px;
}

.score-card {
  padding: 12px;
  border-radius: 16px;
  background: #f8fafc;
}

.score-card span,
.score-card strong {
  display: block;
}

.score-card span {
  color: #64748b;
  font-size: 12px;
}

.score-card strong {
  margin-top: 6px;
  color: #0f172a;
  font-size: 20px;
}

.issue-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.issue-card {
  padding: 14px;
  border: 1px solid #fee2e2;
  border-radius: 16px;
  background: #fff7ed;
}

.issue-card__severity {
  display: inline-block;
  margin-bottom: 8px;
  padding: 3px 8px;
  border-radius: 999px;
  background: #fed7aa;
  color: #9a3412;
  font-size: 12px;
}

.issue-card__severity--high {
  background: #fecaca;
  color: #991b1b;
}

.issue-card strong,
.issue-card p,
.issue-card em {
  display: block;
}

.issue-card strong {
  color: #0f172a;
}

.issue-card p {
  margin: 8px 0;
  color: #475569;
}

.issue-card em {
  color: #0f766e;
  font-style: normal;
}

.advice-box {
  margin-top: 14px;
  padding: 14px;
  border-radius: 16px;
  background: #ecfeff;
  color: #0f766e;
  line-height: 1.7;
}

@media (max-width: 900px) {
  .score-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
