<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, shallowRef, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Minus, Plus } from '@element-plus/icons-vue'
import type { NovelMaterials } from '@/api/novelWriter'

const props = defineProps<{
  modelValue: NovelMaterials
  saving: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: NovelMaterials]
  save: []
  extract: []
  next: []
}>()

interface CanvasNode {
  id: string
  type: 'world' | 'character' | 'conflict' | 'idea'
  title: string
  value: string
  x: number
  y: number
  width: number
  height: number
}

interface CanvasEdge {
  id: string
  from: CanvasNode
  to: CanvasNode
  dashed?: boolean
}

interface CharacterRoleOption {
  label: string
  value: string
  template: string
}

const form = reactive<NovelMaterials>({ ...props.modelValue })
const canvasRef = shallowRef<HTMLElement | null>(null)
const zoomLevel = shallowRef(0.92)
const translateX = shallowRef(80)
const translateY = shallowRef(42)
const isPanning = shallowRef(false)
const startPos = reactive({ x: 0, y: 0, tx: 0, ty: 0 })
const draggingIdea = shallowRef('')
const draggingIdeaIndex = shallowRef(-1)
const dropActive = shallowRef(false)
const activeDropCharacterId = shallowRef('')
const conflictDialogVisible = shallowRef(false)
const selectedConflictCharacterIds = shallowRef<string[]>([])

let panFrame = 0
let pendingPan: PointerEvent | null = null

const characterRoleOptions: CharacterRoleOption[] = [
  { label: '男主角', value: '男主角', template: '名称：\n性格：\n身份：\n欲望：\n弱点：' },
  { label: '女主角', value: '女主角', template: '名称：\n性格：\n身份：\n欲望：\n弱点：' },
  { label: '主要男配角', value: '主要男配角', template: '名称：\n性格：\n身份：\n欲望：\n弱点：' },
  { label: '主要女配角', value: '主要女配角', template: '名称：\n性格：\n身份：\n欲望：\n弱点：' },
  { label: '次要男配角', value: '次要男配角', template: '名称：\n性格：\n身份：\n欲望：\n弱点：' },
  { label: '次要女配角', value: '次要女配角', template: '名称：\n性格：\n身份：\n欲望：\n弱点：' },
  { label: '重要NPC', value: '重要NPC', template: '名称：\n性格：\n身份：\n功能：\n与主线关系：' },
  { label: '普通NPC', value: '普通NPC', template: '名称：\n性格：\n身份：\n出场作用：' }
]

const splitBlocks = (text: string) => {
  return String(text || '')
    .split(/\n\s*\n|\n-/)
    .map((item) => item.replace(/^-/, '').trim())
    .filter(Boolean)
}

const splitLines = (text: string) => {
  return String(text || '')
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean)
}

const appendWithBlankLine = (current: string, next: string) => {
  const trimmed = String(current || '').trim()
  return trimmed ? `${trimmed}\n\n${next}` : next
}

const appendLine = (current: string, next: string) => {
  const trimmed = String(current || '').trim()
  return trimmed ? `${trimmed}\n${next}` : next
}

const readLabeledLine = (text: string, label: string) => {
  const matched = String(text || '').match(new RegExp(`^${label}[:：]\\s*(.+)$`, 'm'))
  return matched?.[1]?.trim() || ''
}

const getCharacterRole = (value: string) => {
  const role = readLabeledLine(value, '角色类型')
  return role || '人物'
}

const getCharacterName = (value: string, fallback: string) => {
  const name = readLabeledLine(value, '名称')
  if (name) return name
  const firstLine = String(value || '').split('\n').map((item) => item.trim()).find(Boolean) || ''
  return firstLine.replace(/^(?:新人物|人物|角色|男主角|女主角|主要男配角|主要女配角|次要男配角|次要女配角|重要NPC|普通NPC)\s*[:：]?/, '').trim() || fallback
}

const getCharacterNodeLabel = (value: string, fallback: string) => {
  const role = getCharacterRole(value)
  const name = getCharacterName(value, fallback)
  return role === '人物' ? name : `${role}：${name}`
}

const normalizeName = (value: string) => String(value || '').replace(/[，,；;。.\s:：]/g, '').toLowerCase()

