import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { notifications } from '@/features/notifications/api'
import { NotificationsPanel } from '@/features/notifications/components/NotificationsPanel'
import { dm } from '@/features/room/api'
import { keys } from '@/lib/queryClient'
import { useAppStore } from '@/stores/store'
import { ModalOverlay } from '@/ui/ModalOverlay'
import { RulesModal } from '@/ui/RulesModal'
import { Settings } from '@/ui/Settings'
import { testId } from '@/utils/testId'
import styles from './AppHeader.module.css'

interface Props {
  onLogout: () => void
}

export function AppHeader({ onLogout }: Props) {
  const player = useAppStore(s => s.player)!
  const { t } = useTranslation()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [notifsOpen, setNotifsOpen] = useState(false)
  const [rulesOpen, setRulesOpen] = useState(false)

  const { data: notifData } = useQuery({
    queryKey: keys.notifications(player.id),
    queryFn: () => notifications.list(player.id),
    refetchInterval: 30_000,
  })
  const notifList = notifData?.items ?? []

  const { data: unreadDMs } = useQuery({
    queryKey: keys.dmUnread(player.id),
    queryFn: () => dm.unreadCount(player.id),
    refetchInterval: 30_000,
  })

  const unreadNotifs = (notifList ?? []).filter(n => !n.read_at).length
  const unreadDMCount = unreadDMs?.count ?? 0

  return (
    <>
      <header className={styles.header}>
        <Link to='/' className={styles.logo}>
          <span className={styles.logoIcon}>♟</span>
          <span className={styles.logoText}>RECESS</span>
        </Link>

        <div className={styles.actions}>
          {/* Rules book */}
          <button type="button"
            className={styles.iconBtn}
            title={t('header.gameRules')}
            onClick={() => setRulesOpen(true)}
            {...testId('rules-btn')}
          >
            <svg
              width='16'
              height='16'
              viewBox='0 0 24 24'
              fill='none'
              stroke='currentColor'
              strokeWidth='1.5'
            >
              <path d='M4 19.5A2.5 2.5 0 0 1 6.5 17H20' />
              <path d='M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z' />
            </svg>
          </button>

          {/* Notifications bell */}
          <button type="button"
            className={styles.iconBtn}
            {...testId('notifications-btn')}
            title={t('header.notifications')}
            onClick={() => setNotifsOpen(true)}
          >
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
          <button type="button"
            className={styles.iconBtn}
            {...testId('dm-envelope-btn')}
            title={t('header.messages')}
            onClick={() => useAppStore.getState().setDmTarget('__inbox__')}
          >
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
          <button type="button" className={styles.iconBtn} title={t('header.settings')} onClick={() => setSettingsOpen(true)}>
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

          <Link
            to='/profile/$playerId'
            params={{ playerId: player.id }}
            className={styles.playerInfo}
          >
            {player.avatar_url && <img src={player.avatar_url} alt='' className={styles.avatar} />}
            <span {...testId('player-username')} className={styles.username}>
              {player.username}
            </span>
          </Link>

          {(player.role === 'manager' || player.role === 'owner') && (
            <Link to='/admin' className='btn btn-ghost btn-sm'>
              {t('header.admin')}
            </Link>
          )}

          <button type="button" className='btn btn-ghost btn-sm' onClick={onLogout}>
            {t('auth.logout')}
          </button>
        </div>
      </header>

      {/* Notifications panel */}
      {notifsOpen && <NotificationsPanel items={notifList} onClose={() => setNotifsOpen(false)} />}

      {/* Rules modal */}
      {rulesOpen && <RulesModal onClose={() => setRulesOpen(false)} />}

      {/* Settings modal */}
      {settingsOpen && (
        <ModalOverlay onClose={() => setSettingsOpen(false)} className={styles.modalOverlay}>
          <div className={styles.modalPanel}>
            <Settings onClose={() => setSettingsOpen(false)} />
          </div>
        </ModalOverlay>
      )}
    </>
  )
}
