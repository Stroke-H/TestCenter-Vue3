<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { NovelChapter, NovelOutline } from '@/api/novelWriter'

const props = defineProps<{
  outline: NovelOutline
  selectedOutlineId: string
  chapters: NovelChapter[]
  selectedChapterId: string
  running: boolean
}>()

const emit = defineEmits<{
  selectOutline: [id: string]
  selectChapter: [id: string]
  generate: []
  audit: [id: string]
  revise: [id: string]
  approve: [id: string]
}>()

const previewVersionId = shallowRef('')

const selectedOutline = computed(() => {
  return props.outline.chapters?.find((chapter) => chapter.id === props.selectedOutlineId) || props.outline.chapters?.[0]
})

const selectedChapter = computed(() => {
  const outlineId = selectedOutline.value?.id || props.selectedOutlineId
  return props.chapters.find((chapter) => chapter.outline_id === outlineId)
    || props.chapters.find((chapter) => chapter.id === props.selectedChapterId)
})

const previewContent = computed(() => {
  const chapter = selectedChapter.value
  if (!chapter) return ''
  const version = chapter.versions?.find((item) => item.id === previewVersionId.value)
  return version?.content || chapter.content
})

const chapterStatusByOutlineId = computed(() => {
  return new Map(props.chapters.map((chapter) => [chapter.outline_id, chapter]))
})

const outlineGenerationText = computed(() => {
  const generated = props.outline.generated_chapters || props.outline.chapters?.length || 0
  const target = props.outline.target_chapters || generated
  const remaining = Math.max(target - generated, 0)

  if (props.outline.generation_status === 'generating') {
    return `已生成前 ${generated} 章完整大纲，剩余 ${remaining} 章正在后台继续加载`
  }
  if (props.outline.generation_status === 'failed') {
    return `后续章节大纲生成失败：${props.outline.generation_error || '请稍后重试'}`
  }
  return ''
})

const outlineGenerationClass = computed(() => [
  'outline-progress',
  { 'outline-progress--failed': props.outline.generation_status === 'failed' }
])

const compactList = (items?: string[], limit = 3) => {
  return (items || [])
    .map((item) => String(item || '').trim())
    .filter(Boolean)
    .slice(0, limit)
}

const selectOutline = (outlineId: string) => {
  previewVersionId.value = ''
  emit('selectOutline', outlineId)
  const chapter = chapterStatusByOutlineId.value.get(outlineId)
  if (chapter) emit('selectChapter', chapter.id)
}
</script>

<template>
  <section class="chapter-panel">
    <div class="panel-head">
      <div>
        <p class="panel-kicker">Step 3</p>
        <h3 class="panel-title">分章生成 / 审计修订</h3>
      </div>
      <el-button type="primary" :loading="running" @click="emit('generate')">生成选中章节</el-button>
    </div>

    <div v-if="outlineGenerationText" :class="outlineGenerationClass">
      <span>{{ outlineGenerationText }}</span>
    </div>

    <div class="chapter-layout">
      <div class="chapter-tabs">
        <button
          v-for="chapter in outline.chapters || []"
          :key="chapter.id"
          :class="['chapter-tab', { 'chapter-tab--active': chapter.id === selectedOutline?.id }]"
          @click="selectOutline(chapter.id)"
        >
          <strong>{{ chapter.title }}</strong>
          <span v-if="chapter.summary" class="chapter-tab__summary">{{ chapter.summary }}</span>
          <span class="chapter-tab__meta">目标：{{ chapter.goal || '待补充' }}</span>
          <span class="chapter-tab__meta">冲突：{{ chapter.conflict || '待补充' }}</span>
          <span
            v-for="event in compactList(chapter.must_happen, 2)"
            :key="`${chapter.id}-must-${event}`"
            class="chapter-tab__spec"
          >
            必发：{{ event }}
          </span>
          <span
            v-for="scene in compactList(chapter.key_scenes, 2)"
            :key="`${chapter.id}-scene-${scene}`"
            class="chapter-tab__spec"
          >
            场景：{{ scene }}
          </span>
          <em class="chapter-tab__hook">钩子：{{ chapter.hook || '待补充' }}</em>
          <small>{{ chapterStatusByOutlineId.get(chapter.id)?.status || '待生成' }}</small>
        </button>
        <el-empty v-if="!outline.chapters?.length" description="先生成大纲/章节结构" />
      </div>

      <article v-if="selectedChapter" class="chapter-preview">
        <div class="chapter-preview__top">
          <div>
            <h4>{{ selectedChapter.title }}</h4>
            <span>{{ selectedChapter.summary || '暂无摘要' }}</span>
          </div>
          <div class="chapter-actions">
            <el-button size="small" :loading="running" @click="emit('audit', selectedChapter.id)">审计</el-button>
            <el-button size="small" :loading="running" @click="emit('revise', selectedChapter.id)">按审计修订</el-button>
            <el-button size="small" type="success" @click="emit('approve', selectedChapter.id)">确认章节</el-button>
          </div>
        </div>

        <div v-if="selectedChapter.versions?.length" class="version-strip">
          <button
            :class="['version-chip', { 'version-chip--active': !previewVersionId }]"
            @click="previewVersionId = ''"
          >
            当前版本
          </button>
          <button
            v-for="version in selectedChapter.versions"
            :key="version.id"
            :class="['version-chip', { 'version-chip--active': previewVersionId === version.id }]"
            @click="previewVersionId = version.id"
          >
            {{ version.type }} · {{ version.created_at }}
          </button>
        </div>

        <pre class="chapter-content">{{ previewContent }}</pre>
      </article>

      <article v-else class="chapter-preview chapter-preview--empty">
        <h4>{{ selectedOutline?.title || '等待选择章节' }}</h4>
        <div v-if="selectedOutline" class="outline-spec">
          <p v-if="selectedOutline.summary">{{ selectedOutline.summary }}</p>
          <div v-if="compactList(selectedOutline.must_happen).length" class="outline-spec__group">
            <strong>必须发生</strong>
            <span v-for="event in compactList(selectedOutline.must_happen)" :key="event">{{ event }}</span>
          </div>
          <div v-if="compactList(selectedOutline.key_scenes).length" class="outline-spec__group">
            <strong>关键场景</strong>
            <span v-for="scene in compactList(selectedOutline.key_scenes)" :key="scene">{{ scene }}</span>
          </div>
          <div v-if="compactList(selectedOutline.new_hooks).length" class="outline-spec__group">
            <strong>新增钩子</strong>
            <span v-for="hook in compactList(selectedOutline.new_hooks)" :key="hook">{{ hook }}</span>
          </div>
        </div>
        <p>当前章节还没有生成正文。确认左侧目标、冲突和钩子后，点击“生成选中章节”。</p>
      </article>
    </div>
  </section>
