import styles from '../Game.module.css'

interface Props {
  gameId: string
  moveCount: number
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
  canPause,
  isPausePending,
  onLobby,
  onPause,
}: Props) {
  return (
    <header className={styles.header}>
      <button
        className='btn btn-ghost'
        onClick={onLobby}
        style={{ padding: '4px 10px', fontSize: 11 }}
      >
        ← Lobby
      </button>
      <div className={styles.gameInfo}>
        <span className={styles.gameId}>{gameId}</span>
        <span className={styles.moveCount}>Move {moveCount}</span>
      </div>
      {canPause && (
        <button
          data-testid='pause-btn'
          className='btn btn-ghost'
          onClick={onPause}
          disabled={isPausePending}
          style={{ padding: '4px 10px', fontSize: 11 }}
        >
          ⏸ Pause
        </button>
      )}
    </header>
  )
}
