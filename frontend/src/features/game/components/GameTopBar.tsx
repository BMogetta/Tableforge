import { useTranslation } from 'react-i18next'
import { testId } from '@/utils/testId'
import styles from './GameTopBar.module.css'
import { TurnTimer } from './TurnTimer'

interface Props {
  gameId: string
  moveCount: number
  turnTimeoutSecs?: number
  lastMoveAt: string
  statusText: string
  isOver: boolean
  isSuspended: boolean
  canPause: boolean
  isPausePending: boolean
  onLobby: () => void
  onPause: () => void
  /** Ref callback for the game-specific extras slot (round, set-aside, …). */
  slotRef?: (el: HTMLElement | null) => void
}

/**
 * Single-row top bar above the game board.
 *
 * Turn indication lives on the per-player board (▶ + glow) and opponent
 * presence lives next to the opponent name, so the bar only carries meta
 * (game id, move count, turn timer) and chrome actions (lobby, pause).
 *
 * `statusText` is still rendered in a visually-hidden `game-status` element
 * so screen readers announce transitions ("Your turn", "You won!") and e2e
 * tests can assert on it without the bar needing to show the text visually.
 */
export function GameTopBar({
  gameId,
  moveCount,
  turnTimeoutSecs,
  lastMoveAt,
  statusText,
  isOver,
  isSuspended,
  canPause,
  isPausePending,
  onLobby,
  onPause,
  slotRef,
}: Props) {
  const { t } = useTranslation()

  return (
    <header className={styles.bar} role='status' aria-live='polite'>
      <button
        type='button'
        className={`btn btn-ghost btn-sm ${styles.lobbyBtn}`}
        {...testId('game-header-lobby-btn')}
        onClick={onLobby}
      >
        {t('lobby.backToLobby')}
      </button>

      <div className={styles.meta}>
        <span className={styles.gameId}>{gameId}</span>
        <div ref={slotRef} className={styles.extras} />
        <span className={styles.sep} aria-hidden='true'>
          ·
        </span>
        <span className={styles.moveCount}>{t('game.turnNumber', { n: moveCount })}</span>
        <TurnTimer
          turnTimeoutSecs={turnTimeoutSecs}
          lastMoveAt={lastMoveAt}
          isOver={isOver}
          isSuspended={isSuspended}
        />
      </div>

      {/* sr-only status carrier — keeps aria-live + e2e testid without visual weight */}
      <span {...testId('game-status')} className={styles.srOnly}>
        {statusText}
      </span>

      <div className={styles.actions}>
        {canPause && (
          <button
            type='button'
            {...testId('pause-btn')}
            className='btn btn-ghost btn-sm'
            onClick={onPause}
            disabled={isPausePending}
          >
            {t('game.pauseGame')}
          </button>
        )}
      </div>
    </header>
  )
}
