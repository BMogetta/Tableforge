import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { leaderboard } from '@/features/lobby/api'
import type { LeaderboardEntry } from '@/lib/schema-generated.zod'
import { keys } from '@/lib/queryClient'
import { LeaderboardSkeleton } from './LeaderboardSkeleton'
import styles from './LeaderboardPanel.module.css'
import { testId } from '@/utils/testId'

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
        <LeaderboardSkeleton />
      ) : entries.length === 0 ? (
        <p className={styles.empty}>No ranked games played yet.</p>
      ) : (
        <table {...testId('leaderboard-table')} className={styles.table}>
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
            {entries.map((e: LeaderboardEntry) => (
              <tr key={e.player_id} {...testId('leaderboard-row')}>
                <td className={styles.rank}>{e.rank}</td>
                <td>
                  <Link to='/profile/$playerId' params={{ playerId: e.player_id }} className={styles.player}>
                    <span>{e.player_id}</span>
                  </Link>
                </td>
                <td className={styles.rating}>{Math.round(e.display_rating)}</td>
                <td className={styles.win}>{e.games_played}</td>
                <td className={styles.loss}>—</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}
