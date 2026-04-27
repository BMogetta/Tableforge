/**
 * Admin-only devtools panel for production builds.
 *
 * This file is loaded ONLY via React.lazy() from main.tsx, gated behind the
 * `useCapability()` hook (owner role + `devtools-for-admins` flag ON). The
 * dynamic import keeps ~300kB of devtools code out of every other user's
 * bundle — they never download it.
 *
 * Why no `TanStackDevtools` aggregator: the `@tanstack/devtools-vite` plugin
 * strips any file that imports from `@tanstack/react-devtools` during a prod
 * build. Using the aggregator would delete this file. Instead we render the
 * individual devtools packages (which are NOT on the strip list) inline. The
 * UX trade-off is several floating triggers instead of a single tabbed panel;
 * acceptable for admin-only debug use.
 *
 * Package selection:
 *   - `TanStackRouterDevtoolsInProd` — dev variant compiles to null in prod.
 *   - `ReactQueryDevtoolsPanel` — works in prod builds as-is.
 *   - `WsDevtools`, `ScenarioPicker` — plain components.
 *   - `pacerDevtoolsPlugin` is omitted entirely; the library replaces it
 *     with a no-op when NODE_ENV !== 'development' and exposes no InProd variant.
 */

import { ReactQueryDevtoolsPanel } from '@tanstack/react-query-devtools'
import { useRouter } from '@tanstack/react-router'
import { TanStackRouterDevtoolsInProd } from '@tanstack/react-router-devtools'
import styles from './AdminDevtoolsPanel.module.css'
import { ScenarioPicker } from './ScenarioPicker'
import { useWsDevtools } from './useWsDevtools'
import { WsDevtools } from './WsDevtools'

function WsCapture() {
  useWsDevtools()
  return null
}

export function AdminDevtoolsPanel() {
  const router = useRouter()

  return (
    <>
      <WsCapture />
      <TanStackRouterDevtoolsInProd router={router} />
      <details className={styles.drawer}>
        <summary className={styles.summary}>Admin Devtools</summary>
        <div className={styles.body}>
          <section>
            <h4 className={styles.sectionTitle}>TanStack Query</h4>
            <ReactQueryDevtoolsPanel />
          </section>
          <section>
            <h4 className={styles.sectionTitle}>WebSocket</h4>
            <WsDevtools />
          </section>
          <section>
            <h4 className={styles.sectionTitle}>Scenarios</h4>
            <ScenarioPicker />
          </section>
        </div>
      </details>
    </>
  )
}
