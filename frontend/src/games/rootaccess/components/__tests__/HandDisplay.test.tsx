import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { CardName } from '../CardDisplay'
import { HandDisplay } from '../HandDisplay'

describe('HandDisplay', () => {
  it('renders all cards in hand', () => {
    render(
      <HandDisplay
        cards={['ping', 'sniffer'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={vi.fn()}
      />,
    )
    expect(screen.getByText('PING')).toBeInTheDocument()
    expect(screen.getByText('SNIFFER')).toBeInTheDocument()
  })

  it('shows empty message when hand is empty', () => {
    render(<HandDisplay cards={[]} selectedCard={null} disabled={false} onSelect={vi.fn()} />)
    expect(screen.getByText('No cards in hand')).toBeInTheDocument()
  })

  it('calls onSelect with the clicked card', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['ping', 'backdoor'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={onSelect}
      />,
    )
    const cards = screen.getAllByTestId('card')
    await user.click(cards[0])
    expect(onSelect).toHaveBeenCalledWith('ping')
  })

  it('does not call onSelect when disabled', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['ping', 'backdoor'] as CardName[]}
        selectedCard={null}
        disabled={true}
        onSelect={onSelect}
      />,
    )
    // Cards are still rendered but disabled — clicking should not fire
    const cards = screen.getAllByTestId('card')
    await user.click(cards[0])
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('shows ENCRYPTED_KEY must play label for blocked cards', () => {
    render(
      <HandDisplay
        cards={['swap', 'encrypted_key'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={vi.fn()}
        blockedCards={['swap']}
      />,
    )
    expect(screen.getByText('Must play ENCRYPTED_KEY')).toBeInTheDocument()
  })

  it('does not call onSelect for blocked cards', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['swap', 'encrypted_key'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={onSelect}
        blockedCards={['swap']}
      />,
    )
    // Swap is blocked (disabled), encrypted_key is playable
    const cards = screen.getAllByTestId('card')
    // Click the blocked king — should not fire
    await user.click(cards[0])
    expect(onSelect).not.toHaveBeenCalled()

    // Click countess — should fire
    await user.click(cards[1])
    expect(onSelect).toHaveBeenCalledWith('encrypted_key')
  })

  describe('fly-out playingCard recovery', () => {
    it('fires onPlayComplete even if motion.animate rejects', async () => {
      vi.doMock('motion', () => ({
        animate: vi.fn().mockRejectedValue(new Error('boom')),
      }))
      const onPlayComplete = vi.fn()
      render(
        <HandDisplay
          cards={['buffer_overflow'] as CardName[]}
          selectedCard={null}
          disabled={false}
          onSelect={vi.fn()}
          playingCard={'buffer_overflow' as CardName}
          onPlayComplete={onPlayComplete}
        />,
      )
      // Allow the microtask queue to drain so the try/finally runs.
      await vi.waitFor(() => expect(onPlayComplete).toHaveBeenCalledTimes(1))
      vi.doUnmock('motion')
    })

    it('safety timeout unblocks the parent if motion.animate never resolves', async () => {
      vi.useFakeTimers()
      vi.doMock('motion', () => ({
        animate: vi.fn().mockReturnValue(new Promise(() => {})),
      }))
      const onPlayComplete = vi.fn()
      render(
        <HandDisplay
          cards={['buffer_overflow'] as CardName[]}
          selectedCard={null}
          disabled={false}
          onSelect={vi.fn()}
          playingCard={'buffer_overflow' as CardName}
          onPlayComplete={onPlayComplete}
        />,
      )
      // Give the dynamic import a chance to resolve before advancing timers.
      await act(async () => {
        await Promise.resolve()
        await Promise.resolve()
      })
      await act(async () => {
        vi.advanceTimersByTime(1600)
      })
      expect(onPlayComplete).toHaveBeenCalledTimes(1)
      vi.useRealTimers()
      vi.doUnmock('motion')
    })

    it('clears inline transform/opacity when playingCard effect is cancelled', async () => {
      let resolveAnimate: (() => void) | null = null
      vi.doMock('motion', () => ({
        animate: vi.fn((el: HTMLElement) => {
          el.style.transform = 'translate(100px, 100px)'
          el.style.opacity = '0'
          return new Promise<void>(resolve => {
            resolveAnimate = resolve
          })
        }),
      }))
      const { rerender } = render(
        <HandDisplay
          cards={['buffer_overflow'] as CardName[]}
          selectedCard={null}
          disabled={false}
          onSelect={vi.fn()}
          playingCard={'buffer_overflow' as CardName}
          onPlayComplete={vi.fn()}
        />,
      )
      // Let the effect's async body run and apply the inline styles.
      await act(async () => {
        await Promise.resolve()
        await Promise.resolve()
      })
      // Cancel mid-flight by clearing playingCard — cleanup runs.
      rerender(
        <HandDisplay
          cards={['buffer_overflow'] as CardName[]}
          selectedCard={null}
          disabled={false}
          onSelect={vi.fn()}
          playingCard={null}
          onPlayComplete={vi.fn()}
        />,
      )
      const el = screen.getByTestId('card') as HTMLElement
      expect(el.style.transform).toBe('')
      expect(el.style.opacity).toBe('')
      resolveAnimate?.()
      vi.doUnmock('motion')
    })
  })
})