const replaceOrPrependLabeledLine = (text: string, label: string, nextValue: string) => {
  const normalized = String(text || '')
  const line = `${label}：${nextValue}`
  const pattern = new RegExp(`^${label}[:：].*$`, 'm')
  if (pattern.test(normalized)) return normalized.replace(pattern, line)
  return normalized.trim() ? `${line}\n${normalized}` : line
}

const worldNode = computed<CanvasNode>(() => ({
  id: 'world',
  type: 'world',
  title: '世界观基础',
  value: form.world_raw,
  x: 520,
  y: 40,
  width: 420,
  height: 210
}))

const characterNodes = computed<CanvasNode[]>(() => {
  const cards = splitBlocks(form.character_raw)
  return cards.map((value, index) => ({
    id: `character-${index}`,
    type: 'character',
    title: getCharacterNodeLabel(value, `人物 ${index + 1}`),
    value,
    x: 160 + (index % 3) * 360,
    y: 360 + Math.floor(index / 3) * 260,
    width: 300,
    height: 210
  }))
})

const conflictNodes = computed<CanvasNode[]>(() => {
  const baseY = 680 + Math.ceil(Math.max(characterNodes.value.length, 1) / 3) * 180
  return splitLines(form.conflict_raw).map((value, index) => ({
    id: `conflict-${index}`,
    type: 'conflict',
    title: `冲突 ${index + 1}`,
    value,
    x: 240 + (index % 2) * 520,
    y: baseY + Math.floor(index / 2) * 230,
    width: 420,
    height: 170
  }))
})

const ideaNodes = computed<CanvasNode[]>(() => {
  return splitLines(form.raw_text).map((value, index) => ({
    id: `idea-${index}`,
    type: 'idea',
    title: `灵感 ${index + 1}`,
    value,
    x: 1180,
    y: 320 + index * 170,
    width: 280,
    height: 130
  }))
})

const canvasNodes = computed(() => [
  worldNode.value,
  ...characterNodes.value,
  ...conflictNodes.value,
  ...ideaNodes.value
])

const canvasEdges = computed<CanvasEdge[]>(() => {
  const edges: CanvasEdge[] = []
  characterNodes.value.forEach((node) => {
    edges.push({ id: `world-${node.id}`, from: worldNode.value, to: node })
  })
  conflictNodes.value.forEach((node, index) => {
    const linkedCharacters = characterNodes.value.filter((character) => {
      const characterName = normalizeName(getCharacterName(character.value, character.title))
      return characterName && normalizeName(node.value).includes(characterName)
    })
    const participants = linkedCharacters.length > 0
      ? linkedCharacters
      : characterNodes.value.slice(index % Math.max(characterNodes.value.length, 1), index % Math.max(characterNodes.value.length, 1) + 1)
    participants.forEach((character) => {
      edges.push({ id: `${character.id}-${node.id}`, from: character, to: node })
    })
  })
  ideaNodes.value.forEach((node) => {
    edges.push({ id: `idea-${node.id}`, from: node, to: worldNode.value, dashed: true })
  })
  return edges
})

const boardSize = computed(() => {
  const maxX = Math.max(...canvasNodes.value.map((node) => node.x + node.width), 1500)
  const maxY = Math.max(...canvasNodes.value.map((node) => node.y + node.height), 980)
  return {
    width: maxX + 180,
    height: maxY + 160
  }
})

const zoomStyle = computed(() => ({
  width: `${boardSize.value.width}px`,
  height: `${boardSize.value.height}px`,
  transform: `translate(${translateX.value}px, ${translateY.value}px) scale(${zoomLevel.value})`,
  transformOrigin: '0 0',
  transition: isPanning.value ? 'none' : 'transform 0.16s cubic-bezier(0.4, 0, 0.2, 1)'
}))

watch(
  () => props.modelValue,
  (value) => Object.assign(form, value),
  { deep: true }
)

watch(
  form,
  () => emit('update:modelValue', { ...form }),
  { deep: true }
)

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max)

