import { useState, useEffect } from 'react'

// Breakpoints mirror industry-standard Tailwind defaults.
// sm: 640, md: 768, lg: 1024, xl: 1280
export type Breakpoint = 'xs' | 'sm' | 'md' | 'lg' | 'xl'

const BREAKPOINTS: { name: Breakpoint; minWidth: number }[] = [
  { name: 'xl',  minWidth: 1280 },
  { name: 'lg',  minWidth: 1024 },
  { name: 'md',  minWidth: 768  },
  { name: 'sm',  minWidth: 640  },
  { name: 'xs',  minWidth: 0    },
]

function getCurrentBreakpoint(): Breakpoint {
  if (typeof window === 'undefined') return 'xs'
  for (const bp of BREAKPOINTS) {
    if (window.innerWidth >= bp.minWidth) return bp.name
  }
  return 'xs'
}

const ORDER: Record<Breakpoint, number> = { xs: 0, sm: 1, md: 2, lg: 3, xl: 4 }

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
    const queries = BREAKPOINTS.slice(0, -1).map(({ name, minWidth }) => {
      const mq = window.matchMedia(`(min-width: ${minWidth}px)`)
      const handler = () => setBreakpoint(getCurrentBreakpoint())
      mq.addEventListener('change', handler)
      return { mq, handler }
    })

    return () => {
      queries.forEach(({ mq, handler }) => mq.removeEventListener('change', handler))
    }
  }, [])

  return {
    breakpoint,
    isMobile:   ORDER[breakpoint] < ORDER['sm'],
    isTablet:   ORDER[breakpoint] >= ORDER['sm'] && ORDER[breakpoint] < ORDER['lg'],
    isDesktop:  ORDER[breakpoint] >= ORDER['lg'],
    isAtLeast:  (bp: Breakpoint) => ORDER[breakpoint] >= ORDER[bp],
  }
}