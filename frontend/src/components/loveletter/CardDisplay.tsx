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
  /** Card is selected/active in hand. */
  selected?: boolean
  /** Card is not playable (disabled interaction). */
  disabled?: boolean
  /** Show card back instead of face (opponent's card). */
  faceDown?: boolean
  /** Click handler — only fires when not disabled and not faceDown. */
  onClick?: () => void
  /** Extra CSS class for layout overrides. */
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
  const meta = CARD_META[card]
  const interactive = !disabled && !faceDown && !!onClick

  return (
    <div
      data-testid='card-display'
      data-selected={selected}
      data-disabled={disabled}
      data-facedown={faceDown}
      className={[
        styles.card,
        selected ? styles.selected : '',
        disabled ? styles.disabled : '',
        faceDown ? styles.faceDown : '',
        interactive ? styles.interactive : '',
        className,
      ]
        .filter(Boolean)
        .join(' ')}
      onClick={interactive ? onClick : undefined}
      role={interactive ? 'button' : undefined}
      tabIndex={interactive ? 0 : undefined}
      onKeyDown={interactive ? e => e.key === 'Enter' && onClick?.() : undefined}
      aria-pressed={selected}
      aria-disabled={disabled}
    >
      {faceDown ? (
        <div className={styles.back}>
          <span className={styles.backPattern}>✦</span>
        </div>
      ) : (
        <>
          <div className={styles.header}>
            <span className={styles.value}>{meta.value}</span>
            <span className={styles.name}>{meta.label}</span>
          </div>
          <div className={styles.effect}>{meta.effect}</div>
          <div className={styles.footer}>
            <span className={styles.valueLarge}>{meta.value}</span>
          </div>
        </>
      )}
    </div>
  )
}

export { CARD_META }
