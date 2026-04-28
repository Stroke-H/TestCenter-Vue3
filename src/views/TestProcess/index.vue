<script setup lang="ts">
import { ref, watch, computed, nextTick, onMounted, onUnmounted } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import { ArrowLeft, Plus, Minus, Monitor, Cellphone } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import MindMapNode from './MindMapNode.vue'
import type { MindNode, NodeStatus, ProcessItem } from './types'

defineOptions({ name: 'TestProcessEditor' })

const props = defineProps<{
  id?: string
}>()

const router = useRouter()
const processName = ref('未命名流程')

// 协助定位后端地址
const getBackendHost = () => {
  return `${window.location.protocol}//${window.location.hostname}:8080`
}

// ===== 核心数据 =====
const treeData = ref<MindNode>({
  id: 'root',
  label: '必测流程根节点',
  status: 'none',
  children: [],
  isRoot: true
})

const flatNodes = computed(() => {
  const nodes: MindNode[] = []
  const walk = (node: MindNode) => {
    if (!node.isRoot) nodes.push(node)
    node.children?.forEach(walk)
  }
  walk(treeData.value)
  return nodes
})

const progressSummary = computed(() => {
  const nodes = flatNodes.value
  const total = nodes.length
  const completed = nodes.filter((node) => node.status === 'completed').length
  const inProgress = nodes.filter((node) => node.status === 'in_progress').length
  const fixing = nodes.filter((node) => node.status === 'fixing').length
  const pending = nodes.filter((node) => node.status === 'none').length
  const percent = total > 0 ? Math.round((completed / total) * 100) : 0

  return { total, completed, inProgress, fixing, pending, percent }
})

const minimapNodes = computed(() => {
  const nodes: Array<MindNode & { depth: number }> = []
  const walk = (node: MindNode, depth = 0) => {
    nodes.push({ ...node, depth })
    node.children?.forEach(child => walk(child, depth + 1))
  }
  walk(treeData.value)
  return nodes
})

const minimapNodeStyle = (depth: number) => ({
  paddingLeft: `${Math.min(depth, 5) * 10 + 10}px`
})

// ===== 节点搜索 =====
const searchKeyword = ref('')
const activeSearchIndex = ref(0)

const searchResults = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword) return []

  return flatNodes.value.filter((node) => node.label.toLowerCase().includes(keyword))
})

const activeSearchNodeId = computed(() => searchResults.value[activeSearchIndex.value]?.id ?? null)

const focusSearchResult = (nextIndex = activeSearchIndex.value) => {
  if (searchResults.value.length === 0) {
    if (searchKeyword.value.trim()) {
      ElMessage.info('没有匹配的流程节点')
    }
    return
  }

  const normalizedIndex = (nextIndex + searchResults.value.length) % searchResults.value.length
  activeSearchIndex.value = normalizedIndex
  const target = searchResults.value[normalizedIndex]
  if (target) focusNodeById(target.id)
}

const focusNextSearchResult = () => {
  focusSearchResult(activeSearchIndex.value + 1)
}

const focusPreviousSearchResult = () => {
  focusSearchResult(activeSearchIndex.value - 1)
}

watch(searchKeyword, () => {
  activeSearchIndex.value = 0
})

// ===== 预览/视图状态 =====
const zoomLevel = ref(1)
const isVertical = ref(false) // 默认水平布局（横向生长）
const translateX = ref(0)
const translateY = ref(0)
const canvasRef = ref<HTMLElement | null>(null)
const scrollerRef = ref<HTMLElement | null>(null)
const miniMapRef = ref<HTMLElement | null>(null)
const isMinimapCollapsed = ref(true)
const miniMapPosition = ref<{ x: number; y: number } | null>(null)
const isDraggingMiniMap = ref(false)
const miniMapDragStart = ref({ x: 0, y: 0, startX: 0, startY: 0 })

const zoomStyle = computed(() => ({
  transform: `translate(${translateX.value}px, ${translateY.value}px) scale(${zoomLevel.value})`,
  transformOrigin: '0 0',
  transition: isPanning.value ? 'none' : 'transform 0.16s cubic-bezier(0.4, 0, 0.2, 1)'
}))

const miniMapStyle = computed(() => {
  if (!miniMapPosition.value) return {}

  return {
    left: `${miniMapPosition.value.x}px`,
    top: `${miniMapPosition.value.y}px`,
    right: 'auto',
    bottom: 'auto'
  }
})

const clamp = (value: number, min: number, max: number) => Math.min(Math.max(value, min), max)

