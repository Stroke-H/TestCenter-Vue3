import { ref } from 'vue';
import type { Piece, PieceRank, PlayerColor, GameStatus } from '../types';

export function useGameLogic() {
  const ROWS = 4;
  const COLS = 8;

  const board = ref<(Piece | null)[][]>(Array.from({ length: ROWS }, () => Array(COLS).fill(null)));
  const currentPlayer = ref<PlayerColor | null>(null);
  const status = ref<GameStatus>('waiting');
  const winner = ref<PlayerColor | null>(null);
  const firstRevealerRole = ref<'host' | 'guest' | null>(null);

  // 棋子職级与名称映射
  const rankNames: Record<PieceRank, string> = {
    8: '象', 7: '狮', 6: '虎', 5: '豹', 4: '狼', 3: '犬', 2: '猫', 1: '鼠'
  };

  // 初始化棋盘
  function initGame(externalBoard?: (Piece | null)[][]) {
    if (externalBoard) {
      board.value = JSON.parse(JSON.stringify(externalBoard));
      currentPlayer.value = null;
      status.value = 'playing';
      winner.value = null;
      firstRevealerRole.value = null;
      return;
    }

    const pieces: Piece[] = [];
    const colors: PlayerColor[] = ['blue', 'red'];

    colors.forEach(color => {
      // 象1, 狮1, 虎1, 豹2, 狼2, 犬2, 猫2, 鼠5 (合计 16)
      const config: Record<number, number> = {
        8: 1, 7: 1, 6: 1, 5: 2, 4: 2, 3: 2, 2: 2, 1: 5
      };

      (Object.entries(config)).forEach(([rankStr, count]) => {
        const rank = Number(rankStr) as PieceRank;
        for (let i = 0; i < count; i++) {
          pieces.push({
            id: `${color}-${rank}-${i}`,
            rank: rank,
            name: rankNames[rank],
            color,
            isRevealed: false,
            isAlive: true
          });
        }
      });
    });

    // 随机洗牌
    for (let i = pieces.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      const temp = pieces[i]!;
      pieces[i] = pieces[j]!;
      pieces[j] = temp;
    }

    // 填充棋盘
    let index = 0;
    const newBoard: (Piece | null)[][] = [];
    for (let r = 0; r < ROWS; r++) {
      const row: (Piece | null)[] = [];
      for (let c = 0; c < COLS; c++) {
        row.push(pieces[index++] || null);
      }
      newBoard.push(row);
    }
    board.value = newBoard;

    currentPlayer.value = null;
    status.value = 'playing';
    winner.value = null;
    firstRevealerRole.value = null;
  }

  // 翻开棋子
  function revealPiece(row: number, col: number, role?: 'host' | 'guest') {
    const boardRow = board.value[row];
    if (!boardRow) return false;
    const piece = boardRow[col];
    if (!piece || piece.isRevealed) return false;

    piece.isRevealed = true;

    // 如果是第一手翻开，决定当前玩家且记录翻开者角色
    if (currentPlayer.value === null) {
      currentPlayer.value = piece.color;
      if (role) firstRevealerRole.value = role;
    }

    switchTurn();
    return true;
  }

  // 切换回合
  function switchTurn() {
    if (currentPlayer.value === 'blue') currentPlayer.value = 'red';
    else if (currentPlayer.value === 'red') currentPlayer.value = 'blue';
    checkWin();
  }

  // 判定是否可以捕食
  function canCapture(attacker: Piece, defender: Piece) {
    if (attacker.color === defender.color) return false;
    
    // 特殊规则：鼠(1)吃象(8)
    if (attacker.rank === 1 && defender.rank === 8) return true;
    // 象(8)不吃鼠(1)
    if (attacker.rank === 8 && defender.rank === 1) return false;
    
    // 通用规则：大吃小 或 同级互吃
    return attacker.rank >= defender.rank;
  }

  // 移动/捕食
  function movePiece(fromR: number, fromC: number, toR: number, toC: number) {
    const fromRow = board.value[fromR];
    const toRow = board.value[toR];
    if (!fromRow || !toRow) return false;

    const attacker = fromRow[fromC];
    const target = toRow[toC];

    if (!attacker || !attacker.isRevealed || attacker.color !== currentPlayer.value) return false;

    // 检查是否是相邻格（上下左右）
    const dist = Math.abs(fromR - toR) + Math.abs(fromC - toC);
    if (dist !== 1) return false;

    if (!target) {
      // 移动到空地
      toRow[toC] = attacker;
      fromRow[fromC] = null;
    } else {
      // 尝试捕食
      if (!target.isRevealed) return false; // 不能捕食未翻开的
      if (canCapture(attacker, target)) {
        target.isAlive = false;
        toRow[toC] = attacker;
        fromRow[fromC] = null;
      } else {
        return false;
      }
    }

    switchTurn();
    return true;
  }

  // 胜负判定
  function checkWin() {
    const pieces = board.value.flat().filter(p => p !== null) as Piece[];
    const blueAlive = pieces.some(p => p.color === 'blue' && p.isAlive);
    const redAlive = pieces.some(p => p.color === 'red' && p.isAlive);

    if (!blueAlive) {
      status.value = 'ended';
      winner.value = 'red';
    } else if (!redAlive) {
      status.value = 'ended';
      winner.value = 'blue';
    }
  }

  return {
    board,
    currentPlayer,
    status,
    winner,
    firstRevealerRole,
    initGame,
    revealPiece,
    movePiece,
    canCapture
  };
}
