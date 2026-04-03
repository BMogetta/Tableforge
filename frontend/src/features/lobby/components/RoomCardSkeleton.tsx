import styles from './RoomCard.module.css'

export function RoomCardSkeleton() {
  return (
    <div className={styles.card} aria-hidden='true'>
      <div className={styles.info}>
        <span className='skeleton' style={{ width: 90, height: 18 }} />
        <span className='skeleton' style={{ width: 60, height: 12, marginTop: 2 }} />
      </div>
      <div className={styles.meta}>
        <span className='skeleton' style={{ width: 70, height: 14 }} />
        <span className='skeleton' style={{ width: 50, height: 26, borderRadius: 3 }} />
      </div>
    </div>
  )
}
