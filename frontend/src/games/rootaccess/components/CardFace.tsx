import type { CardName } from './CardDisplay'
import { CARD_META } from './CardDisplay'
import styles from './CardDisplay.module.css'

interface Props {
  card: CardName
}

/** @package */
export function CardFace({ card }: Props) {
  const meta = CARD_META[card]

  return (
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
  )
}
