import { useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import styles from './errors.module.css'

const COUNTDOWN_SECS = 60

export function RateLimited() {
  const navigate = useNavigate()
  const [seconds, setSeconds] = useState(COUNTDOWN_SECS)

  useEffect(() => {
    if (seconds <= 0) {
      navigate({ to: '/' })
      return
    }
    const id = setTimeout(() => setSeconds(s => s - 1), 1000)
    return () => clearTimeout(id)
  }, [seconds, navigate])

  return (
    <div className={styles.page}>
      <div className={styles.logo}>RECESS</div>
      <div className={styles.heading}>TOO MANY REQUESTS</div>
      <p className={styles.description}>
        You've made too many requests in a short period. Please wait before trying again.
      </p>
      <div className={styles.countdown}>
        <span className={styles.countdownNumber}>{String(seconds).padStart(2, '0')}</span>
        <span className={styles.countdownLabel}>
          {seconds === 1 ? 'second' : 'seconds'} remaining
        </span>
      </div>
      <button type='button' className='btn btn-ghost' onClick={() => navigate({ to: '/' })}>
        Try Now
      </button>
    </div>
  )
}
