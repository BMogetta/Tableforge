import type { GameAsset } from '@/lib/assets'
import { tictactoeAssets } from './tictactoe/assets'
import { loveLetterAssets } from './loveletter/assets'

const GAME_ASSETS: Record<string, GameAsset[]> = {
  tictactoe: tictactoeAssets,
  loveletter: loveLetterAssets,
}

export function getGameAssets(gameId: string): GameAsset[] {
  return GAME_ASSETS[gameId] ?? []
}
