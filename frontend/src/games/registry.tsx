import { type CardName, RootAccess, RootAccessRules, type RootAccessState } from './rootaccess'
import { TicTacToeBoard, TicTacToeRules, type TicTacToeState } from './tictactoe'

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
  players: { id: string; username: string; is_bot?: boolean }[]
}

type RendererComponent = React.FC<RendererProps>

const TicTacToeRenderer: RendererComponent = ({
  gameData,
  localPlayerId,
  onMove,
  disabled,
  players,
}) => (
  <TicTacToeBoard
    state={gameData.data as TicTacToeState}
    currentPlayerId={gameData.current_player_id}
    localPlayerId={localPlayerId}
    onMove={cell => onMove({ cell })}
    disabled={disabled}
    players={players}
  />
)

const RootAccessRenderer: RendererComponent = ({
  gameData,
  localPlayerId,
  onMove,
  disabled,
  isOver,
  players,
}) => (
  <RootAccess
    state={gameData.data as RootAccessState}
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
  rootaccess: RootAccessRenderer,
}

// ---------------------------------------------------------------------------
// Game rules registry — used by RulesModal.
// ---------------------------------------------------------------------------

interface RulesEntry {
  id: string
  label: string
  component: React.FC<{ handCards?: CardName[] }>
}

export const GAME_RULES: RulesEntry[] = [
  { id: 'tictactoe', label: 'TicTacToe', component: TicTacToeRules },
  { id: 'rootaccess', label: 'Root Access', component: RootAccessRules },
]
