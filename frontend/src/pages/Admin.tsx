import { useEffect, useState } from 'react'
import { useAppStore } from '../stores/store'
import { admin, type AllowedEmail, type Player } from '../lib/api'
import { catchToAppError, type AppError } from '../utils/errors'
import { useToast } from '../components/ui/Toast'
import styles from './Admin.module.css'

type Tab = 'emails' | 'players'

export function Admin() {
  const player = useAppStore(s => s.player)!
  const [tab, setTab] = useState<Tab>('emails')

  return (
    <div className={styles.root}>
      <header className={styles.header}>
        <div className={styles.title}>
          <span className={styles.icon}>⚙</span>
          Admin Panel
        </div>
        <span className={styles.roleBadge} data-role={player.role}>
          {player.role}
        </span>
      </header>

      <nav className={styles.tabs}>
        <button
          className={`${styles.tab} ${tab === 'emails' ? styles.tabActive : ''}`}
          onClick={() => setTab('emails')}
        >
          Allowed Emails
        </button>
        <button
          className={`${styles.tab} ${tab === 'players' ? styles.tabActive : ''}`}
          onClick={() => setTab('players')}
        >
          Players
        </button>
      </nav>

      <main className={styles.content}>
        {tab === 'emails' && <EmailsPanel callerRole={player.role} />}
        {tab === 'players' && <PlayersPanel callerRole={player.role} callerID={player.id} />}
      </main>
    </div>
  )
}

// --- Emails panel ------------------------------------------------------------

function EmailsPanel({ callerRole }: { callerRole: string }) {
  const toast = useToast()
  const [entries, setEntries] = useState<AllowedEmail[]>([])
  const [loading, setLoading] = useState(true)
  const [newEmail, setNewEmail] = useState('')
  const [newRole, setNewRole] = useState<'player' | 'manager'>('player')
  const [addError, setAddError] = useState<AppError | null>(null)

  useEffect(() => {
    admin
      .listEmails()
      .then(setEntries)
      .catch(e => toast.showError(catchToAppError(e)))
      .finally(() => setLoading(false))
  }, [])

  async function handleAdd() {
    if (!newEmail.trim()) return
    setAddError(null)
    try {
      const entry = await admin.addEmail(newEmail.trim(), newRole)
      setEntries(prev => [entry, ...prev.filter(e => e.email !== entry.email)])
      setNewEmail('')
    } catch (e) {
      setAddError(catchToAppError(e))
    }
  }

  async function handleRemove(email: string) {
    if (!confirm(`Remove ${email} from whitelist?`)) return
    try {
      await admin.removeEmail(email)
      setEntries(prev => prev.filter(e => e.email !== email))
    } catch (e) {
      toast.showError(catchToAppError(e))
    }
  }

  return (
    <div className={styles.panel}>
      <div className={styles.addRow}>
        <input
          className='input'
          placeholder='email@example.com'
          value={newEmail}
          onChange={e => setNewEmail(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleAdd()}
        />
        {callerRole === 'owner' && (
          <select
            className='input'
            value={newRole}
            onChange={e => setNewRole(e.target.value as 'player' | 'manager')}
          >
            <option value='player'>Player</option>
            <option value='manager'>Manager</option>
          </select>
        )}
        <button className='btn btn-primary' onClick={handleAdd}>
          Add
        </button>
      </div>

      {addError && <p className={styles.error}>{addError.message}</p>}

      {loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : entries.length === 0 ? (
        <p className={styles.empty}>No emails in whitelist.</p>
      ) : (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Email</th>
              <th>Role</th>
              <th>Invited by</th>
              <th>Added</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {entries.map(e => (
              <tr key={e.email}>
                <td>{e.email}</td>
                <td>
                  <span className={styles.roleBadge} data-role={e.role}>
                    {e.role}
                  </span>
                </td>
                <td className={styles.muted}>{e.invited_by ?? '—'}</td>
                <td className={styles.muted}>{new Date(e.created_at).toLocaleDateString()}</td>
                <td>
                  <button
                    className={`btn btn-ghost ${styles.removeBtn}`}
                    onClick={() => handleRemove(e.email)}
                  >
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// --- Players panel -----------------------------------------------------------

const ROLES = ['player', 'manager', 'owner'] as const

function PlayersPanel({ callerRole, callerID }: { callerRole: string; callerID: string }) {
  const toast = useToast()
  const [players, setPlayers] = useState<Player[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    admin
      .listPlayers()
      .then(setPlayers)
      .catch(e => toast.showError(catchToAppError(e)))
      .finally(() => setLoading(false))
  }, [])

  async function handleRoleChange(playerID: string, role: 'player' | 'manager' | 'owner') {
    try {
      await admin.setRole(playerID, role)
      setPlayers(prev => prev.map(p => (p.id === playerID ? { ...p, role } : p)))
    } catch (e) {
      toast.showError(catchToAppError(e))
    }
  }

  const canChangeRole = callerRole === 'owner'

  return (
    <div className={styles.panel}>
      {loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : (
        <table className={styles.table}>
          <thead>
            <tr>
              <th>Player</th>
              <th>Role</th>
              <th>Joined</th>
            </tr>
          </thead>
          <tbody>
            {players.map(p => (
              <tr key={p.id}>
                <td>
                  <div className={styles.playerCell}>
                    {p.avatar_url && <img src={p.avatar_url} alt='' className={styles.avatar} />}
                    <span>{p.username}</span>
                    {p.id === callerID && <span className={styles.youBadge}>you</span>}
                  </div>
                </td>
                <td>
                  {canChangeRole && p.id !== callerID ? (
                    <select
                      className={`input ${styles.roleSelect}`}
                      value={p.role}
                      onChange={e =>
                        handleRoleChange(p.id, e.target.value as (typeof ROLES)[number])
                      }
                      disabled={p.role === 'owner'}
                    >
                      {ROLES.map(r => (
                        <option key={r} value={r}>
                          {r}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <span className={styles.roleBadge} data-role={p.role}>
                      {p.role}
                    </span>
                  )}
                </td>
                <td className={styles.muted}>{new Date(p.created_at).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}