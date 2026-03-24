import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { auth, gameRegistry } from '@/lib/api'
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

      {/* Friends floating button — mocked until backend is built */}
      <button className={styles.friendsBtn} title='Friends (coming soon)' disabled>
        <svg
          width='16'
          height='16'
          viewBox='0 0 24 24'
          fill='none'
          stroke='currentColor'
          strokeWidth='1.5'
        >
          <path d='M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2' />
          <circle cx='9' cy='7' r='4' />
          <path d='M23 21v-2a4 4 0 0 0-3-3.87' />
          <path d='M16 3.13a4 4 0 0 1 0 7.75' />
        </svg>
        Friends
      </button>
    </div>
  )
}
