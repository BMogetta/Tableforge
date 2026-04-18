import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSocketStore } from '@/stores/socketStore'
import { testId } from '@/utils/testId'
import styles from './banners.module.css'

/**
 * Admin broadcast banners. Stacks one banner per incoming `broadcast` WS
 * event. Each banner can be dismissed individually.
 *
 * Scope contrasts with `SystemBanner`:
 *
 *   - SystemBanner is flag-driven (state). It shows while the flag holds;
 *     reload keeps it shown.
 *   - BroadcastBanners is event-driven (push). Events arrive via WS and
 *     live only in this component's state; reload wipes them (server does
 *     not replay missed broadcasts).
 *
 * Severity variants come from `broadcast_type` on the event payload. The
 * set is open — any unknown value falls through to the default "info" look.
 */

type BroadcastEntry = {
  id: string
  message: string
  severity: 'info' | 'warning' | 'error'
}

function normalizeSeverity(raw: string): BroadcastEntry['severity'] {
  if (raw === 'warning' || raw === 'error') return raw
  return 'info'
}

let counter = 0
function nextId(): string {
  counter += 1
  return `bcast-${counter}-${Date.now()}`
}

export function BroadcastBanners() {
  const gateway = useSocketStore(s => s.gateway)
  const [broadcasts, setBroadcasts] = useState<BroadcastEntry[]>([])
  const { t } = useTranslation()

  useEffect(() => {
    if (!gateway) return
    const off = gateway.on(event => {
      if (event.type !== 'broadcast') return
      const entry: BroadcastEntry = {
        id: nextId(),
        message: event.payload.message,
        severity: normalizeSeverity(event.payload.broadcast_type),
      }
      setBroadcasts(prev => [...prev, entry])
    })
    return () => off()
  }, [gateway])

  if (broadcasts.length === 0) return null

  return (
    <>
      {broadcasts.map(b => (
        <div
          key={b.id}
          role='alert'
          aria-live='polite'
          className={styles.banner}
          data-severity={b.severity}
          {...testId(`broadcast-banner-${b.severity}`)}
        >
          <span className={styles.message}>{b.message}</span>
          <button
            type='button'
            className={styles.dismiss}
            aria-label={t('common.close')}
            onClick={() => setBroadcasts(prev => prev.filter(x => x.id !== b.id))}
          >
            ×
          </button>
        </div>
      ))}
    </>
  )
}
