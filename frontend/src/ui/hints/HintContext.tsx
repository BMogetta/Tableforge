import { useAppStore } from '@/stores/store'

/**
 * Returns whether the user has enabled move hints in settings.
 */
export function useHintsEnabled(): boolean {
  return useAppStore(s => s.settings.show_move_hints)
}
