import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAppStore } from '../store'
import { rooms, gameRegistry, wsRoomUrl, type RoomView, type LobbySetting } from '../api'
import RoomSettings from '../components/room/RoomSettings'
import ChatSidebar from '../components/room/ChatSidebar'
import styles from './Room.module.css'

type SocketStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export default function Room() {
  const { roomId } = useParams<{ roomId: string }>()
  const player = useAppStore((s) => s.player)!
  const joinRoom = useAppStore((s) => s.joinRoom)
  const socket = useAppStore((s) => s.socket)
  const leaveRoom = useAppStore((s) => s.leaveRoom)
  const setIsSpectator = useAppStore((s) => s.setIsSpectator)
  const setSpectatorCount = useAppStore((s) => s.setSpectatorCount)
  const navigate = useNavigate()

  const [view, setView] = useState<RoomView | null>(null)
  const [error, setError] = useState('')
  const [starting, setStarting] = useState(false)
  const [socketStatus, setSocketStatus] = useState<SocketStatus>('connecting')
  const [ownerId, setOwnerId] = useState<string | null>(null)
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [settingDescriptors, setSettingDescriptors] = useState<LobbySetting[]>([])
  const [chatOpen, setChatOpen] = useState(true)

  const spectatorCount = useAppStore((s) => s.spectatorCount)
  const setPlayerPresence = useAppStore((s) => s.setPlayerPresence)
  const presenceMap = useAppStore((s) => s.presenceMap)

  const refresh = useCallback(() => {
    rooms.get(roomId!).then((v) => {
      setView(v)
      setSettings(v.settings ?? {})
    }).catch(() => setError('Room not found'))
  }, [roomId])

  // Connect the global socket when entering the room.
  // wsRoomUrl includes the player_id so the server can resolve participant vs spectator.
  // Do NOT close on unmount — Game.tsx reuses the same socket instance.
  // joinRoom is intentionally omitted from deps — it is a stable Zustand action
  // that never changes identity. Including it would cause the effect to re-run
  // on every store update, closing and reopening the socket unnecessarily.
  useEffect(() => {
    joinRoom(roomId!, wsRoomUrl(roomId!, player.id))
  }, [roomId, player.id])

  useEffect(() => {
    if (!view) return
    setOwnerId(view.room.owner_id)
    const participant = view.players.some((p) => p.id === player.id)
    setIsSpectator(!participant)
  }, [view, player.id, setIsSpectator])

  // Load game setting descriptors once we know the game_id.
  useEffect(() => {
    if (!view) return
    gameRegistry.list().then((games) => {
      const game = games.find((g) => g.id === view.room.game_id)
      if (game) setSettingDescriptors(game.settings)
    }).catch(() => {})
  }, [view?.room.game_id])

  useEffect(() => {
    if (!socket) return
    refresh()

    const off = socket.on((event) => {
      if (event.type === 'ws_connected')    setSocketStatus('connected')
      if (event.type === 'ws_reconnecting') setSocketStatus('reconnecting')
      if (event.type === 'ws_disconnected') setSocketStatus('disconnected')

      if (event.type === 'player_joined' || event.type === 'player_left') {
        refresh()
      }

      if (event.type === 'game_started') {
        const payload = event.payload as { session: { id: string } }
        if (!starting) {
          navigate(`/game/${payload.session.id}`)
        }
      }

      if (event.type === 'owner_changed') {
        const payload = event.payload as { owner_id: string }
        setOwnerId(payload.owner_id)
        refresh()
      }

      if (event.type === 'room_closed') {
        navigate('/')
      }

      if (event.type === 'setting_updated') {
        const payload = event.payload as { key: string; value: string }
        setSettings((prev) => ({ ...prev, [payload.key]: payload.value }))
      }

      if (event.type === 'spectator_joined' || event.type === 'spectator_left') {
        const payload = event.payload as { spectator_count: number }
        setSpectatorCount(payload.spectator_count)
      }

      if (event.type === 'presence_update') {
        const payload = event.payload as { player_id: string; online: boolean }
        setPlayerPresence(payload.player_id, payload.online)
      }
    })

    return () => off()
  }, [socket, refresh, navigate, starting])

  async function handleStart() {
    setStarting(true)
    try {
      const session = await rooms.start(roomId!, player.id)
      navigate(`/game/${session.id}`)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to start game')
      setStarting(false)
    }
  }

  async function handleLeave() {
    await rooms.leave(roomId!, player.id).catch(() => {})
    leaveRoom()
    navigate('/')
  }

  function handleSettingChange(key: string, value: string) {
    setSettings((prev) => ({ ...prev, [key]: value }))
  }

  if (!view) return <LoadingScreen />

  const { room } = view

  const isParticipant = view.players.some((p) => p.id === player.id)
  const isSpectator = !isParticipant
  const isOwner = !isSpectator && ownerId === player.id
  const canStart = isOwner && view.players.length >= 2
  const isPrivate = settings['room_visibility'] === 'private'

  const visibleDescriptors = settingDescriptors.filter((s) => {
    if (s.key === 'first_mover_seat') {
      return settings['first_mover_policy'] === 'fixed'
    }
    return true
  })

  return (
    <div className={`${styles.root} page-enter`}>
      <ConnectionBanner status={socketStatus} />

      <div className={styles.layout}>
        <div className={styles.panel}>
          <header className={styles.header}>
            <div>
              <p className={styles.gameLabel}>{room.game_id}</p>
              <h1 data-testid="room-code" className={styles.code}>{room.code}</h1>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span className="badge badge-amber">Waiting</span>
              {isSpectator && <span className="badge badge-muted">Spectating</span>}
            </div>
          </header>

          <hr className="divider" />

          <section className={styles.playersSection}>
            <p className="label" data-testid="player-count">
              Players ({view.players.length}/{room.max_players})
            </p>
            <div className={styles.playerList}>
              {view.players.map((p) => (
                <div key={p.id} className={styles.playerRow}>
                  <span
                    className={styles.presenceDot}
                    data-online={String(presenceMap[p.id] ?? false)}
                    data-testid={`presence-dot-${p.id}`}
                  />
                  {p.avatar_url && <img src={p.avatar_url} alt="" className={styles.avatar} />}
                  <span className={styles.playerName}>{p.username}</span>
                  {p.id === ownerId && <span className="badge badge-amber">Host</span>}
                  {p.id === player.id && <span className="badge badge-muted">You</span>}
                </div>
              ))}
              {Array.from({ length: room.max_players - view.players.length }).map((_, i) => (
                <div key={i} className={`${styles.playerRow} ${styles.empty}`}>
                  <div className={styles.emptySlot} />
                  <span className={styles.waitingText}>Waiting for player...</span>
                </div>
              ))}
            </div>

            {spectatorCount > 0 && (
              <p className={styles.spectatorCount} data-testid="spectator-count">
                👁 {spectatorCount} watching
              </p>
            )}
          </section>

          <hr className="divider" />

          <RoomSettings
            roomId={roomId!}
            playerId={player.id}
            isOwner={isOwner}
            descriptors={visibleDescriptors}
            values={settings}
            onSettingChange={handleSettingChange}
          />

          <hr className="divider" />

          {isParticipant && (
            <div className={styles.shareSection}>
              {isPrivate ? (
                <>
                  <p className="label">🔒 Private Room</p>
                  <p className={styles.privateNote}>
                    Share the room code privately — it won't appear in the public lobby.
                  </p>
                  <div className={styles.codeBox}>
                    <span data-testid="room-code-display" className={styles.codeDisplay}>{room.code}</span>
                    <button
                      className="btn btn-ghost"
                      style={{ padding: '4px 10px', fontSize: 11 }}
                      onClick={() => navigator.clipboard.writeText(room.code)}
                    >
                      Copy
                    </button>
                  </div>
                </>
              ) : (
                <>
                  <p className="label">Invite Code</p>
                  <div className={styles.codeBox}>
                    <span data-testid="room-code-display" className={styles.codeDisplay}>{room.code}</span>
                    <button
                      className="btn btn-ghost"
                      style={{ padding: '4px 10px', fontSize: 11 }}
                      onClick={() => navigator.clipboard.writeText(room.code)}
                    >
                      Copy
                    </button>
                  </div>
                </>
              )}
            </div>
          )}

          {isParticipant && <hr className="divider" />}

          <div className={styles.actions}>
            {isSpectator ? (
              <button className="btn btn-danger" onClick={() => navigate('/')}>Leave</button>
            ) : isOwner ? (
              <>
                <button
                  data-testid="start-game-btn"
                  data-can-start={canStart}
                  className="btn btn-primary"
                  onClick={handleStart}
                  disabled={!canStart || starting}
                  style={{ flex: 1 }}
                >
                  {starting ? 'Starting...' : canStart ? 'Start Game' : `Need ${2 - view.players.length} more player(s)`}
                </button>
                <button className="btn btn-danger" onClick={handleLeave}>Leave</button>
              </>
            ) : (
              <>
                <p className={styles.waitingHost}>Waiting for host to start the game...</p>
                <button className="btn btn-danger" onClick={handleLeave}>Leave</button>
              </>
            )}
          </div>

          {error && <p className={styles.error}>{error}</p>}
        </div>

        <ChatSidebar
          roomId={roomId!}
          open={chatOpen}
          onToggle={() => setChatOpen((v) => !v)}
        />
      </div>
    </div>
  )
}

// --- Connection banner -------------------------------------------------------

function ConnectionBanner({ status }: { status: SocketStatus }) {
  if (status === 'connected') return null

  const config: Record<Exclude<SocketStatus, 'connected'>, { text: string; className: string }> = {
    connecting:   { text: 'Connecting...',                          className: styles.bannerConnecting },
    reconnecting: { text: 'Connection lost — reconnecting...',      className: styles.bannerReconnecting },
    disconnected: { text: 'Disconnected. Please refresh the page.', className: styles.bannerDisconnected },
  }

  const { text, className } = config[status]

  return (
    <div className={`${styles.banner} ${className}`}>
      {status !== 'disconnected' && <span className={styles.bannerDot} />}
      {text}
    </div>
  )
}

// --- Loading screen ----------------------------------------------------------

function LoadingScreen() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
      <p className="pulse" style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>Loading room...</p>
    </div>
  )
}