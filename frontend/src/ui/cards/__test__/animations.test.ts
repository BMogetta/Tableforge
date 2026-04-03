import { describe, it, expect } from 'vitest'
import { getFlyOutTarget, slideInVariants } from '../animations'

function mockRect(
  left: number,
  top: number,
  width: number,
  height: number,
): DOMRect {
  return { left, top, width, height, right: left + width, bottom: top + height, x: left, y: top, toJSON: () => ({}) }
}

describe('getFlyOutTarget', () => {
  it('returns delta between source and target centers', () => {
    const source = { getBoundingClientRect: () => mockRect(100, 200, 120, 168) } as HTMLElement
    const target = { getBoundingClientRect: () => mockRect(400, 50, 120, 168) } as HTMLElement

    const result = getFlyOutTarget(source, target)

    expect(result.x).toBe(300) // (400+60) - (100+60)
    expect(result.y).toBe(-150) // (50+84) - (200+84)
  })

  it('flies off-screen when target is null', () => {
    const source = { getBoundingClientRect: () => mockRect(100, 200, 120, 168) } as HTMLElement

    const result = getFlyOutTarget(source, null)

    expect(result.x).toBe(0)
    expect(result.y).toBe(-window.innerHeight)
  })
})

describe('slideInVariants', () => {
  it('has initial, animate, and exit states', () => {
    expect(slideInVariants).toHaveProperty('initial')
    expect(slideInVariants).toHaveProperty('animate')
    expect(slideInVariants).toHaveProperty('exit')
  })

  it('starts with opacity 0 and ends with opacity 1', () => {
    expect((slideInVariants.initial as Record<string, number>).opacity).toBe(0)
    expect((slideInVariants.animate as Record<string, number>).opacity).toBe(1)
  })
})
