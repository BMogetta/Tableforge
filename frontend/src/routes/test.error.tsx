import { createFileRoute, redirect } from '@tanstack/react-router'
import { TestError } from '../pages/TestError'

export const Route = createFileRoute('/test/error')({
  beforeLoad: () => {
    if (import.meta.env.VITE_TEST_MODE !== 'true') {
      throw redirect({ to: '/' })
    }
  },
  component: TestError,
})
