import { createFileRoute } from '@tanstack/react-router'
import { Room } from '../pages/Room'
import { requireAuth } from '@/lib/authGuards'

export const Route = createFileRoute('/rooms/$roomId')({
  beforeLoad: requireAuth,
  component: () => {
    const { roomId } = Route.useParams()
    return <Room roomId={roomId} />
  },
})
