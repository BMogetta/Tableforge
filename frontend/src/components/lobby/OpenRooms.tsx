import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { rooms } from '../../api'
import { keys } from '../../queryClient'
import RoomCard from './RoomCard'
import styles from './OpenRooms.module.css'

export default function OpenRooms() {
  const navigate = useNavigate()

  const { data: roomList = [], isLoading } = useQuery({
    queryKey: keys.rooms(),
    queryFn: rooms.list,
    refetchInterval: 10_000,
  })

  return (
    <section className={styles.section}>
      <h2 className={styles.title}>
        Open Rooms
        <span className={styles.count}>{roomList.length}</span>
      </h2>

      {isLoading ? (
        <p className={styles.empty}>Loading...</p>
      ) : roomList.length === 0 ? (
        <p className={styles.empty}>No open rooms. Create one to get started.</p>
      ) : (
        <div data-testid="lobby-room-list" className={styles.list}>
          {roomList.map((view) => (
            <RoomCard
              key={view.room.id}
              view={view}
              onJoin={() => navigate(`/rooms/${view.room.id}`)}
            />
          ))}
        </div>
      )}
    </section>
  )
}