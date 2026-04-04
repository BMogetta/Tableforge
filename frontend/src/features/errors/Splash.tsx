import styles from './errors.module.css'

export function Splash() {
  return (
    <div className={styles.page}>
      <div className={styles.logoLarge}>RECESS</div>
      <div className={`pulse ${styles.splashLabel}`}>CONNECTING...</div>
    </div>
  )
}
