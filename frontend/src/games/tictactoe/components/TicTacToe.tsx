import { useTranslation } from 'react-i18next'
import { useAppStore } from '@/stores/store'
import { testId } from '@/utils/testId'
import styles from './TicTacToe.module.css'

/** @package */
export interface TicTacToeState {
  board: string[] // 9 cells, "" = empty, "X" or "O"
  marks: Record<string, string> // playerID → "X" | "O"
  players: string[] // [player0ID, player1ID]
}

interface Props {
  state: TicTacToeState
  currentPlayerId: string
  localPlayerId: string
  onMove: (cell: number) => void
  disabled: boolean
  /** Roster, used to render the opponent label + presence dot. */
  players?: { id: string; username: string; is_bot?: boolean }[]
}

/** @package */
export function TicTacToeBoard({
  state,
  currentPlayerId,
  localPlayerId,
  onMove,
  disabled,
  players = [],
}: Props) {
  const { board, marks } = state
  const localMark = marks[localPlayerId]
  const isMyTurn = currentPlayerId === localPlayerId && !disabled
  const presenceMap = useAppStore(s => s.presenceMap)
  const { t } = useTranslation()

  const opponent = players.find(p => p.id !== localPlayerId) ?? null
  const opponentIsBot = opponent?.is_bot ?? false
  // Bots are always "there" — presence is only meaningful for humans.
  const opponentOnline = opponent && !opponentIsBot ? (presenceMap[opponent.id] ?? false) : true

  return (
    <div className={styles.stage}>
      {opponent && (
        <div className={styles.opponentStrip}>
          {!opponentIsBot && (
            <span
              className={styles.presenceDot}
              data-online={String(opponentOnline)}
              {...testId('opponent-presence-dot')}
            />
          )}
          <span className={styles.opponentName}>{opponent.username}</span>
          {opponentIsBot && <span className={styles.botBadge}>{t('room.bot')}</span>}
        </div>
      )}
      <div className={styles.board}>
      {board.map((cell, i) => {
        const isEmpty = cell === ''
        const isClickable = isEmpty && isMyTurn

        return (
          <button type="button"
            key={i}
            data-cell={i}
            className={`${styles.cell} ${!isEmpty ? styles.cellFilled : ''} ${isClickable ? styles.cellHoverable : ''}`}
            onClick={() => isClickable && onMove(i)}
            disabled={!isClickable}
            aria-label={isEmpty ? `Cell ${i}` : `Cell ${i}: ${cell}`}
          >
            {cell !== '' && (
              <span
                className={`${styles.symbol} ${cell === 'X' ? styles.symbolX : styles.symbolO}`}
              >
                {cell}
              </span>
            )}
            {isEmpty && isMyTurn && <span className={styles.hoverSymbol}>{localMark}</span>}
          </button>
        )
      })}

      {/* Grid lines rendered as absolute elements for clean visuals */}
      <div className={styles.gridLines}>
        <div className={styles.vLine} style={{ left: '33.33%' }} />
        <div className={styles.vLine} style={{ left: '66.66%' }} />
        <div className={styles.hLine} style={{ top: '33.33%' }} />
        <div className={styles.hLine} style={{ top: '66.66%' }} />
      </div>
      </div>
    </div>
  )
}
