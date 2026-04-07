import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock the store before importing authGuards.
const mockGetState = vi.fn()
vi.mock('../stores/store', () => ({
  useAppStore: { getState: () => mockGetState() },
}))

// Mock TanStack Router redirect.
const mockRedirect = vi.fn()
vi.mock('@tanstack/react-router', () => ({
  redirect: (opts: unknown) => {
    mockRedirect(opts)
    throw new Error('REDIRECT')
  },
}))

describe('requireAuth', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does not throw when player exists', async () => {
    mockGetState.mockReturnValue({ player: { id: '1', role: 'player' } })
    const { requireAuth } = await import('./authGuards')
    expect(() => requireAuth()).not.toThrow()
  })

  it('redirects to /login when no player', async () => {
    mockGetState.mockReturnValue({ player: null })
    const { requireAuth } = await import('./authGuards')
    expect(() => requireAuth()).toThrow('REDIRECT')
    expect(mockRedirect).toHaveBeenCalledWith({ to: '/login' })
  })
})

describe('requireRole', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does not throw when role is sufficient', async () => {
    mockGetState.mockReturnValue({ player: { id: '1', role: 'owner' } })
    const { requireRole } = await import('./authGuards')
    expect(() => requireRole('manager')).not.toThrow()
  })

  it('redirects when role is insufficient', async () => {
    mockGetState.mockReturnValue({ player: { id: '1', role: 'player' } })
    const { requireRole } = await import('./authGuards')
    expect(() => requireRole('manager')).toThrow('REDIRECT')
    expect(mockRedirect).toHaveBeenCalledWith({ to: '/' })
  })

  it('redirects when no player', async () => {
    mockGetState.mockReturnValue({ player: null })
    const { requireRole } = await import('./authGuards')
    expect(() => requireRole('manager')).toThrow('REDIRECT')
  })
})
