import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useAppStore } from '@/stores/store'
import { players } from '@/lib/api'
import { keys } from '@/lib/queryClient'
import { useBlockPlayer } from '@/hooks/useBlockPlayer'
import { ProfileHeader } from './components/ProfileHeader'
import { MatchHistory } from './components/MatchHistory'
import { AchievementGrid } from './AchievementGrid'
import styles from './Profile.module.css'
import { testId } from '@/utils/testId'

const PAGE_SIZE = 20

export function Profile({ playerId }: { playerId: string }) {
  const navigate = useNavigate()
  const currentPlayer = useAppStore(s => s.player)
  const [page, setPage] = useState(0)

  const { data: stats, isLoading: statsLoading } = useQuery({
    queryKey: keys.playerStats(playerId),
    queryFn: () => players.stats(playerId),
    staleTime: 30_000,
  })

  const { data: profile, isLoading: profileLoading } = useQuery({
    queryKey: keys.playerProfile(playerId),
    queryFn: () => players.profile(playerId),
    staleTime: 60_000,
  })

  const { data: matchData, isLoading: matchesLoading } = useQuery({
    queryKey: keys.playerMatches(playerId, page),
    queryFn: () => players.matches(playerId, PAGE_SIZE, page * PAGE_SIZE),
    staleTime: 30_000,
  })

  const { data: achievements, isLoading: achievementsLoading } = useQuery({
    queryKey: keys.playerAchievements(playerId),
    queryFn: () => players.achievements(playerId),
    staleTime: 60_000,
  })

  const { t } = useTranslation()
  const isOwnProfile = currentPlayer?.id === playerId
  const isLoading = statsLoading || profileLoading
  const { block, unblock, isBlocked, blockPending, unblockPending } = useBlockPlayer()
  const blocked = isBlocked(playerId)

  return (
    <div className={`${styles.root} page-enter`}>
      <div className={styles.container}>
        <header className={styles.header}>
          <ProfileHeader
            playerId={playerId}
            username={currentPlayer && isOwnProfile ? currentPlayer.username : undefined}
            avatarUrl={currentPlayer && isOwnProfile ? currentPlayer.avatar_url : undefined}
            bio={profile?.bio}
            country={profile?.country}
            isLoading={isLoading}
          />
          {!isOwnProfile && currentPlayer && (
            <div className={styles.profileActions}>
              {blocked ? (
                <button type="button"
                  className='btn btn-ghost btn-sm'
                  onClick={() => unblock(playerId)}
                  disabled={unblockPending}
                  {...testId('profile-unblock-btn')}
                >
                  {unblockPending ? t('profile.unblocking') : t('profile.unblock')}
                </button>
              ) : (
                <button type="button"
                  className={`btn btn-ghost btn-sm ${styles.blockBtn}`}
                  onClick={() =>
                    block({
                      targetId: playerId,
                      username: playerId.slice(0, 8),
                    })
                  }
                  disabled={blockPending}
                  {...testId('profile-block-btn')}
                >
                  {blockPending ? t('profile.blocking') : t('profile.block')}
                </button>
              )}
            </div>
          )}
        </header>

        {stats && (
          <div className={styles.statsBar}>
            <div className={styles.stat}>
              <span className={styles.statLabel}>{t('profile.games')}</span>
              <span className={styles.statValue}>{stats.total_games}</span>
            </div>
            <div className={styles.statDivider} />
            <div className={styles.stat}>
              <span className={styles.statLabel}>{t('profile.wins')}</span>
              <span className={styles.statValue}>{stats.wins}</span>
            </div>
            <div className={styles.statDivider} />
            <div className={styles.stat}>
              <span className={styles.statLabel}>{t('profile.losses')}</span>
              <span className={styles.statValue}>{stats.losses}</span>
            </div>
            <div className={styles.statDivider} />
            <div className={styles.stat}>
              <span className={styles.statLabel}>{t('profile.draws')}</span>
              <span className={styles.statValue}>{stats.draws}</span>
            </div>
            <div className={styles.statDivider} />
            <div className={styles.stat}>
              <span className={styles.statLabel}>{t('profile.winRate')}</span>
              <span className={styles.statValue}>
                {stats.total_games > 0
                  ? `${Math.round((stats.wins / stats.total_games) * 100)}%`
                  : '—'}
              </span>
            </div>
          </div>
        )}

        <div className={styles.sectionTitle}>{t('profile.achievements')}</div>
        <AchievementGrid
          achievements={achievements ?? []}
          isLoading={achievementsLoading}
        />

        <div className={styles.sectionTitle}>{t('profile.matchHistory')}</div>

        <MatchHistory
          matches={matchData?.matches ?? []}
          total={matchData?.total ?? 0}
          page={page}
          pageSize={PAGE_SIZE}
          isLoading={matchesLoading}
          onPageChange={setPage}
          onViewReplay={sessionId =>
            navigate({ to: '/sessions/$sessionId/history', params: { sessionId } })
          }
        />
      </div>
    </div>
  )
}
