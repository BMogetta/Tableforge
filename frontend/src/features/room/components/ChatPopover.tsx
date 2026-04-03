import { useState, useEffect, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { rooms, mutes } from '@/features/room/api'
import type { RoomMessage } from '@/lib/api'
import type { RoomPlayer } from '@/lib/schema-generated.zod'
import { useAppStore } from '@/stores/store'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { keys } from '@/lib/queryClient'
import { sfx } from '@/lib/sfx'
import styles from './ChatPopover.module.css'

interface Props {
  roomId: string
  mutedIds: Set<string>
  muteAll: boolean
  onMute: (playerId: string) => void
  onUnmute: (playerId: string) => void
  onMuteAll: () => void
  onUnmuteAll: () => void
  roomPlayers: RoomPlayer[]
}

interface SystemMessage {
  id: string
  text: string
}

let sysId = 0
function nextSysId() {
  return `sys-${++sysId}`
}

const HELP_TEXT = [
  '/mute <username>     — hide messages from a player (this session only)',
  '/unmute <username>   — show messages from a muted player again',
  '/muteall             — hide messages from everyone except yourself',
  '/unmuteall           — restore all locally muted players',
  '/block <username>    — permanently block a player (persists across sessions)',
  '/unblock <username>  — remove a permanent block',
  '/help                — show this help',
].join('\n')

export function ChatPopover({
  roomId,
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

  const { data: blockedList = [] } = useQuery({
    queryKey: keys.mutes(player.id),
    queryFn: () => mutes.list(player.id),
    staleTime: 60_000,
  })
  const blockedIds = new Set(blockedList.map(m => m.muted_id))

  const blockMutation = useMutation({
    mutationFn: (targetId: string) => mutes.mute(player.id, targetId),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => qc.invalidateQueries({ queryKey: keys.mutes(player.id) }),
  })

  const unblockMutation = useMutation({
    mutationFn: (targetId: string) => mutes.unmute(player.id, targetId),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => qc.invalidateQueries({ queryKey: keys.mutes(player.id) }),
  })

  const { data: messages = [] } = useQuery({
    queryKey: keys.roomMessages(roomId),
    queryFn: () => rooms.messages(roomId),
    refetchInterval: 30_000,
  })

  useEffect(() => {
    if (!socket) return
    const off = socket.on(event => {
      if (event.type === 'chat_message') {
        const msg = event.payload
        if (msg.player_id !== player.id) sfx.play('chat.receive')
        qc.setQueryData<RoomMessage[]>(keys.roomMessages(roomId), (prev = []) => {
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

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, systemMessages])

  // Auto-focus input when popover opens.
  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const sendMessage = useMutation({
    mutationFn: (content: string) => rooms.sendMessage(roomId, player.id, content),
    onSuccess: () => {
      setDraft('')
      inputRef.current?.focus()
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  function addSystemMessage(text: string) {
    setSystemMessages(prev => [...prev, { id: nextSysId(), text }])
  }

  function resolvePlayer(username: string): RoomPlayer | undefined {
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
        if (!username) { addSystemMessage('[Error] Usage: /mute <username>'); return true }
        const target = resolvePlayer(username)
        if (!target) { addSystemMessage(`[Error] Player "${username}" not found in this room.`); return true }
        if (target.id === player.id) { addSystemMessage('[Error] You cannot mute yourself.'); return true }
        onMute(target.id)
        addSystemMessage(`[System] Muted ${target.username} for this session.`)
        return true
      }
      case 'unmute': {
        if (!username) { addSystemMessage('[Error] Usage: /unmute <username>'); return true }
        const target = resolvePlayer(username)
        if (!target) { addSystemMessage(`[Error] Player "${username}" not found in this room.`); return true }
        onUnmute(target.id)
        addSystemMessage(`[System] Unmuted ${target.username}.`)
        return true
      }
      case 'block': {
        if (!username) { addSystemMessage('[Error] Usage: /block <username>'); return true }
        const target = resolvePlayer(username)
        if (!target) { addSystemMessage(`[Error] Player "${username}" not found in this room.`); return true }
        if (target.id === player.id) { addSystemMessage('[Error] You cannot block yourself.'); return true }
        blockMutation.mutate(target.id)
        addSystemMessage(`[System] Blocked ${target.username}.`)
        return true
      }
      case 'unblock': {
        if (!username) { addSystemMessage('[Error] Usage: /unblock <username>'); return true }
        const target = resolvePlayer(username)
        if (!target) { addSystemMessage(`[Error] Player "${username}" not found in this room.`); return true }
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
    if (handleCommand(content)) { setDraft(''); return }
    sendMessage.mutate(content)
  }

  const visibleMessages = messages.filter(m => {
    if (m.hidden) return false
    if (m.player_id === player.id) return true
    if (muteAll) return false
    if (mutedIds.has(m.player_id)) return false
    if (blockedIds.has(m.player_id)) return false
    return true
  })

  return (
    <div className={styles.chat}>
      <div className={styles.chatHeader}>
        <span className={styles.chatTitle}>Room Chat</span>
        <span className={styles.chatCount}>{visibleMessages.length}</span>
      </div>

      <div className={styles.messages} aria-live='polite' aria-relevant='additions'>
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
          aria-label='Chat message'
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
          <svg width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='1.5'>
            <line x1='22' y1='2' x2='11' y2='13' />
            <polygon points='22 2 15 22 11 13 2 9 22 2' />
          </svg>
        </button>
      </div>
    </div>
  )
}

// --- ChatMessage -------------------------------------------------------------

function ChatMessage({ msg, isSelf, senderUsername }: { msg: RoomMessage; isSelf: boolean; senderUsername?: string }) {
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
        <span key={i} className={styles.systemLine}>{line}</span>
      ))}
    </div>
  )
}
