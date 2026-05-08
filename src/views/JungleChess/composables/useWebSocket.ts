import { ref, onUnmounted } from 'vue';
import { buildBackendWsUrl } from '@/utils/runtimeUrl';

export interface JungleMessage {
  type: 'match_start' | 'sync_board' | 'action' | 'error' | 'disconnect';
  payload: any;
}

export function useWebSocket() {
  const socket = ref<WebSocket | null>(null);
  const isMatching = ref(false);
  const isOnline = ref(false);
  const myRole = ref<'host' | 'guest' | null>(null);
  const opponent = ref<{ id: string; nickname: string; username: string } | null>(null);

  // Callbacks for the game logic to hook into
  let onMatchStart: (role: 'host' | 'guest', opp: { id: string; nickname: string; username: string }) => void;
  let onSyncBoard: (board: any) => void;
  let onAction: (action: any) => void;
  let onDisconnect: (reason: string) => void;

  function connect() {
    if (socket.value) return;

    isMatching.value = true;
    const url = buildBackendWsUrl('/api/ws/jungle');

    socket.value = new WebSocket(url);

    socket.value.onmessage = (event) => {
      // 核心防御：只处理当前活跃 Socket 的消息，丢弃旧连接的延迟消息
      if (event.target !== socket.value) return;

      const msg: JungleMessage = JSON.parse(event.data);
      console.log('[JungleWS] Received:', msg);

      switch (msg.type) {
        case 'match_start':
          // 校验数据完整性，防止空对手信息导致异常
          if (!msg.payload || !msg.payload.opponent) {
            console.warn('[JungleWS] match_start received without valid payload', msg);
            return;
          }
          isMatching.value = false;
          isOnline.value = true;
          myRole.value = msg.payload.role;
          opponent.value = msg.payload.opponent;
          if (onMatchStart) onMatchStart(msg.payload.role, msg.payload.opponent);
          break;
        case 'sync_board':
          if (onSyncBoard) onSyncBoard(msg.payload);
          break;
        case 'action':
          if (onAction) onAction(msg.payload);
          break;
        case 'disconnect':
          isOnline.value = false;
          if (onDisconnect) onDisconnect(msg.payload);
          break;
        case 'error':
          alert('联机错误: ' + msg.payload);
          disconnect();
          break;
      }
    };

    socket.value.onclose = (event) => {
      if (event.target !== socket.value) return;
      console.log('[JungleWS] Closed');
      socket.value = null;
      isMatching.value = false;
      isOnline.value = false;
    };

    socket.value.onerror = (err) => {
      if (err.target !== socket.value) return;
      console.error('[JungleWS] Error:', err);
      isMatching.value = false;
    };
  }

  function disconnect() {
    if (socket.value) {
      // 关闭前彻底清理处理器，从源头切断 ghost 回调的可能
      socket.value.onmessage = null;
      socket.value.onclose = null;
      socket.value.onerror = null;
      socket.value.close();
      socket.value = null;
    }
    isMatching.value = false;
    isOnline.value = false;
    myRole.value = null;
    opponent.value = null;
  }

  function sendMessage(type: string, payload: any) {
    if (socket.value && socket.value.readyState === WebSocket.OPEN) {
      socket.value.send(JSON.stringify({ type, payload }));
    }
  }

  onUnmounted(() => {
    disconnect();
  });

  return {
    isMatching,
    isOnline,
    myRole,
    opponent,
    connect,
    disconnect,
    sendMessage,
    onEvents: (handlers: {
      onMatchStart?: typeof onMatchStart,
      onSyncBoard?: typeof onSyncBoard,
      onAction?: typeof onAction,
      onDisconnect?: typeof onDisconnect
    }) => {
      if (handlers.onMatchStart) onMatchStart = handlers.onMatchStart;
      if (handlers.onSyncBoard) onSyncBoard = handlers.onSyncBoard;
      if (handlers.onAction) onAction = handlers.onAction;
      if (handlers.onDisconnect) onDisconnect = handlers.onDisconnect;
    }
  };
}
