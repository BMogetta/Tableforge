import { useTranslation } from 'react-i18next'
import { CardPile } from '@/ui/cards'
import { DimOverlay, useHintsEnabled } from '@/ui/hints'
import styles from './PlayerBoard.module.css'

interface Props {
  playerId: string
  username: string
  tokens: number
  tokensToWin: number
  /** Opponent hand size — for the face-down preview. Ignored when isLocal. */
  handSize?: number
  isEliminated: boolean
  isProtected: boolean
  hasPlayedBackdoor: boolean
  isLocal: boolean
  isCurrentTurn: boolean
  /** When true and hints are enabled, dim this player (FIREWALL active, not targetable). */
  dimProtected?: boolean
  /** Render a subtle "thinking..." indicator — used for bot opponents during their turn. */
  isBotThinking?: boolean
}

/** @package */
export function PlayerBoard({
  username,
  tokens,
  tokensToWin,
  handSize = 0,
  isEliminated,
  isProtected,
  hasPlayedBackdoor,
  isLocal,
  isCurrentTurn,
  dimProtected = false,
  isBotThinking = false,
}: Props) {
  const { t } = useTranslation()
  const hintsEnabled = useHintsEnabled()

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
            {isBotThinking && (
              <span className={styles.thinking} aria-label={t('rootaccess.botThinking')}>
                <span className={styles.dot} />
                <span className={styles.dot} />
                <span className={styles.dot} />
              </span>
            )}
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

        <div className={styles.tokens} aria-label={t('rootaccess.tokenCount', { current: tokens, total: tokensToWin })}>
          {Array.from({ length: tokensToWin }).map((_, i) => (
            <span
              key={i}
              className={[styles.token, i < tokens ? styles.tokenFilled : ''].join(' ')}
              aria-label={i < tokens ? t('rootaccess.tokenEarned') : t('rootaccess.tokenPending')}
            />
          ))}
        </div>

        {!isLocal && handSize > 0 && !isEliminated && (
          <div className={styles.opponentHand} aria-label={t('rootaccess.opponentHand', { name: username })}>
            <CardPile count={handSize} faceDown={true} />
          </div>
        )}
      </div>
    </DimOverlay>
  )
}
