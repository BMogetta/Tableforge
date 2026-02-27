import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAppStore } from './store'
import { auth } from './api'
import Login from './pages/Login'
import Lobby from './pages/Lobby'
import Room from './pages/Room'
import Game from './pages/Game'

function RequireAuth({ children }: { children: React.ReactNode }) {
  const player = useAppStore((s) => s.player)
  if (!player) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  const { player, setPlayer } = useAppStore()
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    auth.me()
      .then(setPlayer)
      .catch(() => setPlayer(null))
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <Splash />

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={player ? <Navigate to="/" replace /> : <Login />} />
        <Route path="/" element={<RequireAuth><Lobby /></RequireAuth>} />
        <Route path="/rooms/:roomId" element={<RequireAuth><Room /></RequireAuth>} />
        <Route path="/game/:sessionId" element={<RequireAuth><Game /></RequireAuth>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

function Splash() {
  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      height: '100vh',
      flexDirection: 'column',
      gap: 16,
    }}>
      <div style={{ fontFamily: 'var(--font-display)', fontSize: 24, color: 'var(--amber)', letterSpacing: '0.2em' }}>
        TABLEFORGE
      </div>
      <div className="pulse" style={{ color: 'var(--text-muted)', fontSize: 11, letterSpacing: '0.1em' }}>
        CONNECTING...
      </div>
    </div>
  )
}
