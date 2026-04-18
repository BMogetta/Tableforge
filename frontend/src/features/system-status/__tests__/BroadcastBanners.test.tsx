import { render, screen } from '@testing-library/react'
import { act } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { BroadcastBanners } from '../BroadcastBanners'

// --- GatewaySocket stub ------------------------------------------------------

type Handler = (event: { type: string; payload: unknown }) => void

const gatewayState: {
  handlers: Set<Handler>
  emit: (event: { type: string; payload: unknown }) => void
} = {
  handlers: new Set(),
  emit: e => gatewayState.handlers.forEach(h => h(e)),
}

vi.mock('@/stores/socketStore', () => ({
  useSocketStore: (selector: (s: { gateway: unknown }) => unknown) =>
    selector({
      gateway: {
        on: (h: Handler) => {
          gatewayState.handlers.add(h)
          return () => gatewayState.handlers.delete(h)
        },
      },
    }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

function emitBroadcast(message: string, type: string) {
  act(() => {
    gatewayState.emit({ type: 'broadcast', payload: { message, broadcast_type: type } })
  })
}

describe('BroadcastBanners', () => {
  beforeEach(() => {
    gatewayState.handlers.clear()
  })

  it('renders nothing until a broadcast arrives', () => {
    const { container } = render(<BroadcastBanners />)
    expect(container.firstChild).toBeNull()
  })

  it('shows a banner when a broadcast event is received', () => {
    render(<BroadcastBanners />)

    emitBroadcast('Server restart in 5 minutes', 'warning')

    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('Server restart in 5 minutes')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveAttribute('data-severity', 'warning')
  })

  it('stacks multiple broadcasts and preserves arrival order', () => {
    render(<BroadcastBanners />)

    emitBroadcast('first', 'info')
    emitBroadcast('second', 'info')
    emitBroadcast('third', 'error')

    const alerts = screen.getAllByRole('alert')
    expect(alerts).toHaveLength(3)
    expect(alerts[0]).toHaveTextContent('first')
    expect(alerts[1]).toHaveTextContent('second')
    expect(alerts[2]).toHaveTextContent('third')
    expect(alerts[2]).toHaveAttribute('data-severity', 'error')
  })

  it('falls back to "info" severity for unknown broadcast_type values', () => {
    render(<BroadcastBanners />)

    emitBroadcast('hello', 'something-weird')

    expect(screen.getByRole('alert')).toHaveAttribute('data-severity', 'info')
  })

  it('dismisses a banner when the × button is clicked', () => {
    render(<BroadcastBanners />)

    emitBroadcast('ephemeral', 'info')
    expect(screen.getByRole('alert')).toBeInTheDocument()

    act(() => {
      screen.getByRole('button', { name: 'common.close' }).click()
    })

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('dismisses only the targeted banner when stack has multiple', () => {
    render(<BroadcastBanners />)

    emitBroadcast('keep-a', 'info')
    emitBroadcast('drop', 'info')
    emitBroadcast('keep-b', 'info')

    // Dismiss the middle banner.
    const buttons = screen.getAllByRole('button', { name: 'common.close' })
    act(() => buttons[1].click())

    const remaining = screen.getAllByRole('alert')
    expect(remaining.map(a => a.textContent)).toEqual(
      expect.arrayContaining(['keep-a×', 'keep-b×']),
    )
    expect(remaining).toHaveLength(2)
  })
})
