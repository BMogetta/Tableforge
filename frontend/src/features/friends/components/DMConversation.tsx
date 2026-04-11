import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { dm } from '@/features/room/api'
import { useAppStore } from '@/stores/store'
import { keys } from '@/lib/queryClient'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { useBlockPlayer } from '@/hooks/useBlockPlayer'
import type { DirectMessage } from '@/lib/schema-generated.zod'
import { sfx } from '@/lib/sfx'
import styles from './DMConversation.module.css'
import { testId } from '@/utils/testId'

interface DMConversationProps {
  otherPlayerId: string
  otherUsername: string
  onBack: () => void
}

export function DMConversation({ otherPlayerId, otherUsername, onBack }: DMConversationProps) {
  const player = useAppStore(s => s.player)!
  const playerSocket = useAppStore(s => s.playerSocket)
  const toast = useToast()
  const qc = useQueryClient()
  const [text, setText] = useState('')
  const listRef = useRef<HTMLDivElement>(null)
  const { block, isBlocked, blockPending } = useBlockPlayer()
  const blocked = isBlocked(otherPlayerId)

  const { data: messages = [] } = useQuery({
    queryKey: keys.dmHistory(player.id, otherPlayerId),
    queryFn: () => dm.history(player.id, otherPlayerId),
    refetchInterval: 10_000,
  })

  const safeMessages = messages ?? []

  // Auto-scroll to bottom
  useEffect(() => {
    if (listRef.current) {
      listRef.current.scrollTop = listRef.current.scrollHeight
    }
  }, [])

  // Listen for real-time DMs
  useEffect(() => {
    if (!playerSocket) {
      return
    }
    const off = playerSocket.on(event => {
      if (event.type === 'dm_received' && event.payload.from === otherPlayerId) {
        sfx.play('chat.receive')
        qc.invalidateQueries({ queryKey: keys.dmHistory(player.id, otherPlayerId) })
        qc.invalidateQueries({ queryKey: keys.dmUnread(player.id) })
        qc.invalidateQueries({ queryKey: keys.dmConversations(player.id) })
      }
    })
    return () => off()
  }, [playerSocket, otherPlayerId, player.id, qc])

  // Mark unread as read on open
  useEffect(() => {
    let marked = false
    for (const msg of safeMessages) {
      if (msg.sender_id !== player.id && !msg.read_at) {
        dm.markRead(player.id, msg.message_id).catch(() => {})
        marked = true
      }
    }
    if (marked) {
      qc.invalidateQueries({ queryKey: keys.dmUnread(player.id) })
      qc.invalidateQueries({ queryKey: keys.dmConversations(player.id) })
    }
  }, [safeMessages, player.id, qc.invalidateQueries])

  const sendMut = useMutation({
    mutationFn: (content: string) => dm.send(player.id, otherPlayerId, content),
    onSuccess: () => setText(''),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: keys.dmHistory(player.id, otherPlayerId) })
      qc.invalidateQueries({ queryKey: keys.dmConversations(player.id) })
    },
  })

  function handleSend(e: React.FormEvent) {
    e.preventDefault()
    const trimmed = text.trim()
    if (!trimmed) {
      return
    }
    sendMut.mutate(trimmed)
  }

  return (
    <div className={styles.root}>
      <div className={styles.header}>
        <button type="button" className={styles.backBtn} {...testId('dm-back-btn')} onClick={onBack}>
          &#8592;
        </button>
        <span className={styles.username}>{otherUsername}</span>
        {!blocked && (
          <button type="button"
            className={`${styles.blockBtn}`}
            onClick={() => block({ targetId: otherPlayerId, username: otherUsername })}
            disabled={blockPending}
            title='Block player'
            {...testId('dm-block-btn')}
          >
            {blockPending ? '...' : 'Block'}
          </button>
        )}
      </div>

      <div className={styles.messages} ref={listRef}>
        {safeMessages.length === 0 ? (
          <p className={styles.empty}>No messages yet. Say hi!</p>
        ) : (
          safeMessages.map((msg: DirectMessage) => (
            <div
              key={msg.message_id}
              {...testId(`dm-message-${msg.message_id}`)}
              className={`${styles.bubble} ${msg.sender_id === player.id ? styles.mine : styles.theirs}`}
            >
              <p className={styles.text}>{msg.content}</p>
              <time className={styles.time}>{formatTime(msg.timestamp)}</time>
            </div>
          ))
        )}
      </div>

      <form className={styles.inputRow} onSubmit={handleSend}>
        <input
          className={styles.input}
          {...testId('dm-input')}
          aria-label='Direct message'
          value={text}
          onChange={e => setText(e.target.value)}
          placeholder='Type a message...'
          disabled={sendMut.isPending}
        />
        <button
          className='btn btn-primary btn-sm'
          {...testId('dm-send-btn')}
          type='submit'
          disabled={sendMut.isPending || !text.trim()}
        >
          Send
        </button>
      </form>
    </div>
  )
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
}
