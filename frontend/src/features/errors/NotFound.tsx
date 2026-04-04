import { useNavigate, useRouter } from '@tanstack/react-router'
import styles from './errors.module.css'

export function NotFound() {
  const navigate = useNavigate()
  const router = useRouter()
  return (
    <div className={styles.page}>
      <div className={styles.logo}>RECESS</div>
      <div className={styles.bigCode}>404</div>
      <div className={styles.heading}>PAGE NOT FOUND</div>
      <p className={styles.description}>
        The route you're looking for doesn't exist. It may have been moved, deleted, or you may have
        mistyped the address.
      </p>
      <div className={styles.actions}>
        <button
          type='button'
          className='btn btn-primary'
          onClick={() => navigate({ to: '/', replace: true, ignoreBlocker: true })}
        >
          Go to Lobby
        </button>
        <button type='button' className='btn btn-ghost' onClick={() => router.history.back()}>
          Go Back
        </button>
      </div>
    </div>
  )
}
