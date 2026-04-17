import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { Move } from '@/lib/schema-generated.zod'
import { ReplayView } from '../Replay'

// Register a dummy renderer so the test does not depend on which games the
// real registry ships — this exercises ONLY the Replay → GAME_RENDERERS wiring.
vi.mock('@/games/registry', () => ({
  GAME_RENDERERS: {
    'audit-game': (props: {
      gameData: { data: unknown }
      disabled: boolean
      players: { id: string; username: string }[]
    }) => (
      <div data-testid='fake-renderer' data-disabled={String(props.disabled)}>
        {JSON.stringify({ data: props.gameData.data, players: props.players })}
      </div>
    ),
  },
}))

function buildMove(stateAfter: unknown): Move {
  const encoded = btoa(JSON.stringify(stateAfter))
  return {
    id: crypto.randomUUID(),
    session_id: crypto.randomUUID(),
    player_id: crypto.randomUUID(),
    move_number: 1,
    payload: {},
    applied_at: new Date().toISOString(),
    state_after: encoded,
  }
}

describe('ReplayView', () => {
  it('shows the starting-position placeholder at step 0 (no game-specific branch)', () => {
    render(
      <ReplayView moves={[buildMove({ current_player_id: '', data: {} })]} gameId='audit-game' />,
    )
    expect(screen.getByTestId('replay-initial-placeholder')).toBeInTheDocument()
    expect(screen.queryByTestId('fake-renderer')).not.toBeInTheDocument()
  })

  it('delegates rendering to the registered plugin for step > 0', async () => {
    const user = userEvent.setup()
    const boardState = { current_player_id: 'p1', data: { signal: 'from-registry' } }
    render(<ReplayView moves={[buildMove(boardState)]} gameId='audit-game' />)

    await user.click(screen.getByTestId('replay-next-btn'))

    const rendered = await screen.findByTestId('fake-renderer')
    expect(rendered).toHaveAttribute('data-disabled', 'true')
    expect(rendered.textContent).toContain('from-registry')
  })

  it('forwards participants to the renderer', async () => {
    const user = userEvent.setup()
    const players = [
      { id: 'p-alice', username: 'alice' },
      { id: 'p-bob', username: 'bob' },
    ]
    render(
      <ReplayView
        moves={[buildMove({ current_player_id: '', data: {} })]}
        gameId='audit-game'
        players={players}
      />,
    )
    await user.click(screen.getByTestId('replay-next-btn'))

    const rendered = await screen.findByTestId('fake-renderer')
    expect(rendered.textContent).toContain('alice')
    expect(rendered.textContent).toContain('bob')
  })

  it('falls back to a JSON dump when the gameId is not in the registry', async () => {
    const user = userEvent.setup()
    render(
      <ReplayView
        moves={[buildMove({ current_player_id: '', data: { foo: 'bar' } })]}
        gameId='unknown-game'
      />,
    )
    await user.click(screen.getByTestId('replay-next-btn'))

    expect(screen.queryByTestId('fake-renderer')).not.toBeInTheDocument()
    expect(screen.getByText(/No visual renderer for/i)).toBeInTheDocument()
    expect(screen.getByText(/"foo": "bar"/)).toBeInTheDocument()
  })
})
