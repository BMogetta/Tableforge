import { testId } from '@/utils/testId'
import styles from './FriendsPanel.module.css'

interface PendingRequestItemProps {
  requesterId: string
  username: string
  avatarUrl?: string
  onAccept: (requesterId: string) => void
  onDecline: (requesterId: string) => void
  pending: boolean
}

export function PendingRequestItem({
  requesterId,
  username,
  avatarUrl,
  onAccept,
  onDecline,
  pending,
}: PendingRequestItemProps) {
  return (
    <div className={styles.friendRow} {...testId(`pending-${requesterId}`)}>
      {avatarUrl && <img src={avatarUrl} alt='' className={styles.avatar} />}
      <span className={styles.friendName}>{username}</span>
      <div className={styles.friendActions}>
        <button
          type='button'
          className='btn btn-primary btn-sm'
          onClick={() => onAccept(requesterId)}
          disabled={pending}
          {...testId('accept-btn')}
        >
          Accept
        </button>
        <button
          type='button'
          className='btn btn-ghost btn-sm'
          onClick={() => onDecline(requesterId)}
          disabled={pending}
          {...testId('decline-btn')}
        >
          Decline
        </button>
      </div>
    </div>
  )
}
