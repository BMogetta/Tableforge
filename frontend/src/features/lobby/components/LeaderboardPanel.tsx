import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { leaderboard } from '@/features/lobby/api'
import type { Rating } from '@/lib/api'
import { keys } from '@/lib/queryClient'
import styles from './LeaderboardPanel.module.css'

interface Props {
  gameId: string
}

export function LeaderboardPanel({ gameId }: Props) {
  const { data: entries = [], isLoading } = useQuery({
    queryKey: keys.leaderboard(gameId),
    queryFn: () => leaderboard.get(gameId, 20),
    enabled: !!gameId,
    refetchInterval: 60_000, // poll every 60s
  })

  return (
    <section className={styles.section}>
      <h2 className={styles.title}>Leaderboard</h2>

      {isLoading ? (
        <p className={styles.empty}>Loading...</p>
      ) : entries.length === 0 ? (
        <p className={styles.empty}>No ranked games played yet.</p>
      ) : (
        <table data-testid='leaderboard-table' className={styles.table}>
          <thead>
            <tr>
              <th>#</th>
              <th>Player</th>
              <th>Rating</th>
              <th>W</th>
              <th>L</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e: Rating, i: number) => (
              <tr key={e.player_id} data-testid='leaderboard-row'>
                <td className={styles.rank}>{i + 1}</td>
                <td>
                  <Link to='/profile/$playerId' params={{ playerId: e.player_id }} className={styles.player}>
                    {e.avatar_url && <img src={e.avatar_url} alt='' className={styles.avatar} />}
                    <span>{e.username}</span>
                  </Link>
                </td>
                <td className={styles.rating}>{Math.round(e.display_rating)}</td>
                <td className={styles.win}>{e.win_streak}</td>
                <td className={styles.loss}>{e.loss_streak}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}
