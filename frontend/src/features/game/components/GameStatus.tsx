import { useTranslation } from 'react-i18next'
import styles from '../Game.module.css'
import { testId } from '@/utils/testId'

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
 * - {...testId('game-status')} contains the expected statusText.
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
  const { t } = useTranslation()
  return (
    <div className={styles.status} role='status' aria-live='polite' aria-atomic='true'>
      <div
        className={`${styles.statusDot} ${isMyTurn ? styles.dotActive : ''} ${isOver || isSuspended ? styles.dotOver : ''}`}
      />
      <span
        {...testId('game-status')}
        className={`${styles.statusText} ${winnerId === playerId ? styles.win : ''} ${winnerId && winnerId !== playerId ? styles.lose : ''} ${isSuspended ? styles.suspended : ''}`}
      >
        {statusText}
      </span>
      {!isSpectator && !isSuspended && opponentId && (
        <span className={styles.opponentPresence} {...testId('opponent-presence')}>
          <span
            className={styles.presenceDot}
            data-online={String(opponentOnline)}
            {...testId('opponent-presence-dot')}
          />
          <span {...testId('opponent-presence-text')}>
            {opponentOnline ? t('game.opponentOnline') : t('game.opponentOffline')}
          </span>
        </span>
      )}
    </div>
  )
}
