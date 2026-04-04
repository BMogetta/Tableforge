import { useTranslation } from 'react-i18next'
import styles from '../Room.module.css'

export type SocketStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export function ConnectionBanner({ status }: { status: SocketStatus }) {
  const { t } = useTranslation()

  if (status === 'connected') {
    return null
  }

  const config: Record<Exclude<SocketStatus, 'connected'>, { text: string; className: string }> = {
    connecting: { text: t('common.connecting'), className: styles.bannerConnecting },
    reconnecting: {
      text: t('common.connectionLost'),
      className: styles.bannerReconnecting,
    },
    disconnected: {
      text: t('common.disconnected'),
      className: styles.bannerDisconnected,
    },
  }

  const { text, className } = config[status]

  return (
    <div className={`${styles.banner} ${className}`}>
      {status !== 'disconnected' && <span className={styles.bannerDot} />}
      {text}
    </div>
  )
}
