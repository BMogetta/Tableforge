import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { RoomPlayer } from '@/lib/schema-generated.zod'
import { mutes } from '@/features/room/api'
import { friends } from '@/features/friends/api'
import { ok, error, catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { useAppStore } from '@/stores/store'
import { PlayerDropdown } from './PlayerDropdown'
import styles from '../Room.module.css'
import { testId } from '@/utils/testId'

interface PlayerListProps {
  players: RoomPlayer[]
  maxPlayers: number
  ownerId: string | null
  currentPlayerId: string
  isOwner: boolean
  mutedIds: Set<string>
  spectatorCount: number
  removingBotId: string | null
  onMute: (id: string) => void
  onUnmute: (id: string) => void
  onRemoveBot: (id: string) => void
}

export function PlayerList({
  players,
  maxPlayers,
  ownerId,
  currentPlayerId,
  isOwner,
  mutedIds,
  spectatorCount,
  removingBotId,
  onMute,
  onUnmute,
  onRemoveBot,
}: PlayerListProps) {
  const { t } = useTranslation()
  const toast = useToast()
  const presenceMap = useAppStore(s => s.presenceMap)
  const [openDropdownId, setOpenDropdownId] = useState<string | null>(null)
  const dropdownRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function onPointerDown(e: PointerEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpenDropdownId(null)
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [])

  async function handleBlock(targetId: string) {
    const [err] = await mutes
      .mute(currentPlayerId, targetId)
      .then(() => ok(null))
      .catch(e => error(catchToAppError(e)))
    if (err) {
      toast.showError(err)
    }
    setOpenDropdownId(null)
  }

  async function handleUnblock(targetId: string) {
    const [err] = await mutes
      .unmute(currentPlayerId, targetId)
      .then(() => ok(null))
      .catch(e => error(catchToAppError(e)))
    if (err) {
      toast.showError(err)
    }
    setOpenDropdownId(null)
  }

  return (
    <section className={styles.playersSection}>
      <p className='label' {...testId('player-count')}>
        {t('room.playersCount', { current: players.length, max: maxPlayers })}
      </p>
      <div className={styles.playerList}>
        {players.map(p => {
          const isSelf = p.id === currentPlayerId
          const isDropdownOpen = openDropdownId === p.id
          const isMuted = mutedIds.has(p.id)

          return (
            <div
              key={p.id}
              className={styles.playerRow}
              {...testId(p.is_bot ? `bot-row-${p.id}` : `player-row-${p.id}`)}
            >
              <span
                className={styles.presenceDot}
                data-online={String(presenceMap[p.id] ?? false)}
                {...testId(`presence-dot-${p.id}`)}
              />
              {p.avatar_url && <img src={p.avatar_url} alt='' className={styles.avatar} />}

              {!isSelf && !p.is_bot ? (
                <div className={styles.playerNameWrapper} ref={isDropdownOpen ? dropdownRef : null}>
                  <button type="button"
                    className={styles.playerNameBtn}
                    onClick={() => setOpenDropdownId(isDropdownOpen ? null : p.id)}
                    title={t('room.optionsFor', { name: p.username })}
                  >
                    {p.username}
                    {isMuted && (
                      <span className={styles.mutedIndicator} title={t('room.locallyMuted')}>
                        🔇
                      </span>
                    )}
                  </button>

                  {isDropdownOpen && (
                    <PlayerDropdown
                      target={p}
                      isMuted={isMuted}
                      onMute={() => {
                        onMute(p.id)
                        setOpenDropdownId(null)
                      }}
                      onUnmute={() => {
                        onUnmute(p.id)
                        setOpenDropdownId(null)
                      }}
                      onBlock={() => handleBlock(p.id)}
                      onUnblock={() => handleUnblock(p.id)}
                      onAddFriend={async () => {
                        const [err] = await friends
                          .sendRequest(currentPlayerId, p.id)
                          .then(() => ok(null))
                          .catch(e => error(catchToAppError(e)))
                        if (err) {
                          toast.showError(err)
                        } else {
                          toast.showInfo('Friend request sent!')
                        }
                        setOpenDropdownId(null)
                      }}
                      onSendDM={() => {
                        useAppStore.getState().setDmTarget(p.id)
                        setOpenDropdownId(null)
                      }}
                    />
                  )}
                </div>
              ) : (
                <span className={styles.playerName}>
                  {p.username}
                  {p.is_bot && (
                    <span className='badge badge-muted' style={{ marginLeft: 6 }}>
                      {t('room.bot')}
                    </span>
                  )}
                </span>
              )}

              {p.id === ownerId && <span className='badge badge-amber'>{t('room.host')}</span>}
              {isSelf && <span className='badge badge-muted'>{t('common.you')}</span>}
              {isOwner && p.is_bot && (
                <button type="button"
                  {...testId(`remove-bot-btn-${p.id}`)}
                  className={styles.removeBotBtn}
                  disabled={removingBotId === p.id}
                  onClick={() => onRemoveBot(p.id)}
                  title='Remove bot'
                >
                  x
                </button>
              )}
            </div>
          )
        })}
        {Array.from({ length: maxPlayers - players.length }).map((_, i) => (
          <div key={i} className={`${styles.playerRow} ${styles.empty}`}>
            <div className={styles.emptySlot} />
            <span className={styles.waitingText}>{t('room.waitingForPlayer')}</span>
          </div>
        ))}
      </div>

      {spectatorCount > 0 && (
        <p className={styles.spectatorCount} {...testId('spectator-count')}>
          {t('room.spectatorCount', { count: spectatorCount })}
        </p>
      )}
    </section>
  )
}
