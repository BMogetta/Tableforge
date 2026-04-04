import type { GameAsset } from '@/lib/assets'
import { tictactoeAssets } from './tictactoe/assets'
import { rootAccessAssets } from './rootaccess/assets'

const GAME_ASSETS: Record<string, GameAsset[]> = {
  tictactoe: tictactoeAssets,
  rootaccess: rootAccessAssets,
}

export function getGameAssets(gameId: string): GameAsset[] {
  return GAME_ASSETS[gameId] ?? []
}
