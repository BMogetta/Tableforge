import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useEnabledGames } from '@/games/useEnabledGames'
import { players } from '@/lib/api'
import { sessions } from '@/lib/api/sessions'
import { useAppStore } from '@/stores/store'
import { useToast } from '@/ui/Toast'
import { catchToAppError } from '@/utils/errors'
import { ActiveGameBanner } from './components/ActiveGameBanner'
import { LeaderboardPanel } from './components/LeaderboardPanel'
import { NewGamePanel } from './components/NewGamePanel'
import { OpenRooms } from './components/OpenRooms'
import styles from './Lobby.module.css'

export function Lobby() {
  const player = useAppStore(s => s.player)!
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()

  const [selectedGame, setSelectedGame] = useState('')

  const { games: gameList } = useEnabledGames()

  const { data: activeSessions = [] } = useQuery({
    queryKey: ['active-sessions', player.id],
    queryFn: () => players.sessions(player.id),
    staleTime: 10_000,
  })

  const activeSession = activeSessions[0] ?? null

  const forfeit = useMutation({
    mutationFn: () => sessions.surrender(activeSession!.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['active-sessions', player.id] })
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  const effectiveGame = selectedGame || gameList[0]?.id || ''

  return (
    <div className={`${styles.root} page-enter`}>
      {activeSession && (
        <ActiveGameBanner
          gameId={activeSession.game_id}
          onRejoin={() =>
            navigate({ to: '/game/$sessionId', params: { sessionId: activeSession.id } })
          }
          onForfeit={() => forfeit.mutate()}
          isForfeitPending={forfeit.isPending}
        />
      )}

      <main className={styles.main}>
        <div className={styles.left}>
          <NewGamePanel
            gameList={gameList}
            effectiveGame={effectiveGame}
            onGameChange={setSelectedGame}
            disabled={!!activeSession}
          />
          <OpenRooms disabled={!!activeSession} />
        </div>

        <div className={styles.right}>
          <LeaderboardPanel gameId={effectiveGame} />
        </div>
      </main>
    </div>
  )
}
