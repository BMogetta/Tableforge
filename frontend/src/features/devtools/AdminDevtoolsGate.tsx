import { lazy, Suspense } from 'react'
import { useCapability } from '@/features/auth/useCapability'

/**
 * Lazy-loaded panel. Tests check that the import path resolves to a chunk
 * containing `TanStackRouterDevtoolsInProd` — if it doesn't, the prod
 * bundle loses the ability to debug router state for admins.
 */
const AdminDevtoolsPanel = lazy(() => import('./AdminDevtoolsPanel'))

/**
 * Renders the admin devtools panel only when the server says the current
 * user is allowed to see it (owner role AND `devtools-for-admins` flag ON).
 *
 * The capability check is authoritative — the server combines role + flag
 * and returns a single boolean. We do not evaluate the flag client-side
 * because a malicious user could flip a localStorage override; the server
 * pairs the live flag with the user's verified JWT role.
 */
export function AdminDevtoolsGate() {
  const { capabilities } = useCapability()
  if (!capabilities.canSeeDevtools) return null

  return (
    <Suspense fallback={null}>
      <AdminDevtoolsPanel />
    </Suspense>
  )
}
