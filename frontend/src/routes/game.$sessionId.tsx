import { createFileRoute } from '@tanstack/react-router'
import { Game } from '../pages/Game'
import { requireAuth } from '../lib/authGuards'

export const Route = createFileRoute('/game/$sessionId')({
  beforeLoad: requireAuth,
  component: () => {
    const { sessionId } = Route.useParams()
    return <Game sessionId={sessionId} />
  },
})
