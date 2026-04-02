import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { dmConversations, type DMConversation as DMConversationType } from '@/features/friends/api'
import { useAppStore } from '@/stores/store'
import { keys } from '@/lib/queryClient'
import { DMConversation } from './DMConversation'
import styles from './DMInboxPanel.module.css'

interface DMInboxPanelProps {
  onClose: () => void
  initialTarget?: string | null
}

export function DMInboxPanel({ onClose, initialTarget }: DMInboxPanelProps) {
  const player = useAppStore(s => s.player)!
  const [selectedId, setSelectedId] = useState<string | null>(initialTarget ?? null)
  const [selectedUsername, setSelectedUsername] = useState('')

  const { data: conversations = [] } = useQuery({
    queryKey: keys.dmConversations(player.id),
    queryFn: () => dmConversations.list(player.id),
    refetchInterval: 15_000,
  })

  const safeConversations = conversations ?? []

  if (selectedId) {
    const name = selectedUsername || safeConversations.find(c => c.other_player_id === selectedId)?.other_username || 'Player'
    return (
      <div className={styles.overlay} onClick={e => e.target === e.currentTarget && onClose()}>
        <div className={styles.panel}>
          <DMConversation
            otherPlayerId={selectedId}
            otherUsername={name}
            onBack={() => setSelectedId(null)}
          />
        </div>
      </div>
    )
  }

  return (
    <div className={styles.overlay} onClick={e => e.target === e.currentTarget && onClose()}>
      <div className={styles.panel} data-testid='dm-inbox-panel' role='dialog' aria-modal='true' aria-labelledby='dm-inbox-title'>
        <div className={styles.header}>
          <h2 className={styles.title} id='dm-inbox-title'>Messages</h2>
          <button className={styles.closeBtn} onClick={onClose}>x</button>
        </div>

        <div className={styles.list}>
          {safeConversations.length === 0 ? (
            <p className={styles.empty}>No conversations yet.</p>
          ) : (
            safeConversations.map((conv: DMConversationType) => (
              <button
                key={conv.other_player_id}
                className={styles.convRow}
                onClick={() => {
                  setSelectedId(conv.other_player_id)
                  setSelectedUsername(conv.other_username)
                }}
              >
                {conv.other_avatar_url && (
                  <img src={conv.other_avatar_url} alt='' className={styles.avatar} />
                )}
                <div className={styles.convInfo}>
                  <span className={styles.convName}>{conv.other_username}</span>
                  <span className={styles.convPreview}>
                    {conv.last_message.length > 40
                      ? conv.last_message.slice(0, 40) + '...'
                      : conv.last_message}
                  </span>
                </div>
                <div className={styles.convMeta}>
                  <time className={styles.convTime}>{formatRelative(conv.last_message_at)}</time>
                  {conv.unread_count > 0 && (
                    <span className={styles.unreadBadge}>{conv.unread_count}</span>
                  )}
                </div>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  )
}

function formatRelative(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const mins = Math.floor(diff / 60_000)
  if (mins < 1) {
    return 'now'
  }
  if (mins < 60) {
    return `${mins}m`
  }
  const hours = Math.floor(mins / 60)
  if (hours < 24) {
    return `${hours}h`
  }
  return `${Math.floor(hours / 24)}d`
}
