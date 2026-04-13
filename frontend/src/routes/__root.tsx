import { createRootRoute, Outlet } from '@tanstack/react-router'
import { useEffect, useState, Component, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAppStore, type ResolvedSettings } from '../stores/store'
import { auth, wsPlayerUrl, playerSettings } from '@/lib/api'
import { friends } from '@/features/friends/api'
import { keys } from '@/lib/queryClient'
import { sfx } from '@/lib/sfx'
import { AppHeader } from '@/features/lobby/components/AppHeader'
import { FriendsPanel, FriendsButton } from '@/features/friends/components/FriendsPanel'
import { DMInboxPanel } from '@/features/friends/components/DMInboxPanel'
import { ErrorScreen } from '@/features/errors/ErrorScreen'
import { NotFound } from '@/features/errors/NotFound'
import { Splash } from '@/features/errors/Splash'
import { ToastProvider } from '../ui/Toast'
import { VersionBadge } from '../ui/VersionBadge'
import { emitErrorLog } from '@/lib/telemetry'
import { getDeviceContextAttrs } from '@/lib/device'

const SETTINGS_CACHE_KEY = 'tf:settings'

function loadSettingsFromCache(): ResolvedSettings | null {
  try {
    const raw = localStorage.getItem(SETTINGS_CACHE_KEY)
    if (!raw) return null
    return JSON.parse(raw) as ResolvedSettings
  } catch {
    return null
  }
}

function saveSettingsToCache(settings: ResolvedSettings) {
  try {
    localStorage.setItem(SETTINGS_CACHE_KEY, JSON.stringify(settings))
  } catch {
    // localStorage may be unavailable in private browsing — fail silently.
  }
}

class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  override state = { error: null }

  static getDerivedStateFromError(error: Error) {
    return { error }
  }

  override componentDidCatch(error: Error, info: React.ErrorInfo) {
    emitErrorLog(error.message, {
      'error.type': 'react.boundary',
      'error.stack': error.stack ?? '',
      'error.component': info.componentStack ?? '',
      ...getDeviceContextAttrs(),
    })
  }

  override render() {
    if (this.state.error) {
      return <ErrorScreen error={this.state.error} onReset={() => this.setState({ error: null })} />
    }
    return this.props.children
  }
}

function RootComponent() {
  const { player, setPlayer, disconnectPlayerSocket, dmTarget, setDmTarget, playerSocket } =
    useAppStore()
  const [friendsOpen, setFriendsOpen] = useState(false)
  const [dmInboxOpen, setDmInboxOpen] = useState(false)

  // Preload notification/UI sounds on login.
  useEffect(() => {
    if (player) {
      sfx.preload('notification.dm', 'notification.invite', 'queue.match_found', 'ui.click')
    }
  }, [player?.id, player])

  // Play notification sounds from playerSocket events.
  useEffect(() => {
    if (!playerSocket) return
    const off = playerSocket.on(event => {
      if (event.type === 'notification_received') {
        const payload = event.payload as { type?: string }
        if (payload.type === 'room_invitation') sfx.play('notification.invite')
      }
      if (event.type === 'dm_received') {
        sfx.play('notification.dm')
      }
    })
    return () => off()
  }, [playerSocket])

  useEffect(() => {
    if (!player) {
      disconnectPlayerSocket()
    }
  }, [player, disconnectPlayerSocket])

  async function handleLogout() {
    await auth.logout()
    setPlayer(null)
    window.location.href = '/login'
  }

  // Open DM inbox when dmTarget is set from anywhere (PlayerDropdown, FriendItem)
  useEffect(() => {
    if (dmTarget) {
      setDmInboxOpen(true)
    }
  }, [dmTarget])

  const { data: pendingRequests = [] } = useQuery({
    queryKey: keys.friendsPending(player?.id ?? ''),
    queryFn: () => friends.pending(player!.id),
    enabled: !!player,
    refetchInterval: 30_000,
  })

  const pendingCount = (pendingRequests ?? []).length

  return (
    <ToastProvider>
      <ErrorBoundary>
        <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
          {player && <AppHeader onLogout={handleLogout} />}
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0 }}>
            <Outlet />
          </div>
        </div>

        {player && (
          <>
            <FriendsButton pendingCount={pendingCount} onClick={() => setFriendsOpen(true)} />

            {friendsOpen && (
              <FriendsPanel
                onClose={() => setFriendsOpen(false)}
                onOpenDM={id => {
                  setFriendsOpen(false)
                  setDmTarget(id)
                  setDmInboxOpen(true)
                }}
              />
            )}

            {dmInboxOpen && (
              <DMInboxPanel
                onClose={() => {
                  setDmInboxOpen(false)
                  setDmTarget(null)
                }}
                initialTarget={dmTarget === '__inbox__' ? null : dmTarget}
              />
            )}
          </>
        )}

        <VersionBadge />
      </ErrorBoundary>
    </ToastProvider>
  )
}

export const Route = createRootRoute({
  // Runs before any child route's beforeLoad, so requireAuth guards always
  // read an already-resolved player from the store instead of null.
  beforeLoad: async () => {
    const { setPlayer, connectPlayerSocket, hydrateSettings, setSettings } = useAppStore.getState()

    // Hydrate from localStorage immediately so settings are available
    // before the first render — avoids a flash of default values.
    const cached = loadSettingsFromCache()
    if (cached) setSettings(cached)

    try {
      const p = await auth.me()
      setPlayer(p)
      connectPlayerSocket(wsPlayerUrl(p.id))

      // Fetch fresh settings from the backend in the background.
      // Non-fatal — cached or default values remain on failure.
      playerSettings
        .get(p.id)
        .then(raw => {
          hydrateSettings(raw)
          saveSettingsToCache(useAppStore.getState().settings)
        })
        .catch(() => {})
    } catch {
      // Unauthenticated or network error — child routes with requireAuth
      // will redirect to /login as expected.
      setPlayer(null)
    }
  },

  // Shown while beforeLoad is awaiting auth.me().
  pendingComponent: Splash,
  pendingMs: 0,

  component: RootComponent,
  notFoundComponent: NotFound,
})
