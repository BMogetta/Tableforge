import { useEffect, useState } from 'react'
import { admin } from '@/features/admin/api'
import { PlayerRole, type AllowedEmail } from '@/lib/api'
import { catchToAppError, type AppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

interface Props {
  callerRole: string
}

export function AllowedEmailsTab({ callerRole }: Props) {
  const toast = useToast()
  const [entries, setEntries] = useState<AllowedEmail[]>([])
  const [loading, setLoading] = useState(true)
  const [newEmail, setNewEmail] = useState('')
  const [newRole, setNewRole] = useState<Exclude<PlayerRole, 'owner'>>(PlayerRole.Player)
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
    <div className={styles.panel} {...testId('emails-panel')}>
      <div className={styles.addRow}>
        <input
          className='input'
          aria-label='Email address'
          placeholder='email@example.com'
          value={newEmail}
          onChange={e => setNewEmail(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleAdd()}
          {...testId('email-input')}
        />
        {callerRole === PlayerRole.Owner && (
          <select
            className='input'
            value={newRole}
            onChange={e => setNewRole(e.target.value as Exclude<PlayerRole, 'owner'>)}
            {...testId('email-role-select')}
          >
            <option value={PlayerRole.Player}>Player</option>
            <option value={PlayerRole.Manager}>Manager</option>
          </select>
        )}
        <button className='btn btn-primary' onClick={handleAdd} {...testId('add-email-btn')}>
          Add
        </button>
      </div>

      {addError && <p className={styles.error}>{addError.message}</p>}

      {loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : entries.length === 0 ? (
        <p className={styles.empty} {...testId('emails-empty')}>No emails in whitelist.</p>
      ) : (
        <table className={styles.table} {...testId('emails-table')}>
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
                    {...testId(`remove-email-${e.email}`)}
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
