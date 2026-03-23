export type SkinId = 'obsidian' | 'parchment' | 'slate' | 'ivory'
export type FontSize = 'small' | 'medium' | 'large'

export interface Skin {
  id: SkinId
  name: string
  description: string
  /** Value applied to document.documentElement's data-theme attribute. */
  dataTheme: string
  /** Whether this is a light-mode skin (affects contrast defaults). */
  isLight: boolean
  /** Preview swatch colors — [bg, surface, accent] */
  preview: [string, string, string]
}

export const SKINS: Skin[] = [
  {
    id: 'obsidian',
    name: 'Obsidian',
    description: 'Midnight tavern',
    dataTheme: 'dark',
    isLight: false,
    preview: ['#0a0a0b', '#111114', '#d4a853'],
  },
  {
    id: 'parchment',
    name: 'Parchment',
    description: 'Illuminated manuscript',
    dataTheme: 'light',
    isLight: true,
    preview: ['#f5f0e8', '#ede5d6', '#7b4f2e'],
  },
  {
    id: 'slate',
    name: 'Slate',
    description: 'Strategy room',
    dataTheme: 'slate',
    isLight: false,
    preview: ['#0b0d12', '#12151e', '#7b8cde'],
  },
  {
    id: 'ivory',
    name: 'Ivory',
    description: 'Academy hall',
    dataTheme: 'ivory',
    isLight: true,
    preview: ['#fafaf7', '#f2f2ec', '#4a6741'],
  },
]

export const DEFAULT_SKIN: SkinId = 'obsidian'

export function getSkin(id: SkinId): Skin {
  return SKINS.find(s => s.id === id) ?? SKINS[0]
}

/** Applies the skin's data-theme to the document root. */
export function applySkin(id: SkinId): void {
  const skin = getSkin(id)
  document.documentElement.setAttribute('data-theme', skin.dataTheme)
}

const FONT_SCALE: Record<FontSize, string> = {
  small:  '14px',
  medium: '16px',
  large:  '18px',
}

export function applyFontSize(size: FontSize): void {
  document.documentElement.style.setProperty('--font-scale', String(FONT_SCALE[size]))
}