import type { RoomViewPlayer } from '@/lib/api'
import styles from '../Room.module.css'

interface PlayerDropdownProps {
  target: RoomViewPlayer
  isMuted: boolean
  onMute: () => void
  onUnmute: () => void
  onBlock: () => void
  onUnblock: () => void
}

export function PlayerDropdown({ isMuted, onMute, onUnmute, onBlock, onUnblock }: PlayerDropdownProps) {
  return (
    <div className={styles.dropdown}>
      {isMuted ? (
        <button className={styles.dropdownItem} onClick={onUnmute}>
          Unmute (this session)
        </button>
      ) : (
        <button className={styles.dropdownItem} onClick={onMute}>
          Mute (this session)
        </button>
      )}
      <button className={styles.dropdownItem} onClick={onBlock}>
        Block
      </button>
      <button className={styles.dropdownItem} onClick={onUnblock}>
        Unblock
      </button>
      <hr className={styles.dropdownDivider} />
      <button className={styles.dropdownItem} disabled title='Coming soon'>
        Add Friend
      </button>
      <button className={styles.dropdownItem} disabled title='Coming soon'>
        Send DM
      </button>
    </div>
  )
}
