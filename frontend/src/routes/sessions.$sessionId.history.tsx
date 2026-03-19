import { createFileRoute } from '@tanstack/react-router'
import { SessionHistory } from '../pages/SessionHistory'
import { requireAuth } from '../lib/authGuards'

export const Route = createFileRoute('/sessions/$sessionId/history')({
  beforeLoad: requireAuth,
  component: () => {
    const { sessionId } = Route.useParams()
    return <SessionHistory sessionId={sessionId} />
  },
})
