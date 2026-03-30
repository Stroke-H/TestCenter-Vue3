<script setup lang="ts">
import type { Piece, PieceRank } from '../types';

defineProps<{
  piece: Piece | null;
  isSelected?: boolean;
  canBeTargeted?: boolean;
}>();

// 根据职级映射头像文件名
function getAvatarUrl(rank: PieceRank) {
  const map: Record<PieceRank, string> = {
    8: 'elephant', 7: 'lion', 6: 'tiger', 5: 'leopard',
    4: 'wolf', 3: 'dog', 2: 'cat', 1: 'rat'
  };
  // 使用 Vite 的动态资源导入方式
  return new URL(`../../../assets/jungle/${map[rank]}.png`, import.meta.url).href;
}

// 加载背面封面图
function getCoverUrl() {
  return new URL(`../../../assets/jungle/cover.png`, import.meta.url).href;
}
</script>

<template>
  <div 
    class="chess-piece-container"
    :class="{ 'is-selected': isSelected, 'can-be-targeted': canBeTargeted }"
  >
    <div v-if="piece" class="chess-piece" :class="{ 'is-revealed': piece.isRevealed }">
      <!-- 背面 (暗棋) -->
      <div class="piece-face piece-back">
        <img :src="getCoverUrl()" class="cover-image" />
        <div class="back-overlay"></div>
      </div>
      
      <!-- 正面 (明棋) -->
      <div 
        class="piece-face piece-front" 
        :class="[piece.color]"
      >
        <div class="avatar-wrapper">
          <img :src="getAvatarUrl(piece.rank)" class="animal-avatar" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chess-piece-container {
  width: 95%; /* 稍微加宽 */
  height: 95%;
  perspective: 1000px;
  cursor: pointer;
  position: relative;
  transition: all 0.3s ease;
  aspect-ratio: 1;
}

.chess-piece {
  position: relative;
  width: 100%;
  height: 100%;
  transition: transform 0.6s cubic-bezier(0.4, 0, 0.2, 1);
  transform-style: preserve-3d;
}

.chess-piece.is-revealed {
  transform: rotateY(180deg);
}

.piece-face {
  position: absolute;
  width: 100%;
  height: 100%;
  backface-visibility: hidden;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 15px rgba(0,0,0,0.3);
  user-select: none;
  overflow: hidden;
  border: 4px solid rgba(0, 0, 0, 0.1);
}

/* 背面样式 */
.piece-back {
  background: #1e293b;
  border: 4px solid #0f172a;
}

.cover-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transform: scale(1.25); /* 放大以遮挡原图可能的边框 */
}

.back-overlay {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: radial-gradient(circle, transparent 40%, rgba(0,0,0,0.4) 100%);
  pointer-events: none;
}

/* 正面样式 */
.piece-front {
  transform: rotateY(180deg);
  display: flex;
  flex-direction: column;
  padding: 0;
  transition: all 0.3s ease;
}

.avatar-wrapper {
  flex: 1;
  width: 100%;
  height: 100%;
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border-radius: 50%;
}

.animal-avatar {
  width: 80%;
  height: 80%;
  object-fit: contain;
  transition: all 0.3s ease;
  z-index: 1;
  /* 透明背景图片：不需要 mix-blend-mode，直接显示底色 */
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
}

/* 阵营配色逻辑 - 纯色方案 */
.piece-front.blue {
  background: radial-gradient(circle at 40% 35%, #60a5fa, #2563eb 60%, #1d4ed8);
  border-color: #1d4ed8;
  box-shadow: 
    0 4px 15px rgba(0,0,0,0.3),
    inset 0 -6px 0 rgba(0, 0, 0, 0.2),
    inset 0 2px 4px rgba(255, 255, 255, 0.3);
}
.piece-front.blue .animal-avatar {
  filter: drop-shadow(0 0 12px rgba(255, 255, 255, 0.4));
}

.piece-front.red {
  background: radial-gradient(circle at 40% 35%, #f87171, #dc2626 60%, #b91c1c);
  border-color: #b91c1c;
  box-shadow: 
    0 4px 15px rgba(0,0,0,0.3),
    inset 0 -6px 0 rgba(0, 0, 0, 0.2),
    inset 0 2px 4px rgba(255, 255, 255, 0.3);
}
.piece-front.red .animal-avatar {
  filter: drop-shadow(0 0 12px rgba(255, 255, 255, 0.4));
}

.piece-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  padding: 0 4px;
}

.animal-name {
  font-size: min(1.8vh, 2.8vw);
  font-weight: 900;
}

.rank-badge {
  font-size: min(1vh, 1.5vw);
  background: rgba(0,0,0,0.05);
  padding: 1px 4px;
  border-radius: 4px;
  font-family: monospace;
}

/* 选中态：叠加位移，锁定 180 度翻面 */
.is-selected .chess-piece.is-revealed {
  transform: rotateY(180deg) translateY(-10px) scale(1.05);
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.3);
  z-index: 100;
}

.is-selected .chess-piece:not(.is-revealed) {
  transform: translateY(-8px) scale(1.05);
}

.is-selected::after {
  content: '';
  position: absolute;
  top: -8px; left: -8px; right: -8px; bottom: -8px;
  border: 3px solid #fbbf24;
  border-radius: 12px;
  animation: breathe 1.5s infinite;
  z-index: 10;
}

/* 可被捕食提示：在 180 度翻翻面基础上呼吸 */
.can-be-targeted .chess-piece.is-revealed {
  animation: pulse 1s infinite alternate;
}

@keyframes breathe {
  0% { opacity: 0.4; transform: scale(0.98); }
  50% { opacity: 1; transform: scale(1.02); }
  100% { opacity: 0.4; transform: scale(0.98); }
}

@keyframes pulse {
  0% { transform: rotateY(180deg) scale(1); }
  100% { transform: rotateY(180deg) scale(1.03) brightness(1.1); }
}
</style>
