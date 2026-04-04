import { useTranslation } from 'react-i18next'
import styles from './errors.module.css'

interface Props {
  error: Error
  onReset: () => void
}

export function ErrorScreen({ error, onReset }: Props) {
  const { t } = useTranslation()
  return (
    <div className={styles.page}>
      <div className={styles.logo}>RECESS</div>
      <div className={styles.heading}>SOMETHING WENT WRONG</div>
      <p className={styles.description}>
        {t('errors.serverError')}
      </p>
      <code className={styles.codeBlock}>{error.message}</code>
      <div className={styles.actions}>
        <button type='button' className='btn btn-primary' onClick={onReset}>
          {t('common.confirm')}
        </button>
        <button
          type='button'
          className='btn btn-ghost'
          onClick={() => { window.location.href = '/' }}
        >
          {t('game.backToLobby')}
        </button>
      </div>
    </div>
  )
}
