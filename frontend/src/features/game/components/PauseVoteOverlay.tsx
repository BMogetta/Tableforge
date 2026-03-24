import styles from '../Game.module.css'

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
  return (
    <div className={styles.voteOverlay} data-testid='pause-vote-overlay'>
      <p className={styles.voteTitle}>Pause requested</p>
      <p className={styles.voteCount}>
        {votes.length} / {required} voted
      </p>
      {!isSpectator && !votedPause && (
        <button
          data-testid='vote-pause-btn'
          className='btn btn-ghost'
          onClick={onVote}
          disabled={isPending}
        >
          Vote to Pause
        </button>
      )}
      {votedPause && <p className={styles.voteWaiting}>Waiting for opponent…</p>}
    </div>
  )
}