const zoomAtPoint = (nextZoom: number, clientX?: number, clientY?: number) => {
  const canvas = canvasRef.value
  if (!canvas) return

  const rect = canvas.getBoundingClientRect()
  const oldZoom = zoomLevel.value
  const zoom = clamp(Number(nextZoom.toFixed(2)), 0.45, 1.8)
  const focusX = clientX ?? rect.left + rect.width / 2
  const focusY = clientY ?? rect.top + rect.height / 2
  const contentX = (focusX - rect.left - translateX.value) / oldZoom
  const contentY = (focusY - rect.top - translateY.value) / oldZoom

  zoomLevel.value = zoom
  translateX.value = focusX - rect.left - contentX * zoom
  translateY.value = focusY - rect.top - contentY * zoom
}

const centerCanvas = () => {
  translateX.value = 80
  translateY.value = 42
  zoomLevel.value = 0.92
}

const handleWheel = (event: WheelEvent) => {
  if (!event.ctrlKey && !event.metaKey) return
  event.preventDefault()
  zoomAtPoint(zoomLevel.value + (event.deltaY < 0 ? 0.08 : -0.08), event.clientX, event.clientY)
}

const isPanTarget = (target: HTMLElement) => {
  return !target.closest('.canvas-node')
    && !target.closest('.canvas-toolbar')
    && !target.closest('.material-map-page__actions')
}

const handlePointerDown = (event: PointerEvent) => {
  const target = event.target as HTMLElement
  if (event.button !== 0 || !isPanTarget(target)) return
  isPanning.value = true
  startPos.x = event.clientX
  startPos.y = event.clientY
  startPos.tx = translateX.value
  startPos.ty = translateY.value
  canvasRef.value?.setPointerCapture(event.pointerId)
}

const handlePointerMove = (event: PointerEvent) => {
  if (!isPanning.value) return
  event.preventDefault()
  pendingPan = event
  if (panFrame) return
  panFrame = window.requestAnimationFrame(() => {
    if (!pendingPan) return
    translateX.value = startPos.tx + pendingPan.clientX - startPos.x
    translateY.value = startPos.ty + pendingPan.clientY - startPos.y
    panFrame = 0
    pendingPan = null
  })
}

const handlePointerUp = (event?: PointerEvent) => {
  isPanning.value = false
  pendingPan = null
  if (panFrame) {
    window.cancelAnimationFrame(panFrame)
    panFrame = 0
  }
  if (event && canvasRef.value?.hasPointerCapture(event.pointerId)) {
    canvasRef.value.releasePointerCapture(event.pointerId)
  }
}

const addCharacter = (role: string) => {
  const option = characterRoleOptions.find((item) => item.value === role) || characterRoleOptions[0]!
  form.character_raw = appendWithBlankLine(form.character_raw, `角色类型：${option.value}\n${option.template}`)
}

const addConflict = () => {
  if (characterNodes.value.length === 0) {
    ElMessage.warning('请先添加人物，再创建冲突')
    return
  }
  selectedConflictCharacterIds.value = []
  conflictDialogVisible.value = true
}

const createConflict = () => {
  const characters = selectedConflictCharacterIds.value
    .map((id) => characterNodes.value.find((node) => node.id === id))
    .filter((node): node is CanvasNode => Boolean(node))
  if (characters.length === 0) {
    ElMessage.warning('请至少选择一个冲突人物')
    return
  }

  if (characters.length === 1) {
    const character = characters[0]
    if (!character) return
    const characterName = getCharacterName(character.value, character.title)
    form.conflict_raw = appendLine(form.conflict_raw, `${characterName}：自身欲望、弱点或处境引发的内部冲突`)
  } else {
    const characterNames = characters.map((node) => getCharacterName(node.value, node.title))
    form.conflict_raw = appendLine(form.conflict_raw, `${characterNames.join(' ↔ ')}：围绕目标、秘密或利益产生直接冲突`)
  }
  conflictDialogVisible.value = false
  selectedConflictCharacterIds.value = []
}

const addIdea = () => {
  form.raw_text = appendLine(form.raw_text, '新的阶段性灵感：')
}

const updateCharacter = (index: number, value: string) => {
  const cards = [...characterNodes.value.map((node) => node.value)]
  cards[index] = value
  form.character_raw = cards.filter((item) => item.trim()).join('\n\n')
}

