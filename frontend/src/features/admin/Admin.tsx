import { useState } from 'react'
import { useAppStore } from '@/stores/store'
import { testId } from '@/utils/testId'
import { AllowedEmailsTab } from './components/AllowedEmailsTab'
import { PlayersTab } from './components/PlayersTab'
import { ModerationTab } from './components/ModerationTab'
import { BansTab } from './components/BansTab'
import { RoomsTab } from './components/RoomsTab'
import styles from './Admin.module.css'

const Tab = {
  emails: 'emails',
  players: 'players',
  moderation: 'moderation',
  bans: 'bans',
  rooms: 'rooms',
} as const
type Tab = (typeof Tab)[keyof typeof Tab]

const TAB_LABELS: Record<Tab, string> = {
  emails: 'Allowed Emails',
  players: 'Players',
  moderation: 'Moderation',
  bans: 'Bans',
  rooms: 'Rooms',
}

export function Admin() {
  const player = useAppStore(s => s.player)!
  const [tab, setTab] = useState<Tab>(Tab.emails)
  const [banTargetId, setBanTargetId] = useState<string | undefined>()

  function handleBanFromReport(playerId: string) {
    setBanTargetId(playerId)
    setTab(Tab.bans)
  }

  return (
    <div className={styles.root} {...testId('admin-panel')}>
      <header className={styles.header}>
        <div className={styles.title}>
          <span className={styles.icon}>&#x2699;</span>
          Admin Panel
        </div>
        <span className={styles.roleBadge} data-role={player.role}>
          {player.role}
        </span>
      </header>

      <nav className={styles.tabs} {...testId('admin-tabs')}>
        {Object.entries(TAB_LABELS).map(([key, label]) => (
          <button
            key={key}
            className={`${styles.tab} ${tab === key ? styles.tabActive : ''}`}
            onClick={() => setTab(key as Tab)}
            {...testId(`tab-${key}`)}
          >
            {label}
          </button>
        ))}
      </nav>

      <main className={styles.content}>
        {tab === Tab.emails && <AllowedEmailsTab callerRole={player.role} />}
        {tab === Tab.players && (
          <PlayersTab callerRole={player.role} callerID={player.id} />
        )}
        {tab === Tab.moderation && (
          <ModerationTab callerRole={player.role} onBanPlayer={handleBanFromReport} />
        )}
        {tab === Tab.bans && (
          <BansTab callerRole={player.role} initialPlayerId={banTargetId} />
        )}
        {tab === Tab.rooms && <RoomsTab callerRole={player.role} />}
      </main>
    </div>
  )
}
