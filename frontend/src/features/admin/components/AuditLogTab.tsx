import { useCallback, useEffect, useState } from 'react'
import { admin, type AuditLog, type AuditLogFilter } from '@/features/admin/api'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

const PAGE_SIZE = 50

const ACTIONS = ['', 'ban_issued', 'ban_lifted', 'report_reviewed', 'email_added', 'email_removed', 'role_changed'] as const
const TARGET_TYPES = ['', 'player', 'email', 'report', 'ban'] as const

export function AuditLogTab() {
  const toast = useToast()
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(true)
  const [action, setAction] = useState('')
  const [targetType, setTargetType] = useState('')
  const [page, setPage] = useState(0)

  const fetchLogs = useCallback(() => {
    setLoading(true)
    const filter: AuditLogFilter = { limit: PAGE_SIZE, offset: page * PAGE_SIZE }
    if (action) filter.action = action
    if (targetType) filter.target_type = targetType

    admin
      .listAuditLogs(filter)
      .then(setLogs)
      .catch(e => toast.showError(catchToAppError(e)))
      .finally(() => setLoading(false))
  }, [action, targetType, page, toast.showError])

  useEffect(() => {
    fetchLogs()
  }, [fetchLogs])

  function handleFilterChange(newAction: string, newTargetType: string) {
    setAction(newAction)
    setTargetType(newTargetType)
    setPage(0)
  }

  return (
    <div className={styles.panel} {...testId('audit-panel')}>
      <div className={styles.toolbar}>
        <div className={styles.filterGroup}>
          <select
            className='input input-sm'
            value={action}
            onChange={e => handleFilterChange(e.target.value, targetType)}
            {...testId('audit-filter-action')}
          >
            <option value=''>All actions</option>
            {ACTIONS.filter(Boolean).map(a => (
              <option key={a} value={a}>{a.replace('_', ' ')}</option>
            ))}
          </select>
          <select
            className='input input-sm'
            value={targetType}
            onChange={e => handleFilterChange(action, e.target.value)}
            {...testId('audit-filter-target')}
          >
            <option value=''>All targets</option>
            {TARGET_TYPES.filter(Boolean).map(t => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
        </div>
        <div className={styles.filterGroup}>
          <button type="button"
            className='btn btn-ghost btn-sm'
            disabled={page === 0}
            onClick={() => setPage(p => p - 1)}
            {...testId('audit-prev')}
          >
            Prev
          </button>
          <span className={styles.muted} style={{ fontSize: 'var(--text-xs)', alignSelf: 'center' }}>
            Page {page + 1}
          </span>
          <button type="button"
            className='btn btn-ghost btn-sm'
            disabled={logs.length < PAGE_SIZE}
            onClick={() => setPage(p => p + 1)}
            {...testId('audit-next')}
          >
            Next
          </button>
        </div>
      </div>

      {loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : logs.length === 0 ? (
        <p className={styles.empty} {...testId('audit-empty')}>No audit logs found.</p>
      ) : (
        <table className={styles.table} {...testId('audit-table')}>
          <thead>
            <tr>
              <th>Time</th>
              <th>Action</th>
              <th>Target</th>
              <th>Target ID</th>
              <th>Actor</th>
            </tr>
          </thead>
          <tbody>
            {logs.map(log => (
              <tr key={log.id} {...testId(`audit-row-${log.id}`)}>
                <td className={styles.muted}>{new Date(log.created_at).toLocaleString()}</td>
                <td>
                  <span className={styles.statusBadge} data-status={log.action}>
                    {log.action.replace('_', ' ')}
                  </span>
                </td>
                <td>{log.target_type}</td>
                <td className={styles.muted} style={{ fontFamily: 'monospace', fontSize: 'var(--text-xs)' }}>
                  {log.target_id.slice(0, 8)}
                </td>
                <td className={styles.muted} style={{ fontFamily: 'monospace', fontSize: 'var(--text-xs)' }}>
                  {log.actor_id.slice(0, 8)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
