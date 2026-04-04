import { useEffect, useRef, useState } from 'react'
import { useAppStore } from '@/stores/store'
import { sessions } from '@/lib/api/sessions'
import { loadAssets, type LoadProgress } from '@/lib/assets'
import { getGameAssets } from '@/games/assets'
import styles from './GameLoading.module.css'

// Minimum time to show the loading screen — prevents flash if assets load instantly.
const MIN_LOADING_MS = 3_000
// Timeout duration must match the server's DefaultReadyTimeout (60s).
const READY_TIMEOUT_SECS = 60

interface Props {
  sessionId: string
  gameId: string
  onReady: () => void
  onTimeout: () => void
}

type Phase = 'loading' | 'waiting' | 'done'

export function GameLoading({ sessionId, gameId, onReady, onTimeout }: Props) {
  const player = useAppStore(s => s.player)!
  const socket = useAppStore(s => s.socket)
  const isSpectator = useAppStore(s => s.isSpectator)

  const [phase, setPhase] = useState<Phase>('loading')
  const [progress, setProgress] = useState<LoadProgress>({ loaded: 0, total: 0, progress: 0 })
  const [countdown, setCountdown] = useState(READY_TIMEOUT_SECS)
  const [waitingPlayers, setWaitingPlayers] = useState<string[]>([])
  const [required, setRequired] = useState(0)

  const sentReady = useRef(false)

  // Phase 1 — load assets + enforce minimum display time.
  useEffect(() => {
    // Spectators don't participate in ready-check — skip straight to game.
    if (isSpectator) {
      setPhase('done')
      onReady()
      return
    }

    let cancelled = false
    const startedAt = Date.now()
    const assets = getGameAssets(gameId)

    loadAssets(assets, p => {
      if (!cancelled) setProgress(p)
    }).then(async () => {
      if (cancelled) return
      // Enforce minimum display time.
      const elapsed = Date.now() - startedAt
      const remaining = MIN_LOADING_MS - elapsed
      if (remaining > 0) {
        await new Promise(r => setTimeout(r, remaining))
      }
      if (cancelled) return
      setProgress({ loaded: 1, total: 1, progress: 1 })
      setPhase('waiting')

      // Send ready confirmation to server.
      if (!sentReady.current) {
        sentReady.current = true

        sessions
          .ready(sessionId)
          .then(res => {
            if (res.all_ready) {
              setPhase('done')
              onReady()
            }
          })
          .catch(() => {})
      }
    })

    return () => {
      cancelled = true
    }
  }, [sessionId, gameId, player.id, isSpectator])

  // Phase 2 — listen for game_ready WS event.
  useEffect(() => {
    if (!socket || phase !== 'waiting') return

    const off = socket.on(event => {
      if (event.type === 'player_ready') {
        setWaitingPlayers(event.payload.ready_players)
        setRequired(event.payload.required)
      }
      if (event.type === 'game_ready') {
        setPhase('done')
        onReady()
      }
      if (event.type === 'game_over') {
        // Server forfeited someone due to ready_timeout.
        onTimeout()
      }
    })

    return () => off()
  }, [socket, phase, onReady, onTimeout])

  // Countdown timer shown during waiting phase.
  useEffect(() => {
    if (phase !== 'waiting') return

    const interval = setInterval(() => {
      setCountdown(c => {
        if (c <= 1) {
          clearInterval(interval)
          return 0
        }
        return c - 1
      })
    }, 1_000)

    return () => clearInterval(interval)
  }, [phase])

  if (phase === 'loading') {
    const pct = Math.round(progress.progress * 100)
    return (
      <div className={styles.root}>
        <div className={styles.inner}>
          <p className={styles.title}>{gameId}</p>
          <div className={styles.progressBar}>
            <div className={styles.progressFill} style={{ width: `${pct}%` }} />
          </div>
          <p className={styles.label}>Loading{progress.total > 0 ? ` ${pct}%` : '...'}</p>
        </div>
      </div>
    )
  }

  return (
    <div className={styles.root}>
      <div className={styles.inner}>
        <p className={styles.title}>{gameId}</p>
        <p className={styles.waiting}>Waiting for opponent...</p>
        {required > 0 && (
          <p className={styles.label}>
            {waitingPlayers.length} / {required} ready
          </p>
        )}
        <p className={styles.countdown}>{countdown}s</p>
        <button
          className='btn btn-ghost'
          style={{ marginTop: 16, fontSize: 12 }}
          onClick={onTimeout}
        >
          ← Back to Lobby
        </button>
      </div>
    </div>
  )
}
