import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useAppStore } from '@/stores/store'
import { useSettingsStore } from '@/stores/settingsStore'
import { DEFAULT_SETTINGS } from '@/lib/api'
import { Settings } from '../Settings'
import { ToastProvider } from '../Toast'
import { SKINS } from '@/lib/skins'
import { applySkin } from '@/lib/skins'

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
vi.mock('@/lib/api', async importOriginal => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return {
    ...actual,
    playerSettings: {
      get: vi.fn(),
      update: vi.fn().mockResolvedValue({ player_id: 'p1', settings: {}, updated_at: '' }),
    },
  }
})

vi.mock('@/lib/skins', async importOriginal => {
  const actual = await importOriginal<typeof import('@/lib/skins')>()
  return {
    ...actual,
    applySkin: vi.fn(),
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
  })
  useSettingsStore.setState({
    settings: { ...DEFAULT_SETTINGS },
  })

  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  return render(
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <Settings onClose={onClose} />
      </ToastProvider>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  useSettingsStore.setState({ settings: { ...DEFAULT_SETTINGS } })
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
    // 'Notifications' appears as both section heading and volume label.
    const notifHeadings = screen.getAllByText('Notifications')
    expect(notifHeadings.length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Sound')).toBeInTheDocument()
    expect(screen.getByText('Social')).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', () => {
    const onClose = vi.fn()
    renderSettings(onClose)
    fireEvent.click(screen.getByText('✕'))
    expect(onClose).toHaveBeenCalledOnce()
  })
})

// ---------------------------------------------------------------------------
// Skin controls
// ---------------------------------------------------------------------------

describe('Skin picker', () => {
  it('renders a button for each skin', () => {
    renderSettings()
    SKINS.forEach(skin => {
      expect(screen.getByRole('button', { name: skin.name })).toBeInTheDocument()
    })
  })

  it('marks the current skin as active', () => {
    // DEFAULT_SETTINGS.theme is 'obsidian'
    renderSettings()
    const obsidianBtn = screen.getByRole('button', { name: 'Obsidian' })
    expect(obsidianBtn.className).toMatch(/skinOptionActive/)
  })

  it('updates store when a skin is clicked', () => {
    renderSettings()
    fireEvent.click(screen.getByRole('button', { name: 'Parchment' }))
    expect(useSettingsStore.getState().settings.theme).toBe('parchment')
  })

  it('calls applySkin with the selected skin id', () => {
    renderSettings()
    fireEvent.click(screen.getByRole('button', { name: 'Slate' }))
    expect(applySkin).toHaveBeenCalledWith('slate')
  })

  it('only one skin is active at a time', () => {
    renderSettings()
    fireEvent.click(screen.getByRole('button', { name: 'Ivory' }))
    const activeButtons = SKINS.map(s => screen.getByRole('button', { name: s.name })).filter(btn =>
      btn.className.includes('skinOptionActive'),
    )
    expect(activeButtons).toHaveLength(1)
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
    expect(useSettingsStore.getState().settings.show_move_hints).toBe(false)
  })

  it('reflects updated state after click', () => {
    renderSettings()
    const toggle = screen.getByRole('switch', { name: 'Confirm Move' })
    // Default is false — click should enable it.
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'true')
  })

  it('mute all toggle updates store', () => {
    renderSettings()
    const toggle = screen.getByRole('switch', { name: 'Mute All' })
    expect(toggle).not.toBeDisabled()
    expect(toggle).toHaveAttribute('aria-checked', String(DEFAULT_SETTINGS.mute_all))
    fireEvent.click(toggle)
    expect(useSettingsStore.getState().settings.mute_all).toBe(!DEFAULT_SETTINGS.mute_all)
  })
})

// ---------------------------------------------------------------------------
// Select controls
// ---------------------------------------------------------------------------

describe('SelectRow', () => {
  it('renders with default value', () => {
    renderSettings()
    const select = screen.getByDisplayValue('Medium')
    expect(select).toBeInTheDocument()
  })

  it('optimistically updates store on change', () => {
    renderSettings()
    // Find the Allow DMs select by its current value.
    const select = screen.getByDisplayValue('Anyone')
    fireEvent.change(select, { target: { value: 'nobody' } })
    expect(useSettingsStore.getState().settings.allow_dms).toBe('nobody')
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

  it('volume slider is interactive', () => {
    renderSettings()
    const slider = screen.getByRole('slider', { name: 'Master Volume volume' })
    expect(slider).not.toBeDisabled()
    fireEvent.change(slider, { target: { value: '0.5' } })
    expect(useSettingsStore.getState().settings.volume_master).toBe(0.5)
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
    const { playerSettings } = await import('@/lib/api')
    renderSettings()

    const toggle = screen.getByRole('switch', { name: 'Show Move Hints' })
    await act(async () => {
      fireEvent.click(toggle)
    })

    expect(playerSettings.update).toHaveBeenCalledOnce()
  })
})