const updateCharacterRole = (node: CanvasNode, role: string) => {
  const index = Number(node.id.split('-')[1] || 0)
  updateCharacter(index, replaceOrPrependLabeledLine(node.value, '角色类型', role))
}

const updateConflict = (index: number, value: string) => {
  const cards = splitLines(form.conflict_raw)
  cards[index] = value
  form.conflict_raw = cards.filter((item) => item.trim()).join('\n')
}

const updateIdea = (index: number, value: string) => {
  const cards = [...ideaNodes.value.map((node) => node.value)]
  cards[index] = value
  form.raw_text = cards.filter((item) => item.trim()).join('\n')
}

const updateNodeValue = (node: CanvasNode, value: string) => {
  if (node.type === 'world') form.world_raw = value
  if (node.type === 'character') updateCharacter(Number(node.id.split('-')[1] || 0), value)
  if (node.type === 'conflict') updateConflict(Number(node.id.split('-')[1] || 0), value)
  if (node.type === 'idea') updateIdea(Number(node.id.split('-')[1] || 0), value)
}

const deleteNode = (node: CanvasNode) => {
  const index = Number(node.id.split('-')[1] || 0)
  if (node.type === 'character') {
    form.character_raw = characterNodes.value
      .filter((_, itemIndex) => itemIndex !== index)
      .map((item) => item.value)
      .join('\n\n')
  }
  if (node.type === 'conflict') {
    form.conflict_raw = splitLines(form.conflict_raw)
      .filter((_, itemIndex) => itemIndex !== index)
      .join('\n')
  }
  if (node.type === 'idea') {
    form.raw_text = splitLines(form.raw_text)
      .filter((_, itemIndex) => itemIndex !== index)
      .join('\n')
  }
}

const handleIdeaDragStart = (node: CanvasNode) => {
  draggingIdea.value = node.value
  draggingIdeaIndex.value = Number(node.id.split('-')[1] || -1)
}

const resetIdeaDrag = () => {
  draggingIdea.value = ''
  draggingIdeaIndex.value = -1
  dropActive.value = false
  activeDropCharacterId.value = ''
}

const handleNodeDragOver = (node: CanvasNode) => {
  if (!draggingIdea.value || node.type !== 'character') return
  dropActive.value = true
  activeDropCharacterId.value = node.id
}

const handleMainDrop = () => {
  resetIdeaDrag()
}

const handleCharacterDrop = (node: CanvasNode) => {
  if (!draggingIdea.value || node.type !== 'character') {
    resetIdeaDrag()
    return
  }
  const characterName = getCharacterName(node.value, node.title)
  form.conflict_raw = appendLine(form.conflict_raw, `${characterName} ← 灵感转入：${draggingIdea.value}`)
  if (draggingIdeaIndex.value >= 0) {
    form.raw_text = splitLines(form.raw_text)
      .filter((_, index) => index !== draggingIdeaIndex.value)
      .join('\n')
  }
  resetIdeaDrag()
}

const edgePath = (edge: CanvasEdge) => {
  const startX = edge.from.x + edge.from.width / 2
  const startY = edge.from.y + edge.from.height / 2
  const endX = edge.to.x + edge.to.width / 2
  const endY = edge.to.y + edge.to.height / 2
  const midY = startY + (endY - startY) * 0.55
  return `M ${startX} ${startY} C ${startX} ${midY}, ${endX} ${midY}, ${endX} ${endY}`
}

const nodeStyle = (node: CanvasNode) => ({
  left: `${node.x}px`,
  top: `${node.y}px`,
  width: `${node.width}px`,
  minHeight: `${node.height}px`
})

onMounted(() => {
  nextTick(centerCanvas)
})
</script>

