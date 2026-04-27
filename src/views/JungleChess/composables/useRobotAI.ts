import type { Piece, PlayerColor, PieceRank, RobotDifficulty } from '../types';

type BoardState = (Piece | null)[][];

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
  const dirs = [{ dr: -1, dc: 0 }, { dr: 1, dc: 0 }, { dr: 0, dc: -1 }, { dr: 0, dc: 1 }];

  const pieceValues: Record<PieceRank, number> = {
    8: 100, 7: 90, 6: 80, 5: 60, 4: 50, 3: 40, 2: 30, 1: 70
  };

  function canCapture(attacker: Piece, defender: Piece): boolean {
    if (attacker.rank === 1 && defender.rank === 8) return true;
    if (attacker.rank === 8 && defender.rank === 1) return false;
    return attacker.rank >= defender.rank;
  }

  function getOpponentColor(playerColor: PlayerColor): PlayerColor {
    return playerColor === 'blue' ? 'red' : 'blue';
  }

  function getLegalActions(board: BoardState, playerColor: PlayerColor): RobotAction[] {
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

  function selectMediumAction(board: BoardState, playerColor: PlayerColor, actions: RobotAction[]): RobotAction | null {
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
      return !isSquareThreatened(board, a.toR, a.toC, playerColor, myPiece);
    });

    const pool = safeActions.length > 0 ? safeActions : actions;
    return pool[Math.floor(Math.random() * pool.length)] || null;
  }

  function selectHardAction(board: BoardState, playerColor: PlayerColor, actions: RobotAction[]): RobotAction | null {
    const scoredActions = actions.map((action) => ({
      action,
      score: scoreAction(board, playerColor, action)
    }));
    const bestScore = Math.max(...scoredActions.map((item) => item.score));
    const bestActions = scoredActions.filter((item) => Math.abs(item.score - bestScore) < 0.001);

    return bestActions[Math.floor(Math.random() * bestActions.length)]?.action || null;
  }

  function scoreAction(board: BoardState, playerColor: PlayerColor, action: RobotAction): number {
    const nextBoard = applyAction(board, action);
    const opponentColor = getOpponentColor(playerColor);
    const opponentActions = getLegalActions(nextBoard, opponentColor);
    const opponentBestResponse = Math.max(0, ...opponentActions.map((nextAction) => scoreImmediateAction(nextBoard, opponentColor, nextAction)));
    const immediateScore = scoreImmediateAction(board, playerColor, action);
    const positionScore = evaluateBoard(nextBoard, playerColor);
    const mobilityScore = getLegalActions(nextBoard, playerColor).length - opponentActions.length;

    return immediateScore + positionScore + mobilityScore * 2 - opponentBestResponse * 0.9;
  }

  function scoreImmediateAction(board: BoardState, playerColor: PlayerColor, action: RobotAction): number {
    if (action.type === 'reveal') {
      if (action.row === undefined || action.col === undefined) return 0;
      const piece = board[action.row]?.[action.col];
      if (!piece) return 0;

      const baseRevealScore = 10;
      const pieceValue = pieceValues[piece.rank] || 0;
      return piece.color === playerColor
        ? baseRevealScore + pieceValue * 0.25
        : baseRevealScore - pieceValue * 0.15;
    }

    if (action.fromR === undefined || action.fromC === undefined || action.toR === undefined || action.toC === undefined) {
      return 0;
    }

    const attacker = board[action.fromR]?.[action.fromC];
    const target = board[action.toR]?.[action.toC];
    if (!attacker) return 0;

    let score = 8 + getAdvanceScore(action, playerColor);
    if (target && target.isRevealed && target.color !== playerColor) {
      score += (pieceValues[target.rank] || 0) * 1.7;
      score += ((pieceValues[target.rank] || 0) - (pieceValues[attacker.rank] || 0)) * 0.25;
    }
    if (isSquareThreatened(board, action.fromR, action.fromC, playerColor, attacker)) {
      score += 22;
    }

    return score;
  }

  function evaluateBoard(board: BoardState, playerColor: PlayerColor): number {
    let score = 0;

    for (let r = 0; r < ROWS; r++) {
      for (let c = 0; c < COLS; c++) {
        const piece = board[r]?.[c];
        if (!piece || !piece.isRevealed) continue;

        const value = pieceValues[piece.rank] || 0;
        const sign = piece.color === playerColor ? 1 : -1;
        score += sign * value * 0.35;
        if (isSquareThreatened(board, r, c, piece.color, piece)) {
          score -= sign * value * 0.3;
        }
      }
    }

    return score;
  }

  function applyAction(board: BoardState, action: RobotAction): BoardState {
    const nextBoard = cloneBoard(board);

    if (action.type === 'reveal') {
      if (action.row !== undefined && action.col !== undefined) {
        const piece = nextBoard[action.row]?.[action.col];
        if (piece) piece.isRevealed = true;
      }
      return nextBoard;
    }

    if (action.fromR === undefined || action.fromC === undefined || action.toR === undefined || action.toC === undefined) {
      return nextBoard;
    }

    const fromRow = nextBoard[action.fromR];
    const toRow = nextBoard[action.toR];
    if (!fromRow || !toRow) return nextBoard;

    const attacker = fromRow[action.fromC];
    const target = toRow[action.toC];
    if (!attacker) return nextBoard;
    if (target) target.isAlive = false;
    toRow[action.toC] = attacker;
    fromRow[action.fromC] = null;

    return nextBoard;
  }

  function cloneBoard(board: BoardState): BoardState {
    return board.map((row) => row.map((piece) => piece ? { ...piece } : null));
  }

  function isSquareThreatened(board: BoardState, row: number, col: number, ownerColor: PlayerColor, piece: Piece): boolean {
    for (const { dr, dc } of dirs) {
      const enemy = board[row + dr]?.[col + dc];
      if (enemy && enemy.isRevealed && enemy.color !== ownerColor && canCapture(enemy, piece)) {
        return true;
      }
    }
    return false;
  }

  function getAdvanceScore(action: RobotAction, playerColor: PlayerColor): number {
    if (action.fromR === undefined || action.toR === undefined) return 0;
    return playerColor === 'blue'
      ? (action.fromR - action.toR) * 2
      : (action.toR - action.fromR) * 2;
  }

  function generateAction(
    board: BoardState, 
    playerColor: PlayerColor, 
    difficulty: RobotDifficulty
  ): RobotAction | null {
    const actions = getLegalActions(board, playerColor);
    if (actions.length === 0) return null;

    if (difficulty === 'easy') {
      return actions[Math.floor(Math.random() * actions.length)] || null;
    }

    if (difficulty === 'hard') {
      return selectHardAction(board, playerColor, actions);
    }

    return selectMediumAction(board, playerColor, actions);
  }

  return { generateAction };
}
