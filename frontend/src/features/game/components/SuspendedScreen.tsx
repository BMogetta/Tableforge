import styles from '../Game.module.css'

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
 * - Assert data-testid="suspended-screen" is present.
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
  return (
    <div className={styles.suspendedScreen} data-testid='suspended-screen'>
      <span className={styles.suspendedIcon}>⏸</span>
      <p className={styles.suspendedTitle}>Game Paused</p>
      <p className={styles.suspendedBody}>All players must vote to resume the game.</p>
      {resumeVotes.length > 0 && (
        <p className={styles.voteCount} data-testid='resume-vote-count'>
          {resumeVotes.length} / {resumeRequired} voted to resume
        </p>
      )}
      {canResume && (
        <button
          data-testid='vote-resume-btn'
          className='btn btn-primary'
          onClick={onResume}
          disabled={isPending}
        >
          Vote to Resume
        </button>
      )}
      {votedResume && <p className={styles.voteWaiting}>Waiting for opponent…</p>}
      <button className='btn btn-ghost' onClick={onBackToLobby} style={{ marginTop: 8 }}>
        ← Back to Lobby
      </button>
    </div>
  )
}
