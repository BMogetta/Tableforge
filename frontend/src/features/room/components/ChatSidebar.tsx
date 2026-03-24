import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { rooms, mutes, type RoomMessage, type RoomViewPlayer } from '@/lib/api'
import { useAppStore } from '@/stores/store'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { keys } from '@/lib/queryClient'
import styles from './ChatSidebar.module.css'

interface Props {
  roomId: string
  open: boolean
  onToggle: () => void
  // Local mute state — owned by Room.tsx so the player list dropdown can
  // also trigger mutes without going through the chat input.
  mutedIds: Set<string>
  muteAll: boolean
  onMute: (playerId: string) => void
  onUnmute: (playerId: string) => void
  onMuteAll: () => void
  onUnmuteAll: () => void
  // roomPlayers is needed to resolve usernames for slash commands.
  roomPlayers: RoomViewPlayer[]
}

// System message — displayed inline in chat, never sent to the server.
interface SystemMessage {
  id: string
  text: string
}

let sysId = 0
function nextSysId() {
  return `sys-${++sysId}`
}

// ---------------------------------------------------------------------------
// Slash command help text
// ---------------------------------------------------------------------------

const HELP_TEXT = [
  '/mute <username>     — hide messages from a player (this session only)',
  '/unmute <username>   — show messages from a muted player again',
  '/muteall             — hide messages from everyone except yourself',
  '/unmuteall           — restore all locally muted players',
  '/block <username>    — permanently block a player (persists across sessions)',
  '/unblock <username>  — remove a permanent block',
  '/help                — show this help',
].join('\n')