const clampMiniMapPosition = (x: number, y: number) => {
  const canvas = canvasRef.value
  const miniMap = miniMapRef.value
  if (!canvas || !miniMap) return { x, y }

  const padding = 12
  const maxX = Math.max(padding, canvas.clientWidth - miniMap.offsetWidth - padding)
  const maxY = Math.max(padding, canvas.clientHeight - miniMap.offsetHeight - padding)

  return {
    x: clamp(x, padding, maxX),
    y: clamp(y, padding, maxY)
  }
}

const ensureMiniMapPosition = () => {
  const canvas = canvasRef.value
  const miniMap = miniMapRef.value
  if (!canvas || !miniMap || miniMapPosition.value) return

  miniMapPosition.value = clampMiniMapPosition(
    canvas.clientWidth - miniMap.offsetWidth - 18,
    18
  )
}

const zoomAtPoint = (nextZoom: number, clientX?: number, clientY?: number) => {
  const canvas = canvasRef.value
  const scroller = scrollerRef.value
  if (!canvas || !scroller) return

  const canvasRect = canvas.getBoundingClientRect()
  const scrollerRect = scroller.getBoundingClientRect()
  const oldZoom = zoomLevel.value
  const zoom = clamp(Number(nextZoom.toFixed(2)), 0.35, 2)
  const focusX = clientX ?? canvasRect.left + canvasRect.width / 2
  const focusY = clientY ?? canvasRect.top + canvasRect.height / 2
  const contentX = (focusX - scrollerRect.left) / oldZoom
  const contentY = (focusY - scrollerRect.top) / oldZoom

  zoomLevel.value = zoom
  translateX.value += (oldZoom - zoom) * contentX
  translateY.value += (oldZoom - zoom) * contentY
}

const handleZoomIn = () => zoomAtPoint(zoomLevel.value + 0.1)
const handleZoomOut = () => zoomAtPoint(zoomLevel.value - 0.1)
const handleZoomReset = () => {
  zoomLevel.value = 1
  centerMindMap()
}

const centerMindMap = () => {
  const canvas = canvasRef.value
  const scroller = scrollerRef.value
  if (!canvas || !scroller) return

  translateX.value = Math.max(80, (canvas.clientWidth - scroller.offsetWidth * zoomLevel.value) / 2)
  translateY.value = Math.max(80, (canvas.clientHeight - scroller.offsetHeight * zoomLevel.value) / 2)
}

const fitMindMap = () => {
  const canvas = canvasRef.value
  const scroller = scrollerRef.value
  if (!canvas || !scroller) return

  const availableWidth = Math.max(320, canvas.clientWidth - 160)
  const availableHeight = Math.max(240, canvas.clientHeight - 160)
  const nextZoom = clamp(Math.min(availableWidth / scroller.offsetWidth, availableHeight / scroller.offsetHeight), 0.35, 1.15)
  zoomLevel.value = Number(nextZoom.toFixed(2))
  centerMindMap()
}

const focusNodeById = (nodeId: string) => {
  const canvas = canvasRef.value
  const scroller = scrollerRef.value
  if (!canvas || !scroller) return

  const nodeElement = scroller.querySelector<HTMLElement>(`[data-node-id="${nodeId}"]`)
  if (!nodeElement) return

  const canvasRect = canvas.getBoundingClientRect()
  const nodeRect = nodeElement.getBoundingClientRect()
  const canvasCenterX = canvasRect.left + canvasRect.width / 2
  const canvasCenterY = canvasRect.top + canvasRect.height / 2
  const nodeCenterX = nodeRect.left + nodeRect.width / 2
  const nodeCenterY = nodeRect.top + nodeRect.height / 2

  translateX.value += canvasCenterX - nodeCenterX
  translateY.value += canvasCenterY - nodeCenterY
  activeOpsNodeId.value = nodeId
  activeMenuNodeId.value = null
  menuState.value = null
}

const focusNextUnfinished = () => {
  const target = flatNodes.value.find((node) => node.status !== 'completed')
  if (!target) {
    ElMessage.success('所有流程节点都已完成')
    return
  }

  focusNodeById(target.id)
}

const handleWheel = (e: WheelEvent) => {
  // 仅当按下 Ctrl 或 Meta (Cmd) 时触发缩放
  if (e.ctrlKey || e.metaKey) {
    e.preventDefault()
    const delta = e.deltaY < 0 ? 0.1 : -0.1
    zoomAtPoint(zoomLevel.value + delta, e.clientX, e.clientY)
  }
}

// ===== 画布平移 (Pointer Events) =====
const isPanning = ref(false)
const startPos = ref({ x: 0, y: 0, tx: 0, ty: 0 })
let panFrame = 0
let pendingPan: PointerEvent | null = null

const isCanvasPanTarget = (target: HTMLElement) => {
  return !target.closest('.node-card')
    && !target.closest('.node-menu')
    && !target.closest('.node-ops')
    && !target.closest('.floating-status-menu')
    && !target.closest('.mindmap-mini-map')
}

