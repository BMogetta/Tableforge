import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { SystemBanner } from '../SystemBanner'

// Flag + flagsReady state controlled per-test.
const flagState = {
  flagsReady: true,
  maintenance: false,
  rankedEnabled: true,
}

vi.mock('@unleash/proxy-client-react', () => ({
  useFlagsStatus: () => ({ flagsReady: flagState.flagsReady, flagsError: null }),
  useFlag: (name: string) => {
    if (name === 'maintenance-mode') return flagState.maintenance
    if (name === 'ranked-matchmaking-enabled') return flagState.rankedEnabled
    return false
  },
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

describe('SystemBanner', () => {
  beforeEach(() => {
    flagState.flagsReady = true
    flagState.maintenance = false
    flagState.rankedEnabled = true
  })

  it('renders nothing when no app-wide flags are active', () => {
    const { container } = render(<SystemBanner />)
    expect(container.firstChild).toBeNull()
  })

  it('renders the maintenance banner when maintenance-mode is ON', () => {
    flagState.maintenance = true
    render(<SystemBanner />)
    const banner = screen.getByRole('status')
    expect(banner).toHaveAttribute('data-variant', 'maintenance')
    expect(screen.getByText('maintenance.title')).toBeInTheDocument()
  })

  it('renders the ranked-paused banner when ranked-matchmaking-enabled is OFF', () => {
    flagState.rankedEnabled = false
    render(<SystemBanner />)
    const banner = screen.getByRole('status')
    expect(banner).toHaveAttribute('data-variant', 'ranked-paused')
    expect(screen.getByText('systemStatus.rankedPausedTitle')).toBeInTheDocument()
  })

  it('prioritizes maintenance over ranked when both are active', () => {
    flagState.maintenance = true
    flagState.rankedEnabled = false
    render(<SystemBanner />)
    const banner = screen.getByRole('status')
    expect(banner).toHaveAttribute('data-variant', 'maintenance')
    // Ranked copy must NOT appear — only one banner is ever rendered.
    expect(screen.queryByText('systemStatus.rankedPausedTitle')).not.toBeInTheDocument()
  })

  it('holds the ranked banner until flagsReady to avoid cold-start flash', () => {
    flagState.flagsReady = false
    flagState.rankedEnabled = false // SDK pre-init default
    const { container } = render(<SystemBanner />)
    expect(container.firstChild).toBeNull()
  })

  it('renders maintenance even before flagsReady (kill switches default OFF)', () => {
    // Maintenance defaults OFF; if the SDK reads true pre-ready, it's
    // because the server said so. Surface immediately — no flash risk.
    flagState.flagsReady = false
    flagState.maintenance = true
    render(<SystemBanner />)
    expect(screen.getByRole('status')).toHaveAttribute('data-variant', 'maintenance')
  })

  it('sets aria-live=polite so screen readers announce softly', () => {
    flagState.maintenance = true
    render(<SystemBanner />)
    expect(screen.getByRole('status')).toHaveAttribute('aria-live', 'polite')
  })
})
