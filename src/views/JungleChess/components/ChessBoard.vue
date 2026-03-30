<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Piece } from '../types';
import ChessPiece from './ChessPiece.vue';
import { ElMessage } from 'element-plus';

const props = defineProps<{
  board: (Piece | null)[][];
  currentPlayer: string | null;
  isOnline?: boolean;
  myRole?: 'host' | 'guest' | null;
  firstRevealerRole?: 'host' | 'guest' | null;
}>();

const emit = defineEmits<{
  (e: 'reveal', row: number, col: number): void;
  (e: 'move', fromR: number, fromC: number, toR: number, toC: number): void;
}>();

// --- 联机逻辑辅助 ---
// 获取第一个被翻开的棋子颜色，以此判断阵营分配
const firstRevealedColor = computed(() => {
  for (const row of props.board) {
    for (const p of row) {
      if (p && p.isRevealed) return p.color;
    }
  }
  return null;
});

// 判断当前本地玩家是否可以操作
const canLocalPlayerAct = computed(() => {
  if (!props.isOnline) return true;
  if (!props.currentPlayer) return true; 

  const firstColor = firstRevealedColor.value;
  const firstRole = props.firstRevealerRole;
  if (!firstColor || !firstRole) return true;

  // 这里的逻辑：
  // 谁翻开了第一个棋子，谁就拥有那个颜色。
  let myColor: string;
  if (props.myRole === 'host') {
    myColor = (firstRole === 'host') ? firstColor : (firstColor === 'blue' ? 'red' : 'blue');
  } else {
    myColor = (firstRole === 'guest') ? firstColor : (firstColor === 'blue' ? 'red' : 'blue');
  }
  
  return props.currentPlayer === myColor;
});

// 选中的棋子坐标
const selectedPos = ref<{ r: number; c: number } | null>(null);

// 计算所有的棋子列表（用于动画渲染）
const piecesList = computed(() => {
  const list: { piece: Piece; r: number; c: number }[] = [];
  props.board.forEach((row, r) => {
    row.forEach((piece, c) => {
      if (piece) {
        list.push({ piece, r, c });
      }
    });
  });
  return list;
});

// 计算选中的棋子
const selectedPiece = computed(() => {
  if (!selectedPos.value) return null;
  const row = props.board[selectedPos.value.r];
  return row ? row[selectedPos.value.c] : null;
});

// 计算合法移动目标格
const validMoves = computed(() => {
  if (!selectedPos.value || !selectedPiece.value) return [];
  const { r, c } = selectedPos.value;
  const possible = [
    { r: r - 1, c }, { r: r + 1, c }, { r: r, c: c - 1 }, { r: r, c: c + 1 }
  ].filter(p => p.r >= 0 && p.r < 4 && p.c >= 0 && p.c < 8);
  
  return possible.filter(p => {
    const row = props.board[p.r];
    if (!row) return false;
    const target = row[p.c];
    if (!target) return true; // 空地
    if (!target.isRevealed) return false; // 不能直接捕食未翻开的
    return target.color !== selectedPiece.value?.color; // 不能吃自己人
  });
});

function isMoveValid(r: number, c: number) {
  return validMoves.value.some(m => m.r === r && m.c === c);
}

function handleCellClick(r: number, c: number) {
  if (!canLocalPlayerAct.value) {
    ElMessage.warning('目前是对方回合');
    return;
  }

  const row = props.board[r];
  if (!row) return;
  const piece = row[c];

  // 1. 翻牌逻辑
  if (piece && !piece.isRevealed) {
    emit('reveal', r, c);
    selectedPos.value = null;
    return;
  }

  // 2. 选中逻辑
  if (piece && piece.isRevealed && piece.color === props.currentPlayer) {
    if (selectedPos.value?.r === r && selectedPos.value?.c === c) {
      selectedPos.value = null;
    } else {
      selectedPos.value = { r, c };
    }
    return;
  }

  // 3. 移动/捕食逻辑
  if (selectedPos.value && isMoveValid(r, c)) {
    emit('move', selectedPos.value.r, selectedPos.value.c, r, c);
    selectedPos.value = null;
    return;
  }

  selectedPos.value = null;
}
</script>

