import { useTranslation } from 'react-i18next'
import styles from '../Game.module.css'
import { testId } from '@/utils/testId'

interface Props {
  votes: string[]
  required: number
  votedPause: boolean
  isPending: boolean
  isSpectator: boolean
  onVote: () => void
}

/**
 * Overlay shown between the status bar and the board while a pause vote
 * is in progress (i.e. someone has voted to pause but consensus isn't reached yet).
 *
 * Only rendered when votes.length > 0 and the game is not yet suspended.
 *
 * @testability
 * - Assert vote count text "N / M voted" is rendered.
 * - Assert "Vote to Pause" button is present when !isSpectator && !votedPause.
 * - Assert "Waiting for opponent…" is shown when votedPause is true.
 * - Assert onVote is called on button click.
 * - Assert button is disabled when isPending is true.
 */
export function PauseVoteOverlay({
  votes,
  required,
  votedPause,
  isPending,
  isSpectator,
  onVote,
}: Props) {
  const { t } = useTranslation()
  return (
    <div className={styles.voteOverlay} {...testId('pause-vote-overlay')}>
      <p className={styles.voteTitle}>{t('game.pauseRequested')}</p>
      <p className={styles.voteCount}>
        {t('game.pauseVotes', { current: votes.length, total: required })}
      </p>
      {!isSpectator && !votedPause && (
        <button type="button"
          {...testId('vote-pause-btn')}
          className='btn btn-ghost'
          onClick={onVote}
          disabled={isPending}
        >
          {t('game.votePause')}
        </button>
      )}
      {votedPause && <p className={styles.voteWaiting}>{t('game.waitingForOpponent')}</p>}
    </div>
  )
}
