import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { RootAccess, type RootAccessState } from '../RootAccess'

vi.mock('@/lib/sfx', () => ({
  sfx: { play: vi.fn() },
}))

vi.mock('@/stores/roomStore', () => ({
  useRoomStore: (selector: (s: unknown) => unknown) => selector({ presenceMap: {} }),
}))

vi.mock('@/features/game/top-bar-slot', () => ({
  useGameTopBarSlot: () => null,
}))

// Fly-out animation: allow tests to control when it resolves, so the
// "hung mid-play" scenario can be reproduced deterministically.
const animateResolvers: Array<() => void> = []
vi.mock('motion', () => ({
  animate: vi.fn(
    () =>
      new Promise<void>(resolve => {
        animateResolvers.push(resolve)
      }),
  ),
}))

function resolveAllAnimations() {
  while (animateResolvers.length > 0) {
    const r = animateResolvers.shift()
    r?.()
  }
}

function baseState(localId: string, opponentId: string): RootAccessState {
  return {
    round: 1,
    phase: 'playing',
    players: [localId, opponentId],
    tokens: { [localId]: 0, [opponentId]: 0 },
    eliminated: [],
    protected: [],
    backdoor_played_by: [],
    discard_piles: { [localId]: [], [opponentId]: [] },
    set_aside_visible: [],
    deck: ['ping', 'ping'],
    hands: { [localId]: ['buffer_overflow', 'swap'], [opponentId]: [] },
    round_winner_id: null,
    game_winner_id: null,
  }
}

describe('RootAccess cancel recovery', () => {
  it('does not submit a cancelled move even if the fly-out animation resolves later', async () => {
    animateResolvers.length = 0
    const user = userEvent.setup()
    const onMove = vi.fn()
    const localId = 'local-1'
    const opponentId = 'opp-1'
    const state = baseState(localId, opponentId)

    render(
      <RootAccess
        state={state}
        currentPlayerId={localId}
        localPlayerId={localId}
        onMove={onMove}
        disabled={false}
        isOver={false}
        players={[
          { id: localId, username: 'me' },
          { id: opponentId, username: 'them' },
        ]}
      />,
    )

    const findHandCard = (label: string): HTMLElement => {
      const match = screen.getAllByTestId('card').find(el => el.textContent?.includes(label))
      if (!match) throw new Error(`hand card ${label} not found`)
      return match
    }

    // Select buffer_overflow, pick a target, press Play — the fly-out stays
    // hung because our mocked motion.animate never resolves on its own.
    await user.click(findHandCard('BUFFER_OVERFLOW'))
    await user.click(screen.getByRole('button', { name: 'them' }))
    await act(async () => {
      await user.click(screen.getByRole('button', { name: /play/i }))
    })

    // User hits Cancel while the fly-out is still in flight.
    await user.click(screen.getByRole('button', { name: /cancel/i }))

    // Now the (stale) animation finally resolves. Without the Cancel fix
    // (clearing playingCard + pendingPayloadRef), onPlayComplete would fire
    // with the pending payload and send the cancelled move to the server.
    await act(async () => {
      resolveAllAnimations()
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(onMove).not.toHaveBeenCalled()
    resolveAllAnimations()
  })
})