<template>
  <section class="material-map-page">
    <div class="material-map-page__header">
      <div>
        <p class="material-map-page__kicker">Mind Canvas</p>
        <h2 class="material-map-page__title">素材图谱</h2>
      </div>
      <div class="material-map-page__switch">
        <slot name="workspace-switch" />
      </div>
      <div class="material-map-page__actions">
        <el-button :loading="saving" @click="emit('save')">保存素材</el-button>
        <el-button type="primary" @click="emit('extract')">信息提取</el-button>
        <el-button @click="emit('next')">生成大纲</el-button>
      </div>
    </div>

    <div
      ref="canvasRef"
      :class="['mindmap-canvas', { 'is-panning': isPanning, 'is-drop-active': dropActive }]"
      @wheel="handleWheel"
      @pointerdown="handlePointerDown"
      @pointermove="handlePointerMove"
      @pointerup="handlePointerUp"
      @pointercancel="handlePointerUp"
      @pointerleave="handlePointerUp"
      @drop.prevent="handleMainDrop"
    >
      <div class="canvas-toolbar">
        <el-button size="small" :icon="Minus" @click="zoomAtPoint(zoomLevel - 0.1)" />
        <el-button size="small" @click="centerCanvas">{{ Math.round(zoomLevel * 100) }}%</el-button>
        <el-button size="small" :icon="Plus" @click="zoomAtPoint(zoomLevel + 0.1)" />
        <el-dropdown trigger="click" @command="(role: string | number | object) => addCharacter(String(role))">
          <el-button size="small">添加人物</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item
                v-for="role in characterRoleOptions"
                :key="role.value"
                :command="role.value"
              >
                {{ role.label }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
        <el-button size="small" @click="addConflict">添加冲突</el-button>
        <el-button size="small" @click="addIdea">添加灵感</el-button>
      </div>

      <div class="mindmap-scroller" :style="zoomStyle">
        <svg class="canvas-edges" :width="boardSize.width" :height="boardSize.height" aria-hidden="true">
          <path
            v-for="edge in canvasEdges"
            :key="edge.id"
            :d="edgePath(edge)"
            :class="{ 'is-dashed': edge.dashed }"
          />
        </svg>

        <article
          v-for="node in canvasNodes"
          :key="node.id"
          :class="[
            'canvas-node',
            `canvas-node--${node.type}`,
            { 'canvas-node--drop-target': node.id === activeDropCharacterId }
          ]"
          :style="nodeStyle(node)"
          :draggable="node.type === 'idea'"
          @dragstart="handleIdeaDragStart(node)"
          @dragend="resetIdeaDrag"
          @dragover.prevent="handleNodeDragOver(node)"
          @dragleave="activeDropCharacterId = activeDropCharacterId === node.id ? '' : activeDropCharacterId"
          @drop.stop.prevent="handleCharacterDrop(node)"
        >
          <div class="canvas-node__header">
            <div>
              <el-dropdown
                v-if="node.type === 'character'"
                trigger="click"
                @command="(role: string | number | object) => updateCharacterRole(node, String(role))"
              >
                <button class="role-chip" type="button">{{ getCharacterRole(node.value) }}</button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item
                      v-for="role in characterRoleOptions"
                      :key="role.value"
                      :command="role.value"
                    >
                      {{ role.label }}
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
              <span v-else>{{ node.type }}</span>
              <strong>{{ node.title }}</strong>
            </div>
            <el-button
              v-if="node.type !== 'world'"
              size="small"
              text
              type="danger"
              @click.stop="deleteNode(node)"
            >
              删除
            </el-button>
          </div>
          <el-input
            :model-value="node.value"
            type="textarea"
            :rows="node.type === 'world' ? 6 : node.type === 'conflict' ? 4 : 5"
            resize="none"
            :placeholder="node.type === 'world' ? '写入世界规则、时代、势力、限制条件...' : '直接在卡片内编辑内容'"
            @update:model-value="updateNodeValue(node, String($event))"
          />
        </article>
      </div>
    </div>

    <el-dialog v-model="conflictDialogVisible" title="选择冲突人物" width="520px">
      <div class="conflict-dialog">
        <el-checkbox-group v-model="selectedConflictCharacterIds" class="conflict-character-list">
          <el-checkbox
            v-for="character in characterNodes"
            :key="character.id"
            :label="character.id"
            border
          >
            {{ character.title }}
          </el-checkbox>
        </el-checkbox-group>
      </div>
      <template #footer>
        <el-button @click="conflictDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createConflict">生成冲突卡片</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<style scoped>
.material-map-page {
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - 150px);
}

.material-map-page__header {
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 14px;
}

