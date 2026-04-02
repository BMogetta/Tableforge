import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useAppStore } from '../stores/store'
import { SessionEventDTO } from '@/lib/api-generated'
import { keys } from '@/lib/queryClient'
import { TicTacToeBoard, type TicTacToeState } from '../games/tictactoe/components/TicTacToe'
import styles from './SessionHistory.module.css'
import { useNavigate } from '@tanstack/react-router'
import { sessions } from '@/lib/api/sessions'
import { MoveDTO } from '@/lib/api-generated'
import { testId } from '@/utils/testId'

// --- Helpers -----------------------------------------------------------------

const EVENT_LABELS: Record<string, string> = {
  game_started: 'Game started',
  move_applied: 'Move played',
  game_over: 'Game over',
  turn_timeout: 'Turn timed out',
  player_connected: 'Player connected',
  player_disconnected: 'Player disconnected',
  spectator_joined: 'Spectator joined',
  spectator_left: 'Spectator left',
  player_surrendered: 'Player surrendered',
  rematch_voted: 'Rematch voted',
  session_suspended: 'Session suspended',
  session_resumed: 'Session resumed',
}

const EVENT_ACCENT: Record<string, string> = {
  game_started: 'var(--amber)',
  game_over: 'var(--amber)',
  move_applied: 'var(--text)',
  turn_timeout: 'var(--danger)',
  player_surrendered: 'var(--danger)',
  player_connected: 'var(--success)',
  player_disconnected: 'var(--text-muted)',
  spectator_joined: 'var(--text-muted)',
  spectator_left: 'var(--text-muted)',
  rematch_voted: 'var(--text)',
}

