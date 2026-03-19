import { createFileRoute } from '@tanstack/react-router'
import { RateLimited } from '../pages/RateLimited'

export const Route = createFileRoute('/rate-limited')({
  component: RateLimited,
})
