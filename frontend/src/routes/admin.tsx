import { createFileRoute } from '@tanstack/react-router'
import { Admin } from '../pages/Admin'
import { requireRole } from '../lib/authGuards'

export const Route = createFileRoute('/admin')({
  beforeLoad: () => requireRole('manager'),
  component: Admin,
})
