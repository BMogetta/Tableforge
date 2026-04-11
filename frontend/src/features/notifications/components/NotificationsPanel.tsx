import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { notifications } from '@/features/notifications/api'
import { useFocusTrap } from '@/hooks/useFocusTrap'
import type { Notification } from '@/lib/api'
import { keys } from '@/lib/queryClient'
import { useAppStore } from '@/stores/store'
import { ModalOverlay } from '@/ui/ModalOverlay'
import { useToast } from '@/ui/Toast'
import { catchToAppError } from '@/utils/errors'
import { testId } from '@/utils/testId'
import { NotificationItem } from './NotificationItem'
import styles from './NotificationsPanel.module.css'

interface NotificationsPanelProps {
  items: Notification[]
  onClose: () => void
}

export function NotificationsPanel({ items, onClose }: NotificationsPanelProps) {
  const { t } = useTranslation()
  const trapRef = useFocusTrap<HTMLDivElement>()
  const player = useAppStore(s => s.player)!
  const queryClient = useQueryClient()
  const toast = useToast()
  const [pendingId, setPendingId] = useState<string | null>(null)

  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: keys.notifications(player.id) })

  const acceptMut = useMutation({
    mutationFn: (id: string) => notifications.accept(player.id, id),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => {
      invalidate()
      setPendingId(null)
    },
  })

  const declineMut = useMutation({
    mutationFn: (id: string) => notifications.decline(player.id, id),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => {
      invalidate()
      setPendingId(null)
    },
  })

  // Mark all unread as read when panel opens — but don't invalidate yet.
  // Invalidation happens on close so items remain visible for actions.
  const safeItems = items ?? []
  const markedRef = useRef(false)
  useEffect(() => {
    const unread = safeItems.filter(n => !n.read_at)
    if (unread.length === 0 || markedRef.current) return
    markedRef.current = true
    for (const n of unread) {
      notifications.markRead(player.id, n.id).catch(() => {})
    }
  }, [safeItems, player.id])

  function handleAccept(id: string) {
    setPendingId(id)
    acceptMut.mutate(id)
  }

  function handleDecline(id: string) {
    setPendingId(id)
    declineMut.mutate(id)
  }

  function handleClose() {
    invalidate()
    onClose()
  }

  return (
    <ModalOverlay onClose={handleClose} className={styles.overlay}>
      <div
        ref={trapRef}
        className={styles.panel}
        {...testId('notifications-panel')}
        role='dialog'
        aria-modal='true'
        aria-labelledby='notifications-title'
      >
        <div className={styles.header}>
          <h2 className={styles.title} id='notifications-title'>
            {t('notifications.title')}
          </h2>
          <button type="button" className={styles.closeBtn} onClick={handleClose}>
            x
          </button>
        </div>

        <div className={styles.list}>
          {safeItems.length === 0 ? (
            <p className={styles.empty}>{t('notifications.empty')}</p>
          ) : (
            safeItems.map(n => (
              <NotificationItem
                key={n.id}
                notification={n}
                onAccept={handleAccept}
                onDecline={handleDecline}
                pending={pendingId === n.id}
              />
            ))
          )}
        </div>
      </div>
    </ModalOverlay>
  )
}
