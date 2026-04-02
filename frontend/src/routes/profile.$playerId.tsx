import { createFileRoute } from '@tanstack/react-router'
import { Profile } from '../features/profile/Profile'
import { requireAuth } from '@/lib/authGuards'

export const Route = createFileRoute('/profile/$playerId')({
  beforeLoad: requireAuth,
  component: () => {
    const { playerId } = Route.useParams()
    return <Profile playerId={playerId} />
  },
})
