import { useTranslation } from 'react-i18next'
import styles from '../Room.module.css'
import { testId } from '@/utils/testId'

interface InviteCodeProps {
  code: string
  isPrivate: boolean
}

export function InviteCode({ code, isPrivate }: InviteCodeProps) {
  const { t } = useTranslation()
  return (
    <div className={styles.shareSection}>
      {isPrivate ? (
        <>
          <p className='label'>{t('room.privateRoom')}</p>
          <p className={styles.privateNote}>
            {t('room.privateRoomHint')}
          </p>
        </>
      ) : (
        <p className='label'>{t('room.inviteCode')}</p>
      )}
      <div className={styles.codeBox}>
        <span {...testId('room-code-display')} className={styles.codeDisplay}>
          {code}
        </span>
        <button type="button"
          className='btn btn-ghost btn-sm'
          onClick={() => navigator.clipboard.writeText(code)}
        >
          {t('common.copy')}
        </button>
      </div>
    </div>
  )
}
