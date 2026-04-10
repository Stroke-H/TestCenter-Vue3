import type { Piece, PlayerColor, PieceRank, RobotDifficulty } from '../types';

export interface RobotAction {
  type: 'reveal' | 'move';
  row?: number;
  col?: number;
  fromR?: number;
  fromC?: number;
  toR?: number;
  toC?: number;
}

export function useRobotAI() {
  const ROWS = 4;
  const COLS = 8;

  const pieceValues: Record<PieceRank, number> = {
    8: 100, 7: 90, 6: 80, 5: 60, 4: 50, 3: 40, 2: 30, 1: 70
  };

  function canCapture(attacker: Piece, defender: Piece): boolean {
    if (attacker.rank === 1 && defender.rank === 8) return true;
    if (attacker.rank === 8 && defender.rank === 1) return false;
    return attacker.rank >= defender.rank;
  }

  function getLegalActions(board: (Piece | null)[][], playerColor: PlayerColor): RobotAction[] {
    const actions: RobotAction[] = [];
    for (let r = 0; r < ROWS; r++) {
      for (let c = 0; c < COLS; c++) {
        const row = board[r];
        if (!row) continue;
        const piece = row[c];
        if (piece && !piece.isRevealed) {
          actions.push({ type: 'reveal', row: r, col: c });
        }
        if (piece && piece.isRevealed && piece.color === playerColor) {
          const dirs = [{ dr: -1, dc: 0 }, { dr: 1, dc: 0 }, { dr: 0, dc: -1 }, { dr: 0, dc: 1 }];
          for (const { dr, dc } of dirs) {
            const nr = r + dr;
            const nc = c + dc;
            if (nr >= 0 && nr < ROWS && nc >= 0 && nc < COLS) {
              const targetRow = board[nr];
              if (!targetRow) continue;
              const target = targetRow[nc];
              if (!target) {
                actions.push({ type: 'move', fromR: r, fromC: c, toR: nr, toC: nc });
              } else if (target && target.isRevealed && target.color !== playerColor && canCapture(piece, target)) {
                actions.push({ type: 'move', fromR: r, fromC: c, toR: nr, toC: nc });
              }
            }
          }
        }
      }
    }
    return actions;
  }

  function generateAction(
    board: (Piece | null)[][], 
    playerColor: PlayerColor, 
    difficulty: RobotDifficulty
  ): RobotAction | null {
    const actions = getLegalActions(board, playerColor);
    if (actions.length === 0) return null;

    if (difficulty === 'easy') {
      return actions[Math.floor(Math.random() * actions.length)] || null;
    }

    const captureActions = actions.filter(a => {
      if (a.type !== 'move') return false;
      if (a.toR === undefined || a.toC === undefined) return false;
      const targetRow = board[a.toR];
      if (!targetRow) return false;
      const target = targetRow[a.toC];
      return target && target.isRevealed && target.color !== playerColor;
    });

    if (captureActions.length > 0) {
      return captureActions.reduce((prev, curr) => {
        if (prev.toR === undefined || prev.toC === undefined) return curr;
        if (curr.toR === undefined || curr.toC === undefined) return prev;
        const rowPrev = board[prev.toR];
        const rowCurr = board[curr.toR];
        if (!rowPrev || !rowCurr) return prev;
        const targetPrev = rowPrev[prev.toC];
        const targetCurr = rowCurr[curr.toC];
        if (!targetPrev || !targetCurr) return prev;
        return (pieceValues[targetCurr.rank] || 0) > (pieceValues[targetPrev.rank] || 0) ? curr : prev;
      });
    }

    const safeActions = actions.filter(a => {
      if (a.type === 'reveal') return true;
      if (a.fromR === undefined || a.fromC === undefined || a.toR === undefined || a.toC === undefined) return false;
      const fromRow = board[a.fromR];
      if (!fromRow) return false;
      const myPiece = fromRow[a.fromC];
      if (!myPiece) return false;
      const dirs = [{ dr: -1, dc: 0 }, { dr: 1, dc: 0 }, { dr: 0, dc: -1 }, { dr: 0, dc: 1 }];
      for (const d of dirs) {
        const er = a.toR + d.dr;
        const ec = a.toC + d.dc;
        if (er >= 0 && er < ROWS && ec >= 0 && ec < COLS) {
          const eRow = board[er];
          if (!eRow) continue;
          const enemy = eRow[ec];
          if (enemy && enemy.isRevealed && enemy.color !== playerColor && canCapture(enemy, myPiece)) return false;
        }
      }
      return true;
    });

    const pool = safeActions.length > 0 ? safeActions : actions;
    return pool[Math.floor(Math.random() * pool.length)] || null;
  }

  return { generateAction };
}
