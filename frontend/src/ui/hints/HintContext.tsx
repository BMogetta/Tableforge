import { useSettingsStore } from '@/stores/settingsStore'

/**
 * Returns whether the user has enabled move hints in settings.
 */
export function useHintsEnabled(): boolean {
  return useSettingsStore(s => s.settings.show_move_hints)
}
