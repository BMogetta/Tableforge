import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useFocusTrap } from '@/hooks/useFocusTrap'
import styles from './RoundSummary.module.css'

interface PlayerResult {
  id: string
  username: string
  tokens: number
  tokensToWin: number
  handCard?: string // card held at end of round (if deck-empty resolution)
  isWinner: boolean
  earnedBackdoorBonus: boolean
}

interface Props {
  round: number
  winnerId: string | null
  players: PlayerResult[]
  /** Auto-dismiss after this many ms. Default 4000. */
  autoDismissMs?: number
  onDismiss: () => void
}

const AUTO_DISMISS_MS = 4000

/** @package */
export function RoundSummary({
  round,
  winnerId,
  players,
  autoDismissMs = AUTO_DISMISS_MS,
  onDismiss,
}: Props) {
  const { t } = useTranslation()
  const trapRef = useFocusTrap<HTMLDivElement>()
  const [remaining, setRemaining] = useState(autoDismissMs)

  useEffect(() => {
    const interval = setInterval(() => {
      setRemaining(r => {
        if (r <= 100) {
          clearInterval(interval)
          onDismiss()
          return 0
        }
        return r - 100
      })
    }, 100)
    return () => clearInterval(interval)
  }, [onDismiss])

  const winner = players.find(p => p.id === winnerId)
  const backdoorBonusPlayers = players.filter(p => p.earnedBackdoorBonus)
  const progressPct = (remaining / autoDismissMs) * 100

  return (
    <div className={styles.overlay}>
      <div
        ref={trapRef}
        className={styles.modal}
        role='dialog'
        aria-modal='true'
        aria-labelledby='round-summary-title'
      >
        <div className={styles.header}>
          <span className={styles.roundLabel} id='round-summary-title'>
            {t('rootaccess.roundComplete', { n: round })}
          </span>
          {winner ? (
            <h2 className={styles.winnerText}>{winner.username} wins the round</h2>
          ) : (
            <h2 className={styles.winnerText}>{t('rootaccess.roundDrawn')}</h2>
          )}
        </div>

        {backdoorBonusPlayers.length > 0 && (
          <div className={styles.spyBonus}>
            <span className={styles.spyLabel}>{t('rootaccess.backdoorBonus')}</span>
            <span className={styles.spyNames}>
              {t('rootaccess.earnedToken', {
                name: backdoorBonusPlayers.map(p => p.username).join(', '),
              })}
            </span>
          </div>
        )}

        <div className={styles.standings}>
          {players.map(p => (
            <div
              key={p.id}
              className={[styles.playerRow, p.isWinner ? styles.winnerRow : '']
                .filter(Boolean)
                .join(' ')}
            >
              <span className={styles.playerName}>{p.username}</span>
              {p.handCard && <span className={styles.handCard}>{p.handCard}</span>}
              <div className={styles.tokenTrack}>
                {Array.from({ length: p.tokensToWin }).map((_, i) => (
                  <span
                    key={i}
                    className={[styles.token, i < p.tokens ? styles.tokenFilled : ''].join(' ')}
                  />
                ))}
              </div>
              <span className={styles.tokenCount}>
                {t('rootaccess.tokenCount', { current: p.tokens, total: p.tokensToWin })}
              </span>
            </div>
          ))}
        </div>

        <div className={styles.footer}>
          <div className={styles.progressBar}>
            <div className={styles.progressFill} style={{ width: `${progressPct}%` }} />
          </div>
          <button type='button' className='btn btn-ghost' onClick={onDismiss}>
            {t('rootaccess.continue')}
          </button>
        </div>
      </div>
    </div>
  )
}
