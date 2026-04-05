import { useTranslation } from 'react-i18next'
import styles from '../Game.module.css'
import { testId } from '@/utils/testId'

interface Props {
  resumeVotes: string[]
  resumeRequired: number
  votedResume: boolean
  isPending: boolean
  canResume: boolean
  onResume: () => void
  onBackToLobby: () => void
}

/**
 * Full-panel screen shown in place of the game board when the session
 * is suspended (all players voted to pause).
 *
 * Shows:
 * - Pause icon and title.
 * - Resume vote progress (only when at least one player has voted to resume).
 * - "Vote to Resume" button (only for participants who haven't voted yet).
 * - "Waiting for opponent…" text once the local player has voted.
 * - ← Back to Lobby button to abandon the paused game.
 *
 * @testability
 * - Assert {...testId('suspended-screen')} is present.
 * - Assert resume vote count is shown when resumeVotes.length > 0.
 * - Assert "Vote to Resume" button calls onResume.
 * - Assert button is absent when canResume is false.
 * - Assert "Waiting for opponent…" is shown when votedResume is true.
 * - Assert onBackToLobby is called on ← Back to Lobby click.
 */
export function SuspendedScreen({
  resumeVotes,
  resumeRequired,
  votedResume,
  isPending,
  canResume,
  onResume,
  onBackToLobby,
}: Props) {
  const { t } = useTranslation()
  return (
    <div className={styles.suspendedScreen} {...testId('suspended-screen')}>
      <span className={styles.suspendedIcon}>⏸</span>
      <p className={styles.suspendedTitle}>{t('game.gamePaused')}</p>
      <p className={styles.suspendedBody}>{t('game.gamePausedDesc')}</p>
      {resumeVotes.length > 0 && (
        <p className={styles.voteCount} {...testId('resume-vote-count')}>
          {t('game.resumeVotes', { current: resumeVotes.length, total: resumeRequired })}
        </p>
      )}
      {canResume && (
        <button type="button"
          {...testId('vote-resume-btn')}
          className='btn btn-primary'
          onClick={onResume}
          disabled={isPending}
        >
          {t('game.voteResume')}
        </button>
      )}
      {votedResume && <p className={styles.voteWaiting}>{t('game.waitingForOpponent')}</p>}
      <button type="button" className='btn btn-ghost' onClick={onBackToLobby} style={{ marginTop: 8 }}>
        {t('game.backToLobby')}
      </button>
    </div>
  )
}
