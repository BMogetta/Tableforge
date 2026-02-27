import { useEffect, useState } from 'react'
import { useAppStore } from '../store'
import { admin, type AllowedEmail, type Player } from '../api'
import styles from './Admin.module.css'

type Tab = 'emails' | 'players' | 'observability'

export default function Admin() {
  const player = useAppStore((s) => s.player)!
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
        <button
          className={`${styles.tab} ${tab === 'observability' ? styles.tabActive : ''}`}
          onClick={() => setTab('observability')}
        >
          Observability
        </button>
      </nav>

      <main className={styles.content}>
        {tab === 'emails' && <EmailsPanel callerRole={player.role} />}
        {tab === 'players' && <PlayersPanel callerRole={player.role} callerID={player.id} />}
        {tab === 'observability' && <ObservabilityPanel />}
      </main>
    </div>
  )
}

// --- Emails panel ------------------------------------------------------------

function EmailsPanel({ callerRole }: { callerRole: string }) {
  const [entries, setEntries] = useState<AllowedEmail[]>([])
  const [loading, setLoading] = useState(true)
  const [newEmail, setNewEmail] = useState('')
  const [newRole, setNewRole] = useState<'player' | 'manager'>('player')
  const [error, setError] = useState('')

  useEffect(() => {
    admin.listEmails()
      .then(setEntries)
      .catch(() => setError('Failed to load emails'))
      .finally(() => setLoading(false))
  }, [])

  async function handleAdd() {
    if (!newEmail.trim()) return
    setError('')
    try {
      const entry = await admin.addEmail(newEmail.trim(), newRole)
      setEntries(prev => [entry, ...prev.filter(e => e.email !== entry.email)])
      setNewEmail('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to add email')
    }
  }

  async function handleRemove(email: string) {
    if (!confirm(`Remove ${email} from whitelist?`)) return
    try {
      await admin.removeEmail(email)
      setEntries(prev => prev.filter(e => e.email !== email))
    } catch {
      setError('Failed to remove email')
    }
  }

  return (
    <div className={styles.panel}>
      <div className={styles.addRow}>
        <input
          className="input"
          placeholder="email@example.com"
          value={newEmail}
          onChange={e => setNewEmail(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleAdd()}
        />
        {callerRole === 'owner' && (
          <select
            className="input"
            value={newRole}
            onChange={e => setNewRole(e.target.value as 'player' | 'manager')}
          >
            <option value="player">Player</option>
            <option value="manager">Manager</option>
          </select>
        )}
        <button className="btn btn-primary" onClick={handleAdd}>
          Add
        </button>
      </div>

      {error && <p className={styles.error}>{error}</p>}

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
                <td className={styles.muted}>
                  {new Date(e.created_at).toLocaleDateString()}
                </td>
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
  const [players, setPlayers] = useState<Player[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    admin.listPlayers()
      .then(setPlayers)
      .catch(() => setError('Failed to load players'))
      .finally(() => setLoading(false))
  }, [])

  async function handleRoleChange(playerID: string, role: 'player' | 'manager' | 'owner') {
    try {
      await admin.setRole(playerID, role)
      setPlayers(prev =>
        prev.map(p => p.id === playerID ? { ...p, role } : p)
      )
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to update role')
    }
  }

  const canChangeRole = callerRole === 'owner'

  return (
    <div className={styles.panel}>
      {error && <p className={styles.error}>{error}</p>}

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
                    {p.avatar_url && (
                      <img src={p.avatar_url} alt="" className={styles.avatar} />
                    )}
                    <span>{p.username}</span>
                    {p.id === callerID && (
                      <span className={styles.youBadge}>you</span>
                    )}
                  </div>
                </td>
                <td>
                  {canChangeRole && p.id !== callerID ? (
                    <select
                      className={`input ${styles.roleSelect}`}
                      value={p.role}
                      onChange={e => handleRoleChange(p.id, e.target.value as typeof ROLES[number])}
                      disabled={p.role === 'owner'}
                    >
                      {ROLES.map(r => (
                        <option key={r} value={r}>{r}</option>
                      ))}
                    </select>
                  ) : (
                    <span className={styles.roleBadge} data-role={p.role}>
                      {p.role}
                    </span>
                  )}
                </td>
                <td className={styles.muted}>
                  {new Date(p.created_at).toLocaleDateString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

// --- Observability panel -----------------------------------------------------

const TOOLS = [
  { id: 'grafana',    label: 'Grafana',    url: '/grafana' },
  { id: 'jaeger',     label: 'Jaeger',     url: '/jaeger' },
  { id: 'prometheus', label: 'Prometheus', url: '/prometheus' },
] as const

function ObservabilityPanel() {
  const [active, setActive] = useState<string>('grafana')
  const tool = TOOLS.find(t => t.id === active)!

  return (
    <div className={styles.observability}>
      <div className={styles.toolTabs}>
        {TOOLS.map(t => (
          <button
            key={t.id}
            className={`${styles.toolTab} ${active === t.id ? styles.toolTabActive : ''}`}
            onClick={() => setActive(t.id)}
          >
            {t.label}
          </button>
        ))}
        <a
          href={tool.url}
          target="_blank"
          rel="noopener noreferrer"
          className={`btn btn-ghost ${styles.openExternal}`}
        >
          Open ↗
        </a>
      </div>
      <iframe
        key={active}
        src={tool.url}
        className={styles.iframe}
        title={tool.label}
      />
    </div>
  )
}