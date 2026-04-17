import { useTranslation } from 'react-i18next'
import { ErrorMessage } from '@/ui/ErrorMessage'
import type { AppError } from '@/utils/errors'
import { testId } from '@/utils/testId'
import styles from '../Room.module.css'

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
  const { t } = useTranslation()
  return (
    <>
      <div className={styles.actions}>
        {isSpectator ? (
          <button type='button' className='btn btn-danger' onClick={onLeave}>
            {t('room.leaveRoom')}
          </button>
        ) : isOwner ? (
          <>
            <button
              type='button'
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
                  ? t('room.startGame')
                  : t('room.needMorePlayers', { count: playersNeeded })}
            </button>
            <button type='button' className='btn btn-danger' onClick={onLeave}>
              {t('room.leaveRoom')}
            </button>
          </>
        ) : (
          <>
            <p className={styles.waitingHost}>{t('room.waitingForHost')}</p>
            <button type='button' className='btn btn-danger' onClick={onLeave}>
              {t('room.leaveRoom')}
            </button>
          </>
        )}
      </div>
      <ErrorMessage error={startError} />
    </>
  )
}
