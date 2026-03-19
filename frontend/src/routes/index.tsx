import { createFileRoute } from '@tanstack/react-router'
import { Lobby } from '../pages/Lobby'
import { requireAuth } from '../lib/authGuards'

export const Route = createFileRoute('/')({
  beforeLoad: requireAuth,
  component: Lobby,
})
