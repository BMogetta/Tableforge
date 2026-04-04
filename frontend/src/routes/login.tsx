import { createFileRoute, redirect } from '@tanstack/react-router'
import { Login } from '@/features/auth/Login'
import { useAppStore } from '../stores/store'

export const Route = createFileRoute('/login')({
  beforeLoad: () => {
    const player = useAppStore.getState().player
    if (player) throw redirect({ to: '/' })
  },
  component: Login,
})
