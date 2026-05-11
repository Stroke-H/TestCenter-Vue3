<script setup lang="ts">
import { computed } from 'vue'
import type { NovelExtractedInfo, NovelOutline, NovelStyleProfile } from '@/api/novelWriter'

const props = defineProps<{
  extracted: NovelExtractedInfo
  outline: NovelOutline
  styleProfile: NovelStyleProfile
  open: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const hasOutline = computed(() => Boolean(props.outline.chapters?.length))
const factCards = computed(() => [
  ...(props.extracted.characters || []).map((item) => ({ ...item, type: '人物' })),
  ...(props.extracted.conflicts || []).map((item) => ({ ...item, type: '冲突' })),
  ...(props.extracted.world_rules || []).map((item) => ({ ...item, type: '世界观' })),
  ...(props.extracted.key_events || []).map((item) => ({ ...item, type: '灵感' }))
])

const toggleOpen = () => emit('update:open', !props.open)
</script>

<template>
  <aside v-if="hasOutline" :class="['insight-floating', { 'insight-floating--open': open }]">
    <button class="insight-toggle" @click="toggleOpen">
      <span>事实卡片 / 文风画像</span>
      <strong>{{ open ? '收起' : '展开' }}</strong>
    </button>

    <div v-if="open" class="insight-panel">
      <section class="insight-card">
        <p class="insight-card__label">Info Cards</p>
        <h3 class="insight-card__title">事实卡片</h3>
        <div class="fact-grid">
          <div v-for="item in factCards" :key="`${item.type}-${item.name}`" class="fact-card">
            <span>{{ item.type }}</span>
            <strong>{{ item.name }}</strong>
            <p>{{ item.description }}</p>
          </div>
          <p v-if="factCards.length === 0" class="empty-text">暂无事实卡片，建议先在素材图谱中执行信息提取。</p>
        </div>
      </section>

      <section class="insight-card">
        <p class="insight-card__label">Style Profile</p>
        <h3 class="insight-card__title">文风画像</h3>
        <p class="style-summary">{{ styleProfile.summary || '暂无文风画像。不添加参考文本时，可让模型自由生成适合题材的文风。' }}</p>
        <div class="rule-list">
          <span v-for="rule in styleProfile.do_rules || []" :key="rule">{{ rule }}</span>
          <span v-if="!styleProfile.do_rules?.length">默认原创文风</span>
        </div>
      </section>
    </div>
  </aside>
</template>

<style scoped>
.insight-floating {
  position: fixed;
  top: 118px;
  right: 28px;
  z-index: 20;
  width: 230px;
}

.insight-floating--open {
  width: min(420px, calc(100vw - 56px));
}

.insight-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 12px;
  padding: 10px 14px;
  border: 1px solid #99f6e4;
  border-radius: 999px;
  background: rgba(240, 253, 250, 0.94);
  color: #0f766e;
  box-shadow: 0 14px 34px rgba(15, 118, 110, 0.14);
  cursor: pointer;
  backdrop-filter: blur(12px);
}

.insight-toggle span {
  overflow: hidden;
  font-size: 13px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.insight-toggle strong {
  flex: 0 0 auto;
  font-size: 12px;
}

.insight-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: calc(100vh - 190px);
  margin-top: 10px;
  overflow: auto;
  padding: 12px;
  border: 1px solid #dbeafe;
  border-radius: 24px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.16);
  backdrop-filter: blur(16px);
}

.insight-card {
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 18px;
  background: #ffffff;
}

.insight-card__label {
  margin: 0 0 5px;
  color: #0f766e;
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.insight-card__title {
  margin: 0 0 12px;
  color: #0f172a;
  font-size: 17px;
}

.fact-grid {
  display: grid;
  gap: 9px;
}

.fact-card {
  padding: 11px;
  border-radius: 14px;
  background: #f8fafc;
}

.fact-card span {
  color: #0f766e;
  font-size: 11px;
  font-weight: 900;
}

.fact-card strong {
  display: block;
  margin: 4px 0;
  color: #0f172a;
}

.fact-card p,
.style-summary,
.empty-text {
  margin: 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.6;
}

.rule-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.rule-list span {
  padding: 6px 10px;
  border-radius: 999px;
  background: #eef2ff;
  color: #3730a3;
  font-size: 12px;
}

@media (max-width: 900px) {
  .insight-floating {
    top: auto;
    right: 14px;
    bottom: 18px;
    width: min(360px, calc(100vw - 28px));
  }
}
</style>