function formatTime(iso: string | undefined): string | null {
  if (!iso) return null
  const d = new Date(iso)
  return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function formatRelative(iso: string, baseIso: string): string {
  const ms = new Date(iso).getTime() - new Date(baseIso).getTime()
  if (ms < 0) return '+0.0s'
  const s = (ms / 1000).toFixed(1)
  return `+${s}s`
}

// --- Components --------------------------------------------------------------

function EventRow({ event, base, index }: { event: SessionEventDTO; base: string; index: number }) {
  const [open, setOpen] = useState(false)
  const label = EVENT_LABELS[event.event_type] ?? event.event_type
  const accent = EVENT_ACCENT[event.event_type] ?? 'var(--text-muted)'
  const hasPayload = event.payload && Object.keys(event.payload).length > 0

  return (
    <div
      className={`${styles.eventRow} ${open ? styles.eventRowOpen : ''}`}
      style={{ animationDelay: `${index * 30}ms` }}
      {...testId('event-row')}
    >
      <div className={styles.eventMeta}>
        <span className={styles.eventIndex}>{String(index + 1).padStart(3, '0')}</span>
        <span className={styles.eventDot} style={{ background: accent }} />
        <span className={styles.eventTime}>{formatRelative(event.occurred_at, base)}</span>
        <span className={styles.eventLabel} style={{ color: accent }}>
          {label}
        </span>
        {hasPayload && (
          <button
            className={styles.eventToggle}
            onClick={() => setOpen(o => !o)}
            aria-label='toggle payload'
            {...testId('event-toggle')}
          >
            {open ? '▲' : '▼'}
          </button>
        )}
      </div>
      {open && hasPayload && (
        <pre className={styles.eventPayload} {...testId('event-payload')}>
          {JSON.stringify(event.payload, null, 2)}
        </pre>
      )}
    </div>
  )
}

// --- Replay ------------------------------------------------------------------

function ReplayView({ moves, gameId }: { moves: MoveDTO[]; gameId: string }) {
  const [step, setStep] = useState(0)

  if (moves.length === 0) {
    return (
      <div className={styles.replayEmpty}>
        <span>No moves recorded for this session.</span>
      </div>
    )
  }

  const currentMove = moves[step - 1] ?? null
  const stateAfter = currentMove
    ? (() => {
        const raw = (currentMove as MoveDTO & { state_after?: string }).state_after
        if (!raw) return null
        try {
          return JSON.parse(atob(raw)) as { current_player_id: string; data: unknown }
        } catch {
          return null
        }
      })()
    : null
  const boardState: TicTacToeState | undefined =
    step === 0 && gameId === 'tictactoe'
      ? { board: Array(9).fill('') as string[], marks: {}, players: [] }
      : (stateAfter?.data as TicTacToeState | undefined)

  return (
    <div className={styles.replayRoot}>
      <div className={styles.replayHeader}>
        <span className={styles.replayStep} {...testId('replay-step-label')}>
          {step === 0 ? 'Initial state' : `Move ${step} of ${moves.length}`}
        </span>
        {currentMove && (
          <span className={styles.replayTime}>
            {new Date(currentMove.applied_at).toLocaleTimeString()}
          </span>
        )}
      </div>

      <div className={styles.replayBoard}>
        {gameId === 'tictactoe' && boardState ? (
          <TicTacToeBoard
            state={boardState}
            currentPlayerId=''
            localPlayerId=''
            onMove={() => {}}
            disabled={true}
          />
        ) : (
          <div className={styles.replayNoRenderer}>
            <span>
              No visual renderer for <strong>{gameId}</strong>.
            </span>
            {stateAfter && (
              <pre className={styles.replayStateJson}>{JSON.stringify(stateAfter, null, 2)}</pre>
            )}
          </div>
        )}
      </div>

      <div className={styles.replayControls}>
        <button
          className={`btn btn-ghost ${styles.replayBtn}`}
          onClick={() => setStep(0)}
          disabled={step === 0}
        >
          ⏮
        </button>
        <button
          className={`btn btn-ghost ${styles.replayBtn}`}
          onClick={() => setStep(s => Math.max(0, s - 1))}
          disabled={step === 0}
        >
          ←
        </button>

        <div className={styles.replaySliderWrap}>
          <input
            type='range'
            aria-label='Replay step'
            min={0}
            max={moves.length}
            value={step}
            onChange={e => setStep(Number(e.target.value))}
            className={styles.replaySlider}
          />
          <div
            className={styles.replaySliderFill}
            style={{ width: `${(step / moves.length) * 100}%` }}
          />
        </div>

        <button
          className={`btn btn-ghost ${styles.replayBtn}`}
          {...testId('replay-next-btn')}
          onClick={() => setStep(s => Math.min(moves.length, s + 1))}
          disabled={step === moves.length}
        >
          →
        </button>
        <button
          className={`btn btn-ghost ${styles.replayBtn}`}
          {...testId('replay-last-btn')}
          onClick={() => setStep(moves.length)}
          disabled={step === moves.length}
        >
          ⏭
        </button>
      </div>
    </div>
  )
}

// --- Page --------------------------------------------------------------------

type Tab = 'events' | 'replay'

export function SessionHistory({ sessionId }: { sessionId: string }) {
  const navigate = useNavigate()
  const player = useAppStore(s => s.player)!
  const [tab, setTab] = useState<Tab>('events')

  const { data: sessionData, isLoading: sessionLoading } = useQuery({
    queryKey: keys.session(sessionId!),
    queryFn: () => sessions.get(sessionId!),
    staleTime: 0,
    gcTime: 0,
  })

  const { data: eventsData, isLoading: eventsLoading } = useQuery({
    queryKey: ['session-events', sessionId],
    queryFn: () => sessions.events(sessionId!),
    staleTime: 60_000,
  })

  const { data: movesData, isLoading: movesLoading } = useQuery({
    queryKey: ['session-history', sessionId],
    queryFn: () => sessions.history(sessionId!),
    staleTime: 60_000,
  })

  const isLoading = sessionLoading || eventsLoading || movesLoading
  const session = sessionData?.session
  const result = sessionData?.result
  const events = eventsData ?? []
  const moves = movesData ?? []
  const baseTime = events[0]?.occurred_at ?? session?.started_at ?? new Date().toISOString()

  // Resolve winner name from player list
  const isWinner = result?.winner_id === player.id
  const isDraw = result?.is_draw ?? false

  return (
    <div className={`${styles.root} page-enter`}>
      <div className={styles.container}>
        {/* Header */}
        <header className={styles.header}>
          <button
            className='btn btn-ghost'
            {...testId('back-to-lobby-btn')}
            onClick={() => navigate({ to: '/' })}
            style={{ fontSize: 11 }}
          >
            ← Lobby
          </button>
          <div className={styles.headerCenter}>
            <span className={styles.gameLabel}>{session?.game_id ?? '—'}</span>
            <span className={styles.sessionId}>{sessionId?.slice(0, 8).toUpperCase()}</span>
          </div>
          {session?.finished_at && (
            <div
              className={styles.resultBadge}
              {...testId('result-badge')}
              data-result={isWinner ? 'win' : isDraw ? 'draw' : 'loss'}
            >
              {isWinner ? 'WIN' : isDraw ? 'DRAW' : result?.winner_id ? 'LOSS' : 'ENDED'}
            </div>
          )}
        </header>

        {/* Stats bar */}
        {session && (
          <div className={styles.statsBar}>
            <div className={styles.stat}>
              <span className={styles.statLabel}>Moves</span>
              <span className={styles.statValue} {...testId('stat-move-count')}>
                {session.move_count}
              </span>
            </div>
            <div className={styles.statDivider} />
            <div className={styles.stat}>
              <span className={styles.statLabel}>Started</span>
              <span className={styles.statValue}>{formatTime(session.started_at)}</span>
            </div>
            {session.finished_at && (
              <>
                <div className={styles.statDivider} />
                <div className={styles.stat}>
                  <span className={styles.statLabel}>Duration</span>
                  <span className={styles.statValue}>
                    {Math.round(
                      (new Date(session.finished_at).getTime() -
                        new Date(session.started_at!).getTime()) /
                        1000,
                    )}
                    s
                  </span>
                </div>
                <div className={styles.statDivider} />
                <div className={styles.stat}>
                  <span className={styles.statLabel}>Ended by</span>
                  <span className={styles.statValue}>{result?.ended_by ?? '—'}</span>
                </div>
              </>
            )}
            <div className={styles.statDivider} />
            <div className={styles.stat}>
              <span className={styles.statLabel}>Events</span>
              <span className={styles.statValue}>{events.length}</span>
            </div>
          </div>
        )}

        {/* Tabs */}
        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${tab === 'events' ? styles.tabActive : ''}`}
            {...testId('tab-events')}
            onClick={() => setTab('events')}
          >
            EVENT LOG
          </button>
          <button
            className={`${styles.tab} ${tab === 'replay' ? styles.tabActive : ''}`}
            {...testId('tab-replay')}
            onClick={() => setTab('replay')}
          >
            REPLAY
          </button>
        </div>

        {/* Content */}
        {isLoading ? (
          <div className={styles.loading}>
            <span className='pulse'>Loading session data…</span>
          </div>
        ) : tab === 'events' ? (
          <div className={styles.eventList}>
            {events.length === 0 ? (
              <div className={styles.empty}>No events recorded for this session.</div>
            ) : (
              events.map((ev, i) => <EventRow key={ev.id} event={ev} base={baseTime} index={i} />)
            )}
          </div>
        ) : (
          <ReplayView moves={moves} gameId={session?.game_id ?? ''} />
        )}
      </div>
    </div>
  )
}
