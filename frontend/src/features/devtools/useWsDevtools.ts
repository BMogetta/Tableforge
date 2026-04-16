import { useEffect } from 'react'
import { useSocketStore } from '@/stores/socketStore'
import { useWsDevtoolsStore } from './store'

/**
 * Hooks into the global gateway socket and forwards every event
 * to the WsDevtools capture store.
 *
 * Mount this once near the top of the app tree (e.g. in __root.tsx) — only
 * in development. It is a pure side-effect hook with no return value.
 *
 * In production this hook is a no-op — it early-returns before subscribing
 * so there is zero overhead.
 */
export function useWsDevtools(): void {
  const gateway = useSocketStore(s => s.gateway)
  const capture = useWsDevtoolsStore(s => s.capture)

  useEffect(() => {
    if (!gateway) return
    return gateway.on(event => {
      capture('gateway', event)
    })
  }, [gateway, capture])
}
