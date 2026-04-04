import { createFileRoute } from '@tanstack/react-router'
import { Replay } from '@/features/game/Replay'
import { requireAuth } from '@/lib/authGuards'

export const Route = createFileRoute('/sessions/$sessionId/history')({
  beforeLoad: requireAuth,
  component: () => {
    const { sessionId } = Route.useParams()
    return <Replay sessionId={sessionId} />
  },
})
