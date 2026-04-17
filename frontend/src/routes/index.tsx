import { createFileRoute } from '@tanstack/react-router'
import { requireAuth } from '@/lib/authGuards'
import { Lobby } from '../features/lobby/Lobby'

export const Route = createFileRoute('/')({
  beforeLoad: requireAuth,
  component: Lobby,
})