export function ChatSidebar({
  roomId,
  open,
  onToggle,
  mutedIds,
  muteAll,
  onMute,
  onUnmute,
  onMuteAll,
  onUnmuteAll,
  roomPlayers,
}: Props) {
  const player = useAppStore(s => s.player)!
  const socket = useAppStore(s => s.socket)
  const qc = useQueryClient()
  const toast = useToast()

  const [draft, setDraft] = useState('')
  const [systemMessages, setSystemMessages] = useState<SystemMessage[]>([])
  const bottomRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Fetch permanently blocked player IDs from the backend once on mount.
  const { data: blockedList = [] } = useQuery({
    queryKey: keys.mutes(player.id),
    queryFn: () => mutes.list(player.id, player.id),
    staleTime: 60_000,
  })
  const blockedIds = new Set(blockedList.map(m => m.muted_id))

  const blockMutation = useMutation({
    mutationFn: (targetId: string) => mutes.mute(player.id, targetId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: keys.mutes(player.id) })
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  const unblockMutation = useMutation({
    mutationFn: (targetId: string) => mutes.unmute(player.id, targetId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: keys.mutes(player.id) })
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  // Initial load + periodic resync every 30s.
  const { data: messages = [] } = useQuery({
    queryKey: keys.roomMessages(roomId),
    queryFn: () => rooms.messages(roomId),
    refetchInterval: 30_000,
  })

  // Listen for chat_message and chat_message_hidden WS events.
  useEffect(() => {
    if (!socket) return
    const off = socket.on(event => {
      if (event.type === 'chat_message') {
        const msg = event.payload
        qc.setQueryData<RoomMessage[]>(keys.roomMessages(roomId), (prev = []) => {
          // Deduplicate by id in case the HTTP resync already added it.
          if (prev.some(m => m.message_id === msg.message_id)) return prev
          return [...prev, msg]
        })
      }
      if (event.type === 'chat_message_hidden') {
        const { message_id } = event.payload
        qc.setQueryData<RoomMessage[]>(keys.roomMessages(roomId), (prev = []) =>
          prev.map(m => (m.message_id === message_id ? { ...m, hidden: true } : m)),
        )
      }
    })
    return () => off()
  }, [socket, roomId, qc])

  // Scroll to bottom when messages or system messages change.
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, systemMessages])

  const sendMessage = useMutation({
    mutationFn: (content: string) => rooms.sendMessage(roomId, player.id, content),
    onSuccess: () => {
      setDraft('')
      inputRef.current?.focus()
    },
  })

  // ---------------------------------------------------------------------------
  // Slash command parser
  // ---------------------------------------------------------------------------

  function addSystemMessage(text: string) {
    setSystemMessages(prev => [...prev, { id: nextSysId(), text }])
  }

  function resolvePlayer(username: string): RoomViewPlayer | undefined {
    return roomPlayers.find(p => p.username.toLowerCase() === username.toLowerCase())
  }

  function handleCommand(raw: string): boolean {
    const trimmed = raw.trim()
    if (!trimmed.startsWith('/')) return false

    const [cmd, ...args] = trimmed.slice(1).split(/\s+/)
    const username = args[0] ?? ''

    switch (cmd.toLowerCase()) {
      case 'help':
        addSystemMessage(HELP_TEXT)
        return true

      case 'muteall':
        onMuteAll()
        addSystemMessage('[System] All players muted for this session.')
        return true

      case 'unmuteall':
        onUnmuteAll()
        addSystemMessage('[System] All local mutes cleared.')
        return true

      case 'mute': {
        if (!username) {
          addSystemMessage('[Error] Usage: /mute <username>')
          return true
        }
        const target = resolvePlayer(username)
        if (!target) {
          addSystemMessage(`[Error] Player "${username}" not found in this room.`)
          return true
        }
        if (target.id === player.id) {
          addSystemMessage('[Error] You cannot mute yourself.')
          return true
        }
        onMute(target.id)
        addSystemMessage(`[System] Muted ${target.username} for this session.`)
        return true
      }

      case 'unmute': {
        if (!username) {
          addSystemMessage('[Error] Usage: /unmute <username>')
          return true
        }
        const target = resolvePlayer(username)
        if (!target) {
          addSystemMessage(`[Error] Player "${username}" not found in this room.`)
          return true
        }
        onUnmute(target.id)
        addSystemMessage(`[System] Unmuted ${target.username}.`)
        return true
      }

      case 'block': {
        if (!username) {
          addSystemMessage('[Error] Usage: /block <username>')
          return true
        }
        const target = resolvePlayer(username)
        if (!target) {
          addSystemMessage(`[Error] Player "${username}" not found in this room.`)
          return true
        }
        if (target.id === player.id) {
          addSystemMessage('[Error] You cannot block yourself.')
          return true
        }
        blockMutation.mutate(target.id)
        addSystemMessage(`[System] Blocked ${target.username}.`)
        return true
      }

      case 'unblock': {
        if (!username) {
          addSystemMessage('[Error] Usage: /unblock <username>')
          return true
        }
        const target = resolvePlayer(username)
        if (!target) {
          addSystemMessage(`[Error] Player "${username}" not found in this room.`)
          return true
        }
        unblockMutation.mutate(target.id)
        addSystemMessage(`[System] Unblocked ${target.username}.`)
        return true
      }

      default:
        addSystemMessage(`[Error] Unknown command "/${cmd}". Type /help for available commands.`)
        return true
    }
  }

  function handleSend() {
    const content = draft.trim()
    if (!content || sendMessage.isPending) return

    if (handleCommand(content)) {
      setDraft('')
      return
    }

    sendMessage.mutate(content)
  }

  // ---------------------------------------------------------------------------
  // Filtering
  // ---------------------------------------------------------------------------

  const visibleMessages = messages.filter(m => {
    if (m.hidden) return false
    if (m.player_id === player.id) return true // always show own messages
    if (muteAll) return false // mute all active
    if (mutedIds.has(m.player_id)) return false // locally muted
    if (blockedIds.has(m.player_id)) return false // permanently blocked
    return true
  })

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <aside className={`${styles.sidebar} ${open ? styles.open : styles.closed}`}>
      <button className={styles.toggle} onClick={onToggle} title={open ? 'Hide chat' : 'Show chat'}>
        <svg
          width='14'
          height='14'
          viewBox='0 0 24 24'
          fill='none'
          stroke='currentColor'
          strokeWidth='1.5'
        >
          <path d='M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z' />
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
            {visibleMessages.length === 0 && systemMessages.length === 0 ? (
              <p className={styles.empty}>No messages yet. Type /help for commands.</p>
            ) : (
              <>
                {visibleMessages.map(msg => {
                  const sender = roomPlayers.find(p => p.id === msg.player_id)
                  return (
                    <ChatMessage
                      key={msg.message_id}
                      msg={msg}
                      isSelf={msg.player_id === player.id}
                      senderUsername={sender?.username}
                    />
                  )
                })}
                {systemMessages.map(sm => (
                  <SystemMessageBubble key={sm.id} text={sm.text} />
                ))}
              </>
            )}
            <div ref={bottomRef} />
          </div>

          <div className={styles.inputRow}>
            <input
              ref={inputRef}
              className={`input ${styles.input}`}
              placeholder='Message or /command...'
              value={draft}
              onChange={e => setDraft(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleSend()}
              maxLength={500}
            />
            <button
              className={styles.sendBtn}
              onClick={handleSend}
              disabled={!draft.trim() || sendMessage.isPending}
              title='Send'
            >
              <svg
                width='14'
                height='14'
                viewBox='0 0 24 24'
                fill='none'
                stroke='currentColor'
                strokeWidth='1.5'
              >
                <line x1='22' y1='2' x2='11' y2='13' />
                <polygon points='22 2 15 22 11 13 2 9 22 2' />
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
  senderUsername?: string
}

function ChatMessage({ msg, isSelf, senderUsername }: ChatMessageProps) {
  const time = msg.timestamp
    ? new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : ''

  return (
    <div className={`${styles.message} ${isSelf ? styles.self : ''}`}>
      <div className={styles.messageMeta}>
        {!isSelf && senderUsername && <span className={styles.senderName}>{senderUsername}</span>}
        <span className={styles.messageTime}>{time}</span>
      </div>
      <div className={styles.messageBubble}>{msg.content}</div>
    </div>
  )
}

// --- SystemMessageBubble -----------------------------------------------------

function SystemMessageBubble({ text }: { text: string }) {
  return (
    <div className={styles.systemMessage}>
      {text.split('\n').map((line, i) => (
        <span key={i} className={styles.systemLine}>
          {line}
        </span>
      ))}
    </div>
  )
}
