import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { useAppStore } from '../store'
import { rooms, auth, leaderboard, gameRegistry, type RoomView, type LeaderboardEntry, type GameInfo } from '../api'
import styles from './Lobby.module.css'

export default function Lobby() {
  const player = useAppStore((s) => s.player)!
  const setPlayer = useAppStore((s) => s.setPlayer)
  const navigate = useNavigate()

  const [roomList, setRoomList] = useState<RoomView[]>([])
  const [board, setBoard] = useState<LeaderboardEntry[]>([])
  const [gameList, setGameList] = useState<GameInfo[]>([])
  const [selectedGame, setSelectedGame] = useState<string>('')
  const [joinCode, setJoinCode] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)

  useEffect(() => {
    Promise.all([rooms.list(), leaderboard.get(), gameRegistry.list()])
      .then(([r, l, g]) => {
        setRoomList(r ?? [])
        setBoard(l ?? [])
        setGameList(g ?? [])
        if (g && g.length > 0) setSelectedGame(g[0].id)
      })
      .catch(() => setError('Failed to load lobby'))
      .finally(() => setLoading(false))
  }, [])

  async function handleCreate() {
    if (!selectedGame) return
    setCreating(true)
    setError('')
    try {
      const view = await rooms.create(selectedGame, player.id)
      navigate(`/rooms/${view.room.id}`)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to create room')
    } finally {
      setCreating(false)
    }
  }

  async function handleJoin() {
    if (!joinCode.trim()) return
    setError('')
    try {
      const view = await rooms.join(joinCode.trim().toUpperCase(), player.id)
      navigate(`/rooms/${view.room.id}`)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Room not found')
    }
  }

  async function handleLogout() {
    await auth.logout()
    setPlayer(null)
  }

  return (
    <div className={`${styles.root} page-enter`}>
      <header className={styles.header}>
        <div className={styles.logo}>
          <span className={styles.logoIcon}>♟</span>
          <span className={styles.logoText}>TABLEFORGE</span>
        </div>
        <div className={styles.playerInfo}>
          {player.avatar_url && (
            <img src={player.avatar_url} alt="" className={styles.avatar} />
          )}
          <span className={styles.username}>{player.username}</span>
          {(player.role === 'manager' || player.role === 'owner') && (
            <Link to="/admin" className="btn btn-ghost" style={{ padding: '6px 12px' }}>
              Admin
            </Link>
          )}
          <button className="btn btn-ghost" onClick={handleLogout} style={{ padding: '6px 12px' }}>
            Logout
          </button>
        </div>
      </header>

      <main className={styles.main}>
        <div className={styles.left}>
          {/* Actions */}
          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>New Game</h2>
            <div className={styles.actionGrid}>
              {gameList.length > 1 && (
                <div>
                  <label className="label">Game</label>
                  <div className={styles.gameSelector}>
                    {gameList.map(g => (
                      <button
                        key={g.id}
                        className={`${styles.gameOption} ${selectedGame === g.id ? styles.gameOptionActive : ''}`}
                        onClick={() => setSelectedGame(g.id)}
                      >
                        {g.name}
                        <span className={styles.gamePlayerCount}>{g.min_players}–{g.max_players}p</span>
                      </button>
                    ))}
                  </div>
                </div>
              )}
              <button className="btn btn-primary" onClick={handleCreate} disabled={creating || !selectedGame}>
                {creating ? 'Creating...' : '+ Create Room'}
              </button>
              <div className={styles.joinRow}>
                <input
                  className="input"
                  placeholder="Room code"
                  value={joinCode}
                  onChange={e => setJoinCode(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handleJoin()}
                  maxLength={6}
                  style={{ textTransform: 'uppercase', letterSpacing: '0.15em' }}
                />
                <button className="btn btn-ghost" onClick={handleJoin}>Join</button>
              </div>
            </div>
            {error && <p className={styles.error}>{error}</p>}
          </section>

          {/* Waiting rooms */}
          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>
              Open Rooms
              <span className={styles.count}>{roomList.length ?? 0}</span>
            </h2>
            {loading ? (
              <p className={styles.empty}>Loading...</p>
            ) : roomList.length === 0 ? (
              <p className={styles.empty}>No open rooms. Create one to get started.</p>
            ) : (
              <div className={styles.roomList}>
                {roomList.map(view => (
                  <RoomCard
                    key={view.room.id}
                    view={view}
                    onJoin={() => navigate(`/rooms/${view.room.id}`)}
                  />
                ))}
              </div>
            )}
          </section>
        </div>

        <div className={styles.right}>
          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>Leaderboard</h2>
            <LeaderboardTable entries={board} />
          </section>
        </div>
      </main>
    </div>
  )
}

function RoomCard({ view, onJoin }: { view: RoomView; onJoin: () => void }) {
  const { room, players } = view
  return (
    <div className={styles.roomCard}>
      <div className={styles.roomInfo}>
        <span className={styles.roomCode}>{room.code}</span>
        <span className={styles.roomGame}>{room.game_id}</span>
      </div>
      <div className={styles.roomMeta}>
        <span className={styles.roomPlayers}>
          {players.length}/{room.max_players} players
        </span>
        <button className="btn btn-ghost" onClick={onJoin} style={{ padding: '4px 12px' }}>
          Join →
        </button>
      </div>
    </div>
  )
}

function LeaderboardTable({ entries }: { entries: LeaderboardEntry[] }) {
  if (!entries || entries.length === 0) {
    return <p style={{ color: 'var(--text-muted)', fontSize: 12 }}>No games played yet.</p>
  }
  return (
    <table className={styles.table}>
      <thead>
        <tr>
          <th>#</th>
          <th>Player</th>
          <th>W</th>
          <th>L</th>
          <th>D</th>
        </tr>
      </thead>
      <tbody>
        {entries.map((e, i) => (
          <tr key={e.player_id}>
            <td className={styles.rank}>{i + 1}</td>
            <td>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                {e.avatar_url && <img src={e.avatar_url} alt="" style={{ width: 20, height: 20, borderRadius: '50%' }} />}
                {e.username}
              </div>
            </td>
            <td style={{ color: 'var(--success)' }}>{e.wins}</td>
            <td style={{ color: 'var(--danger)' }}>{e.losses}</td>
            <td style={{ color: 'var(--text-secondary)' }}>{e.draws}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}