import { TurnTimer } from './TurnTimer'
import styles from '../Game.module.css'
import { testId } from '@/utils/testId'

interface Props {
  gameId: string
  moveCount: number
  turnTimeoutSecs?: number
  lastMoveAt: string
  isSuspended: boolean
  canPause: boolean
  isPausePending: boolean
  isOver: boolean
  isSpectator: boolean
  onLobby: () => void
  onPause: () => void
}

/**
 * Top bar of the game page.
 *
 * Contains:
 * - ← Lobby button: opens surrender modal mid-game, navigates directly if over/spectating.
 * - Game ID and current move count.
 * - Pause button: only shown when canPause is true.
 *
 * @testability
 * Render with props and assert:
 * - onLobby is called on ← Lobby click.
 * - onPause is called on ⏸ Pause click.
 * - Pause button is absent when canPause is false.
 * - Pause button is disabled when isPausePending is true.
 */
export function GameHeader({
  gameId,
  moveCount,
  turnTimeoutSecs,
  lastMoveAt,
  isSuspended,
  canPause,
  isPausePending,
  isOver,
  onLobby,
  onPause,
}: Props) {
  return (
    <header className={styles.header}>
      <button
        className='btn btn-ghost btn-sm'
        onClick={onLobby}
      >
        ← Lobby
      </button>
      <div className={styles.gameInfo}>
        <span className={styles.gameId}>{gameId}</span>
        <span className={styles.moveCount}>Move {moveCount}</span>
        <TurnTimer turnTimeoutSecs={turnTimeoutSecs} lastMoveAt={lastMoveAt} isOver={isOver} isSuspended={isSuspended} />
      </div>
      {canPause && (
        <button
          {...testId('pause-btn')}
          className='btn btn-ghost btn-sm'
          onClick={onPause}
          disabled={isPausePending}
        >
          ⏸ Pause
        </button>
      )}
    </header>
  )
}
