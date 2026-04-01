import styles from './FriendsPanel.module.css'

interface PendingRequestItemProps {
  requesterId: string
  username: string
  avatarUrl?: string
  onAccept: (requesterId: string) => void
  onDecline: (requesterId: string) => void
  pending: boolean
}

export function PendingRequestItem({ requesterId, username, avatarUrl, onAccept, onDecline, pending }: PendingRequestItemProps) {
  return (
    <div className={styles.friendRow} data-testid={`pending-${requesterId}`}>
      {avatarUrl && <img src={avatarUrl} alt='' className={styles.avatar} />}
      <span className={styles.friendName}>{username}</span>
      <div className={styles.friendActions}>
        <button
          className='btn btn-primary'
          style={{ padding: '3px 10px', fontSize: 11 }}
          onClick={() => onAccept(requesterId)}
          disabled={pending}
          data-testid='accept-btn'
        >
          Accept
        </button>
        <button
          className='btn btn-ghost'
          style={{ padding: '3px 10px', fontSize: 11 }}
          onClick={() => onDecline(requesterId)}
          disabled={pending}
          data-testid='decline-btn'
        >
          Decline
        </button>
      </div>
    </div>
  )
}
