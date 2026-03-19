import { redirect } from '@tanstack/react-router'
import { useAppStore } from '../stores/store'

export function requireAuth() {
  const player = useAppStore.getState().player
  if (!player) throw redirect({ to: '/login' })
}

export function requireRole(role: 'manager' | 'owner') {
  const player = useAppStore.getState().player
  const hierarchy = { player: 0, manager: 1, owner: 2 }
  if (!player || hierarchy[player.role] < hierarchy[role]) {
    throw redirect({ to: '/' })
  }
}
