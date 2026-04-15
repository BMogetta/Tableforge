import { useTranslation } from 'react-i18next'
import styles from '../Game.module.css'
import { testId } from '@/utils/testId'

interface Props {
  isSpectator: boolean
  isRanked: boolean
  votedRematch: boolean
  rematchVotes: number
  totalPlayers: number
  isRematchPending: boolean
  isBackToQueuePending?: boolean
  onBackToLobby: () => void
  onViewReplay: () => void
  onRematch: () => void
  onBackToQueue?: () => void
}

/**
 * Action bar shown at the bottom of the game panel when the session is over.
 *
 * Contains:
 * - Back to Lobby: closes the room socket and navigates to /.
 * - View Replay: navigates to the session history page.
 * - Rematch (casual participants only): votes for a rematch. Ranked players
 *   never see this — rematch in ranked would bypass MMR-based seeding.
 * - Back to Queue (ranked participants only): re-enqueues the player for
 *   matchmaking, so a fresh MMR-matched opponent is found.
 */
export function GameOverActions({
  isSpectator,
  isRanked,
  votedRematch,
  rematchVotes,
  totalPlayers,
  isRematchPending,
  isBackToQueuePending = false,
  onBackToLobby,
  onViewReplay,
  onRematch,
  onBackToQueue,
}: Props) {
  const { t } = useTranslation()
  return (
    <div className={styles.gameOver}>
      <button type="button" className='btn btn-ghost' {...testId('back-to-lobby-btn')} onClick={onBackToLobby}>
        {t('game.backToLobby')}
      </button>
      <button type="button" className='btn btn-ghost' {...testId('view-replay-btn')} onClick={onViewReplay}>
        {t('game.viewReplay')}
      </button>
      {!isSpectator && !isRanked && (
        <button type="button"
          className='btn btn-primary'
          {...testId('rematch-btn')}
          onClick={onRematch}
          disabled={votedRematch || isRematchPending}
        >
          {votedRematch
            ? t('game.rematchWaiting', { current: rematchVotes, total: totalPlayers })
            : rematchVotes > 0
              ? t('game.rematchVotes', { current: rematchVotes, total: totalPlayers })
              : t('game.rematch')}
        </button>
      )}
      {!isSpectator && isRanked && onBackToQueue && (
        <button type="button"
          className='btn btn-primary'
          {...testId('back-to-queue-btn')}
          onClick={onBackToQueue}
          disabled={isBackToQueuePending}
        >
          {t('game.backToQueue')}
        </button>
      )}
    </div>
  )
}
