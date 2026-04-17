import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { CARD_META, type CardName } from '../CardDisplay'
import { CardFace } from '../CardFace'

const ALL_CARDS: CardName[] = [
  'backdoor',
  'ping',
  'sniffer',
  'buffer_overflow',
  'firewall',
  'reboot',
  'debugger',
  'swap',
  'encrypted_key',
  'root',
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

  it.each(ALL_CARDS)('renders %s without crashing', card => {
    const { container } = render(<CardFace card={card} />)
    expect(container.textContent).toContain(CARD_META[card].label)
    expect(container.textContent).toContain(String(CARD_META[card].value))
  })
})
