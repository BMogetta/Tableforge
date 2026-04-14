import { useTranslation } from 'react-i18next'
import { testId } from '@/utils/testId'
import styles from '../Profile.module.css'

interface ProfileHeaderProps {
  playerId: string
  username?: string
  avatarUrl?: string
  bio?: string
  country?: string
  isLoading: boolean
  isBot?: boolean
  botProfile?: 'easy' | 'medium' | 'hard' | 'aggressive'
}

export function ProfileHeader({
  playerId,
  username,
  avatarUrl,
  bio,
  country,
  isLoading,
  isBot = false,
  botProfile,
}: ProfileHeaderProps) {
  const { t } = useTranslation()

  if (isLoading) {
    return (
      <div className={styles.headerInfo}>
        <span
          className='pulse'
          style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}
        >
          {t('profile.loadingProfile')}
        </span>
      </div>
    )
  }

  return (
    <>
      {avatarUrl ? (
        <img src={avatarUrl} alt={username ?? 'Player'} className={styles.avatar} />
      ) : (
        <div className={styles.avatarPlaceholder}>
          {(username ?? playerId.slice(0, 2)).charAt(0).toUpperCase()}
        </div>
      )}

      <div className={styles.headerInfo}>
        <div className={styles.usernameRow}>
          <h1 className={styles.username} {...testId('profile-username')}>
            {username ?? playerId.slice(0, 8)}
          </h1>
          {isBot && (
            <span className={styles.botBadge} {...testId('profile-bot-badge')}>
              {t('room.bot')}
              {botProfile && (
                <>
                  <span className={styles.botBadgeSep} aria-hidden='true'>
                    ·
                  </span>
                  <span className={styles.botBadgeProfile}>{botProfile}</span>
                </>
              )}
            </span>
          )}
        </div>
        {bio && <p className={styles.bio}>{bio}</p>}
        {country && <span className={styles.country}>{country}</span>}
      </div>
    </>
  )
}
