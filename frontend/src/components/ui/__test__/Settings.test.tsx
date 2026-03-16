import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { useAppStore } from '../../../stores/store'
import { DEFAULT_SETTINGS } from '../../../lib/api'
import { Settings } from '../Settings'
import { ToastProvider } from '../Toast'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

// Mock @tanstack/react-pacer so useDebouncedCallback executes synchronously
// in tests — we don't want to deal with timers for the debounce itself.
vi.mock('@tanstack/react-pacer', () => ({
  useDebouncedCallback: (fn: (...args: unknown[]) => unknown) => fn,
}))

// Mock the playerSettings API — we test optimistic update independently from
// the network call.
vi.mock('../../../api', async importOriginal => {
  const actual = await importOriginal<typeof import('../../../lib/api')>()
  return {
    ...actual,
    playerSettings: {
      get: vi.fn(),
      update: vi.fn().mockResolvedValue({ player_id: 'p1', settings: {}, updated_at: '' }),
    },
  }
})

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function renderSettings(onClose = vi.fn()) {
  // Ensure the store has a player so Settings can read player.id.
  useAppStore.setState({
    player: {
      id: 'player-1',
      username: 'alice',
      role: 'player',
      is_bot: false,
      created_at: '',
    },
    settings: { ...DEFAULT_SETTINGS },
  })

  return render(
    <ToastProvider>
      <Settings onClose={onClose} />
    </ToastProvider>,
  )
}

beforeEach(() => {
  useAppStore.setState({ settings: { ...DEFAULT_SETTINGS } })
  localStorage.clear()
})

afterEach(() => {
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

describe('Settings rendering', () => {
  it('renders the settings title', () => {
    renderSettings()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('renders all section headings', () => {
    renderSettings()
    expect(screen.getByText('Appearance')).toBeInTheDocument()
    expect(screen.getByText('Gameplay')).toBeInTheDocument()
    // 'Notifications' appears twice — once as section heading, once as volume label.
    // Assert the section heading specifically by checking for the p element.
    const notifHeadings = screen.getAllByText('Notifications')
    expect(notifHeadings.length).toBeGreaterThanOrEqual(1)
    expect(notifHeadings.some(el => el.tagName === 'P')).toBe(true)
    expect(screen.getByText('Audio')).toBeInTheDocument()
    expect(screen.getByText('Privacy')).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', () => {
    const onClose = vi.fn()
    renderSettings(onClose)
    fireEvent.click(screen.getByText('✕'))
    expect(onClose).toHaveBeenCalledOnce()
  })
})

// ---------------------------------------------------------------------------
// Toggle controls
// ---------------------------------------------------------------------------

describe('ToggleRow', () => {
  it('renders with default checked state', () => {
    renderSettings()
    const toggle = screen.getByRole('switch', { name: 'Show Move Hints' })
    expect(toggle).toHaveAttribute('aria-checked', 'true')
  })

  it('optimistically updates store on click', () => {
    renderSettings()
    const toggle = screen.getByRole('switch', { name: 'Show Move Hints' })
    fireEvent.click(toggle)
    expect(useAppStore.getState().settings.show_move_hints).toBe(false)
  })

  it('reflects updated state after click', () => {
    renderSettings()
    const toggle = screen.getByRole('switch', { name: 'Confirm Move' })
    // Default is false — click should enable it.
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'true')
  })

  it('disabled toggle does not update store', () => {
    renderSettings()
    const toggle = screen.getByRole('switch', { name: 'Mute All' })
    expect(toggle).toBeDisabled()
    fireEvent.click(toggle)
    // Store should remain at default.
    expect(useAppStore.getState().settings.mute_all).toBe(DEFAULT_SETTINGS.mute_all)
  })
})

// ---------------------------------------------------------------------------
// Select controls
// ---------------------------------------------------------------------------

describe('SelectRow', () => {
  it('renders with default value', () => {
    renderSettings()
    const select = screen.getByDisplayValue('Dark')
    expect(select).toBeInTheDocument()
  })

  it('optimistically updates store on change', () => {
    renderSettings()
    // Find the Allow DMs select by its current value.
    const select = screen.getByDisplayValue('Anyone')
    fireEvent.change(select, { target: { value: 'nobody' } })
    expect(useAppStore.getState().settings.allow_dms).toBe('nobody')
  })
})

// ---------------------------------------------------------------------------
// Volume controls
// ---------------------------------------------------------------------------

describe('VolumeRow', () => {
  it('renders volume slider', () => {
    renderSettings()
    const slider = screen.getByRole('slider', { name: 'Master Volume volume' })
    expect(slider).toBeInTheDocument()
  })

  it('slider is disabled in stub state', () => {
    renderSettings()
    const slider = screen.getByRole('slider', { name: 'Master Volume volume' })
    expect(slider).toBeDisabled()
  })
})

// ---------------------------------------------------------------------------
// localStorage cache
// ---------------------------------------------------------------------------

describe('localStorage cache', () => {
  it('writes to localStorage on setting change', () => {
    renderSettings()
    const toggle = screen.getByRole('switch', { name: 'Show Move Hints' })
    fireEvent.click(toggle)

    const cached = localStorage.getItem('tf:settings')
    expect(cached).not.toBeNull()
    const parsed = JSON.parse(cached!)
    expect(parsed.show_move_hints).toBe(false)
  })
})

// ---------------------------------------------------------------------------
// Backend sync
// ---------------------------------------------------------------------------

describe('backend sync', () => {
  it('calls playerSettings.update on setting change', async () => {
    const { playerSettings } = await import('../../../lib/api')
    renderSettings()

    const toggle = screen.getByRole('switch', { name: 'Show Move Hints' })
    await act(async () => {
      fireEvent.click(toggle)
    })

    expect(playerSettings.update).toHaveBeenCalledOnce()
  })
})
