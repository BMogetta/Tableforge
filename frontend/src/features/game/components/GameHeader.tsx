import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { CardName } from '@/games/rootaccess'
import { RulesModal } from '@/ui/RulesModal'
import { testId } from '@/utils/testId'
import styles from '../Game.module.css'
import { TurnTimer } from './TurnTimer'

interface Props {
  gameId: string
  moveCount: number
  turnTimeoutSecs?: number
  lastMoveAt: string
  isSuspended: boolean
  canPause: boolean
  isPausePending: boolean
  isOver: boolean
  isSpectator: boolean
  onLobby: () => void
  onPause: () => void
  /** Cards in the local player's hand — passed to RulesModal for highlighting. */
  handCards?: CardName[]
}

/**
 * Top bar of the game page.
 *
 * Contains:
 * - ← Lobby button: opens surrender modal mid-game, navigates directly if over/spectating.
 * - Game ID and current move count.
 * - Pause button: only shown when canPause is true.
 *
 * @testability
 * Render with props and assert:
 * - onLobby is called on ← Lobby click.
 * - onPause is called on ⏸ Pause click.
 * - Pause button is absent when canPause is false.
 * - Pause button is disabled when isPausePending is true.
 */
export function GameHeader({
  gameId,
  moveCount,
  turnTimeoutSecs,
  lastMoveAt,
  isSuspended,
  canPause,
  isPausePending,
  isOver,
  onLobby,
  onPause,
  handCards,
}: Props) {
  const { t } = useTranslation()
  const [rulesOpen, setRulesOpen] = useState(false)

  return (
    <>
      <header className={styles.header}>
        <button type="button" className='btn btn-ghost btn-sm' {...testId('game-header-lobby-btn')} onClick={onLobby}>
          {t('lobby.backToLobby')}
        </button>
        <div className={styles.gameInfo}>
          <span className={styles.gameId}>{gameId}</span>
          <span className={styles.moveCount}>{t('game.moveNumber', { n: moveCount })}</span>
          <TurnTimer
            turnTimeoutSecs={turnTimeoutSecs}
            lastMoveAt={lastMoveAt}
            isOver={isOver}
            isSuspended={isSuspended}
          />
        </div>
        <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
          <button type="button"
            {...testId('rules-btn-ingame')}
            className='btn btn-ghost btn-sm'
            onClick={() => setRulesOpen(true)}
          >
            {t('game.viewRules')}
          </button>
          {canPause && (
            <button type="button"
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

      {rulesOpen && (
        <RulesModal
          initialGameId={gameId}
          handCards={handCards}
          onClose={() => setRulesOpen(false)}
        />
      )}
    </>
  )
}