const handlePointerDown = (e: PointerEvent) => {
  const target = e.target as HTMLElement
  if (e.button !== 0 || !isCanvasPanTarget(target)) return

  menuState.value = null
  activeMenuNodeId.value = null
  isPanning.value = true
  startPos.value = {
    x: e.clientX,
    y: e.clientY,
    tx: translateX.value,
    ty: translateY.value
  }
  canvasRef.value?.setPointerCapture(e.pointerId)
}

const handlePointerMove = (e: PointerEvent) => {
  if (!isPanning.value) return
  e.preventDefault()
  pendingPan = e
  if (panFrame) return

  panFrame = window.requestAnimationFrame(() => {
    if (!pendingPan) return
    const dx = pendingPan.clientX - startPos.value.x
    const dy = pendingPan.clientY - startPos.value.y
    translateX.value = startPos.value.tx + dx
    translateY.value = startPos.value.ty + dy
    panFrame = 0
    pendingPan = null
  })
}

const handlePointerUp = (e?: PointerEvent) => {
  isPanning.value = false
  pendingPan = null
  if (panFrame) {
    window.cancelAnimationFrame(panFrame)
    panFrame = 0
  }
  if (e && canvasRef.value?.hasPointerCapture(e.pointerId)) {
    canvasRef.value.releasePointerCapture(e.pointerId)
  }
}

const handleMiniMapPointerMove = (event: PointerEvent) => {
  if (!isDraggingMiniMap.value) return

  const dx = event.clientX - miniMapDragStart.value.x
  const dy = event.clientY - miniMapDragStart.value.y
  miniMapPosition.value = clampMiniMapPosition(
    miniMapDragStart.value.startX + dx,
    miniMapDragStart.value.startY + dy
  )
}

const handleMiniMapPointerUp = () => {
  if (!isDraggingMiniMap.value) return

  isDraggingMiniMap.value = false
  window.removeEventListener('pointermove', handleMiniMapPointerMove)
  window.removeEventListener('pointerup', handleMiniMapPointerUp)
}

const handleMiniMapDragStart = (event: PointerEvent) => {
  if (event.button !== 0) return
  if ((event.target as HTMLElement).closest('button')) return

  event.preventDefault()
  ensureMiniMapPosition()
  const currentPosition = miniMapPosition.value || { x: 0, y: 0 }
  miniMapDragStart.value = {
    x: event.clientX,
    y: event.clientY,
    startX: currentPosition.x,
    startY: currentPosition.y
  }
  isDraggingMiniMap.value = true
  window.addEventListener('pointermove', handleMiniMapPointerMove)
  window.addEventListener('pointerup', handleMiniMapPointerUp)
}

// ===== 拖拽逻辑 =====
const draggedNodeId = ref<string | null>(null)

const findNodeAndParent = (root: MindNode, id: string, parent: MindNode | null = null): { node: MindNode, parent: MindNode | null } | null => {
  if (root.id === id) return { node: root, parent }
  if (root.children) {
    for (const child of root.children) {
      const result = findNodeAndParent(child, id, root)
      if (result) return result
    }
  }
  return null
}

const getAutoParentStatus = (node: MindNode): NodeStatus => {
  const children = node.children || []
  if (children.length === 0) return node.status
  if (children.every((child) => child.status === 'completed')) return 'completed'
  if (children.some((child) => child.status !== 'none')) return 'in_progress'
  return 'none'
}

const syncAncestorStatuses = (nodeId: string) => {
  let currentId = nodeId

  while (currentId) {
    const current = findNodeAndParent(treeData.value, currentId)
    const parent = current?.parent
    if (!parent) return

    parent.status = getAutoParentStatus(parent)
    currentId = parent.id
  }
}

const syncBranchStatusFrom = (nodeId: string) => {
  const current = findNodeAndParent(treeData.value, nodeId)
  if (!current) return

  current.node.status = getAutoParentStatus(current.node)
  syncAncestorStatuses(current.node.id)
}

const handleNodeDragStart = (id: string) => {
  draggedNodeId.value = id
}

const handleNodeDrop = (targetId: string) => {
  if (!draggedNodeId.value || draggedNodeId.value === targetId) return

  const source = findNodeAndParent(treeData.value, draggedNodeId.value)
  const target = findNodeAndParent(treeData.value, targetId)

  if (!source || !target || !source.parent) {
    ElMessage.warning('根节点不可移动或目标无效')
    return
  }

  // 检查是否将父节点移动到子节点（防止循环）
  let p = target.node
  while (p) {
    if (p.id === source.node.id) {
      ElMessage.error('无法将父节点移动到自身的子节点下')
      return
    }
    const parentInfo = findNodeAndParent(treeData.value, p.id)
    p = parentInfo?.parent as MindNode
  }

  // 执行移动
  const sParent = source.parent
  const sIndex = sParent.children?.findIndex(n => n.id === source.node.id) ?? -1
  if (sIndex !== -1) {
    sParent.children?.splice(sIndex, 1)
    if (!target.node.children) target.node.children = []
    target.node.children.push(source.node)
    syncBranchStatusFrom(sParent.id)
    syncBranchStatusFrom(target.node.id)
    ElMessage.success('节点已移动')
  }
  
  draggedNodeId.value = null
}

