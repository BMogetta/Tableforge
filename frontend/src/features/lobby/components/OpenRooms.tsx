import { useQuery } from '@tanstack/react-query'
import { rooms } from '@/features/room/api'
import { keys } from '@/lib/queryClient'
import { RoomCard } from './RoomCard'
import { RoomCardSkeleton } from './RoomCardSkeleton'
import styles from './OpenRooms.module.css'
import { useNavigate } from '@tanstack/react-router'
import { testId } from '@/utils/testId'

export function OpenRooms({ disabled }: { disabled?: boolean }) {
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
        <div className={styles.list}>
          <RoomCardSkeleton />
          <RoomCardSkeleton />
          <RoomCardSkeleton />
        </div>
      ) : roomList.length === 0 ? (
        <p className={styles.empty}>No open rooms. Create one to get started.</p>
      ) : (
        <div {...testId('lobby-room-list')} className={styles.list}>
          {roomList.map(view => (
            <RoomCard
              key={view.room.id}
              view={view}
              disabled={disabled}
              onJoin={() =>
                navigate({
                  to: '/rooms/$roomId',
                  params: { roomId: view.room.id },
                })
              }
            />
          ))}
        </div>
      )}
    </section>
  )
}
