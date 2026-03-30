<script setup lang="ts">
import { onMounted } from 'vue';
import { useGameLogic } from './composables/useGameLogic';
import { useWebSocket } from './composables/useWebSocket';
import ChessBoard from './components/ChessBoard.vue';
import { Refresh, ArrowLeft, Trophy, Connection, Loading } from '@element-plus/icons-vue';
import { useRouter } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';

const router = useRouter();
const { 
  board, 
  currentPlayer, 
  status, 
  winner, 
  initGame, 
  revealPiece, 
  movePiece,
  firstRevealerRole
} = useGameLogic();

const { 
  isMatching, 
  isOnline, 
  myRole, 
  opponent, 
  connect, 
  disconnect, 
  sendMessage, 
  onEvents 
} = useWebSocket();

onMounted(() => {
  initGame();
  
  onEvents({
    onMatchStart(role, opp) {
      // 二次确认机制：如果当前不再处于匹配或在线状态（例如用户刚点了取消），则忽略延迟的消息
      if (!isMatching.value && !isOnline.value) return;

      const opponentName = opp.username || opp.id || opp.nickname || '神秘对手';
      ElMessage.success(`匹配成功！对手：${opponentName}`);
      
      if (role === 'host') {
        // 先重新初始化棋盘，确保干净的初始状态，不带入对战前的布局
        initGame();
        const freshBoard = JSON.parse(JSON.stringify(board.value));
        sendMessage('sync_board', freshBoard);
      }
    },
    onSyncBoard(remoteBoard) {
      initGame(remoteBoard);
    },
    onAction(action) {
      if (action.type === 'reveal') {
        revealPiece(action.row, action.col, action.role);
      } else if (action.type === 'move') {
        movePiece(action.fromR, action.fromC, action.toR, action.toC);
      }
    },
    onDisconnect(reason) {
      ElMessageBox.confirm(
        reason || '对手已离开房间，对局已中断。',
        '连接断开',
        {
          confirmButtonText: '重新匹配',
          cancelButtonText: '返回单机',
          type: 'warning',
          distinguishCancelAndClose: true
        }
      ).then(() => {
        // 重新匹配
        disconnect();
        connect();
      }).catch((action) => {
        if (action === 'cancel') {
          // 返回单机
          disconnect();
          ElMessage.info('已切换回单机模式');
        }
      });
    }
  });
});

function handleReveal(r: number, c: number) {
  const role = isOnline.value ? (myRole.value as 'host'|'guest') : undefined;
  if (revealPiece(r, c, role)) {
    if (isOnline.value) {
      sendMessage('action', { type: 'reveal', row: r, col: c, role: role });
    }
  }
}

function handleMove(fR: number, fC: number, tR: number, tC: number) {
  if (movePiece(fR, fC, tR, tC)) {
    if (isOnline.value) {
      sendMessage('action', { type: 'move', fromR: fR, fromC: fC, toR: tR, toC: tC, role: myRole.value });
    }
  }
}

function handleRestart() {
  if (isOnline.value) {
    ElMessage.info('联机模式下暂不支持中途重开');
    return;
  }
  initGame();
}

function toggleOnline() {
  if (isOnline.value || isMatching.value) {
    disconnect();
    initGame();
  } else {
    connect();
  }
}

function goBack() {
  router.back();
}
</script>

