import { auth } from '../api'
import styles from './Login.module.css'

export function Login() {
  const error = new URLSearchParams(window.location.search).get('error')

  if (error === 'email_not_allowed') {
    return <AccessDenied />
  }

  return (
    <div className={styles.root}>
      <div className={styles.board} aria-hidden>
        {Array.from({ length: 64 }).map((_, i) => (
          <div key={i} className={styles.cell} style={{ animationDelay: `${(i * 37) % 800}ms` }} />
        ))}
      </div>

      <div className={styles.panel}>
        <header className={styles.header}>
          <div className={styles.emblem}>♟</div>
          <h1 className={styles.title}>TABLEFORGE</h1>
          <p className={styles.subtitle}>Multiplayer board games, forged in steel</p>
        </header>

        <hr className='divider' />

        <div className={styles.actions}>
          <p className={styles.hint}>Access is by invitation only.</p>
          <a href={auth.loginUrl} className={`btn btn-primary ${styles.loginBtn}`}>
            <svg width='18' height='18' viewBox='0 0 24 24' fill='currentColor'>
              <path d='M12 0C5.37 0 0 5.373 0 12c0 5.303 3.438 9.8 8.205 11.387.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.756-1.333-1.756-1.09-.745.083-.729.083-.729 1.205.084 1.84 1.237 1.84 1.237 1.07 1.834 2.807 1.304 3.492.997.108-.775.418-1.305.762-1.605-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.509 11.509 0 0112 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222 0 1.606-.015 2.896-.015 3.286 0 .319.216.694.825.576C20.565 21.795 24 17.298 24 12c0-6.627-5.373-12-12-12z' />
            </svg>
            Continue with GitHub
          </a>
        </div>

        <footer className={styles.footer}>
          <span>BETA</span>
          <span>·</span>
          <span>Invite only</span>
        </footer>
      </div>
    </div>
  )
}

function AccessDenied() {
  return (
    <div className={styles.root}>
      <div className={styles.board} aria-hidden>
        {Array.from({ length: 64 }).map((_, i) => (
          <div key={i} className={styles.cell} style={{ animationDelay: `${(i * 73) % 3000}ms` }} />
        ))}
      </div>

      <div className={styles.panel}>
        <header className={styles.header}>
          <div className={styles.emblem}>⚔️</div>
          <h1 className={styles.title}>TABLEFORGE</h1>
          <p className={styles.subtitle}>Access Restricted</p>
        </header>

        <hr className='divider' />

        <div className={styles.errorBlock}>
          <p className={styles.errorCode}>EMAIL_NOT_ALLOWED</p>
          <p className={styles.errorDesc}>
            This platform operates as a closed circuit. Entry is granted by invitation only. Your
            GitHub account is not on the access list.
          </p>
        </div>

        <div className={styles.actions}>
          <a href={auth.loginUrl} className={`btn btn-ghost ${styles.loginBtn}`}>
            ← Try another account
          </a>
        </div>

        <footer className={styles.footer}>
          <span>Closed beta</span>
          <span>·</span>
          <span>Invite only</span>
        </footer>
      </div>
    </div>
  )
}
