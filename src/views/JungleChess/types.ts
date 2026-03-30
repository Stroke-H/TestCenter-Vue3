export type PieceRank = 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8;
export type PlayerColor = 'blue' | 'red';

export interface Piece {
  id: string;
  rank: PieceRank;
  name: string;
  color: PlayerColor;
  isRevealed: boolean;
  isAlive: boolean;
}

export interface Cell {
  row: number;
  col: number;
  piece: Piece | null;
}

export type GameStatus = 'waiting' | 'playing' | 'ended';

export interface GameState {
  board: (Piece | null)[][];
  currentPlayer: PlayerColor | null; // null until first reveal
  status: GameStatus;
  winner: PlayerColor | null;
  history: string[];
}
