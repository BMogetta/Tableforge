import { useEffect, useState, Component, type ReactNode } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAppStore } from './store'
import { auth, wsPlayerUrl, playerSettings } from './api'
import { type ResolvedSettings } from './store'
import {Login} from './pages/Login'
import {Lobby} from './pages/Lobby'
import {Room} from './pages/Room'
import {Game} from './pages/Game'
import {Admin} from './pages/Admin'
import {RateLimited} from './pages/RateLimited'
import {SessionHistory} from './pages/SessionHistory'
import { emitErrorLog } from './telemetry'
import { ToastProvider } from './components/ui/Toast'
import { lazyNamed } from './utils/lazyNamed'

const TestError = lazyNamed(() => import('./pages/TestError'), 'TestError')

const SETTINGS_CACHE_KEY = 'tf:settings'

// ---------------------------------------------------------------------------
// Settings helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Error boundary
// ---------------------------------------------------------------------------

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  error: Error | null
}

class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info.componentStack)
    emitErrorLog(error.message, {
      'error.type': 'react.boundary',
      'error.stack': error.stack ?? '',
      'error.component': info.componentStack ?? '',
    })
  }

  render() {
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

// ---------------------------------------------------------------------------
// Auth guards
// ---------------------------------------------------------------------------

function RequireAuth({ children }: { children: ReactNode }) {
  const player = useAppStore(s => s.player)
  if (!player) return <Navigate to='/login' replace />
  return <>{children}</>
}

function RequireRole({ role, children }: { role: 'manager' | 'owner'; children: ReactNode }) {
  const player = useAppStore(s => s.player)
  const hierarchy = { player: 0, manager: 1, owner: 2 }
  if (!player || hierarchy[player.role] < hierarchy[role]) {
    return <Navigate to='/' replace />
  }
  return <>{children}</>
}

// ---------------------------------------------------------------------------
// App
// ---------------------------------------------------------------------------

export default function App() {
  const {
    player,
    setPlayer,
    connectPlayerSocket,
    disconnectPlayerSocket,
    hydrateSettings,
    setSettings,
  } = useAppStore()
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    auth
      .me()
      .then(p => {
        setPlayer(p)
        connectPlayerSocket(wsPlayerUrl(p.id))

        // 1. Hydrate from localStorage cache immediately — no flash.
        const cached = loadSettingsFromCache()
        if (cached) {
          setSettings(cached)
        }

        // 2. Fetch from backend in background — updates store and refreshes cache.
        // Runs regardless of cache hit so cross-device changes are picked up.
        playerSettings
          .get(p.id)
          .then(raw => {
            hydrateSettings(raw)
            saveSettingsToCache(useAppStore.getState().settings)
          })
          .catch(() => {
            // Settings fetch failure is non-fatal — cached or default values remain.
          })
      })
      .catch(() => setPlayer(null))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    if (!player) {
      disconnectPlayerSocket()
    }
  }, [player])

  if (loading) return <Splash />

  return (
    <BrowserRouter>
      <ToastProvider>
        <ErrorBoundary>
          <Routes>
            <Route path='/login' element={player ? <Navigate to='/' replace /> : <Login />} />
            <Route
              path='/'
              element={
                <RequireAuth>
                  <Lobby />
                </RequireAuth>
              }
            />
            <Route
              path='/rooms/:roomId'
              element={
                <RequireAuth>
                  <Room />
                </RequireAuth>
              }
            />
            <Route
              path='/game/:sessionId'
              element={
                <RequireAuth>
                  <Game />
                </RequireAuth>
              }
            />
            <Route
              path='/sessions/:sessionId/history'
              element={
                <RequireAuth>
                  <SessionHistory />
                </RequireAuth>
              }
            />
            <Route
              path='/admin'
              element={
                <RequireAuth>
                  <RequireRole role='manager'>
                    <Admin />
                  </RequireRole>
                </RequireAuth>
              }
            />
            <Route path='/rate-limited' element={<RateLimited />} />
            {import.meta.env.VITE_TEST_MODE === 'true' && (
              <Route path='/test/error' element={<TestError />} />
            )}
            <Route path='*' element={<Navigate to='/' replace />} />
          </Routes>
        </ErrorBoundary>
      </ToastProvider>
    </BrowserRouter>
  )
}

// ---------------------------------------------------------------------------
// Splash
// ---------------------------------------------------------------------------

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
