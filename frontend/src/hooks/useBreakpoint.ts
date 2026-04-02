import { useState, useEffect, useCallback } from 'react'

// Breakpoints — desktop-first (max-width).
// sm: 640, md: 768, lg: 1024, xl: 1280
export type Breakpoint = 'xs' | 'sm' | 'md' | 'lg' | 'xl'

// Ordered largest-first so the first match wins.
const BREAKPOINTS: { name: Breakpoint; query: string }[] = [
  { name: 'xl', query: '(min-width: 1280px)' },
  { name: 'lg', query: '(min-width: 1024px)' },
  { name: 'md', query: '(min-width: 768px)' },
  { name: 'sm', query: '(min-width: 640px)' },
]

const ORDER: Record<Breakpoint, number> = { xs: 0, sm: 1, md: 2, lg: 3, xl: 4 }

function getCurrentBreakpoint(): Breakpoint {
  if (typeof window === 'undefined') return 'xl'
  for (const bp of BREAKPOINTS) {
    if (window.matchMedia(bp.query).matches) return bp.name
  }
  return 'xs'
}

export interface BreakpointState {
  breakpoint: Breakpoint
  /** true when viewport width < 640px */
  isMobile: boolean
  /** true when viewport width is 640px–1023px */
  isTablet: boolean
  /** true when viewport width >= 1024px */
  isDesktop: boolean
  /** Returns true if the current breakpoint is >= the given breakpoint. */
  isAtLeast: (bp: Breakpoint) => boolean
}

export function useBreakpoint(): BreakpointState {
  const [breakpoint, setBreakpoint] = useState<Breakpoint>(getCurrentBreakpoint)

  useEffect(() => {
    const handler = () => setBreakpoint(getCurrentBreakpoint())

    const mediaQueries = BREAKPOINTS.map(({ query }) => {
      const mq = window.matchMedia(query)
      mq.addEventListener('change', handler)
      return mq
    })

    return () => {
      mediaQueries.forEach(mq => mq.removeEventListener('change', handler))
    }
  }, [])

  const isAtLeast = useCallback(
    (bp: Breakpoint) => ORDER[breakpoint] >= ORDER[bp],
    [breakpoint],
  )

  return {
    breakpoint,
    isMobile: ORDER[breakpoint] < ORDER.sm,
    isTablet: ORDER[breakpoint] >= ORDER.sm && ORDER[breakpoint] < ORDER.lg,
    isDesktop: ORDER[breakpoint] >= ORDER.lg,
    isAtLeast,
  }
}
