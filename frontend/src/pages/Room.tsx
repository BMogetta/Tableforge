import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAppStore } from '../stores/store'
import {
  rooms,
  bots,
  mutes,
  gameRegistry,
  wsRoomUrl,
  type RoomView,
  type LobbySetting,
  type BotProfile,
  type RoomViewPlayer,
} from '../lib/api'
import { ok, error, catchToAppError, type AppError } from '../utils/errors'
import { useToast } from '../components/ui/Toast'
import { ErrorMessage } from '../components/ui/ErrorMessage'
import { RoomSettings } from '../components/room/RoomSettings'
import { ChatSidebar } from '../components/room/ChatSidebar'
import styles from './Room.module.css'

type SocketStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export function Room() {
  const { roomId } = useParams<{ roomId: string }>()
  const player = useAppStore(s => s.player)!
  const joinRoom = useAppStore(s => s.joinRoom)
  const socket = useAppStore(s => s.socket)
  const leaveRoom = useAppStore(s => s.leaveRoom)
  const setIsSpectator = useAppStore(s => s.setIsSpectator)
  const setSpectatorCount = useAppStore(s => s.setSpectatorCount)
  const navigate = useNavigate()
  const toast = useToast()

  const [view, setView] = useState<RoomView | null>(null)
  const [starting, setStarting] = useState(false)
  const [startError, setStartError] = useState<AppError | null>(null)
  const [socketStatus, setSocketStatus] = useState<SocketStatus>('connecting')
  const [ownerId, setOwnerId] = useState<string | null>(null)
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [settingDescriptors, setSettingDescriptors] = useState<LobbySetting[]>([])
  const [chatOpen, setChatOpen] = useState(true)

  const [botProfiles, setBotProfiles] = useState<BotProfile[]>([])
  const [selectedProfile, setSelectedProfile] = useState('medium')
  const [addingBot, setAddingBot] = useState(false)
  const [botError, setBotError] = useState<AppError | null>(null)
  const [gameHasBotAdapter, setGameHasBotAdapter] = useState(false)
  const [removingBotId, setRemovingBotId] = useState<string | null>(null)

  const spectatorCount = useAppStore(s => s.spectatorCount)
  const setPlayerPresence = useAppStore(s => s.setPlayerPresence)
  const presenceMap = useAppStore(s => s.presenceMap)

  // --- Mute state (elevated from ChatSidebar so the player dropdown can also
  //     trigger mutes without going through the chat input) -------------------

  // Local mutes: in-memory only, reset when the component unmounts.
  // These do NOT call the backend — use block for persistent mutes.
  const [mutedIds, setMutedIds] = useState<Set<string>>(new Set())
  // When true, all messages from other players are hidden regardless of mutedIds.
  const [muteAll, setMuteAll] = useState(false)

  function handleMute(targetId: string) {
    setMutedIds(prev => new Set([...prev, targetId]))
  }

  function handleUnmute(targetId: string) {
    setMutedIds(prev => {
      const next = new Set(prev)
      next.delete(targetId)
      return next
    })
  }

  function handleMuteAll() {
    setMuteAll(true)
  }
  function handleUnmuteAll() {
    setMuteAll(false)
    setMutedIds(new Set())
  }

  // --- Player dropdown -------------------------------------------------------

  const [openDropdownId, setOpenDropdownId] = useState<string | null>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  // Close dropdown when clicking outside.
  useEffect(() => {
    function onPointerDown(e: PointerEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpenDropdownId(null)
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [])

  async function handleBlock(targetId: string) {
    const [err] = await mutes
      .mute(player.id, targetId)
      .then(() => ok(null))
      .catch(e => error(catchToAppError(e)))
    if (err) toast.showError(err)
    setOpenDropdownId(null)
  }

  async function handleUnblock(targetId: string) {
    const [err] = await mutes
      .unmute(player.id, targetId)
      .then(() => ok(null))
      .catch(e => error(catchToAppError(e)))
    if (err) toast.showError(err)
    setOpenDropdownId(null)
  }

  // ---------------------------------------------------------------------------

  const refresh = useCallback(() => {
    rooms
      .get(roomId!)
      .then(v => {
        setView(v)
        setSettings(v.settings ?? {})
      })
      .catch(e => {
        const err = catchToAppError(e)
        // If the room no longer exists, go home instead of looping toasts.
        if (err.reason === 'NOT_FOUND') {
          navigate('/')
        } else {
          toast.showError(err)
        }
      })
  }, [roomId, toast, navigate])

  useEffect(() => {
    joinRoom(roomId!, wsRoomUrl(roomId!, player.id))
  }, [roomId, player.id])

  useEffect(() => {
    if (!view) return
    setOwnerId(view.room.owner_id)
    const participant = view.players.some(p => p.id === player.id)
    setIsSpectator(!participant)
  }, [view, player.id, setIsSpectator])

  // Load game setting descriptors once we know the game_id.
  useEffect(() => {
    if (!view) return
    gameRegistry
      .list()
      .then(games => {
        const game = games.find(g => g.id === view.room.game_id)
        if (game) setSettingDescriptors(game.settings)
      })
      .catch(() => {})
  }, [view?.room.game_id])

  // Load bot profiles once. Also check whether this game has a bot adapter
  // by attempting to add — we determine support optimistically: if profiles
  // exist we show the UI and let the server reject unsupported games.
  useEffect(() => {
    bots
      .profiles()
      .then(profiles => {
        setBotProfiles(profiles)
        setGameHasBotAdapter(profiles.length > 0)
        if (profiles.length > 0) setSelectedProfile(profiles[0].name)
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!socket) return
    refresh()

    const off = socket.on(event => {
      if (event.type === 'ws_connected') setSocketStatus('connected')
      if (event.type === 'ws_reconnecting') setSocketStatus('reconnecting')
      if (event.type === 'ws_disconnected') setSocketStatus('disconnected')

      if (event.type === 'player_joined' || event.type === 'player_left') {
        refresh()
      }

      if (event.type === 'game_started') {
        const payload = event.payload
        if (!starting) {
          navigate(`/game/${payload.session.id}`)
        }
      }

      if (event.type === 'owner_changed') {
        const payload = event.payload
        setOwnerId(payload.owner_id)
        refresh()
      }

      if (event.type === 'room_closed') {
        navigate('/')
      }

      if (event.type === 'setting_updated') {
        const payload = event.payload
        setSettings(prev => ({ ...prev, [payload.key]: payload.value }))
      }

      if (event.type === 'spectator_joined' || event.type === 'spectator_left') {
        const payload = event.payload
        setSpectatorCount(payload.spectator_count)
      }

      if (event.type === 'presence_update') {
        const payload = event.payload
        setPlayerPresence(payload.player_id, payload.online)
      }
    })

    return () => off()
  }, [socket, refresh, navigate, starting])

  async function handleStart() {
    setStarting(true)
    setStartError(null)

    const [err, session] = await rooms
      .start(roomId!, player.id)
      .then(s => ok(s))
      .catch(e => error(catchToAppError(e)))

    if (err) {
      setStartError(err)
      setStarting(false)
      return
    }

    navigate(`/game/${session.id}`)
  }

  async function handleLeave() {
    await rooms.leave(roomId!, player.id).catch(() => {})
    leaveRoom()
    navigate('/')
  }

  async function handleAddBot() {
    setAddingBot(true)
    setBotError(null)

    const [err] = await bots
      .add(roomId!, player.id, selectedProfile)
      .then(p => ok(p))
      .catch(e => error(catchToAppError(e)))

    if (err) {
      setBotError(err)
    } else {
      refresh()
    }

    setAddingBot(false)
  }

  async function handleRemoveBot(botId: string) {
    setRemovingBotId(botId)

    const [err] = await bots
      .remove(roomId!, player.id, botId)
      .then(() => ok(null))
      .catch(e => error(catchToAppError(e)))

    if (err) {
      // Remove is a transient action with no inline location — use toast.
      toast.showError(err)
    } else {
      refresh()
    }

    setRemovingBotId(null)
  }

  function handleSettingChange(key: string, value: string) {
    setSettings(prev => ({ ...prev, [key]: value }))
  }

  if (!view) return <LoadingScreen />

  const { room } = view

  const isParticipant = view.players.some(p => p.id === player.id)
  const isSpectator = !isParticipant
  const isOwner = !isSpectator && ownerId === player.id
  const canStart = isOwner && view.players.length >= 2
  const isPrivate = settings['room_visibility'] === 'private'
  const hasOpenSlot = view.players.length < room.max_players
  const canAddBot = isOwner && hasOpenSlot && gameHasBotAdapter && botProfiles.length > 0

  const visibleDescriptors = settingDescriptors.filter(s => {
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
              <h1 data-testid='room-code' className={styles.code}>
                {room.code}
              </h1>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span className='badge badge-amber'>Waiting</span>
              {isSpectator && <span className='badge badge-muted'>Spectating</span>}
            </div>
          </header>

          <hr className='divider' />

          <section className={styles.playersSection}>
            <p className='label' data-testid='player-count'>
              Players ({view.players.length}/{room.max_players})
            </p>
            <div className={styles.playerList}>
              {view.players.map(p => {
                const isSelf = p.id === player.id
                const isDropdownOpen = openDropdownId === p.id
                const isMuted = mutedIds.has(p.id)

                return (
                  <div
                    key={p.id}
                    className={styles.playerRow}
                    data-testid={p.is_bot ? `bot-row-${p.id}` : `player-row-${p.id}`}
                  >
                    <span
                      className={styles.presenceDot}
                      data-online={String(presenceMap[p.id] ?? false)}
                      data-testid={`presence-dot-${p.id}`}
                    />
                    {p.avatar_url && <img src={p.avatar_url} alt='' className={styles.avatar} />}

                    {/* Username — clickable for other human players to open the
                        action dropdown (mute, block, add friend, send DM) */}
                    {!isSelf && !p.is_bot ? (
                      <div
                        className={styles.playerNameWrapper}
                        ref={isDropdownOpen ? dropdownRef : null}
                      >
                        <button
                          className={styles.playerNameBtn}
                          onClick={() => setOpenDropdownId(isDropdownOpen ? null : p.id)}
                          title={`Options for ${p.username}`}
                        >
                          {p.username}
                          {isMuted && (
                            <span className={styles.mutedIndicator} title='Locally muted'>
                              🔇
                            </span>
                          )}
                        </button>

                        {isDropdownOpen && (
                          <PlayerDropdown
                            target={p}
                            isMuted={isMuted}
                            onMute={() => {
                              handleMute(p.id)
                              setOpenDropdownId(null)
                            }}
                            onUnmute={() => {
                              handleUnmute(p.id)
                              setOpenDropdownId(null)
                            }}
                            onBlock={() => handleBlock(p.id)}
                            onUnblock={() => handleUnblock(p.id)}
                          />
                        )}
                      </div>
                    ) : (
                      <span className={styles.playerName}>
                        {p.username}
                        {p.is_bot && (
                          <span className='badge badge-muted' style={{ marginLeft: 6 }}>
                            Bot
                          </span>
                        )}
                      </span>
                    )}

                    {p.id === ownerId && <span className='badge badge-amber'>Host</span>}
                    {isSelf && <span className='badge badge-muted'>You</span>}
                    {isOwner && p.is_bot && (
                      <button
                        data-testid={`remove-bot-btn-${p.id}`}
                        className={styles.removeBotBtn}
                        disabled={removingBotId === p.id}
                        onClick={() => handleRemoveBot(p.id)}
                        title='Remove bot'
                      >
                        ×
                      </button>
                    )}
                  </div>
                )
              })}
              {Array.from({ length: room.max_players - view.players.length }).map((_, i) => (
                <div key={i} className={`${styles.playerRow} ${styles.empty}`}>
                  <div className={styles.emptySlot} />
                  <span className={styles.waitingText}>Waiting for player...</span>
                </div>
              ))}
            </div>

            {spectatorCount > 0 && (
              <p className={styles.spectatorCount} data-testid='spectator-count'>
                👁 {spectatorCount} watching
              </p>
            )}
          </section>

          <hr className='divider' />

          <RoomSettings
            roomId={roomId!}
            playerId={player.id}
            isOwner={isOwner}
            descriptors={visibleDescriptors}
            values={settings}
            onSettingChange={handleSettingChange}
          />

          {canAddBot && (
            <>
              <hr className='divider' />
              <section className={styles.botSection}>
                <p className='label'>Add Bot</p>
                <div className={styles.botRow}>
                  <select
                    data-testid='add-bot-select'
                    className={styles.botSelect}
                    value={selectedProfile}
                    disabled={addingBot}
                    onChange={e => setSelectedProfile(e.target.value)}
                  >
                    {botProfiles.map(p => (
                      <option key={p.name} value={p.name}>
                        {p.name.charAt(0).toUpperCase() + p.name.slice(1)}
                      </option>
                    ))}
                  </select>
                  <button
                    data-testid='add-bot-btn'
                    className='btn btn-secondary'
                    disabled={addingBot}
                    onClick={handleAddBot}
                  >
                    {addingBot ? 'Adding...' : 'Add Bot'}
                  </button>
                </div>
                <ErrorMessage error={botError} />
              </section>
            </>
          )}

          <hr className='divider' />

          {isParticipant && (
            <div className={styles.shareSection}>
              {isPrivate ? (
                <>
                  <p className='label'>🔒 Private Room</p>
                  <p className={styles.privateNote}>
                    Share the room code privately — it won't appear in the public lobby.
                  </p>
                  <div className={styles.codeBox}>
                    <span data-testid='room-code-display' className={styles.codeDisplay}>
                      {room.code}
                    </span>
                    <button
                      className='btn btn-ghost'
                      style={{ padding: '4px 10px', fontSize: 11 }}
                      onClick={() => navigator.clipboard.writeText(room.code)}
                    >
                      Copy
                    </button>
                  </div>
                </>
              ) : (
                <>
                  <p className='label'>Invite Code</p>
                  <div className={styles.codeBox}>
                    <span data-testid='room-code-display' className={styles.codeDisplay}>
                      {room.code}
                    </span>
                    <button
                      className='btn btn-ghost'
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

          {isParticipant && <hr className='divider' />}

          <div className={styles.actions}>
            {isSpectator ? (
              <button className='btn btn-danger' onClick={() => navigate('/')}>
                Leave
              </button>
            ) : isOwner ? (
              <>
                <button
                  data-testid='start-game-btn'
                  data-can-start={canStart}
                  className='btn btn-primary'
                  onClick={handleStart}
                  disabled={!canStart || starting}
                  style={{ flex: 1 }}
                >
                  {starting
                    ? 'Starting...'
                    : canStart
                      ? 'Start Game'
                      : `Need ${2 - view.players.length} more player(s)`}
                </button>
                <button className='btn btn-danger' onClick={handleLeave}>
                  Leave
                </button>
              </>
            ) : (
              <>
                <p className={styles.waitingHost}>Waiting for host to start the game...</p>
                <button className='btn btn-danger' onClick={handleLeave}>
                  Leave
                </button>
              </>
            )}
          </div>

          <ErrorMessage error={startError} />
        </div>

        <ChatSidebar
          roomId={roomId!}
          open={chatOpen}
          onToggle={() => setChatOpen(v => !v)}
          mutedIds={mutedIds}
          muteAll={muteAll}
          onMute={handleMute}
          onUnmute={handleUnmute}
          onMuteAll={handleMuteAll}
          onUnmuteAll={handleUnmuteAll}
          roomPlayers={view.players}
        />
      </div>
    </div>
  )
}

// --- PlayerDropdown ----------------------------------------------------------

interface PlayerDropdownProps {
  target: RoomViewPlayer
  isMuted: boolean
  onMute: () => void
  onUnmute: () => void
  onBlock: () => void
  onUnblock: () => void
}

function PlayerDropdown({
  target,
  isMuted,
  onMute,
  onUnmute,
  onBlock,
  onUnblock,
}: PlayerDropdownProps) {
  return (
    <div className={styles.dropdown}>
      {isMuted ? (
        <button className={styles.dropdownItem} onClick={onUnmute}>
          🔊 Unmute (this session)
        </button>
      ) : (
        <button className={styles.dropdownItem} onClick={onMute}>
          🔇 Mute (this session)
        </button>
      )}
      <button className={styles.dropdownItem} onClick={onBlock}>
        🚫 Block
      </button>
      <button className={styles.dropdownItem} onClick={onUnblock}>
        ✓ Unblock
      </button>
      <hr className={styles.dropdownDivider} />
      {/* Stubs — enabled when friends system and DMs UI are implemented */}
      <button className={styles.dropdownItem} disabled title='Coming soon'>
        👤 Add Friend
      </button>
      <button className={styles.dropdownItem} disabled title='Coming soon'>
        ✉ Send DM
      </button>
    </div>
  )
}

// --- Connection banner -------------------------------------------------------

function ConnectionBanner({ status }: { status: SocketStatus }) {
  if (status === 'connected') return null

  const config: Record<Exclude<SocketStatus, 'connected'>, { text: string; className: string }> = {
    connecting: { text: 'Connecting...', className: styles.bannerConnecting },
    reconnecting: {
      text: 'Connection lost — reconnecting...',
      className: styles.bannerReconnecting,
    },
    disconnected: {
      text: 'Disconnected. Please refresh the page.',
      className: styles.bannerDisconnected,
    },
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
    <div
      style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}
    >
      <p className='pulse' style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>
        Loading room...
      </p>
    </div>
  )
}
