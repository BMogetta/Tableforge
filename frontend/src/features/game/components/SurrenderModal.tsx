import { useTranslation } from 'react-i18next'
import { useFocusTrap } from '@/hooks/useFocusTrap'
import { testId } from '@/utils/testId'
import styles from './SurrenderModal.module.css'

interface Props {
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
}

export function SurrenderModal({ onConfirm, onCancel, isPending }: Props) {
  const { t } = useTranslation()
  const trapRef = useFocusTrap<HTMLDivElement>()

  return (
    <div className={styles.overlay}>
      <div
        ref={trapRef}
        className={styles.dialog}
        role='dialog'
        aria-modal='true'
        aria-labelledby='surrender-title'
      >
        <h2 id='surrender-title' className={styles.title}>
          {t('game.forfeitTitle')}
        </h2>
        <p className={styles.body}>{t('game.forfeitDesc')}</p>
        <div className={styles.actions}>
          <button type='button' className='btn btn-ghost' onClick={onCancel} disabled={isPending}>
            {t('common.cancel')}
          </button>
          <button
            type='button'
            className='btn btn-danger'
            onClick={onConfirm}
            disabled={isPending}
            {...testId('confirm-surrender-btn')}
          >
            {isPending ? t('game.forfeiting') : t('game.forfeit')}
          </button>
        </div>
      </div>
    </div>
  )
}
