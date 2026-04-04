import type { RoomPlayer } from '@/lib/schema-generated.zod'
import styles from '../Room.module.css'

interface PlayerDropdownProps {
  target: RoomPlayer
  isMuted: boolean
  onMute: () => void
  onUnmute: () => void
  onBlock: () => void
  onUnblock: () => void
  onAddFriend: () => void
  onSendDM: () => void
}

export function PlayerDropdown({
  isMuted,
  onMute,
  onUnmute,
  onBlock,
  onUnblock,
  onAddFriend,
  onSendDM,
}: PlayerDropdownProps) {
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
      <button className={styles.dropdownItem} onClick={onAddFriend}>
        Add Friend
      </button>
      <button className={styles.dropdownItem} onClick={onSendDM}>
        Send DM
      </button>
    </div>
  )
}
