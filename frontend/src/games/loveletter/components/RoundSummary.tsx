import { useEffect, useState } from 'react'
import styles from './RoundSummary.module.css'

interface PlayerResult {
  id: string
  username: string
  tokens: number
  tokensToWin: number
  handCard?: string // card held at end of round (if deck-empty resolution)
  isWinner: boolean
  earnedSpyBonus: boolean
}

interface Props {
  round: number
  winnerId: string | null
  players: PlayerResult[]
  /** Auto-dismiss after this many ms. Default 4000. */
  autoDismissMs?: number
  onDismiss: () => void
}

const AUTO_DISMISS_MS = 4_000

export function RoundSummary({
  round,
  winnerId,
  players,
  autoDismissMs = AUTO_DISMISS_MS,
  onDismiss,
}: Props) {
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
  }, [autoDismissMs, onDismiss])

  const winner = players.find(p => p.id === winnerId)
  const spyBonusPlayers = players.filter(p => p.earnedSpyBonus)
  const progressPct = (remaining / autoDismissMs) * 100

  return (
    <div className={styles.overlay}>
      <div className={styles.modal} role='dialog' aria-modal='true' aria-labelledby='round-summary-title'>
        <div className={styles.header}>
          <span className={styles.roundLabel} id='round-summary-title'>Round {round} complete</span>
          {winner ? (
            <h2 className={styles.winnerText}>{winner.username} wins the round</h2>
          ) : (
            <h2 className={styles.winnerText}>Round drawn</h2>
          )}
        </div>

        {spyBonusPlayers.length > 0 && (
          <div className={styles.spyBonus}>
            <span className={styles.spyLabel}>Spy bonus</span>
            <span className={styles.spyNames}>
              {spyBonusPlayers.map(p => p.username).join(', ')} earned +1 token
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
                {p.tokens}/{p.tokensToWin}
              </span>
            </div>
          ))}
        </div>

        <div className={styles.footer}>
          <div className={styles.progressBar}>
            <div className={styles.progressFill} style={{ width: `${progressPct}%` }} />
          </div>
          <button className='btn btn-ghost' onClick={onDismiss}>
            Continue
          </button>
        </div>
      </div>
    </div>
  )
}