.material-map-page__kicker {
  margin: 0 0 6px;
  color: #0f766e;
  font-size: 12px;
  font-weight: 900;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.material-map-page__title {
  margin: 0;
  color: #0f172a;
  font-size: 22px;
}

.material-map-page__switch {
  position: absolute;
  left: 50%;
  top: 0;
  transform: translateX(-50%);
}

.material-map-page__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.mindmap-canvas {
  position: relative;
  flex: 1;
  min-height: 720px;
  overflow: hidden;
  border: 1px solid #dbeafe;
  border-radius: 30px;
  background-color: #f8fafc;
  background-image:
    linear-gradient(#e2e8f0 1px, transparent 1px),
    linear-gradient(90deg, #e2e8f0 1px, transparent 1px);
  background-size: 30px 30px;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.08);
  cursor: grab;
  touch-action: none;
  user-select: none;
}

.mindmap-canvas.is-panning {
  cursor: grabbing;
}

.mindmap-canvas.is-drop-active {
  box-shadow: inset 0 0 0 2px rgba(20, 184, 166, 0.35), 0 24px 70px rgba(15, 23, 42, 0.08);
}

.canvas-toolbar {
  position: absolute;
  right: 16px;
  top: 16px;
  z-index: 20;
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  max-width: 520px;
  gap: 8px;
  padding: 10px;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 14px 36px rgba(15, 23, 42, 0.08);
}

.canvas-toolbar :deep(.el-dropdown) {
  display: inline-flex;
}

.conflict-dialog {
  display: grid;
  gap: 14px;
}

.conflict-character-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.conflict-character-list :deep(.el-checkbox) {
  height: auto;
  margin: 0;
  padding: 10px 12px;
  white-space: normal;
}

.mindmap-scroller {
  position: absolute;
  left: 0;
  top: 0;
  will-change: transform;
}

.canvas-edges {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.canvas-edges path {
  fill: none;
  stroke: rgba(15, 118, 110, 0.28);
  stroke-width: 3;
  stroke-linecap: round;
}

.canvas-edges path.is-dashed {
  stroke: rgba(124, 58, 237, 0.26);
  stroke-dasharray: 10 10;
}

.canvas-node {
  position: absolute;
  z-index: 3;
  padding: 16px;
  border: 1px solid rgba(226, 232, 240, 0.95);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 18px 48px rgba(15, 23, 42, 0.1);
  cursor: default;
  user-select: text;
}

.canvas-node--world {
  border-color: rgba(37, 99, 235, 0.32);
  background: linear-gradient(180deg, #eff6ff 0%, #ffffff 100%);
}

.canvas-node--character {
  border-color: rgba(20, 184, 166, 0.32);
  background: linear-gradient(180deg, #f0fdfa 0%, #ffffff 100%);
}

.canvas-node--conflict {
  border-color: rgba(249, 115, 22, 0.32);
  background: linear-gradient(180deg, #fff7ed 0%, #ffffff 100%);
}

.canvas-node--idea {
  border-color: rgba(124, 58, 237, 0.28);
  background: linear-gradient(180deg, #faf5ff 0%, #ffffff 100%);
  cursor: grab;
}

.canvas-node--drop-target {
  border-color: rgba(20, 184, 166, 0.85);
  box-shadow: 0 0 0 4px rgba(20, 184, 166, 0.14), 0 22px 56px rgba(15, 23, 42, 0.16);
}

.canvas-node__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
}

.canvas-node__header span,
.role-chip,
.canvas-node__header strong {
  display: block;
}

.canvas-node__header span,
.role-chip {
  width: fit-content;
  padding: 0;
  border: 0;
  background: transparent;
  color: #0f766e;
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  cursor: pointer;
}

.canvas-node__header strong {
  margin-top: 4px;
  color: #0f172a;
  font-size: 18px;
}

.canvas-node--world .canvas-node__header span {
  color: #2563eb;
}

.canvas-node--conflict .canvas-node__header span {
  color: #ea580c;
}

.canvas-node--idea .canvas-node__header span {
  color: #7c3aed;
}

@media (max-width: 900px) {
  .material-map-page__header {
    flex-direction: column;
  }

  .material-map-page__switch {
    position: static;
    transform: none;
  }

  .canvas-toolbar {
    left: 12px;
    right: 12px;
    max-width: none;
  }
}
</style>