<template>
  <div class="board-outer">
    <div class="chess-board">
      <!-- 底部网格层 (32格) -->
      <div 
        v-for="idx in 32" 
        :key="`bg-${idx}`"
        class="board-cell-bg"
        :class="{ 
          'is-valid-target': isMoveValid(Math.floor((idx-1)/8), (idx-1)%8)
        }"
        @click="handleCellClick(Math.floor((idx-1)/8), (idx-1)%8)"
      >
        <div class="cell-highlight" v-if="isMoveValid(Math.floor((idx-1)/8), (idx-1)%8)"></div>
      </div>

      <!-- 棋子层 (绝对定位覆盖在网格上，使用相同的 Grid 布局) -->
      <TransitionGroup 
        tag="div" 
        name="piece" 
        class="pieces-layer"
      >
        <div 
          v-for="{ piece, r, c } in piecesList" 
          :key="piece.id"
          class="piece-slot"
          :style="{ 
            gridRow: r + 1, 
            gridColumn: c + 1 
          }"
          @click.stop="handleCellClick(r, c)"
        >
          <ChessPiece 
            :piece="piece" 
            :is-selected="selectedPos?.r === r && selectedPos?.c === c"
            :can-be-targeted="isMoveValid(r, c)"
          />
        </div>
      </TransitionGroup>
    </div>
  </div>
</template>

<style scoped>
.board-outer {
  background: #fdf6e3;
  padding: 12px; 
  padding-bottom: 20px; 
  border-radius: 16px;
  box-shadow: inset 0 2px 10px rgba(0,0,0,0.1), 0 20px 40px rgba(0,0,0,0.2);
  border: 10px solid #8b4513;
  width: 100%;
  max-width: min(95vw, 140vh);
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
}

.chess-board {
  display: grid;
  grid-template-rows: repeat(4, 1fr);
  grid-template-columns: repeat(8, 1fr);
  gap: 6px;
  width: 100%;
  aspect-ratio: 2/1;
  position: relative;
}

.board-cell-bg {
  aspect-ratio: 1;
  background: rgba(139, 69, 19, 0.05);
  border-radius: 12px;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border: 1px solid rgba(139, 69, 19, 0.08);
}

.pieces-layer {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: grid;
  grid-template-rows: repeat(4, 1fr);
  grid-template-columns: repeat(8, 1fr);
  gap: 6px;
  pointer-events: none;
  z-index: 10;
}

.piece-slot {
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: auto; /* 棋子槽区域捕获点击 */
  width: 100%;
  height: 100%;
}

/* 动画效果 */
.piece-move {
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

/* 优化后的捕食消失动画 */
/* 优化后的捕食消失动画 */
.piece-leave-active {
  /* 移除 position: absolute，确保棋子留在原有的网格格子内进行动画 */
  transition: all 0.4s cubic-bezier(0.5, 0, 0.5, 1);
  z-index: 50;
  pointer-events: none;
  transform-origin: center;
}

.piece-leave-to {
  opacity: 0;
  /* 缩小比例更自然，且保持在格子中心 */
  transform: scale(0.8) rotate(5deg);
}

/* 为消失的棋子添加更细腻的缩放和亮度变化 */
.piece-leave-active :deep(.chess-piece) {
  animation: vanish 0.4s ease-out forwards !important;
  transform-origin: center;
}

@keyframes vanish {
  0% { 
    transform: rotateY(180deg) scale(1); 
    filter: brightness(1);
  }
  20% { 
    transform: rotateY(180deg) scale(1.05); 
    filter: brightness(1.3);
  }
  100% { 
    transform: rotateY(220deg) scale(0); 
    filter: brightness(1.5) blur(2px);
    opacity: 0;
  }
}

.is-valid-target::after {
  content: '';
  width: 14px;
  height: 14px;
  background: #10b981;
  border-radius: 50%;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.6);
  z-index: 5;
}

.cell-highlight {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(16, 185, 129, 0.08);
  border: 2px dashed #10b981;
  border-radius: 12px;
  pointer-events: none;
  z-index: 10;
}
</style>
