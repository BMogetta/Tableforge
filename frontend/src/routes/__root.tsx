import { createRootRoute, Outlet, useNavigate, useRouter } from '@tanstack/react-router'
import { useEffect, useState, Component, type ReactNode } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAppStore, type ResolvedSettings } from '../stores/store'
import { auth, wsPlayerUrl, playerSettings } from '@/lib/api'
import { friends } from '@/features/friends/api'
import { keys } from '@/lib/queryClient'
import { FriendsPanel, FriendsButton } from '@/features/friends/components/FriendsPanel'
import { DMInboxPanel } from '@/features/friends/components/DMInboxPanel'
import { ToastProvider } from '../ui/Toast'
import { emitErrorLog } from '@/lib/telemetry'

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
    })
  }

  override render() {
    if (this.state.error) {
      return <ErrorScreen error={this.state.error} onReset={() => this.setState({ error: null })} />
    }
    return this.props.children
  }
}

function ErrorScreen({ error, onReset }: { error: Error; onReset: () => void }) {
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        gap: 24,
        padding: 32,
        textAlign: 'center',
      }}
    >
      <div
        style={{
          fontFamily: 'var(--font-display)',
          fontSize: 20,
          color: 'var(--amber)',
          letterSpacing: '0.15em',
        }}
      >
        TABLEFORGE
      </div>
      <div
        style={{ color: 'var(--danger)', fontSize: 13, fontWeight: 600, letterSpacing: '0.05em' }}
      >
        SOMETHING WENT WRONG
      </div>
      <p style={{ color: 'var(--text-muted)', fontSize: 12, maxWidth: 400, lineHeight: 1.6 }}>
        An unexpected error occurred. You can try resetting the page or going back to the lobby.
      </p>
      <code
        style={{
          display: 'block',
          background: 'var(--surface)',
          border: '1px solid var(--border)',
          borderRadius: 6,
          padding: '10px 16px',
          fontSize: 11,
          color: 'var(--text-muted)',
          maxWidth: 480,
          wordBreak: 'break-word',
        }}
      >
        {error.message}
      </code>
      <div style={{ display: 'flex', gap: 8 }}>
        <button className='btn btn-primary' onClick={onReset}>
          Try Again
        </button>
        <button className='btn btn-ghost' onClick={() => (window.location.href = '/')}>
          Go to Lobby
        </button>
      </div>
    </div>
  )
}

function Splash() {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        flexDirection: 'column',
        gap: 16,
      }}
    >
      <div
        style={{
          fontFamily: 'var(--font-display)',
          fontSize: 24,
          color: 'var(--amber)',
          letterSpacing: '0.2em',
        }}
      >
        TABLEFORGE
      </div>
      <div
        className='pulse'
        style={{ color: 'var(--text-muted)', fontSize: 11, letterSpacing: '0.1em' }}
      >
        CONNECTING...
      </div>
    </div>
  )
}

function NotFound() {
  const navigate = useNavigate()
  const router = useRouter()
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        gap: 24,
        padding: 32,
        textAlign: 'center',
      }}
    >
      <div
        style={{
          fontFamily: 'var(--font-display)',
          fontSize: 20,
          color: 'var(--amber)',
          letterSpacing: '0.15em',
        }}
      >
        TABLEFORGE
      </div>
      <div
        style={{
          fontSize: 72,
          fontWeight: 800,
          color: 'var(--text-muted)',
          letterSpacing: '-0.05em',
          lineHeight: 1,
          opacity: 0.15,
        }}
      >
        404
      </div>
      <div
        style={{ color: 'var(--danger)', fontSize: 13, fontWeight: 600, letterSpacing: '0.1em' }}
      >
        PAGE NOT FOUND
      </div>
      <p style={{ color: 'var(--text-muted)', fontSize: 12, maxWidth: 360, lineHeight: 1.6 }}>
        The route you're looking for doesn't exist. It may have been moved, deleted, or you may have
        mistyped the address.
      </p>
      <div style={{ display: 'flex', gap: 8 }}>
        <button
          className='btn btn-primary'
          onClick={() => navigate({ to: '/', replace: true, ignoreBlocker: true })}
        >
          Go to Lobby
        </button>
        <button className='btn btn-ghost' onClick={() => router.history.back()}>
          Go Back
        </button>
      </div>
    </div>
  )
}

function RootComponent() {
  const { player, disconnectPlayerSocket, dmTarget, setDmTarget } = useAppStore()
  const [friendsOpen, setFriendsOpen] = useState(false)
  const [dmInboxOpen, setDmInboxOpen] = useState(false)

  useEffect(() => {
    if (!player) {
      disconnectPlayerSocket()
    }
  }, [player])

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
        <Outlet />

        {player && (
          <>
            <FriendsButton
              pendingCount={pendingCount}
              onClick={() => setFriendsOpen(true)}
            />

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