// ===== 持久化逻辑 =====
const isProcessLoaded = ref(false)
const isAutoSaving = ref(false)
const lastSavedSignature = ref('')
const saveState = ref<'saved' | 'dirty' | 'saving' | 'error'>('saved')
let autoSaveTimer: ReturnType<typeof window.setTimeout> | null = null

const currentProcessSignature = computed(() => JSON.stringify({
  name: processName.value,
  data: treeData.value
}))

const hasUnsavedChanges = computed(() => {
  return isProcessLoaded.value && currentProcessSignature.value !== lastSavedSignature.value
})

const saveStateText = computed(() => {
  if (isAutoSaving.value || saveState.value === 'saving') return '保存中'
  if (saveState.value === 'error') return '保存失败'
  if (hasUnsavedChanges.value) return props.id ? '待自动保存' : '未保存'
  return '已保存'
})

const rememberSavedSnapshot = () => {
  lastSavedSignature.value = currentProcessSignature.value
  saveState.value = 'saved'
}

const clearAutoSaveTimer = () => {
  if (!autoSaveTimer) return
  window.clearTimeout(autoSaveTimer)
  autoSaveTimer = null
}

async function loadProcess() {
  isProcessLoaded.value = false
  clearAutoSaveTimer()

  if (!props.id) {
    if (processName.value === '未命名流程') {
      processName.value = '新业务流程-' + new Date().toLocaleDateString()
    }
    await nextTick()
    isProcessLoaded.value = true
    rememberSavedSnapshot()
    return
  }

  try {
    const response = await fetch(`${getBackendHost()}/api/processes/${props.id}`)
    if (response.ok) {
      const item: ProcessItem = await response.json()
      processName.value = item.name
      treeData.value = JSON.parse(JSON.stringify(item.data))
      await nextTick()
      fitMindMap()
      isProcessLoaded.value = true
      rememberSavedSnapshot()
    } else {
      ElMessage.error('该流程不存在')
      router.push({ name: 'TestProcessList' })
    }
  } catch (e) {
    ElMessage.error('加载失败，请检查网络')
  }
}

// 监听 ID 变化（解决保存后不刷新的 bug）
watch(() => props.id, () => {
  loadProcess()
}, { immediate: true })

