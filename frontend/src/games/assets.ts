import type { GameAsset } from '@/lib/assets'
import { rootAccessAssets } from './rootaccess'
import { tictactoeAssets } from './tictactoe'

const GAME_ASSETS: Record<string, GameAsset[]> = {
  tictactoe: tictactoeAssets,
  rootaccess: rootAccessAssets,
}

export function getGameAssets(gameId: string): GameAsset[] {
  return GAME_ASSETS[gameId] ?? []
}
