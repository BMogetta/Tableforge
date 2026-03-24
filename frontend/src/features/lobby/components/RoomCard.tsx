import type { RoomView } from '../../../lib/api'
import styles from './RoomCard.module.css'

interface Props {
  view: RoomView
  onJoin: () => void
}

export function RoomCard({ view, onJoin }: Props) {
  const { room, players, settings } = view
  const isPrivate = settings?.room_visibility === 'private'

  return (
    <div data-testid='room-card' className={styles.card}>
      <div className={styles.info}>
        {isPrivate ? (
          <span
            data-testid='room-card-private-icon'
            className={styles.codePrivate}
            title='Private room'
          >
            🔒
          </span>
        ) : (
          <span data-testid='room-card-code' className={styles.code}>
            {room.code}
          </span>
        )}
        <span className={styles.game}>{room.game_id}</span>
      </div>
      <div className={styles.meta}>
        <span className={styles.players}>
          {players.length}/{room.max_players} players
        </span>
        {!isPrivate && (
          <button className='btn btn-ghost' onClick={onJoin} style={{ padding: '4px 12px' }}>
            Join →
          </button>
        )}
      </div>
    </div>
  )
}
