import { createFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Game } from '../features/game/Game'
import { GameLoading } from '../features/game/GameLoading'
import { requireAuth } from '../lib/authGuards'
import { useAppStore } from '../stores/store'
import { sessions } from '../lib/api/sessions'
import { keys } from '../lib/queryClient'

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

export const Route = createFileRoute('/game/$sessionId')({
  beforeLoad: requireAuth,
  component: () => {
    const { sessionId } = Route.useParams()
    const isSpectator = useAppStore(s => s.isSpectator)
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
  },
})
