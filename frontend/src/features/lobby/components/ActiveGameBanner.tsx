import { testId } from '@/utils/testId'
import styles from './ActiveGameBanner.module.css'

interface Props {
  gameId: string
  onRejoin: () => void
  onForfeit: () => void
  isForfeitPending: boolean
}

export function ActiveGameBanner({ gameId, onRejoin, onForfeit, isForfeitPending }: Props) {
  return (
    <div className={styles.banner} {...testId('active-game-banner')}>
      <div className={styles.info}>
        <span className={styles.dot} />
        <span className={styles.text}>
          You have an active <strong>{gameId}</strong> game
        </span>
      </div>
      <div className={styles.actions}>
        <button className='btn btn-primary' onClick={onRejoin} {...testId('rejoin-btn')}>
          Rejoin
        </button>
        <button
          className='btn btn-danger'
          onClick={onForfeit}
          disabled={isForfeitPending}
          {...testId('forfeit-btn')}
        >
          {isForfeitPending ? 'Forfeiting...' : 'Forfeit'}
        </button>
      </div>
    </div>
  )
}
