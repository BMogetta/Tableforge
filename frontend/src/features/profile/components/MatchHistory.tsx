import type { MatchHistoryEntry } from '@/lib/schema-generated.zod'
import styles from '../Profile.module.css'

interface MatchHistoryProps {
  matches: MatchHistoryEntry[]
  total: number
  page: number
  pageSize: number
  isLoading: boolean
  onPageChange: (page: number) => void
  onViewReplay: (sessionId: string) => void
}

function formatDuration(secs?: number): string {
  if (secs == null) return '—'
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function MatchHistory({
  matches,
  total,
  page,
  pageSize,
  isLoading,
  onPageChange,
  onViewReplay,
}: MatchHistoryProps) {
  if (isLoading) {
    return (
      <div className={styles.loading}>
        <span className='pulse'>Loading matches...</span>
      </div>
    )
  }

  if (matches.length === 0) {
    return <div className={styles.empty}>No matches played yet.</div>
  }

  const totalPages = Math.ceil(total / pageSize)

  return (
    <>
      <div className={styles.matchList}>
        {matches.map((match, i) => (
          <div
            key={match.id}
            className={styles.matchRow}
            style={{ animationDelay: `${i * 30}ms` }}
            onClick={() => onViewReplay(match.session_id)}
            role='button'
            tabIndex={0}
            onKeyDown={e => {
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                onViewReplay(match.session_id)
              }
            }}
          >
            <span className={styles.matchOutcome} data-outcome={match.outcome}>
              {match.outcome}
            </span>
            <span className={styles.matchGame}>{match.game_id}</span>
            <span className={styles.matchMeta}>
              <span>{formatDuration(match.duration_secs)}</span>
              <span>{formatDate(match.created_at)}</span>
            </span>
          </div>
        ))}
      </div>

      {totalPages > 1 && (
        <div className={styles.pagination}>
          <button
            className='btn btn-ghost'
            onClick={() => onPageChange(page - 1)}
            disabled={page === 0}
          >
            ← Prev
          </button>
          <span className={styles.pageInfo}>
            {page + 1} / {totalPages}
          </span>
          <button
            className='btn btn-ghost'
            onClick={() => onPageChange(page + 1)}
            disabled={page >= totalPages - 1}
          >
            Next →
          </button>
        </div>
      )}
    </>
  )
}
