import { useEffect, useState, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { useAppStore } from '@/stores/store'
import { useSocketStore } from '@/stores/socketStore'
import { useRoomStore } from '@/stores/roomStore'
import { rooms, bots } from '@/features/room/api'
import { gameRegistry } from '@/features/lobby/api'
import type { RoomView } from '@/lib/api'
import type { LobbySetting, BotProfile } from '@/lib/schema-generated.zod'
import { ok, error, catchToAppError, type AppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { RoomSettings } from './components/RoomSettings'
import { ChatPopover } from './components/ChatPopover'
import { PlayerList } from './components/PlayerList'
import { BotSection } from './components/BotSection'
import { InviteCode } from './components/InviteCode'
import { RoomActions } from './components/RoomActions'
import { ConnectionBanner } from './components/ConnectionBanner'
import { RoomToolbar, SettingsIcon, BotIcon, InviteIcon, ChatIcon } from './components/RoomToolbar'
import styles from './Room.module.css'
import { testId, testAttr } from '@/utils/testId'

type PopoverId = 'settings' | 'bot' | 'invite' | 'chat'

export function Room({ roomId }: { roomId: string }) {
  const { t } = useTranslation()
  const player = useAppStore(s => s.player)!
  const joinRoom = useRoomStore(s => s.joinRoom)
  const gateway = useSocketStore(s => s.gateway)
  const leaveRoom = useRoomStore(s => s.leaveRoom)
  const setIsSpectator = useRoomStore(s => s.setIsSpectator)
  const setSpectatorCount = useRoomStore(s => s.setSpectatorCount)
  const spectatorCount = useRoomStore(s => s.spectatorCount)
  const socketStatus = useRoomStore(s => s.roomSocketStatus)
  const navigate = useNavigate()
  const toast = useToast()

  const [view, setView] = useState<RoomView | null>(null)
  const [starting, setStarting] = useState(false)
  const [startError, setStartError] = useState<AppError | null>(null)
  const [ownerId, setOwnerId] = useState<string | null>(null)
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [settingDescriptors, setSettingDescriptors] = useState<LobbySetting[]>([])
  const [activePopover, setActivePopover] = useState<PopoverId | null>(null)

  // --- Bot state --------------------------------------------------------------
  const [botProfiles, setBotProfiles] = useState<BotProfile[]>([])
  const [selectedProfile, setSelectedProfile] = useState('medium')
  const [addingBot, setAddingBot] = useState(false)
  const [botError, setBotError] = useState<AppError | null>(null)
  const [gameHasBotAdapter, setGameHasBotAdapter] = useState(false)
  const [removingBotId, setRemovingBotId] = useState<string | null>(null)

  // --- Mute state (local, in-memory only) ------------------------------------
  const [mutedIds, setMutedIds] = useState<Set<string>>(new Set())
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

  // Track whether we're navigating to the game — socket must survive that transition.
  const navigatingToGameRef = useRef(false)

  // --- Stable refs for socket handler ----------------------------------------
  const navigateRef = useRef(navigate)
  const toastRef = useRef(toast)
  useEffect(() => {
    navigateRef.current = navigate
  }, [navigate])
  useEffect(() => {
    toastRef.current = toast
  }, [toast])

  const refresh = useCallback(() => {
    rooms
      .get(roomId)
      .then(v => {
        setView(v)
        setSettings(v.settings ?? {})
      })
      .catch(e => {
        const err = catchToAppError(e)
        if (err.reason === 'NOT_FOUND') {
          navigateRef.current({ to: '/' })
        } else {
          toastRef.current.showError(err)
        }
      })
  }, [roomId])

  const refreshRef = useRef(refresh)
  useEffect(() => {
    refreshRef.current = refresh
  }, [refresh])

  // --- Initial fetch (independent of socket) ---------------------------------
  useEffect(() => {
    refresh()
  }, [refresh])

  // --- WebSocket connection --------------------------------------------------
  useEffect(() => {
    navigatingToGameRef.current = false
    joinRoom(roomId)
    return () => {
      // Clean up socket when leaving the room flow entirely (browser back, URL change).
      // Skip cleanup when navigating to /game — the socket must survive that transition.
      if (!navigatingToGameRef.current) {
        leaveRoom()
      }
    }
  }, [roomId, player.id, joinRoom, leaveRoom])

  useEffect(() => {
    if (!view) return
    setOwnerId(view.room.owner_id)
    const spectator = !view.players.some(p => p.id === player.id)
    setIsSpectator(spectator)

    // If the room already has an active game, navigate directly.
    // This handles spectators arriving after game_started was broadcast,
    // and participants reconnecting after a page refresh.
    if (view.active_session_id) {
      navigatingToGameRef.current = true
      navigate({ to: '/game/$sessionId', params: { sessionId: view.active_session_id } })
    }
  }, [view, player.id, setIsSpectator, navigate])

  useEffect(() => {
    if (!view) return
    gameRegistry
      .list()
      .then(games => {
        const game = games.find(g => g.id === view.room.game_id)
        if (game) setSettingDescriptors(game.settings)
      })
      .catch(() => {})
  }, [view?.room.game_id, view])

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

  // --- Socket event handler --------------------------------------------------
  const startingRef = useRef(starting)
  useEffect(() => {
    startingRef.current = starting
  }, [starting])

  useEffect(() => {
    if (!gateway) return
    refreshRef.current()

    const off = gateway.on(event => {
      if (event.type === 'player_joined' || event.type === 'player_left') refreshRef.current()
      if (event.type === 'game_started') {
        if (!startingRef.current) {
          navigatingToGameRef.current = true
          navigateRef.current({
            to: '/game/$sessionId',
            params: { sessionId: event.payload.session.id },
          })
        }
      }
      if (event.type === 'owner_changed') {
        setOwnerId(event.payload.owner_id)
        refreshRef.current()
      }
      if (event.type === 'room_closed') navigateRef.current({ to: '/' })
      if (event.type === 'setting_updated') {
        setSettings(prev => ({ ...prev, [event.payload.key]: event.payload.value }))
      }
      if (event.type === 'spectator_joined' || event.type === 'spectator_left') {
        setSpectatorCount(event.payload.spectator_count)
      }
    })

    return () => off()
  }, [gateway, setSpectatorCount])

  // --- Actions ---------------------------------------------------------------

  async function handleStart() {
    setStarting(true)
    setStartError(null)

    const [err, session] = await rooms
      .start(roomId)
      .then(s => ok(s))
      .catch(e => error(catchToAppError(e)))

    if (err) {
      setStartError(err)
      setStarting(false)
      return
    }

    navigatingToGameRef.current = true
    navigate({ to: '/game/$sessionId', params: { sessionId: session.id } })
  }

  async function handleLeave() {
    await rooms.leave(roomId).catch(() => {})
    leaveRoom()
    navigate({ to: '/' })
  }

  async function handleAddBot() {
    setAddingBot(true)
    setBotError(null)

    const [err] = await bots
      .add(roomId, player.id, selectedProfile)
      .then(p => ok(p))
      .catch(e => error(catchToAppError(e)))

    if (err) {
      setBotError(err)
    } else {
      refreshRef.current()
      setActivePopover(null)
    }
    setAddingBot(false)
  }

  async function handleRemoveBot(botId: string) {
    setRemovingBotId(botId)

    const [err] = await bots
      .remove(roomId, player.id, botId)
      .then(() => ok(null))
      .catch(e => error(catchToAppError(e)))

    if (err) {
      toast.showError(err)
    } else {
      refreshRef.current()
    }
    setRemovingBotId(null)
  }

  function handleSettingChange(key: string, value: string) {
    setSettings(prev => ({ ...prev, [key]: value }))
  }

  function togglePopover(id: PopoverId) {
    setActivePopover(prev => (prev === id ? null : id))
  }

  // --- Render ----------------------------------------------------------------

  if (!view) {
    return (
      <div
        style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}
      >
        <p className='pulse' style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>
          {t('room.loading')}
        </p>
      </div>
    )
  }

  const { room } = view
  const isParticipant = view.players.some(p => p.id === player.id)
  const isSpectator = !isParticipant
  const isOwner = !isSpectator && ownerId === player.id
  const canStart = isOwner && view.players.length >= 2
  const isPrivate = settings.room_visibility === 'private'
  const hasOpenSlot = view.players.length < room.max_players

  const visibleDescriptors = settingDescriptors.filter(s => {
    if (s.key === 'first_mover_seat') return settings.first_mover_policy === 'fixed'
    return true
  })

  const hasSettings = visibleDescriptors.length > 0

  const showBotButton = isOwner && gameHasBotAdapter && botProfiles.length > 0

  const toolbarItems = [
    { id: 'settings' as const, label: t('room.settings'), icon: SettingsIcon, visible: hasSettings },
    {
      id: 'bot' as const,
      label: t('room.addBot'),
      icon: BotIcon,
      visible: showBotButton,
      disabled: !hasOpenSlot,
    },
    { id: 'invite' as const, label: t('room.inviteCode'), icon: InviteIcon, visible: isParticipant },
    { id: 'chat' as const, label: t('room.chat'), icon: ChatIcon, visible: true },
  ]

  const popoverContent = (() => {
    switch (activePopover) {
      case 'settings':
        return (
          <RoomSettings
            roomId={roomId}
            isOwner={isOwner}
            descriptors={visibleDescriptors}
            values={settings}
            onSettingChange={handleSettingChange}
          />
        )
      case 'bot':
        return (
          <BotSection
            profiles={botProfiles}
            selectedProfile={selectedProfile}
            onSelectProfile={setSelectedProfile}
            onAdd={handleAddBot}
            adding={addingBot}
            error={botError}
          />
        )
      case 'invite':
        return <InviteCode code={room.code} isPrivate={isPrivate} />
      case 'chat':
        return (
          <ChatPopover
            roomId={roomId}
            mutedIds={mutedIds}
            muteAll={muteAll}
            onMute={handleMute}
            onUnmute={handleUnmute}
            onMuteAll={handleMuteAll}
            onUnmuteAll={handleUnmuteAll}
            roomPlayers={view.players}
          />
        )
      default:
        return null
    }
  })()

  return (
    <div className={`${styles.root} page-enter`} {...testAttr('socket-status', socketStatus)}>
      <ConnectionBanner status={socketStatus} />

      <div className={styles.panel}>
        <header className={styles.header}>
          <div>
            <p className={styles.gameLabel}>{room.game_id}</p>
            <h1 {...testId('room-code')} className={styles.code}>
              {room.code}
            </h1>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className='badge badge-amber'>{t('room.waiting')}</span>
            {isSpectator && <span className='badge badge-muted'>{t('room.spectating')}</span>}
          </div>
        </header>

        <RoomToolbar items={toolbarItems} activePopover={activePopover} onToggle={togglePopover}>
          {popoverContent}
        </RoomToolbar>

        <PlayerList
          players={view.players}
          maxPlayers={room.max_players}
          ownerId={ownerId}
          currentPlayerId={player.id}
          isOwner={isOwner}
          mutedIds={mutedIds}
          spectatorCount={spectatorCount}
          removingBotId={removingBotId}
          onMute={handleMute}
          onUnmute={handleUnmute}
          onRemoveBot={handleRemoveBot}
        />

        <hr className='divider' />

        <RoomActions
          isSpectator={isSpectator}
          isOwner={isOwner}
          canStart={canStart}
          starting={starting}
          startError={startError}
          playersNeeded={2 - view.players.length}
          onStart={handleStart}
          onLeave={handleLeave}
        />
      </div>
    </div>
  )
}
