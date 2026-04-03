import styles from './LeaderboardPanel.module.css'

const ROWS = 5

export function LeaderboardSkeleton() {
  return (
    <table className={styles.table} aria-hidden='true'>
      <thead>
        <tr>
          <th>#</th>
          <th>Player</th>
          <th>Rating</th>
          <th>W</th>
          <th>L</th>
        </tr>
      </thead>
      <tbody>
        {Array.from({ length: ROWS }, (_, i) => (
          <tr key={i}>
            <td><span className='skeleton' style={{ width: 16, height: 14, display: 'block' }} /></td>
            <td>
              <div className={styles.player}>
                <span className='skeleton' style={{ width: 20, height: 20, borderRadius: '50%' }} />
                <span className='skeleton' style={{ width: 80 + (i % 3) * 20, height: 14 }} />
              </div>
            </td>
            <td><span className='skeleton' style={{ width: 40, height: 14, display: 'block' }} /></td>
            <td><span className='skeleton' style={{ width: 16, height: 14, display: 'block' }} /></td>
            <td><span className='skeleton' style={{ width: 16, height: 14, display: 'block' }} /></td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
