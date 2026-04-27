<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { Plus, Delete, Check, Timer, Tools } from '@element-plus/icons-vue'
import type { MindNode, NodeStatus } from './types'

const props = defineProps<{
  node: MindNode
  parent?: MindNode
  activeId: string | null
  activeOpsId: string | null
  editingId: string | null
  isVertical: boolean
}>()

const emit = defineEmits<{
  (e: 'add', node: MindNode): void
  (e: 'delete', parent: MindNode, id: string): void
  (e: 'update-status', node: MindNode, status: NodeStatus): void
  (e: 'toggle-status', node: MindNode): void
  (e: 'mouseenter', node: MindNode, event: MouseEvent): void
  (e: 'mouseleave'): void
  (e: 'show-menu', node: MindNode, event: MouseEvent): void
  (e: 'show-ops', node: MindNode): void
  (e: 'start-edit', node: MindNode): void
  (e: 'stop-edit'): void
  (e: 'drag-start', id: string): void
  (e: 'drop', id: string): void
}>()

const inputRef = ref<HTMLInputElement | null>(null)

watch(() => props.editingId, (newId) => {
  if (newId === props.node.id) {
    nextTick(() => {
      inputRef.value?.focus()
      inputRef.value?.select()
    })
  }
})

// === 拖拽处理 ===
const isDragOver = ref(false)

const handleDragStart = (e: DragEvent) => {
  if (props.editingId === props.node.id) {
    e.preventDefault()
    return
  }
  emit('drag-start', props.node.id)
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', props.node.id)
  }
}

const handleDragOver = (e: DragEvent) => {
  e.preventDefault()
  isDragOver.value = true
}

const handleDragLeave = () => {
  isDragOver.value = false
}

const handleDrop = (e: DragEvent) => {
  e.preventDefault()
  isDragOver.value = false
  emit('drop', props.node.id)
}

const handleLabelClick = (node: MindNode) => {
  emit('show-ops', node)
}

const handleLabelDblClick = (node: MindNode) => {
  emit('start-edit', node)
}

const handleShowMenu = (event: MouseEvent) => {
  emit('show-menu', props.node, event)
}

const getStatusIcon = (status: NodeStatus) => {
  switch (status) {
    case 'completed': return Check
    case 'in_progress': return Timer
    case 'fixing': return Tools
    default: return null
  }
}

const getStatusClass = (status: NodeStatus) => {
  return `status-${status}`
}
</script>

<template>
  <div 
    class="tree-branch" 
    :class="[
      isVertical ? 'branch-v' : 'branch-h',
      { 'is-root-branch': node.isRoot }
    ]"
  >
    <div 
      class="node-container"
      :class="[
        isVertical ? 'container-v' : 'container-h',
        { 'is-active-layer': activeId === node.id || activeOpsId === node.id || editingId === node.id }
      ]"
    >
      <!-- 节点卡片 -->
      <div 
        class="node-card" 
        :class="[
          getStatusClass(node.status), 
          { 
            'is-root': node.isRoot, 
            'is-active-ops': activeOpsId === node.id, 
            'is-editing': editingId === node.id,
            'is-drag-over': isDragOver
          }
        ]"
        :draggable="!node.isRoot && editingId !== node.id"
        @dragstart="handleDragStart"
        @dragover="handleDragOver"
        @dragleave="handleDragLeave"
        @drop="handleDrop"
        @click.stop="emit('show-ops', node)"
        @contextmenu.prevent.stop="handleShowMenu"
        @mouseenter="emit('mouseenter', node, $event)"
        @mouseleave="emit('mouseleave')"
      >
        <!-- 状态标识 -->
        <div v-if="node.status !== 'none'" class="node-status-badge" :class="'bg-' + node.status">
          <el-icon><component :is="getStatusIcon(node.status)" /></el-icon>
        </div>

        <div class="node-inner" @click.stop="handleLabelClick(node)" @dblclick.stop="handleLabelDblClick(node)">
          <input 
            v-if="editingId === node.id"
            ref="inputRef"
            v-model="node.label" 
            class="node-input"
            @click.stop
            @blur="emit('stop-edit')"
            @keyup.enter="emit('stop-edit')"
            placeholder="命名流程..."
          />
          <span v-else class="node-text">{{ node.label || '命名流程...' }}</span>
        </div>

        <!-- 操作按钮 -->
        <div class="node-ops" @click.stop>
          <button class="op-btn add" @click="emit('add', node)" title="添加子节点">
            <el-icon><Plus /></el-icon>
          </button>
          <button v-if="!node.isRoot && parent" class="op-btn del" @click="emit('delete', parent, node.id)" title="删除节点">
            <el-icon><Delete /></el-icon>
          </button>
        </div>

      </div>

      <!-- 子节点容器 -->
      <div v-if="node.children && node.children.length > 0" class="children-container" :class="isVertical ? 'children-v' : 'children-h'">
        <MindMapNode 
          v-for="child in node.children" 
          :key="child.id" 
          :node="child" 
          :parent="node"
          :is-vertical="isVertical"
          :active-id="activeId"
          :active-ops-id="activeOpsId"
          :editing-id="editingId"
          @add="n => emit('add', n)"
          @delete="(p, id) => emit('delete', p, id)"
          @update-status="(n, s) => emit('update-status', n, s)"
          @toggle-status="n => emit('toggle-status', n)"
          @mouseenter="(n, event) => emit('mouseenter', n, event)"
          @mouseleave="emit('mouseleave')"
          @show-menu="(n, event) => emit('show-menu', n, event)"
          @show-ops="n => emit('show-ops', n)"
          @start-edit="n => emit('start-edit', n)"
          @stop-edit="emit('stop-edit')"
          @drag-start="id => emit('drag-start', id)"
          @drop="id => emit('drop', id)"
        />
      </div>
    </div>
  </div>