<template>
  <div class="jungle-game-container">
    <!-- 顶部状态栏 -->
    <div class="game-header">
      <div class="header-left">
        <el-button :icon="ArrowLeft" circle @click="goBack" />
        <el-button 
          :type="isOnline || isMatching ? 'danger' : 'success'" 
          :icon="Connection"
          round
          plain
          @click="toggleOnline"
        >
          {{ isMatching ? '匹配中...' : isOnline ? '退出联机' : '联机匹配' }}
        </el-button>
      </div>

      <div class="turn-indicator">
        <template v-if="status === 'playing'">
          <div 
            class="player-dot" 
            :class="currentPlayer || 'waiting'"
          ></div>
          <span class="turn-text">
            {{ currentPlayer === 'blue' ? '蓝方回合' : currentPlayer === 'red' ? '红方回合' : '等待开局' }}
            <span v-if="isOnline && opponent" class="opponent-name"> (对阵: {{ opponent.nickname }})</span>
          </span>
        </template>
        <template v-else-if="status === 'ended'">
          <el-icon color="#f59e0b"><component :is="Trophy" /></el-icon>
          <span class="win-text">游戏结束：{{ winner === 'blue' ? '蓝方胜' : '红方胜' }}</span>
        </template>
      </div>
      <el-button type="primary" :icon="Refresh" @click="handleRestart" :disabled="isOnline">重开</el-button>
    </div>

    <!-- 核心棋盘区 -->
    <div class="game-body">
      <!-- 正在匹配遮罩 -->
      <div v-if="isMatching" class="matching-overlay">
        <el-icon class="is-loading" :size="40"><Loading /></el-icon>
        <p>正在寻找丛林对手中...</p>
        <el-button type="info" size="small" @click="toggleOnline">取消匹配</el-button>
      </div>

      <!-- 呼吸回合提示 -->
      <div 
        v-if="status === 'playing' && currentPlayer" 
        class="turn-banner"
        :class="currentPlayer"
      >
        <template v-if="isOnline">
          {{ (myRole === 'host' && currentPlayer === board.flat().find(p => p?.isRevealed)?.color) || 
             (myRole === 'guest' && currentPlayer !== board.flat().find(p => p?.isRevealed)?.color) 
             ? '请你行动' : '等待对方行动...' }}
          <!-- 注意：斗兽棋阵营是动态决定的，这里简化逻辑 -->
          当前：{{ currentPlayer === 'blue' ? '蓝方' : '红方' }} 回合
        </template>
        <template v-else>
          当前为 {{ currentPlayer === 'blue' ? '蓝方' : '红方' }} 行动回合
        </template>
      </div>

      <ChessBoard 
        :board="board" 
        :current-player="currentPlayer"
        :is-online="isOnline"
        :my-role="myRole"
        :first-revealer-role="firstRevealerRole"
        @reveal="handleReveal"
        @move="handleMove"
      />
    </div>

    <!-- 底部操作说明 -->
    <div class="game-footer">
      <div class="rule-hint">
        💡 提示：翻开暗棋决定阵营。鼠吃象，象不吃鼠，大吃小。
      </div>
    </div>

    <!-- 胜利弹窗 -->
    <el-dialog
      v-model="status"
      :show-close="false"
      width="300px"
      center
      class="win-dialog"
      v-if="status === 'ended'"
    >
      <div class="win-content">
        <div class="win-icon" :class="winner">🏆</div>
        <h3>恭喜 {{ winner === 'blue' ? '蓝方' : '红方' }} 获胜！</h3>
        <p>清剿了对方所有势力。</p>
        <el-button type="primary" round @click="handleRestart">再来一局</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.jungle-game-container {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 80px);
  max-width: 1000px;
  margin: 0 auto;
  gap: 12px;
  padding: 10px;
  color: #1e293b;
  position: relative;
}

.game-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: white;
  padding: 8px 16px;
  border-radius: 12px;
  box-shadow: 0 4px 15px rgba(0,0,0,0.05);
  flex-shrink: 0;
}

.header-left {
  display: flex;
  gap: 10px;
  align-items: center;
}

.turn-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
  font-size: 15px;
}

.opponent-name {
  color: #64748b;
  font-weight: 400;
  font-size: 0.9em;
}

.player-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.player-dot.blue { background: #3498db; box-shadow: 0 0 10px rgba(52, 152, 219, 0.5); }
.player-dot.red { background: #e74c3c; box-shadow: 0 0 10px rgba(231, 76, 60, 0.5); }
.player-dot.waiting { background: #94a3b8; }

.game-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 0;
  gap: 12px;
  position: relative;
}

.matching-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(4px);
  z-index: 100;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  border-radius: 16px;
  gap: 16px;
}

.matching-overlay p {
  font-weight: 700;
  color: #1e293b;
}

.turn-banner {
  font-size: 18px;
  font-weight: 800;
  padding: 6px 24px;
  border-radius: 20px;
  animation: breathing 2s infinite ease-in-out;
  letter-spacing: 1px;
}

.turn-banner.blue {
  color: #2563eb;
  background: rgba(37, 99, 235, 0.1);
  border: 1px solid rgba(37, 99, 235, 0.2);
}

.turn-banner.red {
  color: #dc2626;
  background: rgba(220, 38, 38, 0.1);
  border: 1px solid rgba(220, 38, 38, 0.2);
}

@keyframes breathing {
  0% { opacity: 0.6; transform: scale(0.98); }
  50% { opacity: 1; transform: scale(1.02); }
  100% { opacity: 0.6; transform: scale(0.98); }
}

.game-footer {
  text-align: center;
  padding: 8px;
  flex-shrink: 0;
}

.rule-hint {
  font-size: 13px;
  color: #64748b;
  background: #f1f5f9;
  padding: 8px 16px;
  border-radius: 20px;
  display: inline-block;
}

.win-content {
  text-align: center;
  padding: 20px 0;
}

.win-icon {
  font-size: 64px;
  margin-bottom: 20px;
}

.win-icon.blue { filter: drop-shadow(0 0 15px rgba(52, 152, 219, 0.6)); }
.win-icon.red { filter: drop-shadow(0 0 15px rgba(231, 76, 60, 0.6)); }

.win-dialog :deep(.el-dialog__header) {
  display: none;
}
</style>
