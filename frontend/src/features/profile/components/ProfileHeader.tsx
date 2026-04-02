import styles from '../Profile.module.css'

interface ProfileHeaderProps {
  playerId: string
  username?: string
  avatarUrl?: string
  bio?: string
  country?: string
  isLoading: boolean
}

export function ProfileHeader({
  playerId,
  username,
  avatarUrl,
  bio,
  country,
  isLoading,
}: ProfileHeaderProps) {
  if (isLoading) {
    return (
      <div className={styles.headerInfo}>
        <span className='pulse' style={{ color: 'var(--color-text-muted)', fontSize: 'var(--text-sm)' }}>
          Loading profile...
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
        <h1 className={styles.username}>{username ?? playerId.slice(0, 8)}</h1>
        {bio && <p className={styles.bio}>{bio}</p>}
        {country && <span className={styles.country}>{country}</span>}
      </div>
    </>
  )
}
