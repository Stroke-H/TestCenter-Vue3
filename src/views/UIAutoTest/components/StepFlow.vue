<script setup lang="ts">
import draggable from 'vuedraggable'
import { Delete, Rank } from '@element-plus/icons-vue'
import { type TestStep } from '@/stores/modules/playwright'

const props = defineProps<{
  steps: TestStep[]
  selectedIndex: number
}>()

const emit = defineEmits(['update:steps', 'update:selectedIndex'])

const handleSelect = (index: number) => {
  emit('update:selectedIndex', index)
}

const handleDelete = (index: number) => {
  const newSteps = [...props.steps]
  newSteps.splice(index, 1)
  emit('update:steps', newSteps)
  if (props.selectedIndex === index) {
    emit('update:selectedIndex', -1)
  } else if (props.selectedIndex > index) {
    emit('update:selectedIndex', props.selectedIndex - 1)
  }
}

const getSummary = (step: TestStep) => {
  if (step.keyword === 'Goto') return step.args.url
  if (step.keyword === 'Click') return step.args.selector
  if (step.keyword === 'Fill') return `${step.args.selector} ← ${step.args.value}`
  if (step.keyword === 'Press') return `${step.args.selector} ⌨ ${step.args.key}`
  return step.description || step.keyword
}

</script>

<template>
  <div class="step-flow">
    <draggable 
      :list="steps" 
      item-key="id" 
      handle=".drag-handle"
      class="draggable-list"
      ghost-class="ghost-step"
      @change="emit('update:steps', steps)"
    >
      <template #item="{ element, index }">
        <div 
          class="step-item" 
          :class="{ active: index === selectedIndex, disabled: element.disabled }"
          @click="handleSelect(index)"
        >
          <div class="drag-handle">
            <el-icon><Rank /></el-icon>
          </div>
          
          <div class="step-index">{{ index + 1 }}</div>
          
          <div class="step-main">
            <div class="step-keyword">{{ element.keyword }}</div>
            <div class="step-summary">{{ getSummary(element) }}</div>
          </div>
          
          <div class="step-actions">
            <el-button 
              :icon="Delete" 
              type="danger" 
              link 
              @click.stop="handleDelete(index)" 
            />
          </div>
        </div>
      </template>
    </draggable>
  </div>
</template>

<style scoped>
.step-flow {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.draggable-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.step-item {
  background: #fff;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  display: flex;
  align-items: center;
  padding: 12px;
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
}

.step-item:hover {
  border-color: #409eff;
  background: #fdfdfd;
}

.step-item.active {
  border-color: #409eff;
  box-shadow: 0 0 0 1px #409eff, 0 4px 12px rgba(64, 158, 255, 0.1);
  background: #f5faff;
}

.step-item.disabled {
  opacity: 0.5;
  background: #f5f7fa;
}

.drag-handle {
  padding: 0 12px 0 4px;
  cursor: grab;
  color: #c0c4cc;
}

.drag-handle:active {
  cursor: grabbing;
}

.step-index {
  width: 24px;
  height: 24px;
  background: #f0f2f5;
  border-radius: 50%;
  display: flex;
  justify-content: center;
  align-items: center;
  font-size: 12px;
  font-weight: bold;
  color: #909399;
  margin-right: 15px;
}

.step-item.active .step-index {
  background: #409eff;
  color: #fff;
}

.step-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.step-keyword {
  font-size: 11px;
  font-weight: bold;
  color: #409eff;
  text-transform: uppercase;
  margin-bottom: 2px;
}

.step-summary {
  font-size: 13.5px;
  color: #303133;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.step-actions {
  opacity: 0;
  transition: opacity 0.2s;
}

.step-item:hover .step-actions {
  opacity: 1;
}

.ghost-step {
  opacity: 0.4;
  background: #f5f7fa;
  border: 1px dashed #409eff;
}
</style>
