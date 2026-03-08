import { useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAppStore } from '../store'
import { rooms, auth, leaderboard, gameRegistry, type RoomView, type LeaderboardEntry, type GameInfo } from '../api'
import { keys } from '../queryClient'
import { useState } from 'react'
import styles from './Lobby.module.css'

export default function Lobby() {
  const player = useAppStore((s) => s.player)!
  const setPlayer = useAppStore((s) => s.setPlayer)
  const navigate = useNavigate()
  const qc = useQueryClient()

  const [joinCode, setJoinCode] = useState('')
  const [selectedGame, setSelectedGame] = useState<string>('')

  const { data: roomList = [], isLoading: roomsLoading } = useQuery({
    queryKey: keys.rooms(),
    queryFn: rooms.list,
    refetchInterval: 10_000, // poll every 10s so open rooms stay current
  })

  const { data: board = [] } = useQuery({
    queryKey: keys.leaderboard(),
    queryFn: () => leaderboard.get(),
  })

  const { data: gameList = [] } = useQuery({
    queryKey: keys.games(),
    queryFn: gameRegistry.list,
    select: (data) => data,
  })

  // Set selectedGame once games load.
  const { data: games = [] } = useQuery({
    queryKey: keys.games(),
    queryFn: gameRegistry.list,
    enabled: !selectedGame,
    select: (data) => data,
  })

  // Use the first game as default if none selected.
  const effectiveGame = selectedGame || games[0]?.id || gameList[0]?.id || ''

  const createRoom = useMutation({
    mutationFn: () => rooms.create(effectiveGame, player.id),
    onSuccess: (view) => {
      qc.invalidateQueries({ queryKey: keys.rooms() })
      navigate(`/rooms/${view.room.id}`)
    },
  })

  const joinRoom = useMutation({
    mutationFn: () => rooms.join(joinCode.trim().toUpperCase(), player.id),
    onSuccess: (view) => {
      navigate(`/rooms/${view.room.id}`)
    },
  })

  async function handleLogout() {
    await auth.logout()
    setPlayer(null)
  }

  const error = createRoom.error?.message || joinRoom.error?.message || ''

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
          <span data-testid="player-username" className={styles.username}>{player.username}</span>
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
          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>New Game</h2>
            <div className={styles.actionGrid}>
              {gameList.length > 1 && (
                <div>
                  <label className="label">Game</label>
                  <div className={styles.gameSelector}>
                    {gameList.map((g: GameInfo) => (
                      <button
                        key={g.id}
                        className={`${styles.gameOption} ${effectiveGame === g.id ? styles.gameOptionActive : ''}`}
                        onClick={() => setSelectedGame(g.id)}
                      >
                        {g.name}
                        <span className={styles.gamePlayerCount}>{g.min_players}–{g.max_players}p</span>
                      </button>
                    ))}
                  </div>
                </div>
              )}
              <button
                data-testid="create-room-btn"
                className="btn btn-primary"
                onClick={() => createRoom.mutate()}
                disabled={createRoom.isPending || !effectiveGame}
              >
                {createRoom.isPending ? 'Creating...' : '+ Create Room'}
              </button>
              <div className={styles.joinRow}>
                <input
                  data-testid="join-code-input"
                  className="input"
                  placeholder="Room code"
                  value={joinCode}
                  onChange={e => setJoinCode(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && joinRoom.mutate()}
                  maxLength={6}
                  style={{ textTransform: 'uppercase', letterSpacing: '0.15em' }}
                />
                <button
                  data-testid="join-btn"
                  className="btn btn-ghost"
                  onClick={() => joinRoom.mutate()}
                  disabled={joinRoom.isPending}
                >
                  Join
                </button>
              </div>
            </div>
            {error && <p className={styles.error}>{error}</p>}
          </section>

          <section className={styles.section}>
            <h2 className={styles.sectionTitle}>
              Open Rooms
              <span className={styles.count}>{roomList.length}</span>
            </h2>
            {roomsLoading ? (
              <p className={styles.empty}>Loading...</p>
            ) : roomList.length === 0 ? (
              <p className={styles.empty}>No open rooms. Create one to get started.</p>
            ) : (
              <div data-testid="lobby-room-list" className={styles.roomList}>
                {roomList.map((view: RoomView) => (
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
  const { room, players, settings } = view
  const isPrivate = settings?.room_visibility === 'private'

  return (
    <div data-testid="room-card" className={styles.roomCard}>
      <div className={styles.roomInfo}>
        {isPrivate ? (
          <span
            data-testid="room-card-private-icon"
            className={styles.roomCodePrivate}
            title="Private room"
          >
            🔒
          </span>
        ) : (
          <span data-testid="room-card-code" className={styles.roomCode}>{room.code}</span>
        )}
        <span className={styles.roomGame}>{room.game_id}</span>
      </div>
      <div className={styles.roomMeta}>
        <span className={styles.roomPlayers}>
          {players.length}/{room.max_players} players
        </span>
        {!isPrivate && (
          <button className="btn btn-ghost" onClick={onJoin} style={{ padding: '4px 12px' }}>
            Join →
          </button>
        )}
      </div>
    </div>
  )
}

function LeaderboardTable({ entries }: { entries: LeaderboardEntry[] }) {
  if (!entries || entries.length === 0) {
    return <p style={{ color: 'var(--text-muted)', fontSize: 12 }}>No games played yet.</p>
  }
  return (
    <table data-testid="leaderboard-table" className={styles.table}>
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
        {entries.map((e: LeaderboardEntry, i: number) => (
          <tr key={e.player_id} data-testid="leaderboard-row">
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