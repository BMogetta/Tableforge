import { useState } from 'react'
import { admin } from '@/features/admin/api'
import type { Ban } from '@/lib/schema-generated.zod'
import { useToast } from '@/ui/Toast'
import { catchToAppError } from '@/utils/errors'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

interface Props {
  callerRole: string
  initialPlayerId?: string
}

export function BansTab({ callerRole, initialPlayerId }: Props) {
  const toast = useToast()
  const [searchId, setSearchId] = useState(initialPlayerId ?? '')
  const [bans, setBans] = useState<Ban[]>([])
  const [loading, setLoading] = useState(false)
  const [searched, setSearched] = useState(false)

  // Ban dialog state
  const [showDialog, setShowDialog] = useState(false)
  const [banPlayerId, setBanPlayerId] = useState(initialPlayerId ?? '')
  const [banReason, setBanReason] = useState('')
  const [banDuration, setBanDuration] = useState<'permanent' | 'temp'>('temp')
  const [banDays, setBanDays] = useState(7)

  async function handleSearch() {
    if (!searchId.trim()) return
    setLoading(true)
    setSearched(true)
    try {
      const result = await admin.listPlayerBans(searchId.trim())
      setBans(result)
    } catch (e) {
      toast.showError(catchToAppError(e))
      setBans([])
    } finally {
      setLoading(false)
    }
  }

  async function handleLiftBan(banId: string) {
    if (!confirm('Lift this ban?')) return
    try {
      await admin.liftBan(banId)
      setBans(prev =>
        prev.map(b => (b.id === banId ? { ...b, lifted_at: new Date().toISOString() } : b)),
      )
    } catch (e) {
      toast.showError(catchToAppError(e))
    }
  }

  async function handleBanSubmit() {
    if (!banPlayerId.trim()) return
    const expiresAt =
      banDuration === 'permanent'
        ? undefined
        : new Date(Date.now() + banDays * 86_400_000).toISOString()
    try {
      const ban = await admin.banPlayer(banPlayerId.trim(), banReason || undefined, expiresAt)
      setBans(prev => [ban, ...prev])
      setShowDialog(false)
      setBanReason('')
      setBanDays(7)
    } catch (e) {
      toast.showError(catchToAppError(e))
    }
  }

  const activeBans = bans.filter(
    b => !b.lifted_at && (!b.expires_at || new Date(b.expires_at) > new Date()),
  )
  const historyBans = bans.filter(
    b => b.lifted_at || (b.expires_at && new Date(b.expires_at) <= new Date()),
  )

  const canBan = callerRole === 'manager' || callerRole === 'owner'

  return (
    <div className={styles.panel} {...testId('bans-panel')}>
      <div className={styles.toolbar}>
        <div className={styles.addRow}>
          <input
            className='input'
            aria-label='Player ID'
            placeholder='Search by player ID...'
            value={searchId}
            onChange={e => setSearchId(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleSearch()}
            {...testId('bans-search-input')}
          />
          <button
            type='button'
            className='btn btn-secondary'
            onClick={handleSearch}
            {...testId('bans-search-btn')}
          >
            Search
          </button>
          {canBan && (
            <button
              type='button'
              className='btn btn-danger'
              onClick={() => setShowDialog(true)}
              {...testId('open-ban-dialog-btn')}
            >
              Ban Player
            </button>
          )}
        </div>
      </div>

      {showDialog && (
        <div
          className={styles.dialogBackdrop}
          role='button'
          tabIndex={0}
          aria-label='Close dialog'
          onClick={() => setShowDialog(false)}
          onKeyDown={e => e.key === 'Escape' && setShowDialog(false)}
        >
          <div
            className={`card ${styles.dialog}`}
            role='dialog'
            aria-modal='true'
            aria-labelledby='ban-dialog-title'
            onClick={e => e.stopPropagation()}
            {...testId('ban-dialog')}
          >
            <h3 id='ban-dialog-title'>Ban Player</h3>
            <label className='label' htmlFor='ban-player-id'>
              Player ID
            </label>
            <input
              id='ban-player-id'
              className='input'
              value={banPlayerId}
              onChange={e => setBanPlayerId(e.target.value)}
              {...testId('ban-player-id-input')}
            />
            <label className='label' htmlFor='ban-reason'>
              Reason
            </label>
            <input
              id='ban-reason'
              className='input'
              placeholder='Optional reason'
              value={banReason}
              onChange={e => setBanReason(e.target.value)}
              {...testId('ban-reason-input')}
            />
            <label className='label' htmlFor='ban-duration'>
              Duration
            </label>
            <select
              id='ban-duration'
              className='input'
              value={banDuration}
              onChange={e => setBanDuration(e.target.value as 'permanent' | 'temp')}
              {...testId('ban-duration-select')}
            >
              <option value='temp'>Temporary</option>
              <option value='permanent'>Permanent</option>
            </select>
            {banDuration === 'temp' && (
              <>
                <label className='label' htmlFor='ban-days'>
                  Days
                </label>
                <input
                  id='ban-days'
                  className='input'
                  type='number'
                  min={1}
                  value={banDays}
                  onChange={e => setBanDays(Number(e.target.value))}
                  {...testId('ban-days-input')}
                />
              </>
            )}
            <div className={styles.actionRow}>
              <button type='button' className='btn btn-ghost' onClick={() => setShowDialog(false)}>
                Cancel
              </button>
              <button
                type='button'
                className='btn btn-danger'
                onClick={handleBanSubmit}
                {...testId('confirm-ban-btn')}
              >
                Confirm Ban
              </button>
            </div>
          </div>
        </div>
      )}

      {loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : !searched ? (
        <p className={styles.empty} {...testId('bans-empty')}>
          Search for a player to view ban history.
        </p>
      ) : activeBans.length === 0 && historyBans.length === 0 ? (
        <p className={styles.empty} {...testId('bans-no-results')}>
          No bans found for this player.
        </p>
      ) : (
        <>
          {activeBans.length > 0 && (
            <>
              <h4 className={styles.sectionTitle}>Active Bans</h4>
              <table className={styles.table} {...testId('active-bans-table')}>
                <thead>
                  <tr>
                    <th>Player</th>
                    <th>Reason</th>
                    <th>Issued</th>
                    <th>Expires</th>
                    <th>Issued by</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {activeBans.map(b => (
                    <tr key={b.id}>
                      <td>{b.player_id}</td>
                      <td>{b.reason ?? '—'}</td>
                      <td className={styles.muted}>
                        {new Date(b.created_at).toLocaleDateString()}
                      </td>
                      <td className={styles.muted}>
                        {b.expires_at ? new Date(b.expires_at).toLocaleDateString() : 'Permanent'}
                      </td>
                      <td className={styles.muted}>{b.banned_by}</td>
                      <td>
                        {canBan && (
                          <button
                            type='button'
                            className='btn btn-ghost btn-sm'
                            onClick={() => handleLiftBan(b.id)}
                            {...testId(`lift-ban-${b.id}`)}
                          >
                            Lift Ban
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}

          {historyBans.length > 0 && (
            <details className={styles.historySection}>
              <summary className={styles.sectionTitle} {...testId('ban-history-toggle')}>
                History ({historyBans.length})
              </summary>
              <table className={styles.table} {...testId('history-bans-table')}>
                <thead>
                  <tr>
                    <th>Player</th>
                    <th>Reason</th>
                    <th>Issued</th>
                    <th>Expired / Lifted</th>
                    <th>Issued by</th>
                  </tr>
                </thead>
                <tbody>
                  {historyBans.map(b => (
                    <tr key={b.id} className={styles.mutedRow}>
                      <td>{b.player_id}</td>
                      <td>{b.reason ?? '—'}</td>
                      <td className={styles.muted}>
                        {new Date(b.created_at).toLocaleDateString()}
                      </td>
                      <td className={styles.muted}>
                        {b.lifted_at
                          ? `Lifted ${new Date(b.lifted_at).toLocaleDateString()}`
                          : b.expires_at
                            ? new Date(b.expires_at).toLocaleDateString()
                            : '—'}
                      </td>
                      <td className={styles.muted}>{b.banned_by}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </details>
          )}
        </>
      )}
    </div>
  )
}
