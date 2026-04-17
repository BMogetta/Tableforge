import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import type {
  Notification,
  NotificationPayloadBanIssued,
  NotificationPayloadFriendRequest,
  NotificationPayloadRoomInvitation,
  NotificationType,
} from '@/lib/api'
import { testId } from '@/utils/testId'
import styles from './NotificationItem.module.css'

interface NotificationItemProps {
  notification: Notification
  onAccept?: (id: string) => void
  onDecline?: (id: string) => void
  pending?: boolean
}

function useLabels(): Record<NotificationType, string> {
  const { t } = useTranslation()
  return {
    friend_request: t('notifications.friendRequest'),
    friend_request_accepted: t('notifications.friendRequestAccepted'),
    room_invitation: t('notifications.roomInvitation'),
    ban_issued: t('notifications.banIssued'),
    achievement_unlocked: t('notifications.achievementUnlocked'),
  }
}

export function NotificationItem({
  notification: n,
  onAccept,
  onDecline,
  pending,
}: NotificationItemProps) {
  const { t } = useTranslation()
  const labels = useLabels()
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
          {formatRelative(n.created_at, t)}
        </time>
      </div>

      <p className={styles.body}>{describeNotification(n, t)}</p>

      {n.action_taken && (
        <span className={styles.actionTaken} {...testId('action-taken')}>
          {n.action_taken === 'accepted'
            ? t('notifications.accepted')
            : t('notifications.declined')}
        </span>
      )}

      {hasAction && !isExpired && (
        <div className={styles.actions}>
          <button
            type='button'
            className='btn btn-primary btn-sm'
            disabled={pending}
            onClick={() => onAccept?.(n.id)}
            {...testId('accept-btn')}
          >
            {t('notifications.accept')}
          </button>
          <button
            type='button'
            className='btn btn-ghost btn-sm'
            disabled={pending}
            onClick={() => onDecline?.(n.id)}
            {...testId('decline-btn')}
          >
            {t('notifications.decline')}
          </button>
        </div>
      )}

      {hasAction && isExpired && (
        <span className={styles.expired} {...testId('expired')}>
          {t('notifications.expired')}
        </span>
      )}
    </div>
  )
}

function describeNotification(n: Notification, t: TFunction): string {
  switch (n.type) {
    case 'friend_request': {
      const p = n.payload as NotificationPayloadFriendRequest
      return t('notifications.friendRequestMessage', { username: p.from_username })
    }
    case 'friend_request_accepted': {
      const p = n.payload as NotificationPayloadFriendRequest
      return t('notifications.friendAcceptedMessage', { username: p.from_username })
    }
    case 'room_invitation': {
      const p = n.payload as NotificationPayloadRoomInvitation
      return t('notifications.roomInvitationMessage', { username: p.from_username })
    }
    case 'ban_issued': {
      const p = n.payload as NotificationPayloadBanIssued
      const reason =
        p.reason === 'decline_threshold'
          ? t('notifications.banReasonDeclines')
          : t('notifications.banReasonModerator')
      return (
        t('notifications.banMessage', { reason }) +
        (p.expires_at ? ` Expires: ${new Date(p.expires_at).toLocaleString()}` : '')
      )
    }
    default:
      return t('notifications.defaultMessage')
  }
}

function formatRelative(iso: string, t: TFunction): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) return t('notifications.timeJustNow')
  if (mins < 60) return t('notifications.timeMinutes', { count: mins })
  const hours = Math.floor(mins / 60)
  if (hours < 24) return t('notifications.timeHours', { count: hours })
  const days = Math.floor(hours / 24)
  return t('notifications.timeDays', { count: days })
}
