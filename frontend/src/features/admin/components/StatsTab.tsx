import { useCallback, useEffect, useRef, useState } from 'react'
import { admin, type SystemStats } from '@/features/admin/api'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

export function StatsTab() {
  const toast = useToast()
  const [stats, setStats] = useState<SystemStats | null>(null)
  const [loading, setLoading] = useState(true)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchStats = useCallback(() => {
    admin
      .getStats()
      .then(setStats)
      .catch(e => toast.showError(catchToAppError(e)))
      .finally(() => setLoading(false))
  }, [toast.showError])

  useEffect(() => {
    fetchStats()
    intervalRef.current = setInterval(fetchStats, 30_000)
    return () => clearInterval(intervalRef.current)
  }, [fetchStats])

  if (loading) {
    return <p className={styles.empty}>Loading...</p>
  }

  if (!stats) {
    return <p className={styles.empty} {...testId('stats-empty')}>No stats available.</p>
  }

  const cards: { label: string; value: number; id: string }[] = [
    { label: 'Online Players', value: stats.online_players, id: 'online-players' },
    { label: 'Active Rooms', value: stats.active_rooms, id: 'active-rooms' },
    { label: 'Active Sessions', value: stats.active_sessions, id: 'active-sessions' },
    { label: 'Total Players', value: stats.total_players, id: 'total-players' },
    { label: 'Sessions Today', value: stats.total_sessions_today, id: 'sessions-today' },
  ]

  return (
    <div className={styles.panel} {...testId('stats-panel')}>
      <div className={styles.statsGrid}>
        {cards.map(c => (
          <div key={c.id} className={`card ${styles.statCard}`} {...testId(`stat-${c.id}`)}>
            <span className={styles.statLabel}>{c.label}</span>
            <span className={styles.statValue}>{c.value}</span>
          </div>
        ))}
      </div>
      <p className={styles.muted} style={{ textAlign: 'center', fontSize: 'var(--text-xs)' }}>
        Auto-refreshes every 30s
      </p>
    </div>
  )
}
