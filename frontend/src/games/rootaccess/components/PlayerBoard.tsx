import { CardZone, type CardZoneEntry } from '@/ui/cards'
import { CardFace } from './CardFace'
import type { CardName } from './CardDisplay'
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
}

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
}: Props) {
  const zoneCards: CardZoneEntry<CardName>[] = discardPile.map((card, i) => ({
    key: `${card}-${i}`,
    data: card,
  }))

  return (
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
          {isLocal && <span className={styles.youBadge}>you</span>}
        </div>
        <div className={styles.badges}>
          {isProtected && (
            <span className={styles.badge} data-variant='protected'>
              Shielded
            </span>
          )}
          {isEliminated && (
            <span className={styles.badge} data-variant='eliminated'>
              Eliminated
            </span>
          )}
          {hasPlayedBackdoor && (
            <span className={styles.badge} data-variant='backdoor'>
              Backdoor
            </span>
          )}
        </div>
      </div>

      <div className={styles.tokens}>
        {Array.from({ length: tokensToWin }).map((_, i) => (
          <span
            key={i}
            className={[styles.token, i < tokens ? styles.tokenFilled : ''].join(' ')}
            aria-label={i < tokens ? 'Token earned' : 'Token pending'}
          />
        ))}
      </div>

      <div className={styles.discardSection}>
        <span className={styles.discardLabel}>Discards</span>
        {discardPile.length === 0 ? (
          <span className={styles.empty}>—</span>
        ) : (
          <CardZone<CardName>
            cards={zoneCards}
            renderCard={(card) => (
              <div className={styles.discardFace}>
                <CardFace card={card} />
              </div>
            )}
            layout="spread"
          />
        )}
      </div>
    </div>
  )
}
