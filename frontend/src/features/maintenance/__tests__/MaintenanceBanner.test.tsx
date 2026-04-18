import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MaintenanceBanner } from '../MaintenanceBanner'

// useFlag is controlled via this ref so each test can change the flag value.
const flagValue = { current: false }

vi.mock('@unleash/proxy-client-react', () => ({
  useFlag: (_name: string) => flagValue.current,
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}))

describe('MaintenanceBanner', () => {
  beforeEach(() => {
    flagValue.current = false
  })

  it('renders nothing when maintenance-mode flag is OFF', () => {
    flagValue.current = false
    const { container } = render(<MaintenanceBanner />)
    expect(container.firstChild).toBeNull()
  })

  it('renders the banner with translated title + message when flag is ON', () => {
    flagValue.current = true
    render(<MaintenanceBanner />)
    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.getByText('maintenance.title')).toBeInTheDocument()
    expect(screen.getByText('maintenance.message')).toBeInTheDocument()
  })

  it('sets aria-live=polite so screen readers announce without interrupting', () => {
    flagValue.current = true
    render(<MaintenanceBanner />)
    expect(screen.getByRole('status')).toHaveAttribute('aria-live', 'polite')
  })
})
