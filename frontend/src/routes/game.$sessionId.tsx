import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Game } from '../features/game/Game'
import { GameLoading } from '../features/game/GameLoading'
import { requireAuth } from '@/lib/authGuards'
import { useRoomStore } from '../stores/roomStore'
import { sessions } from '@/lib/api/sessions'
import { keys } from '@/lib/queryClient'

function GameLoadingWrapper({
  sessionId,
  onReady,
  onTimeout,
}: {
  sessionId: string
  onReady: () => void
  onTimeout: () => void
}) {
  const { data } = useQuery({
    queryKey: keys.session(sessionId),
    queryFn: () => sessions.get(sessionId),
    staleTime: Infinity,
  })

  if (!data?.session) {
    return null
  }

  return (
    <GameLoading
      sessionId={sessionId}
      gameId={data.session.game_id}
      onReady={onReady}
      onTimeout={onTimeout}
    />
  )
}

/**
 * Gate component that manages the loading → game transition.
 * Keyed by sessionId in the parent so React remounts it entirely
 * on rematch — no stale gameReady state leaking between sessions.
 */
function GameGate({ sessionId }: { sessionId: string }) {
  const isSpectator = useRoomStore(s => s.isSpectator)
  const [gameReady, setGameReady] = useState(false)

  const handleReady = () => setGameReady(true)
  const handleTimeout = () => {
    window.location.href = '/'
  }

  if (!isSpectator && !gameReady) {
    return (
      <GameLoadingWrapper sessionId={sessionId} onReady={handleReady} onTimeout={handleTimeout} />
    )
  }

  return <Game sessionId={sessionId} />
}

export const Route = createFileRoute('/game/$sessionId')({
  beforeLoad: requireAuth,
  component: () => {
    const { sessionId } = Route.useParams()
    return <GameGate key={sessionId} sessionId={sessionId} />
  },
})
