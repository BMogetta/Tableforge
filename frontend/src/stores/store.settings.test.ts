import { describe, it, expect, beforeEach } from 'vitest'
import { useAppStore } from '../stores/store'
import { DEFAULT_SETTINGS, type PlayerSettings } from '../lib/api'

// Reset store state before each test so tests are independent.
beforeEach(() => {
  useAppStore.setState({ settings: { ...DEFAULT_SETTINGS } })
})

// ---------------------------------------------------------------------------
// hydrateSettings
// ---------------------------------------------------------------------------

describe('hydrateSettings', () => {
  it('merges stored values over defaults', () => {
    const raw: PlayerSettings = {
      player_id: 'p1',
      settings: { theme: 'parchment', language: 'es' },
      updated_at: new Date().toISOString(),
    }

    useAppStore.getState().hydrateSettings(raw)
    const s = useAppStore.getState().settings

    expect(s.theme).toBe('parchment')
    expect(s.language).toBe('es')
  })

  it('preserves defaults for keys absent in stored settings', () => {
    const raw: PlayerSettings = {
      player_id: 'p1',
      settings: { theme: 'parchment' },
      updated_at: new Date().toISOString(),
    }

    useAppStore.getState().hydrateSettings(raw)
    const s = useAppStore.getState().settings

    // theme overridden
    expect(s.theme).toBe('parchment')
    // everything else stays at default
    expect(s.language).toBe(DEFAULT_SETTINGS.language)
    expect(s.notify_dm).toBe(DEFAULT_SETTINGS.notify_dm)
    expect(s.volume_master).toBe(DEFAULT_SETTINGS.volume_master)
  })

  it('all keys are present after hydration — no undefined', () => {
    const raw: PlayerSettings = {
      player_id: 'p1',
      settings: {},
      updated_at: new Date().toISOString(),
    }

    useAppStore.getState().hydrateSettings(raw)
    const s = useAppStore.getState().settings

    for (const key of Object.keys(DEFAULT_SETTINGS) as (keyof typeof DEFAULT_SETTINGS)[]) {
      expect(s[key]).not.toBeUndefined()
    }
  })

  it('handles empty settings object', () => {
    const raw: PlayerSettings = {
      player_id: 'p1',
      settings: {},
      updated_at: new Date().toISOString(),
    }

    useAppStore.getState().hydrateSettings(raw)
    const s = useAppStore.getState().settings

    expect(s.theme).toBe(DEFAULT_SETTINGS.theme)
  })
})

// ---------------------------------------------------------------------------
// updateSetting
// ---------------------------------------------------------------------------

describe('updateSetting', () => {
  it('updates a single string key', () => {
    useAppStore.getState().updateSetting('theme', 'parchment')
    expect(useAppStore.getState().settings.theme).toBe('parchment')
  })

  it('updates a boolean key', () => {
    useAppStore.getState().updateSetting('notify_dm', false)
    expect(useAppStore.getState().settings.notify_dm).toBe(false)
  })

  it('updates a numeric key', () => {
    useAppStore.getState().updateSetting('volume_master', 0.5)
    expect(useAppStore.getState().settings.volume_master).toBe(0.5)
  })

  it('does not affect other keys', () => {
    const before = { ...useAppStore.getState().settings }
    useAppStore.getState().updateSetting('theme', 'parchment')
    const after = useAppStore.getState().settings

    expect(after.language).toBe(before.language)
    expect(after.notify_dm).toBe(before.notify_dm)
    expect(after.volume_master).toBe(before.volume_master)
  })

  it('multiple sequential updates accumulate correctly', () => {
    useAppStore.getState().updateSetting('theme', 'parchment')
    useAppStore.getState().updateSetting('language', 'es')
    useAppStore.getState().updateSetting('volume_master', 0.3)

    const s = useAppStore.getState().settings
    expect(s.theme).toBe('parchment')
    expect(s.language).toBe('es')
    expect(s.volume_master).toBe(0.3)
  })
})

// ---------------------------------------------------------------------------
// setSettings
// ---------------------------------------------------------------------------

describe('setSettings', () => {
  it('replaces the entire settings object', () => {
    const override = {
      ...DEFAULT_SETTINGS,
      theme: 'parchment' as const,
      language: 'es' as const,
    }

    useAppStore.getState().setSettings(override)
    const s = useAppStore.getState().settings

    expect(s.theme).toBe('parchment')
    expect(s.language).toBe('es')
  })

  it('does not affect other store state', () => {
    const playerBefore = useAppStore.getState().player

    useAppStore.getState().setSettings({ ...DEFAULT_SETTINGS, theme: 'parchment' })

    expect(useAppStore.getState().player).toBe(playerBefore)
  })
})
