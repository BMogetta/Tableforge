import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
  return (
    <div className={styles.dropdown}>
      {isMuted ? (
        <button className={styles.dropdownItem} onClick={onUnmute}>
          {t('room.unmuteSession')}
        </button>
      ) : (
        <button className={styles.dropdownItem} onClick={onMute}>
          {t('room.muteSession')}
        </button>
      )}
      <button className={styles.dropdownItem} onClick={onBlock}>
        {t('profile.block')}
      </button>
      <button className={styles.dropdownItem} onClick={onUnblock}>
        {t('profile.unblock')}
      </button>
      <hr className={styles.dropdownDivider} />
      <button className={styles.dropdownItem} onClick={onAddFriend}>
        {t('room.addFriend')}
      </button>
      <button className={styles.dropdownItem} onClick={onSendDM}>
        {t('room.sendDm')}
      </button>
    </div>
  )
}
