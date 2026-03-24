import { useState, useEffect, useRef } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { rooms, queue, type GameInfo } from '@/lib/api'
import { useAppStore } from '@/stores/store'
import { keys } from '@/lib/queryClient'
import styles from './NewGamePanel.module.css'
import { useNavigate } from '@tanstack/react-router'

type Tab = 'casual' | 'ranked'

interface Props {
  gameList: GameInfo[]
  effectiveGame: string
  onGameChange: (id: string) => void
}

export function NewGamePanel({ gameList, effectiveGame, onGameChange }: Props) {
  const player = useAppStore(s => s.player)!
  const playerSocket = useAppStore(s => s.playerSocket)
  const queueStatus = useAppStore(s => s.queueStatus)
  const queueJoinedAt = useAppStore(s => s.queueJoinedAt)
  const setQueued = useAppStore(s => s.setQueued)
  const setMatchFound = useAppStore(s => s.setMatchFound)
  const clearQueue = useAppStore(s => s.clearQueue)

  const navigate = useNavigate()
  const qc = useQueryClient()

  const [tab, setTab] = useState<Tab>('casual')
  const [joinCode, setJoinCode] = useState('')
  const [elapsed, setElapsed] = useState(0)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Tick elapsed time while queued
  useEffect(() => {
    if (queueStatus === 'queued' && queueJoinedAt) {
      timerRef.current = setInterval(() => {
        setElapsed(Math.floor((Date.now() - queueJoinedAt) / 1000))
      }, 1000)
    } else {
      if (timerRef.current) clearInterval(timerRef.current)
      setElapsed(0)
    }
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [queueStatus, queueJoinedAt])

  // Listen for queue events on the player channel
  useEffect(() => {
    if (!playerSocket) return

    const off = playerSocket.on(event => {
      if (event.type === 'match_found') {
        const payload = event.payload
        setMatchFound(payload.match_id)
      }

      if (event.type === 'match_cancelled') {
        clearQueue()
      }

      if (event.type === 'match_ready') {
        const payload = event.payload
        clearQueue()
        navigate({
          to: '/game/$sessionId',
          params: { sessionId: payload.session_id },
        })
      }

      if (event.type === 'queue_left') {
        const payload = event.payload
        if (payload.reason !== 'opponent_declined') {
          clearQueue()
        }
      }

      if (event.type === 'queue_joined') {
        const payload = event.payload
        if (payload.reason === 'opponent_declined') {
          setQueued(Date.now())
        }
      }
    })

    return () => off()
  }, [playerSocket, setMatchFound, clearQueue, navigate, setQueued])

  const createRoom = useMutation({
    mutationFn: () => rooms.create(effectiveGame, player.id),
    onSuccess: view => {
      qc.invalidateQueries({ queryKey: keys.rooms() })
      navigate({
        to: '/rooms/$roomId',
        params: { roomId: view.room.id },
      })
    },
  })

  const joinRoom = useMutation({
    mutationFn: () => rooms.join(joinCode.trim().toUpperCase(), player.id),
    onSuccess: view =>
      navigate({
        to: '/rooms/$roomId',
        params: { roomId: view.room.id },
      }),
  })

  const joinQueue = useMutation({
    mutationFn: () => queue.join(player.id),
    onSuccess: () => setQueued(Date.now()),
  })

  const leaveQueue = useMutation({
    mutationFn: () => queue.leave(player.id),
    onSuccess: () => clearQueue(),
  })

  const error =
    createRoom.error?.message ||
    joinRoom.error?.message ||
    joinQueue.error?.message ||
    leaveQueue.error?.message ||
    ''

  const formatElapsed = (secs: number) => {
    const m = Math.floor(secs / 60)
      .toString()
      .padStart(2, '0')
    const s = (secs % 60).toString().padStart(2, '0')
    return `${m}:${s}`
  }

  return (
    <section className={styles.section}>
      <h2 className={styles.title}>New Game</h2>

      {/* Game selector — shown for both tabs when multiple games exist */}
      {gameList.length > 1 && (
        <div className={styles.gameSelector}>
          {gameList.map((g: GameInfo) => (
            <button
              key={g.id}
              data-testid={`game-option-${g.id}`}
              className={`${styles.gameOption} ${effectiveGame === g.id ? styles.gameOptionActive : ''}`}
              onClick={() => onGameChange(g.id)}
            >
              {g.name}
              <span className={styles.playerCount}>
                {g.min_players}–{g.max_players}p
              </span>
            </button>
          ))}
        </div>
      )}

      {/* Tabs */}
      <div className={styles.tabs}>
        <button
          className={`${styles.tab} ${tab === 'casual' ? styles.tabActive : ''}`}
          onClick={() => setTab('casual')}
        >
          Casual
        </button>
        <button
          className={`${styles.tab} ${tab === 'ranked' ? styles.tabActive : ''}`}
          onClick={() => setTab('ranked')}
        >
          Ranked
        </button>
      </div>

      <div className={styles.body}>
        {tab === 'casual' && (
          <div className={styles.casualBody}>
            <button
              data-testid='create-room-btn'
              className='btn btn-primary'
              onClick={() => createRoom.mutate()}
              disabled={createRoom.isPending || !effectiveGame}
            >
              {createRoom.isPending ? 'Creating...' : '+ Create Room'}
            </button>

            <div className={styles.joinRow}>
              <input
                data-testid='join-code-input'
                className='input'
                placeholder='Room code'
                value={joinCode}
                onChange={e => setJoinCode(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && joinRoom.mutate()}
                maxLength={6}
                style={{ textTransform: 'uppercase', letterSpacing: '0.15em' }}
              />
              <button
                data-testid='join-btn'
                className='btn btn-ghost'
                onClick={() => joinRoom.mutate()}
                disabled={joinRoom.isPending}
              >
                Join
              </button>
            </div>
          </div>
        )}

        {tab === 'ranked' && (
          <div className={styles.rankedBody}>
            {queueStatus === 'idle' && (
              <button
                className='btn btn-primary'
                onClick={() => joinQueue.mutate()}
                disabled={joinQueue.isPending || !effectiveGame}
              >
                {joinQueue.isPending ? 'Joining...' : 'Find Match'}
              </button>
            )}

            {queueStatus === 'queued' && (
              <div className={styles.queueStatus}>
                <div className={styles.queueTop}>
                  <span className={styles.queueLabel}>
                    <span className={styles.spinner} />
                    Searching for opponent
                  </span>
                  <span className={styles.queueTimer}>{formatElapsed(elapsed)}</span>
                </div>
                <button
                  className='btn btn-ghost'
                  onClick={() => leaveQueue.mutate()}
                  disabled={leaveQueue.isPending}
                  style={{ width: '100%', marginTop: 8 }}
                >
                  Cancel
                </button>
              </div>
            )}

            {queueStatus === 'match_found' && (
              <div className={styles.matchFound}>
                <span className={styles.matchFoundLabel}>⚔ Match found!</span>
                <p className={styles.matchFoundHint}>Check your notifications to accept.</p>
              </div>
            )}

            <p className={styles.rankedNote}>
              Ranked games affect your <span className={styles.ratingWord}>display rating</span>.
            </p>
          </div>
        )}
      </div>

      {error && <p className={styles.error}>{error}</p>}
    </section>
  )
}
