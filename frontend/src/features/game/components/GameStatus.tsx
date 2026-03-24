import styles from '../Game.module.css'

interface Props {
  statusText: string
  isMyTurn: boolean
  isOver: boolean
  isSuspended: boolean
  isSpectator: boolean
  winnerId: string | null
  playerId: string
  opponentId: string | null
  opponentOnline: boolean
}

/**
 * Status bar shown below the game header.
 *
 * Contains:
 * - Animated dot: amber + pulsing on your turn, muted when over/suspended.
 * - Status text: "Your turn", "Opponent's turn", "You won!", etc.
 * - Opponent presence indicator: shown during active play for participants.
 *
 * @testability
 * Render with props and assert:
 * - data-testid="game-status" contains the expected statusText.
 * - Presence indicator is absent when isSpectator is true or isSuspended is true.
 * - data-online="true" when opponentOnline is true.
 */
export function GameStatus({
  statusText,
  isMyTurn,
  isOver,
  isSuspended,
  isSpectator,
  winnerId,
  playerId,
  opponentId,
  opponentOnline,
}: Props) {
  return (
    <div className={styles.status}>
      <div
        className={`${styles.statusDot} ${isMyTurn ? styles.dotActive : ''} ${isOver || isSuspended ? styles.dotOver : ''}`}
      />
      <span
        data-testid='game-status'
        className={`${styles.statusText} ${winnerId === playerId ? styles.win : ''} ${winnerId && winnerId !== playerId ? styles.lose : ''} ${isSuspended ? styles.suspended : ''}`}
      >
        {statusText}
      </span>
      {!isSpectator && !isSuspended && opponentId && (
        <span className={styles.opponentPresence} data-testid='opponent-presence'>
          <span
            className={styles.presenceDot}
            data-online={String(opponentOnline)}
            data-testid='opponent-presence-dot'
          />
          <span data-testid='opponent-presence-text'>
            {opponentOnline ? 'Opponent online' : 'Opponent offline'}
          </span>
        </span>
      )}
    </div>
  )
}
