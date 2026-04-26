import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useFlag, useFlagsStatus } from '@unleash/proxy-client-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { queue } from '@/features/lobby/api'
import { rooms } from '@/features/room/api'
import { Flags } from '@/lib/flags'
import { keys } from '@/lib/queryClient'
import type { GameInfo } from '@/lib/schema-generated.zod'
import { sfx } from '@/lib/sfx'
import { useQueueStore } from '@/stores/queueStore'
import { useRoomStore } from '@/stores/roomStore'
import { useSocketStore } from '@/stores/socketStore'
import { useAppStore } from '@/stores/store'
import { testId } from '@/utils/testId'
import styles from './NewGamePanel.module.css'

type Tab = 'casual' | 'ranked'

interface Props {
  gameList: GameInfo[]
  effectiveGame: string
  onGameChange: (id: string) => void
  disabled?: boolean
}

export function NewGamePanel({ gameList, effectiveGame, onGameChange, disabled }: Props) {
  const player = useAppStore(s => s.player)!
  const gateway = useSocketStore(s => s.gateway)
  const queueStatus = useQueueStore(s => s.queueStatus)
  const queueJoinedAt = useQueueStore(s => s.queueJoinedAt)
  const setQueued = useQueueStore(s => s.setQueued)
  const setMatchFound = useQueueStore(s => s.setMatchFound)
  const clearQueue = useQueueStore(s => s.clearQueue)
  const handleMatchReady = useRoomStore(s => s.handleMatchReady)

  const { t } = useTranslation()
  const navigate = useNavigate()
  const qc = useQueryClient()

  // Default-on gate: cold-start treats ranked as enabled to avoid flashing a
  // disabled tab during the first Unleash fetch. The app-wide explanation
  // lives in SystemBanner; this hook only drives local UI (tab enabled,
  // Find Match button enabled).
  const { flagsReady } = useFlagsStatus()
  const rankedFlag = useFlag(Flags.RankedMatchmakingEnabled)
  const rankedEnabled = !flagsReady || rankedFlag

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
    if (!gateway) return

    const off = gateway.on(event => {
      if (event.type === 'match_found') {
        const payload = event.payload
        sfx.play('queue.match_found')
        setMatchFound(payload.match_id)
      }

      if (event.type === 'match_cancelled') {
        clearQueue()
      }

      if (event.type === 'match_ready') {
        const payload = event.payload
        handleMatchReady(payload.room_id)
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
  }, [gateway, setMatchFound, clearQueue, navigate, setQueued])

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
    mutationFn: () => rooms.join(joinCode.trim().toUpperCase()),
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

  const matchId = useQueueStore(s => s.matchId)

  const acceptMatch = useMutation({
    mutationFn: () => queue.accept(player.id, matchId!),
  })

  const declineMatch = useMutation({
    mutationFn: () => queue.decline(player.id, matchId!),
    onSuccess: () => clearQueue(),
  })

  const error =
    createRoom.error?.message ||
    joinRoom.error?.message ||
    joinQueue.error?.message ||
    leaveQueue.error?.message ||
    acceptMatch.error?.message ||
    declineMatch.error?.message ||
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
      <h2 className={styles.title}>{t('lobby.newGame')}</h2>

      {/* Game selector — always visible when at least one game is playable.
          With a single game (e.g. after a feature-flag disable) we still show
          the chip so the user knows which game they're about to create. */}
      {gameList.length > 0 && (
        <div className={styles.gameSelector}>
          {gameList.map((g: GameInfo) => (
            <button
              type='button'
              key={g.id}
              {...testId(`game-option-${g.id}`)}
              className={`${styles.gameOption} ${effectiveGame === g.id ? styles.gameOptionActive : ''}`}
              onClick={() => onGameChange(g.id)}
            >
              {g.name}
              <span className={styles.playerCount}>
                {t('lobby.playerCount', { min: g.min_players, max: g.max_players })}
              </span>
            </button>
          ))}
        </div>
      )}

      {/* Tabs */}
      <div className={styles.tabs}>
        <button
          type='button'
          {...testId('tab-casual')}
          className={`${styles.tab} ${tab === 'casual' ? styles.tabActive : ''}`}
          onClick={() => setTab('casual')}
        >
          {t('lobby.casual')}
        </button>
        <button
          type='button'
          {...testId('tab-ranked')}
          className={`${styles.tab} ${tab === 'ranked' ? styles.tabActive : ''}`}
          onClick={() => setTab('ranked')}
          disabled={!rankedEnabled}
          title={rankedEnabled ? undefined : t('systemStatus.rankedPausedTitle')}
        >
          {t('lobby.ranked')}
        </button>
      </div>

      <div className={styles.body}>
        {tab === 'casual' && (
          <div className={styles.casualBody}>
            <button
              type='button'
              {...testId('create-room-btn')}
              className='btn btn-primary'
              onClick={() => createRoom.mutate()}
              disabled={disabled || createRoom.isPending || !effectiveGame}
            >
              {createRoom.isPending ? t('lobby.creatingRoom') : `+ ${t('lobby.createRoom')}`}
            </button>

            <div className={styles.joinRow}>
              <input
                {...testId('join-code-input')}
                className='input'
                aria-label={t('lobby.enterCode')}
                placeholder={t('lobby.enterCode')}
                value={joinCode}
                onChange={e => setJoinCode(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && joinRoom.mutate()}
                maxLength={6}
                style={{ textTransform: 'uppercase', letterSpacing: '0.15em' }}
              />
              <button
                type='button'
                {...testId('join-btn')}
                className='btn btn-ghost'
                onClick={() => joinRoom.mutate()}
                disabled={disabled || joinRoom.isPending}
              >
                {t('lobby.join')}
              </button>
            </div>
          </div>
        )}

        {tab === 'ranked' && (
          <div className={styles.rankedBody}>
            {queueStatus === 'idle' && (
              <button
                type='button'
                {...testId('find-match-btn')}
                className='btn btn-primary'
                onClick={() => joinQueue.mutate()}
                disabled={disabled || joinQueue.isPending || !effectiveGame || !rankedEnabled}
              >
                {joinQueue.isPending ? t('lobby.joiningMatch') : t('lobby.findMatch')}
              </button>
            )}

            {queueStatus === 'queued' && (
              <div {...testId('queue-searching')} className={styles.queueStatus}>
                <div className={styles.queueTop}>
                  <span className={styles.queueLabel}>
                    <span className={styles.spinner} />
                    {t('lobby.searchingForOpponent')}
                  </span>
                  <span className={styles.queueTimer}>{formatElapsed(elapsed)}</span>
                </div>
                <button
                  type='button'
                  {...testId('cancel-queue-btn')}
                  className='btn btn-ghost'
                  onClick={() => leaveQueue.mutate()}
                  disabled={leaveQueue.isPending}
                  style={{ width: '100%', marginTop: 8 }}
                >
                  {t('common.cancel')}
                </button>
              </div>
            )}

            {queueStatus === 'match_found' && (
              <div {...testId('match-found')} className={styles.matchFound}>
                <span className={styles.matchFoundLabel}>{t('lobby.matchFound')}</span>
                <div className={styles.matchActions}>
                  <button
                    type='button'
                    {...testId('accept-match-btn')}
                    className='btn btn-primary'
                    onClick={() => acceptMatch.mutate()}
                    disabled={acceptMatch.isPending || declineMatch.isPending}
                  >
                    {t('lobby.acceptMatch')}
                  </button>
                  <button
                    type='button'
                    {...testId('decline-match-btn')}
                    className='btn btn-ghost'
                    onClick={() => declineMatch.mutate()}
                    disabled={acceptMatch.isPending || declineMatch.isPending}
                  >
                    {t('lobby.declineMatch')}
                  </button>
                </div>
              </div>
            )}

            <p className={styles.rankedNote}>
              {t('lobby.rankedNote')}{' '}
              <span className={styles.ratingWord}>{t('lobby.displayRating')}</span>.
            </p>
          </div>
        )}
      </div>

      {error && <p className={styles.error}>{error}</p>}
    </section>
  )
}
