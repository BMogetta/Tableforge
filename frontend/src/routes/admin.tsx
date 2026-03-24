import { createFileRoute } from '@tanstack/react-router'
import { Admin } from '../features/admin/Admin'
import { requireRole } from '@/lib/authGuards'
import { PlayerRole } from '@/lib/api'

export const Route = createFileRoute('/admin')({
  beforeLoad: () => requireRole(PlayerRole.Manager),
  component: Admin,
})
