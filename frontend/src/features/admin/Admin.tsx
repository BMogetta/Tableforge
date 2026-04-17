import { useState } from 'react'
import { useAppStore } from '@/stores/store'
import { testId } from '@/utils/testId'
import styles from './Admin.module.css'
import { AllowedEmailsTab } from './components/AllowedEmailsTab'
import { AuditLogTab } from './components/AuditLogTab'
import { BansTab } from './components/BansTab'
import { BroadcastTab } from './components/BroadcastTab'
import { ModerationTab } from './components/ModerationTab'
import { PlayersTab } from './components/PlayersTab'
import { RoomsTab } from './components/RoomsTab'
import { StatsTab } from './components/StatsTab'

const Tab = {
  stats: 'stats',
  emails: 'emails',
  players: 'players',
  moderation: 'moderation',
  bans: 'bans',
  rooms: 'rooms',
  audit: 'audit',
  broadcast: 'broadcast',
} as const
type Tab = (typeof Tab)[keyof typeof Tab]

const TAB_LABELS: Record<Tab, string> = {
  stats: 'Stats',
  emails: 'Allowed Emails',
  players: 'Players',
  moderation: 'Moderation',
  bans: 'Bans',
  rooms: 'Rooms',
  audit: 'Audit Log',
  broadcast: 'Broadcast',
}

export function Admin() {
  const player = useAppStore(s => s.player)!
  const [tab, setTab] = useState<Tab>(Tab.stats)
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
            type='button'
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
        {tab === Tab.stats && <StatsTab />}
        {tab === Tab.emails && <AllowedEmailsTab callerRole={player.role} />}
        {tab === Tab.players && <PlayersTab callerRole={player.role} callerID={player.id} />}
        {tab === Tab.moderation && (
          <ModerationTab callerRole={player.role} onBanPlayer={handleBanFromReport} />
        )}
        {tab === Tab.bans && <BansTab callerRole={player.role} initialPlayerId={banTargetId} />}
        {tab === Tab.rooms && <RoomsTab callerRole={player.role} />}
        {tab === Tab.audit && <AuditLogTab />}
        {tab === Tab.broadcast && <BroadcastTab />}
      </main>
    </div>
  )
}
