import { useTranslation } from 'react-i18next'
import { CardZone, type CardZoneEntry } from '@/ui/cards'
import { DimOverlay, useHintsEnabled } from '@/ui/hints'
import type { CardName } from './CardDisplay'
import { CardFace } from './CardFace'
import styles from './PlayerBoard.module.css'

interface Props {
  playerId: string
  username: string
  tokens: number
  tokensToWin: number
  discardPile: CardName[]
  isEliminated: boolean
  isProtected: boolean
  hasPlayedBackdoor: boolean
  isLocal: boolean
  isCurrentTurn: boolean
  /** When true and hints are enabled, dim this player (FIREWALL active, not targetable). */
  dimProtected?: boolean
}

/** @package */
export function PlayerBoard({
  username,
  tokens,
  tokensToWin,
  discardPile,
  isEliminated,
  isProtected,
  hasPlayedBackdoor,
  isLocal,
  isCurrentTurn,
  dimProtected = false,
}: Props) {
  const { t } = useTranslation()
  const hintsEnabled = useHintsEnabled()
  const zoneCards: CardZoneEntry<CardName>[] = discardPile.map((card, i) => ({
    key: `${card}-${i}`,
    data: card,
  }))

  return (
    <DimOverlay dimmed={dimProtected && hintsEnabled && isProtected && !isLocal}>
      <div
        className={[
          styles.root,
          isEliminated ? styles.eliminated : '',
          isCurrentTurn ? styles.activeTurn : '',
          isLocal ? styles.local : '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        <div className={styles.header}>
          <div className={styles.nameRow}>
            {isCurrentTurn && <span className={styles.turnIndicator}>▶</span>}
            <span className={styles.username}>{username}</span>
            {isLocal && <span className={styles.youBadge}>{t('common.you')}</span>}
          </div>
          <div className={styles.badges}>
            {isProtected && (
              <span className={styles.badge} data-variant='protected'>
                {t('rootaccess.shielded')}
              </span>
            )}
            {isEliminated && (
              <span className={styles.badge} data-variant='eliminated'>
                {t('rootaccess.eliminated')}
              </span>
            )}
            {hasPlayedBackdoor && (
              <span className={styles.badge} data-variant='backdoor'>
                {t('rootaccess.backdoor')}
              </span>
            )}
          </div>
        </div>

        <div className={styles.tokens}>
          {Array.from({ length: tokensToWin }).map((_, i) => (
            <span
              key={i}
              className={[styles.token, i < tokens ? styles.tokenFilled : ''].join(' ')}
              aria-label={i < tokens ? t('rootaccess.tokenEarned') : t('rootaccess.tokenPending')}
            />
          ))}
        </div>

        <div className={styles.discardSection}>
          <span className={styles.discardLabel}>{t('rootaccess.discards')}</span>
          {discardPile.length === 0 ? (
            <span className={styles.empty}>—</span>
          ) : (
            <CardZone<CardName>
              cards={zoneCards}
              renderCard={card => (
                <div className={styles.discardFace}>
                  <CardFace card={card} />
                </div>
              )}
              layout='spread'
            />
          )}
        </div>
      </div>
    </DimOverlay>
  )
}
