import { createFileRoute, redirect } from '@tanstack/react-router'
import { DesignPage } from '@/features/devtools/DesignPage'

export const Route = createFileRoute('/design')({
  beforeLoad: () => {
    if (!import.meta.env.DEV) {
      throw redirect({ to: '/' })
    }
  },
  component: DesignPage,
})
