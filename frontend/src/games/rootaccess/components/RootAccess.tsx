import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { useGameTopBarSlot } from '@/features/game/top-bar-slot'
import { sfx } from '@/lib/sfx'
import { useAppStore } from '@/stores/store'
import { Card, CardPile } from '@/ui/cards'
import { ModalOverlay } from '@/ui/ModalOverlay'
import { CardFace } from './CardFace'
import { CentralDiscard } from './CentralDiscard'
import type { CardName } from './CardDisplay'
import { DebuggerModal } from './DebuggerModal'
import { HandDisplay } from './HandDisplay'
import { PlayerBoard } from './PlayerBoard'
import styles from './RootAccess.module.css'
import { RoundSummary } from './RoundSummary'
import { TargetPicker } from './TargetPicker'

// ---------------------------------------------------------------------------
// State shape mirroring the backend Root Access engine (filtered view).
// ---------------------------------------------------------------------------

/** @package */
export interface RootAccessDiscardEntry {
  player_id: string
  card: string
}

/** @package */
export interface RootAccessState {
  round: number
  phase: 'playing' | 'debugger_pending' | 'round_over' | 'game_over'
  players: string[]
  tokens: Record<string, number>
  eliminated: string[]
  protected: string[]
  backdoor_played_by: string[]
  discard_piles: Record<string, string[]>
  /**
   * Global chronological discard order (oldest first).
   * Populated by the backend in phase 2; absent means UI falls back to per-player piles.
   */
  discard_order?: RootAccessDiscardEntry[]
  set_aside_visible: string[]
  deck: string[]
  hands: Record<string, string[]>
  /**
   * Winner of the most recently completed round. Persists across round
   * transitions so the client can render a round summary during the hand-off
   * to the new round. Null until the first round ends.
   */
  round_winner_id: string | null
  /** Snapshot of backdoor_played_by at round end, for the round summary UI. */
  last_round_backdoor_bonus_by?: string[]
  game_winner_id: string | null
  debugger_choices?: string[]
  private_reveals?: Record<string, string>
}

// ---------------------------------------------------------------------------
// Cards that require a target player.
// ---------------------------------------------------------------------------

const CARDS_REQUIRING_TARGET: CardName[] = ['ping', 'sniffer', 'buffer_overflow', 'swap']

function requiresTarget(card: CardName): boolean {
  return CARDS_REQUIRING_TARGET.includes(card)
}

function canTargetSelf(card: CardName): boolean {
  return card === 'reboot'
}

// ---------------------------------------------------------------------------
// Encrypted Key blocking — which cards cannot be played when Encrypted Key is in hand.
// ---------------------------------------------------------------------------

function getBlockedCards(hand: CardName[]): CardName[] {
  if (!hand.includes('encrypted_key')) return []
  const hasSwapOrReboot = hand.includes('swap') || hand.includes('reboot')
  if (!hasSwapOrReboot) return []
  return hand.filter(c => c !== 'encrypted_key')
}

// ---------------------------------------------------------------------------
// Access Tokens needed to win by player count.
// ---------------------------------------------------------------------------

function tokensToWin(playerCount: number): number {
  if (playerCount === 2) return 7
  if (playerCount === 3) return 5
  if (playerCount === 4) return 4
  return 3
}

interface PlayerInfo {
  id: string
  username: string
  is_bot?: boolean
  bot_profile?: 'easy' | 'medium' | 'hard' | 'aggressive'
}

