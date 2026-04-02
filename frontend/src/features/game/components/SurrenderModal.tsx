import { useFocusTrap } from '@/hooks/useFocusTrap'
import styles from './SurrenderModal.module.css'
import { testId } from '@/utils/testId'

interface Props {
  onConfirm: () => void
  onCancel: () => void
  isPending: boolean
}

export function SurrenderModal({ onConfirm, onCancel, isPending }: Props) {
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
          Forfeit game?
        </h2>
        <p className={styles.body}>You will be recorded as the loser. Your opponent wins.</p>
        <div className={styles.actions}>
          <button className='btn btn-ghost' onClick={onCancel} disabled={isPending}>
            Cancel
          </button>
          <button
            className='btn btn-danger'
            onClick={onConfirm}
            disabled={isPending}
            {...testId('confirm-surrender-btn')}
          >
            {isPending ? 'Forfeiting...' : 'Forfeit'}
          </button>
        </div>
      </div>
    </div>
  )
}
