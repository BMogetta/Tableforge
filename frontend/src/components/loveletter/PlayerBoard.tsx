import CardDisplay, { type CardName } from './CardDisplay'
import styles from './PlayerBoard.module.css'

interface Props {
  playerId: string
  username: string
  tokens: number
  tokensToWin: number
  discardPile: CardName[]
  isEliminated: boolean
  isProtected: boolean
  hasPlayedSpy: boolean
  /** Whether this player is the local user. */
  isLocal: boolean
  /** Whether this player is the current turn player. */
  isCurrentTurn: boolean
}

export default function PlayerBoard({
  username,
  tokens,
  tokensToWin,
  discardPile,
  isEliminated,
  isProtected,
  hasPlayedSpy,
  isLocal,
  isCurrentTurn,
}: Props) {
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
          {hasPlayedSpy && (
            <span className={styles.badge} data-variant='spy'>
              Spy
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
        <div className={styles.discardPile}>
          {discardPile.length === 0 ? (
            <span className={styles.empty}>—</span>
          ) : (
            discardPile.map((card, i) => (
              <CardDisplay key={`${card}-${i}`} card={card} className={styles.discardCard} />
            ))
          )}
        </div>
      </div>
    </div>
  )
}
