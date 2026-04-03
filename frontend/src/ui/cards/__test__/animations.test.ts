import { describe, it, expect } from 'vitest'
import { getFlyOutTarget } from '../animations'

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
