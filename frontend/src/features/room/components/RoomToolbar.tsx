import type { ReactNode } from 'react'
import { testId } from '@/utils/testId'
import styles from '../Room.module.css'

type PopoverId = 'settings' | 'bot' | 'invite' | 'chat'

interface ToolbarItem {
  id: PopoverId
  label: string
  icon: ReactNode
  badge?: number
  visible: boolean
  disabled?: boolean
}

interface Props {
  items: ToolbarItem[]
  activePopover: PopoverId | null
  onToggle: (id: PopoverId) => void
  children?: ReactNode
}

export function RoomToolbar({ items, activePopover, onToggle, children }: Props) {
  const visibleItems = items.filter(i => i.visible)

  return (
    <div className={styles.popoverWrap}>
      <div className={styles.toolbar}>
        {visibleItems.map(item => (
          <button
            type='button'
            key={item.id}
            className={`${styles.toolbarBtn} ${activePopover === item.id ? styles.toolbarBtnActive : ''}`}
            onClick={() => onToggle(item.id)}
            title={item.label}
            disabled={item.disabled}
            {...testId(`toolbar-${item.id}`)}
          >
            {item.icon}
            {item.badge != null && item.badge > 0 && (
              <span className={styles.toolbarBadge}>{item.badge > 9 ? '9+' : item.badge}</span>
            )}
          </button>
        ))}
      </div>

      {activePopover && (
        <>
          <div
            className={styles.popoverBackdrop}
            role='button'
            tabIndex={0}
            aria-label='Close popover'
            onClick={() => onToggle(activePopover)}
            onKeyDown={e => e.key === 'Escape' && onToggle(activePopover)}
          />
          <div className={styles.popover}>{children}</div>
        </>
      )}
    </div>
  )
}

// --- Icon SVGs ---------------------------------------------------------------

export const SettingsIcon = (
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
)

export const BotIcon = (
  <svg
    width='16'
    height='16'
    viewBox='0 0 24 24'
    fill='none'
    stroke='currentColor'
    strokeWidth='1.5'
  >
    <rect x='3' y='8' width='18' height='12' rx='2' />
    <circle cx='9' cy='14' r='1.5' />
    <circle cx='15' cy='14' r='1.5' />
    <path d='M12 2v4' />
    <path d='M8 6h8' />
  </svg>
)

export const InviteIcon = (
  <svg
    width='16'
    height='16'
    viewBox='0 0 24 24'
    fill='none'
    stroke='currentColor'
    strokeWidth='1.5'
  >
    <rect x='8' y='2' width='8' height='4' rx='1' />
    <path d='M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2' />
  </svg>
)

export const ChatIcon = (
  <svg
    width='16'
    height='16'
    viewBox='0 0 24 24'
    fill='none'
    stroke='currentColor'
    strokeWidth='1.5'
  >
    <path d='M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z' />
  </svg>
)
