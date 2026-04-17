import { fireEvent, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import type { CapturedEvent } from '../store'
import { useWsDevtoolsStore } from '../store'
import { WsDevtools } from '../WsDevtools'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeEvent(overrides?: Partial<CapturedEvent>): CapturedEvent {
  return {
    id: Math.random(),
    timestamp: Date.now(),
    source: 'gateway',
    type: 'move_applied',
    payload: { session: { id: 'abc', move_count: 1 } },
    ...overrides,
  }
}

function seedEvents(events: CapturedEvent[]) {
  useWsDevtoolsStore.setState({ events })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('WsDevtools', () => {
  beforeEach(() => {
    useWsDevtoolsStore.setState({ events: [] })
  })

  // ── Empty state ──────────────────────────────────────────────────────────

  it('shows empty state when no events', () => {
    render(<WsDevtools />)
    expect(screen.getByTestId('empty-state')).toHaveTextContent('No events captured yet.')
  })

  it('shows event count in toolbar', () => {
    seedEvents([makeEvent(), makeEvent()])
    render(<WsDevtools />)
    expect(screen.getByTestId('ws-devtools-toolbar')).toHaveTextContent('2 / 2 events')
  })

  // ── Event rows ────────────────────────────────────────────────────────────

  it('renders a row for each event', () => {
    seedEvents([makeEvent(), makeEvent(), makeEvent()])
    render(<WsDevtools />)
    expect(screen.getAllByTestId('event-row')).toHaveLength(3)
  })

  it('shows event type in summary row', () => {
    seedEvents([makeEvent({ type: 'game_over' })])
    render(<WsDevtools />)
    expect(screen.getByTestId('event-row')).toHaveTextContent('game_over')
  })

  it('shows source badge', () => {
    seedEvents([makeEvent({ source: 'gateway' })])
    render(<WsDevtools />)
    expect(screen.getByTestId('event-source')).toHaveTextContent('gateway')
  })

  // ── Expand / collapse ─────────────────────────────────────────────────────

  it('payload is hidden by default', () => {
    seedEvents([makeEvent()])
    render(<WsDevtools />)
    expect(screen.queryByTestId('event-payload')).not.toBeInTheDocument()
  })

  it('clicking summary expands payload', () => {
    seedEvents([makeEvent({ payload: { foo: 'bar' } })])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('event-summary'))
    expect(screen.getByTestId('event-payload')).toHaveTextContent('foo')
  })

  it('clicking again collapses payload', () => {
    seedEvents([makeEvent()])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('event-summary'))
    expect(screen.getByTestId('event-payload')).toBeInTheDocument()
    fireEvent.click(screen.getByTestId('event-summary'))
    expect(screen.queryByTestId('event-payload')).not.toBeInTheDocument()
  })

  it('expanding one row does not expand others', () => {
    seedEvents([makeEvent(), makeEvent(), makeEvent()])
    render(<WsDevtools />)
    const summaries = screen.getAllByTestId('event-summary')
    fireEvent.click(summaries[1])
    expect(screen.getAllByTestId('event-payload')).toHaveLength(1)
  })

  // ── Clear ─────────────────────────────────────────────────────────────────

  it('clear button removes all events', () => {
    seedEvents([makeEvent(), makeEvent()])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('clear-btn'))
    expect(screen.getByTestId('empty-state')).toBeInTheDocument()
    expect(useWsDevtoolsStore.getState().events).toHaveLength(0)
  })

  // ── Filter — mode ─────────────────────────────────────────────────────────

  it('filter mode defaults to all', () => {
    render(<WsDevtools />)
    expect(screen.getByTestId('filter-mode-all')).toHaveAttribute('data-active', 'true')
    expect(screen.getByTestId('filter-mode-include')).toHaveAttribute('data-active', 'false')
    expect(screen.getByTestId('filter-mode-exclude')).toHaveAttribute('data-active', 'false')
  })

  it('include mode shows filter input', () => {
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-include'))
    expect(screen.getByTestId('filter-input')).toBeInTheDocument()
  })

  it('exclude mode shows filter input', () => {
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-exclude'))
    expect(screen.getByTestId('filter-input')).toBeInTheDocument()
  })

  it('all mode hides filter input', () => {
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-include'))
    fireEvent.click(screen.getByTestId('filter-mode-all'))
    expect(screen.queryByTestId('filter-input')).not.toBeInTheDocument()
  })

  // ── Filter — include ──────────────────────────────────────────────────────

  it('include filter shows only matching events', () => {
    seedEvents([
      makeEvent({ type: 'move_applied' }),
      makeEvent({ type: 'game_over' }),
      makeEvent({ type: 'move_applied' }),
    ])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-include'))
    fireEvent.change(screen.getByTestId('filter-input'), { target: { value: 'game_over' } })
    expect(screen.getAllByTestId('event-row')).toHaveLength(1)
    expect(screen.getByTestId('event-row')).toHaveTextContent('game_over')
  })

  it('include filter with no match shows filter empty state', () => {
    seedEvents([makeEvent({ type: 'move_applied' })])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-include'))
    fireEvent.change(screen.getByTestId('filter-input'), { target: { value: 'game_over' } })
    expect(screen.getByTestId('empty-state')).toHaveTextContent('No events match')
    expect(screen.getByTestId('ws-devtools-toolbar')).toHaveTextContent('0 / 1 events')
  })

  // ── Filter — exclude ──────────────────────────────────────────────────────

  it('exclude filter hides matching events', () => {
    seedEvents([
      makeEvent({ type: 'move_applied' }),
      makeEvent({ type: 'presence_update' }),
      makeEvent({ type: 'move_applied' }),
    ])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-exclude'))
    fireEvent.change(screen.getByTestId('filter-input'), { target: { value: 'presence_update' } })
    expect(screen.getAllByTestId('event-row')).toHaveLength(2)
    screen.getAllByTestId('event-row').forEach(row => {
      expect(row).not.toHaveTextContent('presence_update')
    })
  })

  it('exclude filter with empty input shows all events', () => {
    seedEvents([makeEvent(), makeEvent()])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-exclude'))
    expect(screen.getAllByTestId('event-row')).toHaveLength(2)
  })

  // ── Filter — comma separated ──────────────────────────────────────────────

  it('include filter supports comma-separated types', () => {
    seedEvents([
      makeEvent({ type: 'move_applied' }),
      makeEvent({ type: 'game_over' }),
      makeEvent({ type: 'presence_update' }),
    ])
    render(<WsDevtools />)
    fireEvent.click(screen.getByTestId('filter-mode-include'))
    fireEvent.change(screen.getByTestId('filter-input'), {
      target: { value: 'move_applied, game_over' },
    })
    expect(screen.getAllByTestId('event-row')).toHaveLength(2)
  })
})
