import { createFileRoute } from '@tanstack/react-router'
import { Lobby } from '../features/lobby/Lobby'
import { requireAuth } from '@/lib/authGuards'

export const Route = createFileRoute('/')({
  beforeLoad: requireAuth,
  component: Lobby,
})
