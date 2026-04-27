import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AdminDevtoolsGate } from '../AdminDevtoolsGate'

// useCapability return value is controlled via this ref so each test can
// flip canSeeDevtools without setting up a real QueryClient.
const capabilityState = { canSeeDevtools: false }

vi.mock('@/features/auth/useCapability', () => ({
  useCapability: () => ({
    capabilities: capabilityState,
    isLoading: false,
    isError: false,
  }),
}))

// AdminDevtoolsPanel is heavy (TanStack router devtools, etc). Stub it with
// a trivial marker so the test just verifies the Suspense resolves when the
// gate opens. The real panel is exercised in E2E.
vi.mock('../AdminDevtoolsPanel', () => ({
  AdminDevtoolsPanel: () => <div data-testid='admin-devtools-panel'>panel</div>,
}))

describe('AdminDevtoolsGate', () => {
  beforeEach(() => {
    capabilityState.canSeeDevtools = false
  })

  it('renders nothing when the user cannot see devtools', () => {
    capabilityState.canSeeDevtools = false
    const { container } = render(<AdminDevtoolsGate />)
    expect(container.firstChild).toBeNull()
  })

  it('lazy-loads the panel when canSeeDevtools becomes true', async () => {
    capabilityState.canSeeDevtools = true
    render(<AdminDevtoolsGate />)
    await waitFor(() => {
      expect(screen.getByTestId('admin-devtools-panel')).toBeInTheDocument()
    })
  })
})
