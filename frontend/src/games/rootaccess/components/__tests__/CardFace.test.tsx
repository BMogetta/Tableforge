import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { CardFace } from '../CardFace'
import { CARD_META, type CardName } from '../CardDisplay'

const ALL_CARDS: CardName[] = [
  'backdoor', 'ping', 'sniffer', 'buffer_overflow', 'firewall',
  'reboot', 'debugger', 'swap', 'encrypted_key', 'root',
]

describe('CardFace', () => {
  it('renders card name and effect', () => {
    render(<CardFace card='ping' />)
    expect(screen.getByText('PING')).toBeInTheDocument()
    expect(screen.getByText(CARD_META.ping.effect)).toBeInTheDocument()
  })

  it('renders value in header and footer', () => {
    const { container } = render(<CardFace card='sniffer' />)
    const values = container.querySelectorAll('[class*="value"]')
    const valueTexts = Array.from(values).map(el => el.textContent)
    expect(valueTexts).toContain('2')
  })

  it.each(ALL_CARDS)('renders %s without crashing', (card) => {
    const { container } = render(<CardFace card={card} />)
    expect(container.textContent).toContain(CARD_META[card].label)
    expect(container.textContent).toContain(String(CARD_META[card].value))
  })
})
