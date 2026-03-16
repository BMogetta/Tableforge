import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

const COUNTDOWN_SECS = 60

export default function RateLimited() {
  const navigate = useNavigate()
  const [seconds, setSeconds] = useState(COUNTDOWN_SECS)

  useEffect(() => {
    if (seconds <= 0) {
      navigate('/')
      return
    }
    const id = setTimeout(() => setSeconds(s => s - 1), 1000)
    return () => clearTimeout(id)
  }, [seconds, navigate])

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        gap: 24,
        padding: 32,
        textAlign: 'center',
      }}
    >
      <div
        style={{
          fontFamily: 'var(--font-display)',
          fontSize: 20,
          color: 'var(--amber)',
          letterSpacing: '0.15em',
        }}
      >
        TABLEFORGE
      </div>

      <div
        style={{
          color: 'var(--danger)',
          fontSize: 13,
          fontWeight: 600,
          letterSpacing: '0.05em',
        }}
      >
        TOO MANY REQUESTS
      </div>

      <p
        style={{
          color: 'var(--text-muted)',
          fontSize: 12,
          maxWidth: 360,
          lineHeight: 1.6,
        }}
      >
        You've made too many requests in a short period. Please wait before trying again.
      </p>

      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: 6,
        }}
      >
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 32,
            color: 'var(--amber)',
            letterSpacing: '0.1em',
          }}
        >
          {String(seconds).padStart(2, '0')}
        </span>
        <span
          style={{
            fontSize: 11,
            color: 'var(--text-secondary)',
            letterSpacing: '0.08em',
            textTransform: 'uppercase',
          }}
        >
          {seconds === 1 ? 'second' : 'seconds'} remaining
        </span>
      </div>

      <button className='btn btn-ghost' onClick={() => navigate('/')} style={{ marginTop: 8 }}>
        Try Now
      </button>
    </div>
  )
}
