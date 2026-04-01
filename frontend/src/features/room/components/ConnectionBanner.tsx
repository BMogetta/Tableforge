import styles from '../Room.module.css'

export type SocketStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export function ConnectionBanner({ status }: { status: SocketStatus }) {
  if (status === 'connected') {
    return null
  }

  const config: Record<Exclude<SocketStatus, 'connected'>, { text: string; className: string }> = {
    connecting: { text: 'Connecting...', className: styles.bannerConnecting },
    reconnecting: {
      text: 'Connection lost — reconnecting...',
      className: styles.bannerReconnecting,
    },
    disconnected: {
      text: 'Disconnected. Please refresh the page.',
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
