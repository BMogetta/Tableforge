import { useTranslation } from 'react-i18next'
import { CardPile } from '@/ui/cards'
import { DimOverlay, useHintsEnabled } from '@/ui/hints'
import { testId } from '@/utils/testId'
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
  /**
   * Presence indicator next to the name. Undefined means "don't render a dot"
   * (used for the local player and bots where the signal isn't meaningful).
   */
  isOnline?: boolean
  /** Show a "BOT" badge next to the name. */
  isBot?: boolean
  /** Bot difficulty profile — appended to the badge as `BOT · EASY`. */
  botProfile?: 'easy' | 'medium' | 'hard' | 'aggressive'
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
  isOnline,
  isBot = false,
  botProfile,
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
            {isOnline !== undefined && (
              <span
                className={styles.presenceDot}
                data-online={String(isOnline)}
                role='img'
                aria-label={isOnline ? t('game.opponentOnline') : t('game.opponentOffline')}
                {...testId('opponent-presence-dot')}
              />
            )}
            <span className={styles.username}>{username}</span>
            {isBot && (
              <span className={styles.botBadge}>
                {t('room.bot')}
                {botProfile && (
                  <>
                    <span className={styles.botBadgeSep} aria-hidden='true'>
                      ·
                    </span>
                    <span className={styles.botBadgeProfile}>{botProfile}</span>
                  </>
                )}
              </span>
            )}
            {isLocal && <span className={styles.youBadge}>{t('common.you')}</span>}
            {isBotThinking && (
              <span
                className={styles.thinking}
                role='status'
                aria-label={t('rootaccess.botThinking')}
              >
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

        <div className={styles.tokens}>
          {Array.from({ length: tokensToWin }).map((_, i) => (
            <span
              // biome-ignore lint/suspicious/noArrayIndexKey: tokens are positional and index-keyed by design
              key={i}
              className={[styles.token, i < tokens ? styles.tokenFilled : ''].join(' ')}
              role='img'
              aria-label={i < tokens ? t('rootaccess.tokenEarned') : t('rootaccess.tokenPending')}
            />
          ))}
        </div>

        {!isLocal && handSize > 0 && !isEliminated && (
          // biome-ignore lint/a11y/useSemanticElements: fieldset would break layout
          <div
            className={styles.opponentHand}
            role='group'
            aria-label={t('rootaccess.opponentHand', { name: username })}
          >
            <CardPile count={handSize} faceDown={true} />
          </div>
        )}
      </div>
    </DimOverlay>
  )
}
