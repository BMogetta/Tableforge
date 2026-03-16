import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { notifications, dm, auth } from '../../lib/api'
import { useAppStore } from '../../store'
import { keys } from '../../lib/queryClient'
import { Settings } from '../ui/Settings'
import styles from './LobbyHeader.module.css'

interface Props {
  onLogout: () => void
}

export function LobbyHeader({ onLogout }: Props) {
  const player = useAppStore(s => s.player)!
  const [settingsOpen, setSettingsOpen] = useState(false)

  const { data: notifList = [] } = useQuery({
    queryKey: keys.notifications(player.id),
    queryFn: () => notifications.list(player.id),
    refetchInterval: 30_000,
  })

  const { data: unreadDMs } = useQuery({
    queryKey: keys.dmUnread(player.id),
    queryFn: () => dm.unreadCount(player.id),
    refetchInterval: 30_000,
  })

  const unreadNotifs = notifList.filter(n => !n.read_at).length
  const unreadDMCount = unreadDMs?.count ?? 0

  return (
    <>
      <header className={styles.header}>
        <div className={styles.logo}>
          <span className={styles.logoIcon}>♟</span>
          <span className={styles.logoText}>TABLEFORGE</span>
        </div>

        <div className={styles.actions}>
          {/* Notifications bell */}
          <button className={styles.iconBtn} title='Notifications'>
            <svg
              width='16'
              height='16'
              viewBox='0 0 24 24'
              fill='none'
              stroke='currentColor'
              strokeWidth='1.5'
            >
              <path d='M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9' />
              <path d='M13.73 21a2 2 0 0 1-3.46 0' />
            </svg>
            {unreadNotifs > 0 && (
              <span className={styles.badge}>{unreadNotifs > 9 ? '9+' : unreadNotifs}</span>
            )}
          </button>

          {/* DMs envelope */}
          <button className={styles.iconBtn} title='Messages'>
            <svg
              width='16'
              height='16'
              viewBox='0 0 24 24'
              fill='none'
              stroke='currentColor'
              strokeWidth='1.5'
            >
              <rect x='2' y='4' width='20' height='16' rx='2' />
              <path d='m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7' />
            </svg>
            {unreadDMCount > 0 && (
              <span className={styles.badge}>{unreadDMCount > 9 ? '9+' : unreadDMCount}</span>
            )}
          </button>

          {/* Settings gear */}
          <button className={styles.iconBtn} title='Settings' onClick={() => setSettingsOpen(true)}>
            <svg
              width='16'
              height='16'
              viewBox='0 0 24 24'
              fill='none'
              stroke='currentColor'
              strokeWidth='1.5'
            >
              <circle cx='12' cy='12' r='3' />
              <path d='M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z' />
            </svg>
          </button>

          <div className={styles.divider} />

          <div className={styles.playerInfo}>
            {player.avatar_url && <img src={player.avatar_url} alt='' className={styles.avatar} />}
            <span data-testid='player-username' className={styles.username}>
              {player.username}
            </span>
          </div>

          {(player.role === 'manager' || player.role === 'owner') && (
            <Link to='/admin' className='btn btn-ghost' style={{ padding: '6px 12px' }}>
              Admin
            </Link>
          )}

          <button className='btn btn-ghost' onClick={onLogout} style={{ padding: '6px 12px' }}>
            Logout
          </button>
        </div>
      </header>

      {/* Settings modal */}
      {settingsOpen && (
        <div
          className={styles.modalOverlay}
          onClick={e => e.target === e.currentTarget && setSettingsOpen(false)}
        >
          <div className={styles.modalPanel}>
            <Settings onClose={() => setSettingsOpen(false)} />
          </div>
        </div>
      )}
    </>
  )
}
