import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { rooms, type RoomMessage } from '../../api'
import { useAppStore } from '../../store'
import { keys } from '../../queryClient'
import styles from './ChatSidebar.module.css'

interface Props {
  roomId: string
  open: boolean
  onToggle: () => void
}

export default function ChatSidebar({ roomId, open, onToggle }: Props) {
  const player = useAppStore((s) => s.player)!
  const socket = useAppStore((s) => s.socket)
  const qc = useQueryClient()

  const [draft, setDraft] = useState('')
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Initial load + periodic resync every 30s
  const { data: messages = [] } = useQuery({
    queryKey: keys.roomMessages(roomId),
    queryFn: () => rooms.messages(roomId),
    refetchInterval: 30_000,
  })

  // Listen for chat_message WS events and append optimistically to cache
  useEffect(() => {
    if (!socket) return
    const off = socket.on((event) => {
      if (event.type === 'chat_message') {
        const msg = event.payload
        qc.setQueryData<RoomMessage[]>(keys.roomMessages(roomId), (prev = []) => {
          // Deduplicate by id in case the HTTP resync already added it
          if (prev.some((m) => m.message_id === msg.message_id)) return prev
          return [...prev, msg]
        })
      }
      if (event.type === 'chat_message_hidden') {
        const { message_id } = event.payload
        qc.setQueryData<RoomMessage[]>(keys.roomMessages(roomId), (prev = []) =>
          prev.map((m) => m.message_id === message_id ? { ...m, hidden: true } : m)
        )
      }
    })
    return () => off()
  }, [socket, roomId, qc])

  // Scroll to bottom when messages change
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages])

  const sendMessage = useMutation({
    mutationFn: (content: string) => rooms.sendMessage(roomId, player.id, content),
    onSuccess: (newMsg: RoomMessage) => {
      setDraft('')
      inputRef.current?.focus()
      },
  })

  function handleSend() {
    const content = draft.trim()
    if (!content || sendMessage.isPending) return
    sendMessage.mutate(content)
  }

  const visibleMessages = messages.filter((m) => !m.hidden)

  return (
    <aside className={`${styles.sidebar} ${open ? styles.open : styles.closed}`}>
      <button className={styles.toggle} onClick={onToggle} title={open ? 'Hide chat' : 'Show chat'}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
          <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
        </svg>
        {!open && <span className={styles.toggleLabel}>Chat</span>}
      </button>

      {open && (
        <>
          <div className={styles.header}>
            <span className={styles.title}>Room Chat</span>
            <span className={styles.count}>{visibleMessages.length}</span>
          </div>

          <div className={styles.messages}>
            {visibleMessages.length === 0 ? (
              <p className={styles.empty}>No messages yet.</p>
            ) : (
              visibleMessages.map((msg) => (
                <ChatMessage key={msg.message_id} msg={msg} isSelf={msg.player_id === player.id} />
              ))
            )}
            <div ref={bottomRef} />
          </div>

          <div className={styles.inputRow}>
            <input
              ref={inputRef}
              className={`input ${styles.input}`}
              placeholder="Message..."
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSend()}
              maxLength={500}
            />
            <button
              className={styles.sendBtn}
              onClick={handleSend}
              disabled={!draft.trim() || sendMessage.isPending}
              title="Send"
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <line x1="22" y1="2" x2="11" y2="13" />
                <polygon points="22 2 15 22 11 13 2 9 22 2" />
              </svg>
            </button>
          </div>
        </>
      )}
    </aside>
  )
}

// --- ChatMessage -------------------------------------------------------------

interface ChatMessageProps {
  msg: RoomMessage
  isSelf: boolean
}

function ChatMessage({ msg, isSelf }: ChatMessageProps) {
  const time = msg.timestamp
    ? new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : ''

  return (
    <div className={`${styles.message} ${isSelf ? styles.self : ''}`}>
      <div className={styles.messageMeta}>
        <span className={styles.messageTime}>{time}</span>
      </div>
      <div className={styles.messageBubble}>{msg.content}</div>
    </div>
  )
}