const saveProcess = async (options: { silent?: boolean; redirectNew?: boolean } = {}) => {
  const now = new Date().toLocaleString()
  const dataToSave = JSON.parse(JSON.stringify(treeData.value))
  const signatureToSave = JSON.stringify({
    name: processName.value,
    data: dataToSave
  })
  
  const idToSave = props.id || 'proc-' + Math.random().toString(36).substring(2, 9)
  
  const payload: ProcessItem = {
    id: idToSave,
    name: processName.value,
    data: dataToSave,
    updatedAt: now
  }

  try {
    saveState.value = 'saving'
    isAutoSaving.value = Boolean(options.silent)
    const response = await fetch(`${getBackendHost()}/api/processes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    })
    
    if (response.ok) {
      lastSavedSignature.value = signatureToSave
      saveState.value = currentProcessSignature.value === signatureToSave ? 'saved' : 'dirty'
      if (!options.silent) ElMessage.success('保存成功')
      if (!props.id && options.redirectNew) {
        router.replace({ name: 'TestProcessEditor', params: { id: idToSave } })
      }
      if (currentProcessSignature.value !== signatureToSave) scheduleAutoSave()
      return true
    } else {
      saveState.value = 'error'
      if (!options.silent) ElMessage.error('保存失败')
    }
  } catch (e) {
    saveState.value = 'error'
    if (!options.silent) ElMessage.error('网络错误，保存失败')
  } finally {
    isAutoSaving.value = false
  }

  return false
}

const handleSave = () => {
  void saveProcess({ redirectNew: true })
}

const scheduleAutoSave = () => {
  clearAutoSaveTimer()
  if (!props.id) return

  autoSaveTimer = window.setTimeout(() => {
    void saveProcess({ silent: true })
  }, 1200)
}

watch(currentProcessSignature, () => {
  if (!isProcessLoaded.value) return

  if (!hasUnsavedChanges.value) {
    saveState.value = 'saved'
    clearAutoSaveTimer()
    return
  }

  saveState.value = 'dirty'
  scheduleAutoSave()
})

// ===== 节点操作 =====
const generateId = () => Math.random().toString(36).substring(2, 9)

const handleAddNode = (parent: MindNode) => {
  if (!parent.children) parent.children = []
  parent.children.push({
    id: generateId(),
    label: '新业务流程',
    status: 'none',
    children: []
  })
  syncBranchStatusFrom(parent.id)
  nextTick(fitMindMap)
}

const handleDeleteNode = (parent: MindNode, id: string) => {
  if (!parent.children) return
  const index = parent.children.findIndex(n => n.id === id)
  if (index !== -1) {
    parent.children.splice(index, 1)
    syncBranchStatusFrom(parent.id)
    nextTick(fitMindMap)
  }
}

const handleUpdateStatus = (node: MindNode, status: NodeStatus) => {
  node.status = status
  syncAncestorStatuses(node.id)
  activeMenuNodeId.value = null
  menuState.value = null
}

const updateSubtreeStatus = (node: MindNode, status: NodeStatus) => {
  node.status = status
  node.children?.forEach(child => updateSubtreeStatus(child, status))
}

const handleUpdateSubtreeStatus = (node: MindNode, status: NodeStatus) => {
  updateSubtreeStatus(node, status)
  syncAncestorStatuses(node.id)
  activeMenuNodeId.value = null
  menuState.value = null
  ElMessage.success(status === 'completed' ? '当前分支已全部标记完成' : '当前分支已全部重置')
}

const handleToggleStatus = (node: MindNode) => {
  const statusFlow: NodeStatus[] = ['none', 'in_progress', 'fixing', 'completed']
  const currentIndex = statusFlow.indexOf(node.status)
  node.status = statusFlow[(currentIndex + 1) % statusFlow.length] || 'none'
  syncAncestorStatuses(node.id)
}

// ===== 交互状态 =====
const activeMenuNodeId = ref<string | null>(null)
const activeOpsNodeId = ref<string | null>(null)
const editingNodeId = ref<string | null>(null)
const menuState = ref<{ node: MindNode; x: number; y: number } | null>(null)
const menuStyle = computed(() => ({
  left: `${menuState.value?.x ?? 0}px`,
  top: `${menuState.value?.y ?? 0}px`
}))
let hoverTimer: any = null

const openStatusMenu = (node: MindNode, source: HTMLElement) => {
  const rect = source.getBoundingClientRect()
  const menuWidth = 156
  const menuHeight = 190
  const gap = 12
  const rightSpace = window.innerWidth - rect.right
  const bottomSpace = window.innerHeight - rect.top
  const x = rightSpace >= menuWidth + gap
    ? rect.right + gap
    : Math.max(gap, rect.left - menuWidth - gap)
  const y = bottomSpace >= menuHeight
    ? rect.top
    : Math.max(gap, window.innerHeight - menuHeight - gap)

  menuState.value = { node, x, y }
  activeMenuNodeId.value = node.id
  activeOpsNodeId.value = null
}

const onNodeMouseEnter = (node: MindNode, event: MouseEvent) => {
  const source = event.currentTarget as HTMLElement
  hoverTimer = setTimeout(() => {
    openStatusMenu(node, source)
  }, 2000)
}

const onNodeMouseLeave = () => {
  if (hoverTimer) {
    clearTimeout(hoverTimer)
    hoverTimer = null
  }
}

const handleShowMenu = (node: MindNode, event: MouseEvent) => {
  openStatusMenu(node, event.currentTarget as HTMLElement)
}
const handleShowOps = (node: MindNode) => { activeOpsNodeId.value = node.id }
const handleStartEdit = (node: MindNode) => {
  editingNodeId.value = node.id
  activeOpsNodeId.value = null
}
const handleStopEdit = () => { editingNodeId.value = null }
const clearActiveStates = () => {
  activeMenuNodeId.value = null
  activeOpsNodeId.value = null
  editingNodeId.value = null
  menuState.value = null
}

watch(isVertical, () => {
  nextTick(fitMindMap)
})

watch(isMinimapCollapsed, () => {
  nextTick(() => {
    if (!miniMapPosition.value) return
    miniMapPosition.value = clampMiniMapPosition(miniMapPosition.value.x, miniMapPosition.value.y)
  })
})

const handleBeforeUnload = (event: BeforeUnloadEvent) => {
  if (!hasUnsavedChanges.value) return

  event.preventDefault()
  event.returnValue = ''
}

const handleWindowResize = () => {
  fitMindMap()
  if (!miniMapPosition.value) return
  miniMapPosition.value = clampMiniMapPosition(miniMapPosition.value.x, miniMapPosition.value.y)
}

onBeforeRouteLeave(() => {
  if (!hasUnsavedChanges.value) return true

  return window.confirm('当前流程还有未保存的改动，确定要离开吗？')
})

onMounted(() => {
  nextTick(() => {
    fitMindMap()
    ensureMiniMapPosition()
  })
  window.addEventListener('resize', handleWindowResize)
  window.addEventListener('beforeunload', handleBeforeUnload)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize)
  window.removeEventListener('beforeunload', handleBeforeUnload)
  handleMiniMapPointerUp()
  clearAutoSaveTimer()
  if (panFrame) window.cancelAnimationFrame(panFrame)
})
</script>

<template>
  <div class="process-view" @click="clearActiveStates" @contextmenu.prevent>
    <!-- 顶部导航栏 -->
    <div class="process-header">
      <div class="header-left">
        <el-button link @click="router.push({ name: 'TestProcessList' })">
          <el-icon><ArrowLeft /></el-icon>
          返回仓库
        </el-button>
        <div class="divider-v"></div>
        <el-input 
          v-model="processName" 
          placeholder="请输入流程名称" 
          class="name-input"
          size="small"
        />
      </div>

      <div class="header-center">
        <div class="progress-pill" :title="`已完成 ${progressSummary.completed}/${progressSummary.total}`">
          <span class="progress-pill__label">完成度</span>
          <strong>{{ progressSummary.percent }}%</strong>
          <span>{{ progressSummary.completed }}/{{ progressSummary.total }}</span>
        </div>
        <el-button size="small" :disabled="progressSummary.pending + progressSummary.inProgress + progressSummary.fixing === 0" @click="focusNextUnfinished">
          下一个未完成
        </el-button>
        <div class="node-search">
          <el-input
            v-model="searchKeyword"
            size="small"
            clearable
            placeholder="搜索节点"
            @keyup.enter="focusSearchResult()"
          />
          <span v-if="searchKeyword" class="node-search__count">
            {{ searchResults.length ? activeSearchIndex + 1 : 0 }}/{{ searchResults.length }}
          </span>
          <el-button-group v-if="searchKeyword">
            <el-button size="small" :disabled="searchResults.length === 0" @click="focusPreviousSearchResult">上一个</el-button>
            <el-button size="small" :disabled="searchResults.length === 0" @click="focusNextSearchResult">下一个</el-button>
          </el-button-group>
        </div>
        <div class="divider-v"></div>
        <el-button-group>
          <el-button size="small" :icon="Minus" @click="handleZoomOut" title="缩小" />
          <el-button size="small" @click="handleZoomReset" title="重置缩放">
            {{ Math.round(zoomLevel * 100) }}%
          </el-button>
          <el-button size="small" :icon="Plus" @click="handleZoomIn" title="放大" />
        </el-button-group>
        <el-button-group class="view-tools">
          <el-button size="small" @click="fitMindMap" title="完整显示当前流程">适应屏幕</el-button>
          <el-button size="small" @click="centerMindMap" title="回到流程中心">回到中心</el-button>
        </el-button-group>
        
        <div class="divider-v"></div>

        <el-button-group>
          <el-button 
            size="small" 
            :type="!isVertical ? 'primary' : ''" 
            :icon="Monitor" 
            @click="isVertical = false"
            title="水平布局"
          >水平</el-button>
          <el-button 
            size="small" 
            :type="isVertical ? 'primary' : ''" 
            :icon="Cellphone" 
            @click="isVertical = true"
            title="垂直布局"
          >垂直</el-button>
        </el-button-group>
      </div>

      <div class="header-right">
        <div class="status-summary">
          <span class="status-dot status-dot--pending"></span>待 {{ progressSummary.pending }}
          <span class="status-dot status-dot--progress"></span>进行 {{ progressSummary.inProgress }}
          <span class="status-dot status-dot--fixing"></span>修复 {{ progressSummary.fixing }}
          <span class="save-state" :class="'save-state--' + saveState">{{ saveStateText }}</span>
        </div>
        <el-button type="primary" size="small" @click="handleSave">
          保存修改
        </el-button>
        <el-tooltip placement="bottom">
          <template #content>
            单击:显示操作 | 双击:编辑名称<br/>
            点击状态角标:快速切换状态<br/>
            右键/悬浮2s:状态菜单<br/>
            <b>拖拽:调整流程层级</b>
          </template>
          <el-button type="info" plain size="small" style="margin-left: 12px">
            说明
          </el-button>
        </el-tooltip>
      </div>
    </div>

    <!-- 画布区域 -->
    <div 
      ref="canvasRef"
      class="mindmap-canvas" 
      :class="[
        isVertical ? 'layout-vertical' : 'layout-horizontal',
        { 'is-panning': isPanning }
      ]"
      @wheel="handleWheel"
      @pointerdown="handlePointerDown"
      @pointermove="handlePointerMove"
      @pointerup="handlePointerUp"
      @pointercancel="handlePointerUp"
      @pointerleave="handlePointerUp"
    >
      <div ref="scrollerRef" class="mindmap-scroller" :style="zoomStyle">
        <MindMapNode 
          :node="treeData"
          :is-vertical="isVertical"
          @add="handleAddNode"
          @delete="handleDeleteNode"
          @update-status="handleUpdateStatus"
          @toggle-status="handleToggleStatus"
          @mouseenter="onNodeMouseEnter"
          @mouseleave="onNodeMouseLeave"
          @show-menu="handleShowMenu"
          @show-ops="handleShowOps"
          @start-edit="handleStartEdit"
          @stop-edit="handleStopEdit"
          @drag-start="handleNodeDragStart"
          @drop="handleNodeDrop"
          :active-id="activeMenuNodeId"
          :active-ops-id="activeOpsNodeId"
          :editing-id="editingNodeId"
          :highlighted-id="activeSearchNodeId"
        />
      </div>

      <aside
        ref="miniMapRef"
        class="mindmap-mini-map"
        :class="{ 'is-collapsed': isMinimapCollapsed, 'is-dragging': isDraggingMiniMap }"
        :style="miniMapStyle"
        @click.stop
      >
        <div class="mini-map-header" @pointerdown="handleMiniMapDragStart">
          <div>
            <strong>流程小地图</strong>
            <span>{{ minimapNodes.length }} 个节点</span>
          </div>
          <button type="button" class="mini-map-toggle" @click="isMinimapCollapsed = !isMinimapCollapsed">
            {{ isMinimapCollapsed ? '展开' : '收起' }}
          </button>
        </div>
        <div v-if="!isMinimapCollapsed" class="mini-map-list">
          <button
            v-for="node in minimapNodes"
            :key="node.id"
            type="button"
            class="mini-map-node"
            :class="[
              'mini-map-node--' + node.status,
              { 'is-active': activeOpsNodeId === node.id || activeMenuNodeId === node.id || activeSearchNodeId === node.id }
            ]"
            :style="minimapNodeStyle(node.depth)"
            @click="focusNodeById(node.id)"
          >
            <span class="mini-map-dot"></span>
            <span class="mini-map-label">{{ node.label || '未命名节点' }}</span>
          </button>
        </div>
      </aside>
    </div>

    <Transition name="fade">
      <div
        v-if="menuState"
        class="floating-status-menu"
        :style="menuStyle"
        @click.stop
      >
        <div class="menu-header">修改状态</div>
        <button
          v-if="menuState.node.status !== 'completed'"
          type="button"
          class="menu-item"
          @click="handleUpdateStatus(menuState.node, 'completed')"
        >
          已完成
        </button>
        <button
          v-if="menuState.node.status !== 'none'"
          type="button"
          class="menu-item"
          @click="handleUpdateStatus(menuState.node, 'none')"
        >
          取消完成
        </button>
        <button
          v-if="menuState.node.status !== 'in_progress'"
          type="button"
          class="menu-item"
          @click="handleUpdateStatus(menuState.node, 'in_progress')"
        >
          进行中
        </button>
        <button
          v-if="menuState.node.status !== 'fixing'"
          type="button"
          class="menu-item"
          @click="handleUpdateStatus(menuState.node, 'fixing')"
        >
          修复中
        </button>
        <div v-if="menuState.node.children?.length" class="menu-header menu-header--sub">批量标记</div>
        <button
          v-if="menuState.node.children?.length"
          type="button"
          class="menu-item"
          @click="handleUpdateSubtreeStatus(menuState.node, 'completed')"
        >
          本分支全部完成
        </button>
        <button
          v-if="menuState.node.children?.length"
          type="button"
          class="menu-item"
          @click="handleUpdateSubtreeStatus(menuState.node, 'none')"
        >
          本分支全部重置
        </button>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.process-view {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f1f5f9;
  font-family: 'Inter', -apple-system, sans-serif;
}

.process-header {
  min-height: 56px;
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 20px;
  z-index: 1000;
  flex-shrink: 0;
}

.header-left, .header-right, .header-center {
  display: flex;
  align-items: center;
}

.header-left {
  min-width: 260px;
}

.header-right {
  justify-content: flex-end;
  min-width: 260px;
}

.header-center {
  justify-content: center;
  flex-wrap: wrap;
  gap: 10px;
}

.progress-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 30px;
  padding: 0 12px;
  color: #0f172a;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
}

.progress-pill strong {
  color: #2563eb;
  font-size: 14px;
}

.progress-pill__label {
  color: #64748b;
}

.status-summary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-right: 12px;
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

.save-state {
  margin-left: 4px;
  padding: 2px 7px;
  border-radius: 999px;
  color: #64748b;
  background: #f1f5f9;
  font-size: 11px;
}

.save-state--saved {
  color: #047857;
  background: #d1fae5;
}

.save-state--dirty {
  color: #b45309;
  background: #fef3c7;
}

.save-state--saving {
  color: #1d4ed8;
  background: #dbeafe;
}

.save-state--error {
  color: #b91c1c;
  background: #fee2e2;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-dot--pending {
  background: #94a3b8;
}

.status-dot--progress {
  background: #3b82f6;
}

.status-dot--fixing {
  background: #f59e0b;
}

.node-search {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.node-search :deep(.el-input) {
  width: 180px;
}

.node-search__count {
  min-width: 42px;
  color: #64748b;
  font-size: 12px;
  font-weight: 800;
  text-align: center;
}

.view-tools {
  margin-left: 2px;
}

.name-input {
  width: 220px;
}

.divider-v {
  width: 1px;
  height: 20px;
  background: #e2e8f0;
  margin: 0 16px;
}

.mindmap-canvas {
  position: relative;
  flex: 1;
  overflow: hidden;
  background-image: 
    linear-gradient(#e2e8f0 1px, transparent 1px),
    linear-gradient(90deg, #e2e8f0 1px, transparent 1px);
  background-size: 30px 30px;
  background-color: #f8fafc;
  cursor: grab;
  touch-action: none;
  user-select: none;
}

.mindmap-canvas.is-panning {
  cursor: grabbing;
}

.mindmap-scroller {
  position: absolute;
  top: 0;
  left: 0;
  display: inline-block;
  padding: 80px;
  will-change: transform;
}

.layout-horizontal .mindmap-scroller {
  transform-origin: 0 0;
}

.mindmap-mini-map {
  position: absolute;
  right: 18px;
  top: 18px;
  z-index: 500;
  width: 260px;
  max-height: min(420px, calc(100% - 36px));
  overflow: hidden;
  color: #0f172a;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid rgba(148, 163, 184, 0.35);
  border-radius: 18px;
  box-shadow: 0 18px 50px rgba(15, 23, 42, 0.18);
  backdrop-filter: blur(16px);
  cursor: default;
  user-select: none;
}

.mindmap-mini-map.is-collapsed {
  width: 178px;
}

.mindmap-mini-map.is-dragging {
  box-shadow: 0 24px 60px rgba(15, 23, 42, 0.24);
}

.mini-map-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 12px 10px;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
  cursor: grab;
}

.mindmap-mini-map.is-dragging .mini-map-header {
  cursor: grabbing;
}

.mini-map-header strong,
.mini-map-header span {
  display: block;
}

.mini-map-header strong {
  font-size: 13px;
  font-weight: 900;
}

.mini-map-header span {
  margin-top: 2px;
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
}

.mini-map-toggle {
  flex-shrink: 0;
  padding: 4px 8px;
  color: #2563eb;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 800;
  cursor: pointer;
}

.mini-map-list {
  max-height: 340px;
  overflow: auto;
  padding: 8px;
}

.mini-map-node {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  height: 28px;
  color: #334155;
  background: transparent;
  border: 0;
  border-radius: 9px;
  font-size: 12px;
  font-weight: 700;
  text-align: left;
  cursor: pointer;
}

.mini-map-node:hover,
.mini-map-node.is-active {
  color: #0f172a;
  background: #eef6ff;
}

.mini-map-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #94a3b8;
}

.mini-map-node--completed .mini-map-dot {
  background: #10b981;
}

.mini-map-node--in_progress .mini-map-dot {
  background: #3b82f6;
}

.mini-map-node--fixing .mini-map-dot {
  background: #f59e0b;
}

.mini-map-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.floating-status-menu {
  position: fixed;
  z-index: 3000;
  min-width: 146px;
  padding: 6px;
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  box-shadow: 0 18px 42px rgba(15, 23, 42, 0.18);
}

.floating-status-menu .menu-header {
  padding: 6px 12px;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  border-bottom: 1px solid #f1f5f9;
}

.floating-status-menu .menu-header--sub {
  margin-top: 4px;
}

.floating-status-menu .menu-item {
  display: block;
  width: 100%;
  padding: 8px 12px;
  color: #475569;
  font-size: 13px;
  font-weight: 700;
  text-align: left;
  background: transparent;
  border: 0;
  border-radius: 6px;
  cursor: pointer;
}

.floating-status-menu .menu-item:hover {
  color: #0f172a;
  background: #f1f5f9;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.16s ease, transform 0.16s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: scale(0.96);
}
</style>
