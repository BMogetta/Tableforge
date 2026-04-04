import { useCallback, useEffect, useState } from 'react'
import { admin } from '@/features/admin/api'
import type { PlayerReport } from '@/lib/schema-generated.zod'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

type FilterStatus = 'pending' | 'reviewed' | 'all'

interface Props {
  callerRole: string
  onBanPlayer?: (playerId: string) => void
}

export function ModerationTab({ callerRole, onBanPlayer }: Props) {
  const toast = useToast()
  const [reports, setReports] = useState<PlayerReport[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<FilterStatus>('pending')
  const [selected, setSelected] = useState<PlayerReport | null>(null)

  const fetchReports = useCallback(() => {
    setLoading(true)
    admin
      .listReports(filter)
      .then(setReports)
      .catch(e => toast.showError(catchToAppError(e)))
      .finally(() => setLoading(false))
  }, [filter])

  useEffect(() => {
    fetchReports()
  }, [fetchReports])

  async function handleReview(reportId: string, resolution: string) {
    try {
      await admin.reviewReport(reportId, resolution)
      setReports(prev => prev.filter(r => r.id !== reportId))
      setSelected(null)
    } catch (e) {
      toast.showError(catchToAppError(e))
    }
  }

  function handleBan(playerId: string) {
    if (onBanPlayer) onBanPlayer(playerId)
  }

  const pendingCount = reports.filter(r => r.status === 'pending').length

  return (
    <div className={styles.panel} {...testId('moderation-panel')}>
      <div className={styles.toolbar}>
        <div className={styles.filterGroup}>
          {(['pending', 'reviewed', 'all'] as FilterStatus[]).map(s => (
            <button
              key={s}
              className={`btn btn-sm ${filter === s ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => setFilter(s)}
              {...testId(`filter-${s}`)}
            >
              {s}
              {s === 'pending' && pendingCount > 0 && (
                <span className={styles.badge}>{pendingCount}</span>
              )}
            </button>
          ))}
        </div>
      </div>

      {selected ? (
        <div className={`card ${styles.detailCard}`} {...testId('report-detail')}>
          <div className={styles.detailHeader}>
            <h3>Report Detail</h3>
            <button className='btn btn-ghost btn-sm' onClick={() => setSelected(null)}>
              Back
            </button>
          </div>
          <dl className={styles.detailList}>
            <dt>Reporter</dt>
            <dd>{selected.reporter_id}</dd>
            <dt>Reported</dt>
            <dd>{selected.reported_id}</dd>
            <dt>Reason</dt>
            <dd>{selected.reason}</dd>
            <dt>Status</dt>
            <dd>
              <span className={styles.statusBadge} data-status={selected.status}>
                {selected.status}
              </span>
            </dd>
            <dt>Date</dt>
            <dd>{new Date(selected.created_at).toLocaleString()}</dd>
          </dl>
          {selected.status === 'pending' && (
            <div className={styles.actionRow}>
              <button
                className='btn btn-ghost btn-sm'
                onClick={() => handleReview(selected.id, 'dismiss')}
                {...testId('dismiss-report-btn')}
              >
                Dismiss
              </button>
              <button
                className='btn btn-secondary btn-sm'
                onClick={() => handleReview(selected.id, 'warn')}
                {...testId('warn-report-btn')}
              >
                Warn
              </button>
              {(callerRole === 'manager' || callerRole === 'owner') && (
                <button
                  className='btn btn-danger btn-sm'
                  onClick={() => handleBan(selected.reported_id)}
                  {...testId('ban-from-report-btn')}
                >
                  Ban
                </button>
              )}
            </div>
          )}
        </div>
      ) : loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : reports.length === 0 ? (
        <p className={styles.empty} {...testId('moderation-empty')}>
          No reports found.
        </p>
      ) : (
        <table className={styles.table} {...testId('reports-table')}>
          <thead>
            <tr>
              <th>Reporter</th>
              <th>Reported</th>
              <th>Reason</th>
              <th>Status</th>
              <th>Date</th>
            </tr>
          </thead>
          <tbody>
            {reports.map(r => (
              <tr
                key={r.id}
                className={styles.clickableRow}
                onClick={() => setSelected(r)}
                {...testId(`report-row-${r.id}`)}
              >
                <td className={styles.muted}>{r.reporter_id}</td>
                <td>{r.reported_id}</td>
                <td>{r.reason}</td>
                <td>
                  <span className={styles.statusBadge} data-status={r.status}>
                    {r.status}
                  </span>
                </td>
                <td className={styles.muted}>{new Date(r.created_at).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
