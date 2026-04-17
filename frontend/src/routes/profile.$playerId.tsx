import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/lib/authGuards'
import { Profile } from '../features/profile/Profile'

export const Route = createFileRoute('/profile/$playerId')({
  beforeLoad: requireAuth,
  component: () => {
    const { playerId } = Route.useParams()
    return <Profile playerId={playerId} />
  },
})
