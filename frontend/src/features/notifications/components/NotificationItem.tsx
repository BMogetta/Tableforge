import type {
  Notification,
  NotificationPayloadFriendRequest,
  NotificationPayloadRoomInvitation,
  NotificationPayloadBanIssued,
  NotificationType,
} from '@/lib/api'
import styles from './NotificationItem.module.css'
import { testId } from '@/utils/testId'

interface NotificationItemProps {
  notification: Notification
  onAccept?: (id: string) => void
  onDecline?: (id: string) => void
  pending?: boolean
}

const labels: Record<NotificationType, string> = {
  friend_request: 'Friend Request',
  friend_request_accepted: 'Friend Request Accepted',
  room_invitation: 'Room Invitation',
  ban_issued: 'Ban Notice',
}

export function NotificationItem({ notification: n, onAccept, onDecline, pending }: NotificationItemProps) {
  const isRead = !!n.read_at
  const hasAction = !n.action_taken && n.type !== 'ban_issued'
  const isExpired = n.action_expires_at ? new Date(n.action_expires_at) < new Date() : false

  return (
    <div
      className={`${styles.item} ${isRead ? styles.read : styles.unread}`}
      {...testId(`notification-${n.id}`)}
    >
      <div className={styles.header}>
        <span className={styles.type}>{labels[n.type]}</span>
        <time className={styles.time} dateTime={n.created_at}>
          {formatRelative(n.created_at)}
        </time>
      </div>

      <p className={styles.body}>{describeNotification(n)}</p>

      {n.action_taken && (
        <span className={styles.actionTaken} {...testId('action-taken')}>
          {n.action_taken === 'accepted' ? 'Accepted' : 'Declined'}
        </span>
      )}

      {hasAction && !isExpired && (
        <div className={styles.actions}>
          <button
            className='btn btn-primary btn-sm'
            disabled={pending}
            onClick={() => onAccept?.(n.id)}
            {...testId('accept-btn')}
          >
            Accept
          </button>
          <button
            className='btn btn-ghost btn-sm'
            disabled={pending}
            onClick={() => onDecline?.(n.id)}
            {...testId('decline-btn')}
          >
            Decline
          </button>
        </div>
      )}

      {hasAction && isExpired && (
        <span className={styles.expired} {...testId('expired')}>Expired</span>
      )}
    </div>
  )
}

function describeNotification(n: Notification): string {
  switch (n.type) {
    case 'friend_request': {
      const p = n.payload as NotificationPayloadFriendRequest
      return `${p.from_username} sent you a friend request.`
    }
    case 'room_invitation': {
      const p = n.payload as NotificationPayloadRoomInvitation
      return `${p.from_username} invited you to a room.`
    }
    case 'ban_issued': {
      const p = n.payload as NotificationPayloadBanIssued
      const reason = p.reason === 'decline_threshold' ? 'too many declines' : 'moderator action'
      return `You were banned for ${reason}.${p.expires_at ? ` Expires: ${new Date(p.expires_at).toLocaleString()}` : ''}`
    }
    default:
      return 'You have a new notification.'
  }
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) {
    return 'just now'
  }
  if (mins < 60) {
    return `${mins}m ago`
  }
  const hours = Math.floor(mins / 60)
  if (hours < 24) {
    return `${hours}h ago`
  }
  const days = Math.floor(hours / 24)
  return `${days}d ago`
}
