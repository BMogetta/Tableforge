import { testId } from '@/utils/testId'
import { Card } from '@/ui/cards'
import { CardFace } from './CardFace'
import styles from './CardDisplay.module.css'

export type CardName =
  | 'spy'
  | 'guard'
  | 'priest'
  | 'baron'
  | 'handmaid'
  | 'prince'
  | 'chancellor'
  | 'king'
  | 'countess'
  | 'princess'

interface CardMeta {
  label: string
  value: number
  effect: string
}

const CARD_META: Record<CardName, CardMeta> = {
  spy: { label: 'Spy', value: 0, effect: 'If only you played a Spy this round, gain 1 token.' },
  guard: {
    label: 'Guard',
    value: 1,
    effect: 'Name a card. If target holds it, they are eliminated.',
  },
  priest: { label: 'Priest', value: 2, effect: "Look at another player's hand." },
  baron: { label: 'Baron', value: 3, effect: 'Compare hands. Lower value is eliminated.' },
  handmaid: {
    label: 'Handmaid',
    value: 4,
    effect: 'Protected from all effects until your next turn.',
  },
  prince: {
    label: 'Prince',
    value: 5,
    effect: 'Choose a player to discard their hand and draw a new card.',
  },
  chancellor: {
    label: 'Chancellor',
    value: 6,
    effect: 'Draw 2 cards. Keep 1, return 2 to the bottom of the deck.',
  },
  king: { label: 'King', value: 7, effect: 'Trade hands with another player.' },
  countess: {
    label: 'Countess',
    value: 8,
    effect: 'Must be played if you hold the King or Prince.',
  },
  princess: {
    label: 'Princess',
    value: 9,
    effect: 'If you play or discard this card, you are eliminated.',
  },
}

interface Props {
  card: CardName
  selected?: boolean
  disabled?: boolean
  faceDown?: boolean
  onClick?: () => void
  className?: string
}

export function CardDisplay({
  card,
  selected = false,
  disabled = false,
  faceDown = false,
  onClick,
  className = '',
}: Props) {
  return (
    <div
      {...testId('card-display')}
      data-selected={selected}
      data-disabled={disabled}
      data-facedown={faceDown}
      className={[styles.wrapper, selected ? styles.selected : '', className]
        .filter(Boolean)
        .join(' ')}
    >
      <Card
        front={
          <div className={styles.face}>
            <CardFace card={card} />
          </div>
        }
        faceDown={faceDown}
        disabled={disabled}
        onClick={onClick}
      />
    </div>
  )
}

export { CARD_META }
