import styles from './FriendsPanel.module.css'
import { testId } from '@/utils/testId'

interface FriendItemProps {
  friendId: string
  username: string
  avatarUrl?: string
  online: boolean
  onDM: (friendId: string) => void
  onRemove: (friendId: string) => void
  removePending: boolean
}

export function FriendItem({ friendId, username, avatarUrl, online, onDM, onRemove, removePending }: FriendItemProps) {
  return (
    <div className={styles.friendRow} {...testId(`friend-${friendId}`)}>
      <span className={styles.presenceDot} data-online={String(online)} />
      {avatarUrl && <img src={avatarUrl} alt='' className={styles.avatar} />}
      <span className={styles.friendName}>{username}</span>
      <div className={styles.friendActions}>
        <button
          className={styles.actionBtn}
          onClick={() => onDM(friendId)}
          title='Send DM'
          {...testId('dm-btn')}
        >
          DM
        </button>
        <button
          className={styles.actionBtn}
          onClick={() => onRemove(friendId)}
          disabled={removePending}
          title='Remove friend'
          {...testId('remove-btn')}
        >
          {removePending ? '...' : 'x'}
        </button>
      </div>
    </div>
  )
}