interface Props {
  state: RootAccessState
  currentPlayerId: string
  localPlayerId: string
  onMove: (payload: Record<string, unknown>) => void
  disabled: boolean
  isOver: boolean
  players?: PlayerInfo[]
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

/** @package */
export function RootAccess({
  state,
  currentPlayerId,
  localPlayerId,
  onMove,
  disabled,
  isOver,
  players = [],
}: Props) {
  const { t } = useTranslation()
  const presenceMap = useAppStore(s => s.presenceMap)
  const topBarSlot = useGameTopBarSlot()
  const [selectedCard, setSelectedCard] = useState<CardName | null>(null)
  const [selectedTarget, setSelectedTarget] = useState<string | null>(null)
  const [selectedGuess, setSelectedGuess] = useState<CardName | null>(null)
  const [showRoundSummary, setShowRoundSummary] = useState(false)
  const [lastSeenRound, setLastSeenRound] = useState(state.round)
  const [showSetAside, setShowSetAside] = useState(false)

  // Fly-out animation state — when set, HandDisplay animates this card toward
  // the central discard pile before the parent fires the move mutation.
  const [playingCard, setPlayingCard] = useState<CardName | null>(null)
  const pendingPayloadRef = useRef<Record<string, unknown> | null>(null)

  const discardZoneRef = useRef<HTMLElement | null>(null)

  // Show round summary when the round number advances. The engine applies the
  // round result (tokens, round_winner_id, last_round_backdoor_bonus_by) and
  // the next-round deal atomically in a single ApplyMove, so the client uses
  // the round delta as the trigger rather than a discrete "round_over" phase.
  useEffect(() => {
    if (state.round > lastSeenRound) {
      sfx.play('game.round_end')
      setShowRoundSummary(true)
      setLastSeenRound(state.round)
    }
  }, [state.round, lastSeenRound])

  // Reset selection whenever the active player changes, so stale picks from
  // a previous turn/round don't carry over to the next "Play <card>" button.
  useEffect(() => {
    setSelectedCard(null)
    setSelectedTarget(null)
    setSelectedGuess(null)
  }, [currentPlayerId])

  const localHand = (state.hands[localPlayerId] ?? []) as CardName[]
  const isMyTurn = currentPlayerId === localPlayerId && !disabled && !isOver
  const blockedCards = getBlockedCards(localHand)
  const target = tokensToWin(state.players.length)

  // Track hand size for card deal/draw SFX.
  // On mount, if we already have cards, play a deal SFX (initial hand). Subsequent
  // growth in hand size means a card was drawn — play the draw SFX (timed to match
  // the staggered card entry animation in HandDisplay so the sound lands with the
  // second card appearing).
  const prevHandSize = useRef(0)
  useEffect(() => {
    if (prevHandSize.current === 0 && localHand.length > 0) {
      sfx.play('card.deal')
      if (localHand.length > 1) {
        const t = window.setTimeout(() => sfx.play('card.draw'), 350)
        prevHandSize.current = localHand.length
        return () => window.clearTimeout(t)
      }
    } else if (localHand.length > prevHandSize.current) {
      sfx.play('card.draw')
    }
    prevHandSize.current = localHand.length
  }, [localHand.length])

  // Track eliminations for SFX.
  const prevEliminated = useRef(state.eliminated.length)
  useEffect(() => {
    if (state.eliminated.length > prevEliminated.current) {
      sfx.play('game.elimination')
    }
    prevEliminated.current = state.eliminated.length
  }, [state.eliminated.length])

  // Token gain SFX — only for the local player to avoid end-of-round cacophony.
  const localTokens = state.tokens[localPlayerId] ?? 0
  const prevLocalTokens = useRef(localTokens)
  useEffect(() => {
    if (localTokens > prevLocalTokens.current) {
      sfx.play('chip.place')
    }
    prevLocalTokens.current = localTokens
  }, [localTokens])

  function getUsername(id: string): string {
    return players.find(p => p.id === id)?.username ?? id.slice(0, 8)
  }

  const opponents = state.players
    .filter(id => id !== localPlayerId)
    .map(id => ({ id, username: getUsername(id) }))

  function opponentHandSize(id: string): number {
    if (state.eliminated.includes(id)) return 0
    const reported = state.hands[id]?.length
    if (reported && reported > 0) return reported
    // Server may filter opponent hands to empty. Assume 1 card as a safe default.
    return 1
  }

  // A target-requiring card resolves as a no-op when every opponent is
  // firewalled or eliminated. In that case, skip the TargetPicker and let
  // the play button submit immediately — the server accepts the move with
  // no target_player_id (and, for Ping, no guess).
  const hasValidTargets = opponents.some(
    o => !state.eliminated.includes(o.id) && !state.protected.includes(o.id),
  )

  const needsTarget = selectedCard
    ? (requiresTarget(selectedCard) || canTargetSelf(selectedCard)) && hasValidTargets
    : false

  function isReadyToSubmit(): boolean {
    if (!selectedCard || !isMyTurn) return false
    if (blockedCards.includes(selectedCard)) return false
    if (requiresTarget(selectedCard) && hasValidTargets) {
      if (!selectedTarget) return false
      if (selectedCard === 'ping' && !selectedGuess) return false
    }
    return true
  }

  function handleSubmit() {
    if (!isReadyToSubmit() || !selectedCard) return
    sfx.play('card.play')
    const payload: Record<string, unknown> = { card: selectedCard }
    if (selectedTarget) payload.target_player_id = selectedTarget
    if (selectedGuess) payload.guess = selectedGuess
    // Trigger fly-out animation first; onMove fires when it completes.
    pendingPayloadRef.current = payload
    setPlayingCard(selectedCard)
  }

  function handlePlayComplete() {
    const payload = pendingPayloadRef.current
    pendingPayloadRef.current = null
    setPlayingCard(null)
    setSelectedCard(null)
    setSelectedTarget(null)
    setSelectedGuess(null)
    if (payload) onMove(payload)
  }

  function handleDebuggerConfirm(keep: CardName, returnCards: [CardName, CardName]) {
    onMove({
      card: 'debugger_resolve',
      keep,
      return: returnCards,
    })
  }

  const snifferReveal = state.private_reveals?.[localPlayerId]

  const backdoorBonusRecipients = state.last_round_backdoor_bonus_by ?? []
  const roundSummaryPlayers = state.players.map(id => ({
    id,
    username: getUsername(id),
    tokens: state.tokens[id] ?? 0,
    tokensToWin: target,
    isWinner: id === state.round_winner_id,
    earnedBackdoorBonus:
      backdoorBonusRecipients.length === 1 && backdoorBonusRecipients[0] === id,
  }))

  const discardPilesTyped = state.discard_piles as Record<string, CardName[]>
  const discardOrderTyped = state.discard_order as
    | { player_id: string; card: CardName }[]
    | undefined

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  const topBarExtras = (
    <span className={styles.roundBadge}>{t('rootaccess.round', { n: state.round })}</span>
  )

  return (
    <div className={styles.root}>
      {topBarSlot && createPortal(topBarExtras, topBarSlot)}

      {/* Opponents row */}
      <div className={styles.opponents}>
        {opponents.map(({ id }) => {
          const p = players.find(pl => pl.id === id)
          const isBot = p?.is_bot ?? false
          const isBotThinking = id === currentPlayerId && isBot && !isOver
          // Presence dot only for human opponents — bots are always "there".
          const isOnline = isBot ? undefined : (presenceMap[id] ?? false)
          return (
            <PlayerBoard
              key={id}
              playerId={id}
              username={getUsername(id)}
              tokens={state.tokens[id] ?? 0}
              tokensToWin={target}
              handSize={opponentHandSize(id)}
              isEliminated={state.eliminated.includes(id)}
              isProtected={state.protected.includes(id)}
              hasPlayedBackdoor={state.backdoor_played_by.includes(id)}
              isLocal={false}
              isCurrentTurn={id === currentPlayerId}
              isBotThinking={isBotThinking}
              isBot={isBot}
              botProfile={p?.bot_profile}
              isOnline={isOnline}
              dimProtected={selectedCard !== null && needsTarget}
            />
          )
        })}
      </div>

      {/* Central table — set-aside (left) + deck + discard pile. */}
      <div className={styles.table}>
        <div className={styles.setAsideSlot}>
          {state.set_aside_visible.length > 0 ? (
            <>
              <button
                type='button'
                className={styles.setAsideStack}
                onClick={() => setShowSetAside(true)}
                aria-label={t('rootaccess.setAsideLabel')}
              >
                {state.set_aside_visible.slice(0, 3).map((card, i, arr) => {
                  const isTop = i === arr.length - 1
                  return (
                    <div
                      key={`${card}-${i}`}
                      className={styles.setAsideLayer}
                      style={{ transform: `translate(${-i * 3}px, ${-i * 3}px)`, zIndex: i }}
                    >
                      <Card
                        disabled
                        front={
                          isTop ? (
                            <div className={styles.setAsideFace}>
                              <CardFace card={card as CardName} />
                            </div>
                          ) : (
                            <div className={styles.setAsideFace} />
                          )
                        }
                      />
                    </div>
                  )
                })}
              </button>
              <span className={styles.setAsideLabel}>{t('rootaccess.setAsideLabel')}</span>
            </>
          ) : (
            <>
              <div className={styles.setAsidePlaceholder} aria-hidden='true' />
              <span className={styles.setAsideLabel}>{t('rootaccess.setAsideLabel')}</span>
            </>
          )}
        </div>
        <div className={styles.deckSlot}>
          <CardPile count={state.deck.length} faceDown={true} />
        </div>
        <CentralDiscard
          discardOrder={discardOrderTyped}
          discardPiles={discardPilesTyped}
          getUsername={getUsername}
          pileRef={discardZoneRef}
        />
      </div>

      {/* Sniffer reveal notification */}
      {snifferReveal && (
        <div className={styles.priestReveal}>
          <span className={styles.priestLabel}>You sniffed --</span>
          <span className={styles.priestCard}>{snifferReveal}</span>
        </div>
      )}

      {/* Local player area: hand above, info strip pinned to the viewport
          bottom so it stays visually aligned with the Friends FAB. */}
      <div className={styles.localArea}>
        {localHand.length > 0 && (
          <HandDisplay
            cards={localHand}
            selectedCard={selectedCard}
            disabled={!isMyTurn}
            onSelect={setSelectedCard}
            blockedCards={blockedCards}
            targetRef={discardZoneRef}
            playingCard={playingCard}
            onPlayComplete={handlePlayComplete}
          />
        )}

        <div className={styles.localStripFixed}>
          <PlayerBoard
            playerId={localPlayerId}
            username={getUsername(localPlayerId)}
            tokens={localTokens}
            tokensToWin={target}
            isEliminated={state.eliminated.includes(localPlayerId)}
            isProtected={state.protected.includes(localPlayerId)}
            hasPlayedBackdoor={state.backdoor_played_by.includes(localPlayerId)}
            isLocal={true}
            isCurrentTurn={currentPlayerId === localPlayerId}
          />
        </div>

        {/* Floating action overlay — picker + play button, anchored above the hand
            so it doesn't push content and cause a scroll. */}
        {isMyTurn && selectedCard && (
          <div className={styles.actionOverlay} role='dialog' aria-label={t('rootaccess.play', { card: selectedCard })}>
            {needsTarget && (
              <TargetPicker
                opponents={opponents}
                eliminatedIds={state.eliminated}
                protectedIds={state.protected}
                selectedTarget={selectedTarget}
                cardBeingPlayed={selectedCard}
                selectedGuess={selectedGuess}
                onSelectTarget={setSelectedTarget}
                onSelectGuess={setSelectedGuess}
              />
            )}
            <div className={styles.actions}>
              <button
                type='button'
                className='btn btn-ghost'
                onClick={() => {
                  setSelectedCard(null)
                  setSelectedTarget(null)
                  setSelectedGuess(null)
                }}
              >
                {t('common.cancel')}
              </button>
              <button
                type='button'
                className='btn btn-primary'
                onClick={handleSubmit}
                disabled={!isReadyToSubmit()}
              >
                {t('rootaccess.play', { card: selectedCard })}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Debugger modal */}
      {state.phase === 'debugger_pending' &&
        currentPlayerId === localPlayerId &&
        state.debugger_choices && (
          <DebuggerModal
            choices={state.debugger_choices as CardName[]}
            onConfirm={handleDebuggerConfirm}
          />
        )}

      {/* Round summary overlay */}
      {showRoundSummary && (
        <RoundSummary
          round={state.round - 1}
          winnerId={state.round_winner_id}
          players={roundSummaryPlayers}
          onDismiss={() => setShowRoundSummary(false)}
        />
      )}

      {/* Set-aside reveal modal — matches the discard history modal style. */}
      {showSetAside && state.set_aside_visible.length > 0 && (
        <ModalOverlay onClose={() => setShowSetAside(false)}>
          <div
            className={styles.setAsideModal}
            role='dialog'
            aria-modal='true'
            aria-labelledby='set-aside-title'
          >
            <header className={styles.setAsideHeader}>
              <h2 id='set-aside-title' className={styles.setAsideTitle}>
                {t('rootaccess.setAsideLabel')}
              </h2>
              <button
                type='button'
                className='btn btn-ghost btn-sm'
                onClick={() => setShowSetAside(false)}
                aria-label={t('common.closeDialog')}
              >
                ✕
              </button>
            </header>
            <div className={styles.setAsideBody}>
              {state.set_aside_visible.map((card, i) => (
                <div key={`${card}-${i}`} className={styles.setAsideRevealCard}>
                  <CardFace card={card as CardName} />
                </div>
              ))}
            </div>
          </div>
        </ModalOverlay>
      )}
    </div>
  )
}
