import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { type Breakpoint, useBreakpoint } from './useBreakpoint'

// Mock matchMedia — each test configures which breakpoint is "active".
function mockMatchMedia(activeBreakpoint: Breakpoint) {
  const thresholds: Record<string, number> = {
    '(min-width: 1280px)': 1280,
    '(min-width: 1024px)': 1024,
    '(min-width: 768px)': 768,
    '(min-width: 640px)': 640,
  }
  const order: Record<Breakpoint, number> = { xs: 0, sm: 640, md: 768, lg: 1024, xl: 1280 }
  const fakeWidth = order[activeBreakpoint] || 0

  window.matchMedia = vi.fn().mockImplementation((query: string) => {
    const threshold = thresholds[query] ?? Infinity
    return {
      matches: fakeWidth >= threshold,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      onchange: null,
      dispatchEvent: vi.fn(),
    }
  })
}

describe('useBreakpoint', () => {
  it('returns xl for large screens', () => {
    mockMatchMedia('xl')
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current.breakpoint).toBe('xl')
    expect(result.current.isDesktop).toBe(true)
    expect(result.current.isMobile).toBe(false)
    expect(result.current.isTablet).toBe(false)
  })

  it('returns sm for small screens', () => {
    mockMatchMedia('sm')
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current.breakpoint).toBe('sm')
    expect(result.current.isMobile).toBe(false)
    expect(result.current.isTablet).toBe(true)
  })

  it('returns xs when no breakpoints match', () => {
    mockMatchMedia('xs')
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current.breakpoint).toBe('xs')
    expect(result.current.isMobile).toBe(true)
  })

  it('isAtLeast works correctly', () => {
    mockMatchMedia('lg')
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current.isAtLeast('md')).toBe(true)
    expect(result.current.isAtLeast('lg')).toBe(true)
    expect(result.current.isAtLeast('xl')).toBe(false)
  })

  it('md is classified as tablet', () => {
    mockMatchMedia('md')
    const { result } = renderHook(() => useBreakpoint())
    expect(result.current.breakpoint).toBe('md')
    expect(result.current.isTablet).toBe(true)
    expect(result.current.isDesktop).toBe(false)
  })
})
