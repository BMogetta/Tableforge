import { useState, useEffect, useRef } from 'react'
import { CardPile } from '@/ui/cards'
import { sfx } from '@/lib/sfx'
import type { CardName } from './CardDisplay'
import { HandDisplay } from './HandDisplay'
import { TargetPicker } from './TargetPicker'
import { DebuggerModal } from './DebuggerModal'
import { PlayerBoard } from './PlayerBoard'
import { RoundSummary } from './RoundSummary'
import styles from './RootAccess.module.css'

// ---------------------------------------------------------------------------
// State shape mirroring the backend Root Access engine (filtered view).
// ---------------------------------------------------------------------------

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
  set_aside_visible: string[]
  deck: string[]
  hands: Record<string, string[]>
  round_winner_id: string | null
  game_winner_id: string | null
  // Only present for the active player during debugger_pending.
  debugger_choices?: string[]
  // Only present for the Sniffer caster on their turn.
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

// ---------------------------------------------------------------------------
// Player username lookup — state only has IDs; we receive a map from parent.
// ---------------------------------------------------------------------------

interface PlayerInfo {
  id: string
  username: string
}

interface Props {
  state: RootAccessState
  currentPlayerId: string
  localPlayerId: string
  onMove: (payload: Record<string, unknown>) => void
  disabled: boolean
  isOver: boolean
  /** Map of player ID -> username for display. Provided by Game.tsx. */
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
  const [selectedCard, setSelectedCard] = useState<CardName | null>(null)
  const [selectedTarget, setSelectedTarget] = useState<string | null>(null)
  const [selectedGuess, setSelectedGuess] = useState<CardName | null>(null)
  const [showRoundSummary, setShowRoundSummary] = useState(false)
  const [lastSeenRound, setLastSeenRound] = useState(state.round)

  const discardZoneRef = useRef<HTMLElement>(null)

  // Show round summary when a round completes.
  useEffect(() => {
    if (state.phase === 'round_over' && state.round !== lastSeenRound) {
      sfx.play('game.round_end')
      setShowRoundSummary(true)
      setLastSeenRound(state.round)
    }
  }, [state.phase, state.round, lastSeenRound])

  // Reset selection when turn changes.
  useEffect(() => {
    setSelectedCard(null)
    setSelectedTarget(null)
    setSelectedGuess(null)
  }, [currentPlayerId])

  const localHand = (state.hands[localPlayerId] ?? []) as CardName[]
  const isMyTurn = currentPlayerId === localPlayerId && !disabled && !isOver
  const blockedCards = getBlockedCards(localHand)
  const target = tokensToWin(state.players.length)

  // Track hand size for card draw SFX.
  const prevHandSize = useRef(localHand.length)
  useEffect(() => {
    if (localHand.length > prevHandSize.current) {
      sfx.play('game.card_draw')
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

  function getUsername(id: string): string {
    return players.find(p => p.id === id)?.username ?? id.slice(0, 8)
  }

  const opponents = state.players
    .filter(id => id !== localPlayerId)
    .map(id => ({ id, username: getUsername(id) }))

  // Determine whether a target picker should be shown for the selected card.
  const needsTarget = selectedCard
    ? requiresTarget(selectedCard) ||
      (canTargetSelf(selectedCard) &&
        opponents.some(o => !state.eliminated.includes(o.id) && !state.protected.includes(o.id)))
    : false

  // Whether the current selection is ready to submit.
  function isReadyToSubmit(): boolean {
    if (!selectedCard || !isMyTurn) return false
    if (blockedCards.includes(selectedCard)) return false
    if (requiresTarget(selectedCard)) {
      if (!selectedTarget) return false
      if (selectedCard === 'ping' && !selectedGuess) return false
    }
    return true
  }

  function handleSubmit() {
    if (!isReadyToSubmit() || !selectedCard) return
    sfx.play('game.card_play')
    const payload: Record<string, unknown> = { card: selectedCard }
    if (selectedTarget) payload.target_player_id = selectedTarget
    if (selectedGuess) payload.guess = selectedGuess
    onMove(payload)
  }

  function handleDebuggerConfirm(keep: CardName, returnCards: [CardName, CardName]) {
    onMove({
      card: 'debugger_resolve',
      keep,
      return: returnCards,
    })
  }

  // ---------------------------------------------------------------------------
  // Sniffer reveal
  // ---------------------------------------------------------------------------

  const snifferReveal = state.private_reveals?.[localPlayerId]

  // ---------------------------------------------------------------------------
  // Round summary data
  // ---------------------------------------------------------------------------

  const roundSummaryPlayers = state.players.map(id => ({
    id,
    username: getUsername(id),
    tokens: state.tokens[id] ?? 0,
    tokensToWin: target,
    isWinner: id === state.round_winner_id,
    earnedBackdoorBonus:
      state.backdoor_played_by.length === 1 && state.backdoor_played_by[0] === id,
  }))

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className={styles.root}>
      {/* Round / game info bar + deck */}
      <div className={styles.infoBar}>
        <span className={styles.roundBadge}>Round {state.round}</span>
        <div className={styles.deckArea}>
          <CardPile count={state.deck.length} faceDown={true} />
        </div>
        {state.set_aside_visible.length > 0 && (
          <span className={styles.setAside}>Set aside: {state.set_aside_visible.join(', ')}</span>
        )}
      </div>

      {/* Player boards */}
      <div className={styles.boards}>
        {state.players.map(id => (
          <PlayerBoard
            key={id}
            playerId={id}
            username={getUsername(id)}
            tokens={state.tokens[id] ?? 0}
            tokensToWin={target}
            discardPile={(state.discard_piles[id] ?? []) as CardName[]}
            isEliminated={state.eliminated.includes(id)}
            isProtected={state.protected.includes(id)}
            hasPlayedBackdoor={state.backdoor_played_by.includes(id)}
            isLocal={id === localPlayerId}
            isCurrentTurn={id === currentPlayerId}
            dimProtected={selectedCard !== null && needsTarget}
          />
        ))}
      </div>

      {/* Sniffer reveal notification */}
      {snifferReveal && (
        <div className={styles.priestReveal}>
          <span className={styles.priestLabel}>You sniffed --</span>
          <span className={styles.priestCard}>{snifferReveal}</span>
        </div>
      )}

      {/* Local player hand */}
      {localHand.length > 0 && (
        <div className={styles.handSection}>
          <span className={styles.sectionLabel}>Your hand</span>
          <HandDisplay
            cards={localHand}
            selectedCard={selectedCard}
            disabled={!isMyTurn}
            onSelect={setSelectedCard}
            blockedCards={blockedCards}
            targetRef={discardZoneRef}
          />
        </div>
      )}

      {/* Target / guess picker */}
      {isMyTurn && selectedCard && needsTarget && (
        <div className={styles.pickerSection}>
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
        </div>
      )}

      {/* Play button */}
      {isMyTurn && selectedCard && (
        <div className={styles.actions}>
          <button
            className='btn btn-ghost'
            onClick={() => {
              setSelectedCard(null)
              setSelectedTarget(null)
              setSelectedGuess(null)
            }}
          >
            Cancel
          </button>
          <button className='btn btn-primary' onClick={handleSubmit} disabled={!isReadyToSubmit()}>
            Play {selectedCard}
          </button>
        </div>
      )}

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
    </div>
  )
}
