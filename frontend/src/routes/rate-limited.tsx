import { createFileRoute } from '@tanstack/react-router'
import { RateLimited } from '@/features/errors/RateLimited'

export const Route = createFileRoute('/rate-limited')({
  component: RateLimited,
})
