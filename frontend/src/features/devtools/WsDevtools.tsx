import { useState, useMemo, useRef, useEffect } from 'react'
import { useWsDevtoolsStore, type CapturedEvent } from './store'
import type { WsEventType } from '@/lib/ws'
import { useClipboard } from '@/hooks/useClipboard'
import { testId } from '@/utils/testId'

// ---------------------------------------------------------------------------
// Filter mode
// ---------------------------------------------------------------------------

const FilterMode = {
  all: 'all',
  include: 'include',
  exclude: 'exclude',
} as const

type FilterMode = (typeof FilterMode)[keyof typeof FilterMode]

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatTime(ts: number): string {
  const d = new Date(ts)
  return (
    d.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    }) +
    '.' +
    String(d.getMilliseconds()).padStart(3, '0')
  )
}

function sourceLabel(source: CapturedEvent['source']): string {
  return source
}

// ---------------------------------------------------------------------------
// EventRow
// ---------------------------------------------------------------------------

interface EventRowProps {
  event: CapturedEvent
  isExpanded: boolean
  onToggle: () => void
}

function EventRow({ event, isExpanded, onToggle }: EventRowProps) {
  const { copy, copied } = useClipboard(1200)

  function copyPayload(e: React.MouseEvent) {
    // Don't toggle the row when clicking the copy button.
    e.stopPropagation()
    copy(JSON.stringify(event.payload, null, 2))
  }

  return (
    <div
      {...testId('event-row')}
      data-event-type={event.type}
      style={{
        borderBottom: '1px solid var(--color-border, rgba(255,255,255,0.08))',
        fontFamily: 'var(--font-mono, monospace)',
        fontSize: 11,
      }}
    >
      {/* Summary row */}
      <div
        {...testId('event-summary')}
        onClick={onToggle}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '5px 10px',
          cursor: 'pointer',
          userSelect: 'none',
        }}
      >
        <span style={{ color: 'var(--color-text-muted, #555)', minWidth: 90 }}>
          {formatTime(event.timestamp)}
        </span>
        <span
          style={{
            padding: '1px 5px',
            borderRadius: 3,
            fontSize: 9,
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            background: 'var(--color-interactive-glow, rgba(123,140,222,0.15))',
            color: 'var(--color-interactive, #7b8cde)',
          }}
          {...testId('event-source')}
        >
          {sourceLabel(event.source)}
        </span>
        <span style={{ flex: 1, color: 'var(--color-text-primary, #e8e4d9)' }}>{event.type}</span>
        <button
          type='button'
          onClick={copyPayload}
          aria-label={copied ? 'Copied' : 'Copy payload'}
          title={copied ? 'Copied!' : 'Copy payload'}
          {...testId('event-copy')}
          style={{
            padding: '1px 6px',
            fontSize: 10,
            background: 'transparent',
            border: '1px solid var(--color-border, rgba(255,255,255,0.1))',
            color: copied
              ? 'var(--color-interactive, #7b8cde)'
              : 'var(--color-text-muted, #555)',
            borderRadius: 3,
            cursor: 'pointer',
            fontFamily: 'inherit',
          }}
        >
          {copied ? '✓' : 'copy'}
        </button>
        <span style={{ color: 'var(--color-text-muted, #555)', fontSize: 10 }}>
          {isExpanded ? '▲' : '▼'}
        </span>
      </div>

      {/* Expanded payload */}
      {isExpanded && (
        <div style={{ borderTop: '1px solid var(--color-border, rgba(255,255,255,0.06))' }}>
          <pre
            {...testId('event-payload')}
            style={{
              margin: 0,
              padding: '6px 10px 6px 24px',
              fontSize: 11,
              color: 'var(--color-text-secondary, #7a7568)',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
            }}
          >
            {JSON.stringify(event.payload, null, 2)}
          </pre>
        </div>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// WsDevtools
// ---------------------------------------------------------------------------

/**
 * WebSocket event inspector panel for TanStack DevTools.
 *
 * Features:
 * - Live event feed from room and player sockets.
 * - Filter by event type (all / include / exclude).
 * - Expand any event to inspect its payload.
 * - Clear all captured events.
 * - Auto-scroll to latest event (pauses when scrolled up).
 *
 * Designed to be rendered inside TanStackDevtools plugins — inherits all
 * CSS variables from the host, no hardcoded colors or sizes.
 *
 * @testability
 * Use useWsDevtoolsStore.setState({ events: [...] }) to inject events.
 * Assert event rows, filter behavior, expand/collapse, and clear button.
 */
export function WsDevtools() {
  const events = useWsDevtoolsStore(s => s.events)
  const clear = useWsDevtoolsStore(s => s.clear)

  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set())
  const [filterMode, setFilterMode] = useState<FilterMode>(FilterMode.all)
  const [filterInput, setFilterInput] = useState('')
  const [autoScroll, setAutoScroll] = useState(true)

  const listRef = useRef<HTMLDivElement>(null)

  // Newest events render at the top of the list — auto-scroll keeps the
  // newest in view by snapping to scrollTop = 0 whenever events change.
  useEffect(() => {
    if (!autoScroll || !listRef.current) return
    listRef.current.scrollTop = 0
  }, [autoScroll])

  // Pause auto-scroll when the user scrolls down to inspect older events.
  function handleScroll() {
    const el = listRef.current
    if (!el) return
    const atTop = el.scrollTop < 32
    setAutoScroll(atTop)
  }

  // Parse filter input — comma or space separated event type names.
  const filterTypes = useMemo<WsEventType[]>(() => {
    return filterInput
      .split(/[\s,]+/)
      .map(s => s.trim())
      .filter(Boolean) as WsEventType[]
  }, [filterInput])

  // Apply filter to events.
  const visibleEvents = useMemo(() => {
    if (filterMode === FilterMode.all || filterTypes.length === 0) return events
    if (filterMode === FilterMode.include) return events.filter(e => filterTypes.includes(e.type))
    return events.filter(e => !filterTypes.includes(e.type))
  }, [events, filterMode, filterTypes])

  function toggleExpand(id: number) {
    setExpandedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  return (
    <div
      {...testId('ws-devtools')}
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        minHeight: 0,
        fontFamily: 'var(--font-mono, monospace)',
      }}
    >
      {/* Toolbar */}
      <div
        {...testId('ws-devtools-toolbar')}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '8px 10px',
          borderBottom: '1px solid var(--color-border, rgba(255,255,255,0.08))',
          flexShrink: 0,
          flexWrap: 'wrap',
        }}
      >
        {/* Event count */}
        <span style={{ fontSize: 11, color: 'var(--color-text-muted, #555)', minWidth: 60 }}>
          {visibleEvents.length} / {events.length} events
        </span>

        {/* Filter mode toggle */}
        <div style={{ display: 'flex', gap: 2 }}>
          {Object.values(FilterMode).map(mode => (
            <button type="button"
              key={mode}
              {...testId(`filter-mode-${mode}`)}
              data-active={filterMode === mode}
              onClick={() => setFilterMode(mode)}
              style={{
                padding: '2px 7px',
                fontSize: 10,
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
                border: '1px solid var(--color-border, rgba(255,255,255,0.1))',
                borderRadius: 3,
                cursor: 'pointer',
                background:
                  filterMode === mode
                    ? 'var(--color-interactive-glow, rgba(123,140,222,0.2))'
                    : 'transparent',
                color:
                  filterMode === mode
                    ? 'var(--color-interactive, #7b8cde)'
                    : 'var(--color-text-muted, #555)',
              }}
            >
              {mode}
            </button>
          ))}
        </div>

        {/* Filter input */}
        {filterMode !== FilterMode.all && (
          <input
            {...testId('filter-input')}
            value={filterInput}
            onChange={e => setFilterInput(e.target.value)}
            placeholder='move_applied, game_over...'
            style={{
              flex: 1,
              minWidth: 120,
              padding: '3px 7px',
              fontSize: 11,
              border: '1px solid var(--color-border, rgba(255,255,255,0.1))',
              borderRadius: 3,
              background: 'transparent',
              color: 'var(--color-text-primary, #e8e4d9)',
              outline: 'none',
              fontFamily: 'var(--font-mono, monospace)',
            }}
          />
        )}

        {/* Spacer */}
        <div style={{ flex: 1 }} />

        {/* Auto-scroll indicator */}
        <span
          {...testId('autoscroll-indicator')}
          style={{
            fontSize: 10,
            color: autoScroll
              ? 'var(--color-interactive, #7b8cde)'
              : 'var(--color-text-muted, #555)',
          }}
        >
          {autoScroll ? '↓ live' : '⏸ paused'}
        </span>

        {/* Clear button */}
        <button type="button"
          {...testId('clear-btn')}
          onClick={clear}
          style={{
            padding: '2px 8px',
            fontSize: 10,
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            border: '1px solid var(--color-border, rgba(255,255,255,0.1))',
            borderRadius: 3,
            cursor: 'pointer',
            background: 'transparent',
            color: 'var(--color-danger, #c0392b)',
          }}
        >
          Clear
        </button>
      </div>

      {/* Event list */}
      <div
        ref={listRef}
        {...testId('event-list')}
        onScroll={handleScroll}
        style={{
          flex: 1,
          overflowY: 'auto',
          minHeight: 0,
        }}
      >
        {visibleEvents.length === 0 ? (
          <div
            {...testId('empty-state')}
            style={{
              padding: '24px 10px',
              textAlign: 'center',
              fontSize: 11,
              color: 'var(--color-text-muted, #555)',
            }}
          >
            {events.length === 0
              ? 'No events captured yet.'
              : 'No events match the current filter.'}
          </div>
        ) : (
          visibleEvents.map(event => (
            <EventRow
              key={event.id}
              event={event}
              isExpanded={expandedIds.has(event.id)}
              onToggle={() => toggleExpand(event.id)}
            />
          ))
        )}
      </div>
    </div>
  )
}