</template>

<style scoped>
.chapter-panel {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 24px;
  padding: 22px;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 18px;
}

.outline-progress {
  display: flex;
  align-items: center;
  min-height: 40px;
  margin: -4px 0 16px;
  padding: 10px 13px;
  border: 1px solid #99f6e4;
  border-radius: 12px;
  background: #f0fdfa;
  color: #0f766e;
  font-size: 13px;
  font-weight: 800;
  line-height: 1.5;
}

.outline-progress--failed {
  border-color: #fecaca;
  background: #fef2f2;
  color: #b91c1c;
}

.panel-kicker {
  margin: 0 0 4px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 800;
}

.panel-title {
  margin: 0;
  color: #0f172a;
  font-size: 20px;
}

.chapter-layout {
  display: grid;
  grid-template-columns: 330px 1fr;
  gap: 18px;
  align-items: stretch;
}

.chapter-tabs {
  display: flex;
  max-height: 660px;
  flex-direction: column;
  gap: 10px;
  overflow-y: auto;
  padding-right: 4px;
}

.chapter-tab {
  padding: 14px;
  border: 1px solid #e2e8f0;
  border-radius: 15px;
  background: #f8fafc;
  text-align: left;
  cursor: pointer;
}

.chapter-tab--active {
  border-color: #14b8a6;
  background: #ecfeff;
}

.chapter-tab strong,
.chapter-tab span,
.chapter-tab em,
.chapter-tab small {
  display: block;
}

.chapter-tab strong {
  margin-bottom: 8px;
  color: #0f172a;
}

.chapter-tab__meta,
.chapter-tab__hook,
.chapter-tab__summary,
.chapter-tab__spec {
  color: #64748b;
  font-size: 12px;
  font-style: normal;
  line-height: 1.55;
}

.chapter-tab__summary {
  margin-bottom: 6px;
  color: #475569;
}

.chapter-tab__spec {
  margin-top: 4px;
  color: #0f766e;
}

.chapter-tab small {
  width: fit-content;
  margin-top: 9px;
  padding: 4px 8px;
  border-radius: 999px;
  background: #ffffff;
  color: #0f766e;
  font-size: 11px;
  font-weight: 800;
}

.chapter-preview {
  min-width: 0;
}

.chapter-preview--empty {
  display: flex;
  min-height: 260px;
  flex-direction: column;
  justify-content: center;
  padding: 26px;
  border: 1px dashed #cbd5e1;
  border-radius: 18px;
  background: #f8fafc;
}

.chapter-preview--empty h4 {
  margin: 0 0 10px;
  color: #0f172a;
  font-size: 18px;
}

.chapter-preview--empty p {
  margin: 0;
  color: #64748b;
  line-height: 1.7;
}

.outline-spec {
  display: grid;
  gap: 12px;
  margin: 0 0 16px;
}

.outline-spec__group {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.outline-spec__group strong,
.outline-spec__group span {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 5px 9px;
  border-radius: 999px;
  font-size: 12px;
}

.outline-spec__group strong {
  background: #ecfeff;
  color: #0f766e;
}

.outline-spec__group span {
  background: #f1f5f9;
  color: #475569;
}

.chapter-preview__top {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid #e2e8f0;
}

.chapter-preview__top h4 {
  margin: 0 0 6px;
  color: #0f172a;
  font-size: 18px;
}

.chapter-preview__top span {
  color: #64748b;
}

.chapter-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.version-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 14px 0;
}

.version-chip {
  padding: 6px 10px;
  border: 1px solid #cbd5e1;
  border-radius: 999px;
  background: #ffffff;
  color: #475569;
  cursor: pointer;
}

.version-chip--active {
  border-color: #14b8a6;
  background: #ccfbf1;
  color: #0f766e;
}

.chapter-content {
  max-height: 660px;
  overflow: auto;
  padding: 20px;
  border-radius: 18px;
  background: #0f172a;
  color: #e2e8f0;
  white-space: pre-wrap;
  line-height: 1.9;
  font-family: "Songti SC", "Noto Serif CJK SC", serif;
  font-size: 15px;
}

@media (max-width: 1000px) {
  .chapter-layout {
    grid-template-columns: 1fr;
  }

  .panel-head,
  .chapter-preview__top {
    flex-direction: column;
  }
}
</style>
