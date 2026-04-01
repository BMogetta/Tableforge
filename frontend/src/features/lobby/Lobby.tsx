import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { auth } from '@/lib/api'
import { gameRegistry } from '@/features/lobby/api'
import { useAppStore } from '@/stores/store'
import { keys } from '@/lib/queryClient'
import { LobbyHeader } from './components/LobbyHeader'
import { NewGamePanel } from './components/NewGamePanel'
import { OpenRooms } from './components/OpenRooms'
import { LeaderboardPanel } from './components/LeaderboardPanel'
import styles from './Lobby.module.css'

export function Lobby() {
  const setPlayer = useAppStore(s => s.setPlayer)

  const [selectedGame, setSelectedGame] = useState('')

  const { data: gameList = [] } = useQuery({
    queryKey: keys.games(),
    queryFn: gameRegistry.list,
  })

  const effectiveGame = selectedGame || gameList[0]?.id || ''

  async function handleLogout() {
    await auth.logout()
    setPlayer(null)
    window.location.href = '/login'
  }

  return (
    <div className={`${styles.root} page-enter`}>
      <LobbyHeader onLogout={handleLogout} />

      <main className={styles.main}>
        <div className={styles.left}>
          <NewGamePanel
            gameList={gameList}
            effectiveGame={effectiveGame}
            onGameChange={setSelectedGame}
          />
          <OpenRooms />
        </div>

        <div className={styles.right}>
          <LeaderboardPanel gameId={effectiveGame} />
        </div>
      </main>

    </div>
  )
}
