import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppError } from '@/utils/errors'
import { ErrorMessage } from '../ErrorMessage'

const devError: AppError = {
  reason: 'NOT_FOUND',
  status: 404,
  message: 'record not found in pg',
}

const prodError: AppError = {
  reason: 'NOT_FOUND',
  status: 404,
  message: 'The requested resource was not found.',
  code: 'ERR_A3F2',
}

describe('ErrorMessage', () => {
  it('renders nothing when error is null', () => {
    const { container } = render(<ErrorMessage error={null} />)
    expect(container.firstChild).toBeNull()
  })

  describe('dev mode', () => {
    it('shows reason badge', () => {
      render(<ErrorMessage error={devError} />)
      expect(screen.getByText('NOT_FOUND')).toBeInTheDocument()
    })

    it('shows status code', () => {
      render(<ErrorMessage error={devError} />)
      expect(screen.getByText('404')).toBeInTheDocument()
    })

    it('shows raw server message', () => {
      render(<ErrorMessage error={devError} />)
      expect(screen.getByText('record not found in pg')).toBeInTheDocument()
    })

    it('does not show error code', () => {
      render(<ErrorMessage error={devError} />)
      expect(screen.queryByText(/^ERR_/)).toBeNull()
    })
  })

  describe('prod mode', () => {
    beforeEach(() => {
      vi.stubEnv('DEV', false)
    })

    afterEach(() => {
      vi.unstubAllEnvs()
    })

    it('shows friendly message', () => {
      render(<ErrorMessage error={prodError} />)
      expect(screen.getByText('The requested resource was not found.')).toBeInTheDocument()
    })

    it('shows error code', () => {
      render(<ErrorMessage error={prodError} />)
      expect(screen.getByText('ERR_A3F2')).toBeInTheDocument()
    })

    it('does not show reason badge', () => {
      render(<ErrorMessage error={prodError} />)
      expect(screen.queryByText('NOT_FOUND')).toBeNull()
    })

    it('does not show raw pg message', () => {
      render(
        <ErrorMessage error={{ ...prodError, message: 'The requested resource was not found.' }} />,
      )
      expect(screen.queryByText(/pg/)).toBeNull()
    })
  })

  it('accepts optional className', () => {
    const { container } = render(<ErrorMessage error={devError} className='custom' />)
    expect(container.firstChild).toHaveClass('custom')
  })
})
