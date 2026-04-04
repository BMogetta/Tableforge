import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { auth } from '@/lib/api'
import styles from './Login.module.css'

function openOAuthPopup(onSuccess: () => void) {
  const width = 500
  const height = 700
  const left = window.screenX + (window.outerWidth - width) / 2
  const top = window.screenY + (window.outerHeight - height) / 2
  const popup = window.open(
    auth.loginUrl,
    'github-oauth',
    `width=${width},height=${height},left=${left},top=${top}`,
  )
  if (!popup) {
    // Popup blocked — fall back to redirect.
    window.location.href = auth.loginUrl
    return
  }

  // Poll until the popup closes (user completed or cancelled OAuth).
  const interval = setInterval(() => {
    if (popup.closed) {
      clearInterval(interval)
      // Check if login succeeded by calling /auth/me.
      auth
        .me()
        .then(() => onSuccess())
        .catch(() => {})
    }
  }, 500)
}

export function Login() {
  const { t } = useTranslation()
  const error = new URLSearchParams(window.location.search).get('error')
  const [loading, setLoading] = useState(false)

  if (error === 'email_not_allowed') {
    return <AccessDenied />
  }

  function handleLogin() {
    setLoading(true)
    openOAuthPopup(() => {
      window.location.href = '/'
    })
  }

  return (
    <div className={styles.root}>
      <div className={styles.board} aria-hidden={true}>
        {Array.from({ length: 64 }).map((_, i) => (
          <div key={i} className={styles.cell} style={{ animationDelay: `${(i * 37) % 800}ms` }} />
        ))}
      </div>

      <div className={styles.panel}>
        <header className={styles.header}>
          <div className={styles.emblem}>&#9823;</div>
          <h1 className={styles.title}>RECESS</h1>
          <p className={styles.subtitle}>{t('auth.subtitle')}</p>
        </header>

        <hr className='divider' />

        <div className={styles.actions}>
          <p className={styles.hint}>{t('auth.invitationOnly')}</p>
          <button
            className={`btn btn-primary ${styles.loginBtn}`}
            onClick={handleLogin}
            disabled={loading}
          >
            <svg width='18' height='18' viewBox='0 0 24 24' fill='currentColor'>
              <path d='M12 0C5.37 0 0 5.373 0 12c0 5.303 3.438 9.8 8.205 11.387.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.756-1.333-1.756-1.09-.745.083-.729.083-.729 1.205.084 1.84 1.237 1.84 1.237 1.07 1.834 2.807 1.304 3.492.997.108-.775.418-1.305.762-1.605-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.509 11.509 0 0112 5.803c1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222 0 1.606-.015 2.896-.015 3.286 0 .319.216.694.825.576C20.565 21.795 24 17.298 24 12c0-6.627-5.373-12-12-12z' />
            </svg>
            {loading ? t('auth.waitingForGithub') : t('auth.loginWithGithub')}
          </button>
        </div>

        <footer className={styles.footer}>
          <span>{t('auth.beta')}</span>
          <span>&middot;</span>
          <span>{t('auth.inviteOnly')}</span>
        </footer>
      </div>
    </div>
  )
}

function AccessDenied() {
  const { t } = useTranslation()
  return (
    <div className={styles.root}>
      <div className={styles.board} aria-hidden={true}>
        {Array.from({ length: 64 }).map((_, i) => (
          <div key={i} className={styles.cell} style={{ animationDelay: `${(i * 73) % 3000}ms` }} />
        ))}
      </div>

      <div className={styles.panel}>
        <header className={styles.header}>
          <div className={styles.emblem}>&#9876;&#65039;</div>
          <h1 className={styles.title}>RECESS</h1>
          <p className={styles.subtitle}>{t('auth.accessRestricted')}</p>
        </header>

        <hr className='divider' />

        <div className={styles.errorBlock}>
          <p className={styles.errorCode}>{t('auth.emailNotAllowed')}</p>
          <p className={styles.errorDesc}>{t('auth.accessDeniedDesc')}</p>
        </div>

        <div className={styles.actions}>
          <button
            className={`btn btn-ghost ${styles.loginBtn}`}
            onClick={() =>
              openOAuthPopup(() => {
                window.location.href = '/'
              })
            }
          >
            &#8592; {t('auth.tryAnotherAccount')}
          </button>
        </div>

        <footer className={styles.footer}>
          <span>{t('auth.closedBeta')}</span>
          <span>&middot;</span>
          <span>{t('auth.inviteOnly')}</span>
        </footer>
      </div>
    </div>
  )
}
