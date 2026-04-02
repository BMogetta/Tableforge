import type { AppError } from '@/utils/errors'
import { ErrorMessage } from '@/ui/ErrorMessage'
import styles from '../Room.module.css'
import { testId } from '@/utils/testId'

interface RoomActionsProps {
  isSpectator: boolean
  isOwner: boolean
  canStart: boolean
  starting: boolean
  startError: AppError | null
  playersNeeded: number
  onStart: () => void
  onLeave: () => void
}

export function RoomActions({
  isSpectator,
  isOwner,
  canStart,
  starting,
  startError,
  playersNeeded,
  onStart,
  onLeave,
}: RoomActionsProps) {
  return (
    <>
      <div className={styles.actions}>
        {isSpectator ? (
          <button className='btn btn-danger' onClick={onLeave}>
            Leave
          </button>
        ) : isOwner ? (
          <>
            <button
              {...testId('start-game-btn')}
              data-can-start={canStart}
              className='btn btn-primary'
              onClick={onStart}
              disabled={!canStart || starting}
              style={{ flex: 1 }}
            >
              {starting
                ? 'Starting...'
                : canStart
                  ? 'Start Game'
                  : `Need ${playersNeeded} more player(s)`}
            </button>
            <button className='btn btn-danger' onClick={onLeave}>
              Leave
            </button>
          </>
        ) : (
          <>
            <p className={styles.waitingHost}>Waiting for host to start the game...</p>
            <button className='btn btn-danger' onClick={onLeave}>
              Leave
            </button>
          </>
        )}
      </div>
      <ErrorMessage error={startError} />
    </>
  )
}
