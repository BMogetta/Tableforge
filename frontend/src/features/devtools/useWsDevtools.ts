import { useEffect } from 'react'
import { useAppStore } from '../../stores/store'
import { useWsDevtoolsStore } from './store'

/**
 * Hooks into the global room and player sockets and forwards every event
 * to the WsDevtools capture store.
 *
 * Mount this once near the top of the app tree (e.g. in __root.tsx) — only
 * in development. It is a pure side-effect hook with no return value.
 *
 * In production this hook is a no-op — it early-returns before subscribing
 * so there is zero overhead.
 */
export function useWsDevtools(): void {
  const socket = useAppStore(s => s.socket)
  const playerSocket = useAppStore(s => s.playerSocket)
  const capture = useWsDevtoolsStore(s => s.capture)

  useEffect(() => {
    if (!socket) return
    return socket.on(event => {
      capture('room', event)
    })
  }, [socket, capture])

  useEffect(() => {
    if (!playerSocket) return
    return playerSocket.on(event => capture('player', event))
  }, [playerSocket, capture])
}
