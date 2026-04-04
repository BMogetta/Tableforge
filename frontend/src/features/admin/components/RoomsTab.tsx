import { useCallback, useEffect, useRef, useState } from 'react'
import { admin } from '@/features/admin/api'
import { PlayerRole, type RoomView } from '@/lib/api'
import type { Room } from '@/lib/schema-generated.zod'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

type RoomFilter = 'all' | 'waiting' | 'in_progress' | 'finished'

interface Props {
  callerRole: string
}

export function RoomsTab({ callerRole }: Props) {
  const toast = useToast()
  const [rooms, setRooms] = useState<Room[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<RoomFilter>('all')
  const [selectedRoom, setSelectedRoom] = useState<RoomView | null>(null)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const fetchRooms = useCallback(() => {
    admin
      .listRooms()
      .then(setRooms)
      .catch(e => toast.showError(catchToAppError(e)))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    fetchRooms()
    intervalRef.current = setInterval(fetchRooms, 30000)
    return () => clearInterval(intervalRef.current)
  }, [fetchRooms])

  async function handleSelectRoom(roomId: string) {
    setLoadingDetail(true)
    try {
      const detail = await admin.getRoom(roomId)
      setSelectedRoom(detail)
    } catch (e) {
      toast.showError(catchToAppError(e))
    } finally {
      setLoadingDetail(false)
    }
  }

  async function handleForceEnd(sessionId: string) {
    if (!confirm('Force-end this session?')) return
    try {
      await admin.forceEndSession(sessionId)
      toast.showInfo('Session ended.')
      setSelectedRoom(null)
      fetchRooms()
    } catch (e) {
      toast.showError(catchToAppError(e))
    }
  }

  const filtered =
    filter === 'all' ? rooms : rooms.filter(r => r.status === filter)

  const canForceEnd = callerRole === PlayerRole.Manager || callerRole === PlayerRole.Owner

  return (
    <div className={styles.panel} {...testId('rooms-panel')}>
      <div className={styles.toolbar}>
        <div className={styles.filterGroup}>
          {(['all', 'waiting', 'in_progress', 'finished'] as RoomFilter[]).map(f => (
            <button
              key={f}
              className={`btn btn-sm ${filter === f ? 'btn-primary' : 'btn-ghost'}`}
              onClick={() => { setFilter(f); setSelectedRoom(null) }}
              {...testId(`rooms-filter-${f}`)}
            >
              {f === 'in_progress' ? 'In Game' : f}
            </button>
          ))}
        </div>
      </div>

      {selectedRoom ? (
        <div className={`card ${styles.detailCard}`} {...testId('room-detail')}>
          <div className={styles.detailHeader}>
            <h3>Room: {selectedRoom.room.code}</h3>
            <button className='btn btn-ghost btn-sm' onClick={() => setSelectedRoom(null)}>
              Back
            </button>
          </div>
          <dl className={styles.detailList}>
            <dt>Game</dt>
            <dd>{selectedRoom.room.game_id}</dd>
            <dt>Status</dt>
            <dd>
              <span className={styles.statusBadge} data-status={selectedRoom.room.status}>
                {selectedRoom.room.status}
              </span>
            </dd>
            <dt>Created</dt>
            <dd>{new Date(selectedRoom.room.created_at).toLocaleString()}</dd>
          </dl>
          <h4 className={styles.sectionTitle}>Players</h4>
          <ul className={styles.playerList}>
            {selectedRoom.players.map(p => (
              <li key={p.id} className={styles.playerCell}>
                {p.avatar_url && <img src={p.avatar_url} alt='' className={styles.avatar} />}
                <span>{p.username}</span>
                <span className={styles.roleBadge} data-role={p.role}>{p.role}</span>
              </li>
            ))}
          </ul>
          {selectedRoom.room.status === 'in_progress' && canForceEnd && (
            <div className={styles.actionRow}>
              <button
                className='btn btn-danger btn-sm'
                onClick={() => handleForceEnd(selectedRoom.room.id)}
                {...testId('force-end-btn')}
              >
                Force End Session
              </button>
            </div>
          )}
        </div>
      ) : loading ? (
        <p className={styles.empty}>Loading...</p>
      ) : loadingDetail ? (
        <p className={styles.empty}>Loading room...</p>
      ) : filtered.length === 0 ? (
        <p className={styles.empty} {...testId('rooms-empty')}>No rooms found.</p>
      ) : (
        <table className={styles.table} {...testId('rooms-table')}>
          <thead>
            <tr>
              <th>Code</th>
              <th>Game</th>
              <th>Status</th>
              <th>Players</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(r => (
              <tr
                key={r.id}
                className={styles.clickableRow}
                onClick={() => handleSelectRoom(r.id)}
                {...testId(`room-row-${r.id}`)}
              >
                <td>{r.code}</td>
                <td>{r.game_id}</td>
                <td>
                  <span className={styles.statusBadge} data-status={r.status}>
                    {r.status === 'in_progress' ? 'In Game' : r.status}
                  </span>
                </td>
                <td>{r.max_players}</td>
                <td className={styles.muted}>{new Date(r.created_at).toLocaleDateString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
