<script setup lang="ts">
interface MonkeyEvidence {
  source: string
  message: string
  timestamp?: string
}

defineProps<{
  summary?: string
  evidence?: MonkeyEvidence[]
}>()
</script>

<template>
  <span v-if="summary" class="monkey-node-summary">
    <span>摘要：{{ summary }}</span>
    <el-tooltip
      v-if="evidence?.length"
      placement="top"
      effect="dark"
      :show-after="180"
    >
      <template #content>
        <div class="monkey-node-summary__details">
          <div v-for="(item, index) in evidence" :key="`${item.source}-${item.message}-${index}`">
            <strong>{{ item.source }}</strong>
            <span v-if="item.timestamp"> · {{ item.timestamp }}</span>
            <div>{{ item.message }}</div>
          </div>
        </div>
      </template>
      <button type="button" class="monkey-node-summary__detail">Detail</button>
    </el-tooltip>
  </span>
</template>

<style scoped>
.monkey-node-summary {
  display: inline;
  line-height: 1.65;
}

.monkey-node-summary__detail {
  margin-left: 8px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #22d3ee;
  cursor: help;
  font: inherit;
  font-weight: 700;
}

.monkey-node-summary__details {
  display: grid;
  gap: 10px;
  max-width: min(620px, 72vw);
  max-height: 320px;
  overflow-y: auto;
  line-height: 1.55;
  overflow-wrap: anywhere;
  white-space: normal;
}
</style>
