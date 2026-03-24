import { useState, useEffect } from 'react'
import { type CardName } from './CardDisplay'
import { HandDisplay } from './HandDisplay'
import { TargetPicker } from './TargetPicker'
import { ChancellorModal } from './ChancellorModal'
import { PlayerBoard } from './PlayerBoard'
import { RoundSummary } from './RoundSummary'
import styles from './LoveLetter.module.css'

// ---------------------------------------------------------------------------
// State shape mirroring the backend LoveLetter engine (filtered view).
// ---------------------------------------------------------------------------

export interface LoveLetterState {
  round: number
  phase: 'playing' | 'chancellor_pending' | 'round_over' | 'game_over'
  players: string[]
  tokens: Record<string, number>
  eliminated: string[]
  protected: string[]
  spy_played_by: string[]
  discard_piles: Record<string, string[]>
  set_aside_visible: string[]
  deck: string[]
  hands: Record<string, string[]>
  round_winner_id: string | null
  game_winner_id: string | null
  // Only present for the active player during chancellor_pending.
  chancellor_choices?: string[]
  // Only present for the Priest caster on their turn.
  private_reveals?: Record<string, string>
}

// ---------------------------------------------------------------------------
// Cards that require a target player.
// ---------------------------------------------------------------------------

const CARDS_REQUIRING_TARGET: CardName[] = ['guard', 'priest', 'baron', 'king']

function requiresTarget(card: CardName): boolean {
  return CARDS_REQUIRING_TARGET.includes(card)
}

function canTargetSelf(card: CardName): boolean {
  return card === 'prince'
}

// ---------------------------------------------------------------------------
// Countess blocking — which cards cannot be played when Countess is in hand.
// ---------------------------------------------------------------------------

function getBlockedCards(hand: CardName[]): CardName[] {
  if (!hand.includes('countess')) return []
  const hasKingOrPrince = hand.includes('king') || hand.includes('prince')
  if (!hasKingOrPrince) return []
  return hand.filter(c => c !== 'countess')
}

// ---------------------------------------------------------------------------
// Tokens needed to win by player count.
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
  state: LoveLetterState
  currentPlayerId: string
  localPlayerId: string
  onMove: (payload: Record<string, unknown>) => void
  disabled: boolean
  isOver: boolean
  /** Map of player ID → username for display. Provided by Game.tsx. */
  players?: PlayerInfo[]
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export function LoveLetter({
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

  // Show round summary when a round completes.
  useEffect(() => {
    if (state.phase === 'round_over' && state.round !== lastSeenRound) {
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
      if (selectedCard === 'guard' && !selectedGuess) return false
    }
    return true
  }

  function handleSubmit() {
    if (!isReadyToSubmit() || !selectedCard) return
    const payload: Record<string, unknown> = { card: selectedCard }
    if (selectedTarget) payload.target_player_id = selectedTarget
    if (selectedGuess) payload.guess = selectedGuess
    onMove(payload)
  }

  function handleChancellorConfirm(keep: CardName, returnCards: [CardName, CardName]) {
    onMove({
      card: 'chancellor_resolve',
      keep,
      return: returnCards,
    })
  }

  // ---------------------------------------------------------------------------
  // Priest reveal
  // ---------------------------------------------------------------------------

  const priestReveal = state.private_reveals?.[localPlayerId]

  // ---------------------------------------------------------------------------
  // Round summary data
  // ---------------------------------------------------------------------------

  const roundSummaryPlayers = state.players.map(id => ({
    id,
    username: getUsername(id),
    tokens: state.tokens[id] ?? 0,
    tokensToWin: target,
    isWinner: id === state.round_winner_id,
    earnedSpyBonus: state.spy_played_by.length === 1 && state.spy_played_by[0] === id,
  }))

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------

  return (
    <div className={styles.root}>
      {/* Round / game info bar */}
      <div className={styles.infoBar}>
        <span className={styles.roundBadge}>Round {state.round}</span>
        <span className={styles.deckCount}>
          {state.deck.length} card{state.deck.length !== 1 ? 's' : ''} remaining
        </span>
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
            hasPlayedSpy={state.spy_played_by.includes(id)}
            isLocal={id === localPlayerId}
            isCurrentTurn={id === currentPlayerId}
          />
        ))}
      </div>

      {/* Priest reveal notification */}
      {priestReveal && (
        <div className={styles.priestReveal}>
          <span className={styles.priestLabel}>You peeked —</span>
          <span className={styles.priestCard}>{priestReveal}</span>
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

      {/* Chancellor modal */}
      {state.phase === 'chancellor_pending' &&
        currentPlayerId === localPlayerId &&
        state.chancellor_choices && (
          <ChancellorModal
            choices={state.chancellor_choices as CardName[]}
            onConfirm={handleChancellorConfirm}
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
