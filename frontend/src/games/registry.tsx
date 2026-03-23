import { TicTacToeBoard, type TicTacToeState } from '../components/TicTacToe'
import { LoveLetter, type LoveLetterState } from '../components/loveletter/LoveLetter'

// ---------------------------------------------------------------------------
// Game renderer registry — add new games here.
// ---------------------------------------------------------------------------

export interface GameData {
  current_player_id: string
  data: unknown
}

interface RendererProps {
  gameId: string
  gameData: GameData
  localPlayerId: string
  onMove: (payload: Record<string, unknown>) => void
  disabled: boolean
  isOver: boolean
  players: { id: string; username: string }[]
}


type RendererComponent = React.FC<RendererProps>

const TicTacToeRenderer: RendererComponent = ({ gameData, localPlayerId, onMove, disabled }) => (
  <TicTacToeBoard
    state={gameData.data as TicTacToeState}
    currentPlayerId={gameData.current_player_id}
    localPlayerId={localPlayerId}
    onMove={cell => onMove({ cell })}
    disabled={disabled}
  />
)

const LoveLetterRenderer: RendererComponent = ({
  gameData,
  localPlayerId,
  onMove,
  disabled,
  isOver,
  players,
}) => (
  <LoveLetter
    state={gameData.data as LoveLetterState}
    currentPlayerId={gameData.current_player_id}
    localPlayerId={localPlayerId}
    onMove={onMove}
    disabled={disabled}
    isOver={isOver}
    players={players}
  />
)

export const GAME_RENDERERS: Record<string, RendererComponent> = {
  tictactoe: TicTacToeRenderer,
  loveletter: LoveLetterRenderer,
}