</template>

<script lang="ts">
export default {
  name: 'MindMapNode'
}
</script>

<style>
/* ==================== 基础布局 ==================== */
.mindmap-canvas .tree-branch {
  display: inline-flex;
  vertical-align: top;
  position: relative;
}

/* 垂直布局 (Top to Bottom) */
.mindmap-canvas .branch-v {
  flex-direction: column;
  align-items: center;
}

/* 水平布局 (Left to Right) */
.mindmap-canvas .branch-h {
  flex-direction: row;
  align-items: center;
}

.mindmap-canvas .node-container {
  display: flex;
  position: relative;
  z-index: 1;
}

.mindmap-canvas .container-v {
  flex-direction: column;
  align-items: center;
}

.mindmap-canvas .container-h {
  flex-direction: row;
  align-items: center;
}

.mindmap-canvas .node-container.is-active-layer {
  z-index: 100;
}

/* ==================== 连线逻辑 (垂直) ==================== */
.mindmap-canvas .children-v {
  display: flex;
  flex-direction: row;
  margin-top: 40px;
  position: relative;
  gap: 30px;
}

.mindmap-canvas .children-v::before {
  content: '';
  position: absolute;
  top: -40px;
  left: 50%;
  width: 2px;
  height: 20px;
  background: #cbd5e1;
  transform: translateX(-50%);
}

.mindmap-canvas .branch-v:not(:only-child)::before {
  content: '';
  position: absolute;
  top: -20px;
  left: 0;
  width: 100%;
  height: 2px;
  background: #cbd5e1;
}

.mindmap-canvas .branch-v:first-child:not(:only-child)::before { left: 50%; width: 50%; }
.mindmap-canvas .branch-v:last-child:not(:only-child)::before { width: 50%; }

.mindmap-canvas .branch-v::after {
  content: '';
  position: absolute;
  top: -20px;
  left: 50%;
  width: 2px;
  height: 20px;
  background: #cbd5e1;
  transform: translateX(-50%);
}

.mindmap-canvas .branch-v.is-root-branch::after { display: none !important; }

/* ==================== 连线逻辑 (水平) ==================== */
.mindmap-canvas .children-h {
  display: flex;
  flex-direction: column;
  margin-left: 40px;
  position: relative;
  gap: 20px;
}

.mindmap-canvas .children-h::before {
  content: '';
  position: absolute;
  top: 50%;
  left: -40px;
  width: 20px;
  height: 2px;
  background: #cbd5e1;
  transform: translateY(-50%);
}

.mindmap-canvas .branch-h:not(:only-child)::before {
  content: '';
  position: absolute;
  left: -20px;
  top: 0;
  width: 2px;
  height: 100%;
  background: #cbd5e1;
}

.mindmap-canvas .branch-h:first-child:not(:only-child)::before { top: 50%; height: 50%; }
.mindmap-canvas .branch-h:last-child:not(:only-child)::before { height: 50%; }

