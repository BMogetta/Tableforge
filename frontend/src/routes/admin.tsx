import { createFileRoute } from '@tanstack/react-router'
import { PlayerRole } from '@/lib/api'
import { requireRole } from '@/lib/authGuards'
import { Admin } from '../features/admin/Admin'

export const Route = createFileRoute('/admin')({
  beforeLoad: () => requireRole(PlayerRole.Manager),
  component: Admin,
})
