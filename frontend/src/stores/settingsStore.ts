import { create } from 'zustand'
import type { PlayerSettingMap, PlayerSettings } from '@/lib/api'
import { DEFAULT_SETTINGS } from '@/lib/api'
import { applyFontSize, applySkin, type FontSize, type SkinId } from '@/lib/skins'
import { i18n } from '@/lib/i18n'

/** All keys guaranteed non-optional after merging stored values over defaults. */
export type ResolvedSettings = Required<PlayerSettingMap>

interface SettingsState {
  settings: ResolvedSettings

  /** Merge raw API response over defaults. */
  hydrateSettings: (raw: PlayerSettings) => void

  /** Optimistic single-key update. Caller debounces the backend sync. */
  updateSetting: <K extends keyof PlayerSettingMap>(key: K, value: PlayerSettingMap[K]) => void

  /** Replace all settings at once (localStorage cache load). */
  setSettings: (settings: ResolvedSettings) => void
}

export const useSettingsStore = create<SettingsState>(set => ({
  settings: { ...DEFAULT_SETTINGS },

  hydrateSettings: (raw: PlayerSettings) => {
    const merged: ResolvedSettings = { ...DEFAULT_SETTINGS, ...raw.settings }
    set({ settings: merged })
    if (merged.theme) applySkin(merged.theme as SkinId)
    if (merged.font_size) applyFontSize(merged.font_size as FontSize)
    if (merged.language) i18n.changeLanguage(merged.language)
  },

  updateSetting: (key, value) => {
    set(state => ({
      settings: { ...state.settings, [key]: value },
    }))
    if (key === 'theme') applySkin(value as SkinId)
    if (key === 'font_size') applyFontSize(value as FontSize)
    if (key === 'language') i18n.changeLanguage(value as string)
  },

  setSettings: settings => set({ settings }),
}))
