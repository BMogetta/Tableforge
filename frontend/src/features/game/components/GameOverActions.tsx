import styles from '../Game.module.css'

interface Props {
  isSpectator: boolean
  votedRematch: boolean
  rematchVotes: number
  totalPlayers: number
  isRematchPending: boolean
  onBackToLobby: () => void
  onViewReplay: () => void
  onRematch: () => void
}

/**
 * Action bar shown at the bottom of the game panel when the session is over.
 *
 * Contains:
 * - Back to Lobby: closes the room socket and navigates to /.
 * - View Replay: navigates to the session history page.
 * - Rematch (participants only): votes for a rematch. Shows vote progress
 *   while waiting for the opponent, and disables after the local player votes.
 *
 * @testability
 * - Assert "Back to Lobby" calls onBackToLobby.
 * - Assert "View Replay" calls onViewReplay.
 * - Assert "Rematch" button calls onRematch.
 * - Assert rematch button is disabled when votedRematch is true.
 * - Assert rematch button shows vote count when rematchVotes > 0.
 * - Assert rematch button is absent when isSpectator is true.
 */
export function GameOverActions({
  isSpectator,
  votedRematch,
  rematchVotes,
  totalPlayers,
  isRematchPending,
  onBackToLobby,
  onViewReplay,
  onRematch,
}: Props) {
  return (
    <div className={styles.gameOver}>
      <button className='btn btn-ghost' onClick={onBackToLobby}>
        Back to Lobby
      </button>
      <button className='btn btn-ghost' data-testid='view-replay-btn' onClick={onViewReplay}>
        View Replay
      </button>
      {!isSpectator && (
        <button
          className='btn btn-primary'
          data-testid='rematch-btn'
          onClick={onRematch}
          disabled={votedRematch || isRematchPending}
        >
          {votedRematch
            ? `Waiting for opponent… (${rematchVotes}/${totalPlayers})`
            : rematchVotes > 0
              ? `Rematch (${rematchVotes}/${totalPlayers} voted)`
              : 'Rematch'}
        </button>
      )}
    </div>
  )
}
