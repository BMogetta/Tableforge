import type { AppError } from '../../utils/errors'
import styles from './ErrorMessage.module.css'

interface Props {
  error: AppError | null
  className?: string
}

/**
 * Inline error display component.
 *
 * Dev:  shows reason + status + raw message for fast debugging.
 * Prod: shows friendly message + reportable code.
 *
 * Returns null when error is null — safe to render unconditionally.
 */
export function ErrorMessage({ error, className }: Props) {
  if (!error) return null

  const isDev = import.meta.env.DEV

  return (
    <div className={`${styles.root} ${className ?? ''}`} role='alert'>
      {isDev ? (
        <>
          <span className={styles.reason}>{error.reason}</span>
          {error.status && <span className={styles.status}>{error.status}</span>}
          <span className={styles.message}>{error.message}</span>
        </>
      ) : (
        <>
          <span className={styles.message}>{error.message}</span>
          {error.code && <span className={styles.code}>{error.code}</span>}
        </>
      )}
    </div>
  )
}