.mindmap-canvas .branch-h::after {
  content: '';
  position: absolute;
  left: -20px;
  top: 50%;
  width: 20px;
  height: 2px;
  background: #cbd5e1;
  transform: translateY(-50%);
}

.mindmap-canvas .branch-h.is-root-branch::after { display: none !important; }

/* ==================== 节点卡片强化 ==================== */
.mindmap-canvas .node-card {
  position: relative;
  min-width: 160px;
  max-width: 240px;
  background: white;
  border: 2px solid #e2e8f0;
  border-radius: 12px;
  padding: 12px;
  box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05);
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: grab;
  z-index: 10;
}

.mindmap-canvas .node-card:active { cursor: grabbing; }

.mindmap-canvas .node-card.is-drag-over {
  border-color: #3b82f6;
  background: #eff6ff;
  transform: scale(1.05);
  box-shadow: 0 0 15px rgba(59, 130, 246, 0.3);
}

.mindmap-canvas .node-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 10px 15px -3px rgba(0,0,0,0.1);
  border-color: #94a3b8;
}

.mindmap-canvas .node-card.is-root {
  background: #0f172a;
  border-color: #0f172a;
  cursor: default;
}

.mindmap-canvas .node-card.is-root .node-text { color: white; }

/* 状态样式 */
.mindmap-canvas .status-completed { border-color: #10b981 !important; background-color: #f0fdf4; }
.mindmap-canvas .status-in_progress { border-color: #3b82f6 !important; border-style: dashed !important; }
.mindmap-canvas .status-fixing { border-color: #f59e0b !important; background-color: #fffbeb; }

.mindmap-canvas .bg-completed { background: #10b981; }
.mindmap-canvas .bg-in_progress { background: #3b82f6; }
.mindmap-canvas .bg-fixing { background: #f59e0b; }

.mindmap-canvas .node-status-badge {
  position: absolute;
  top: -12px;
  right: -12px;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 14px;
  box-shadow: 0 2px 8px rgba(0,0,0,0.2);
  z-index: 20;
}

.mindmap-canvas .node-inner {
  width: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 24px;
}

.mindmap-canvas .node-text {
  font-weight: 700;
  color: #1e293b;
  font-size: 14px;
  user-select: none;
}

.mindmap-canvas .node-input {
  width: 100%;
  border: none;
  background: transparent;
  text-align: center;
  font-weight: 700;
  color: #1e293b;
  font-size: 14px;
  outline: none;
}

/* 操作按钮 */
.mindmap-canvas .node-ops {
  position: absolute;
  bottom: -32px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 8px;
  opacity: 0;
  transition: all 0.2s;
  pointer-events: none;
}

.mindmap-canvas .node-card.is-active-ops .node-ops {
  opacity: 1;
  bottom: -40px;
  pointer-events: auto;
}

.mindmap-canvas .op-btn {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  border: 1px solid #e2e8f0;
  background: white;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 2px 4px rgba(0,0,0,0.1);
}

.mindmap-canvas .op-btn:hover { transform: scale(1.1); }
.mindmap-canvas .op-btn.add:hover { color: #3b82f6; border-color: #3b82f6; }
.mindmap-canvas .op-btn.del:hover { color: #ef4444; border-color: #ef4444; }

/* 悬浮菜单 */
.mindmap-canvas .node-menu {
  position: absolute;
  background: white;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  box-shadow: 0 10px 25px rgba(0,0,0,0.1);
  padding: 6px;
  min-width: 120px;
  z-index: 100;
}

.mindmap-canvas .menu-v { top: 0; left: 105%; }
.mindmap-canvas .menu-h { bottom: 105%; left: 50%; transform: translateX(-50%); }

.mindmap-canvas .menu-header {
  padding: 6px 12px;
  font-size: 11px;
  font-weight: 800;
  color: #94a3b8;
  text-transform: uppercase;
  border-bottom: 1px solid #f1f5f9;
}

.mindmap-canvas .menu-item {
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 600;
  color: #475569;
  border-radius: 6px;
  cursor: pointer;
}

.mindmap-canvas .menu-item:hover { background: #f1f5f9; color: #0f172a; }

.mindmap-canvas .fade-enter-active, .mindmap-canvas .fade-leave-active { transition: opacity 0.2s, transform 0.2s; }
.mindmap-canvas .fade-enter-from, .mindmap-canvas .fade-leave-to { opacity: 0; transform: scale(0.95); }
</style>
