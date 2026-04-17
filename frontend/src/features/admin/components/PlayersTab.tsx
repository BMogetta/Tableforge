import { useEffect, useState } from 'react'
import { admin } from '@/features/admin/api'
import { PlayerRole } from '@/lib/api'
import type { Player } from '@/lib/schema-generated.zod'
import { useToast } from '@/ui/Toast'
import { catchToAppError } from '@/utils/errors'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

const ROLES = Object.values(PlayerRole)

interface Props {
  callerRole: string
  callerID: string
}

export function PlayersTab({ callerRole, callerID }: Props) {
  const toast = useToast()
  const [players, setPlayers] = useState<Player[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    admin
      .listPlayers()
      .then(setPlayers)
      .catch(e => toast.showError(catchToAppError(e)))
      .finally(() => setLoading(false))
  }, [toast.showError])

  async function handleRoleChange(playerID: string, role: PlayerRole) {
    try {
      await admin.setRole(playerID, role)
      setPlayers(prev => prev.map(p => (p.id === playerID ? { ...p, role } : p)))
    } catch (e) {
      toast.showError(catchToAppError(e))
    }
  }

  const canChangeRole = callerRole === PlayerRole.Owner

  return (
    <div className={styles.panel} {...testId('players-panel')}>
      {loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : (
        <table className={styles.table} {...testId('players-table')}>
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
                      disabled={p.role === PlayerRole.Owner}
                      {...testId(`role-select-${p.id}`)}
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
