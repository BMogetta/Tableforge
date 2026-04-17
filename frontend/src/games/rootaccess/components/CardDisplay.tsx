import { Card } from '@/ui/cards'
import { testId } from '@/utils/testId'
import styles from './CardDisplay.module.css'
import { CardFace } from './CardFace'

/** @package */
export type CardName =
  | 'backdoor'
  | 'ping'
  | 'sniffer'
  | 'buffer_overflow'
  | 'firewall'
  | 'reboot'
  | 'debugger'
  | 'swap'
  | 'encrypted_key'
  | 'root'

interface CardMeta {
  label: string
  value: number
  effect: string
}

/** @package */
const CARD_META: Record<CardName, CardMeta> = {
  backdoor: {
    label: 'BACKDOOR',
    value: 0,
    effect: '+1 Access Token if you were the only one to execute it this round.',
  },
  ping: {
    label: 'PING',
    value: 1,
    effect: "Scan another user's process; if you guess right, terminate them.",
  },
  sniffer: {
    label: 'SNIFFER',
    value: 2,
    effect: "Inspect another process's memory (hand).",
  },
  buffer_overflow: {
    label: 'BUFFER_OVERFLOW',
    value: 3,
    effect: 'Compare privileges; the lower one is terminated.',
  },
  firewall: {
    label: 'FIREWALL',
    value: 4,
    effect: 'Block external requests for 1 turn.',
  },
  reboot: {
    label: 'REBOOT',
    value: 5,
    effect: 'Force a process to discard and load a new one from the Repository.',
  },
  debugger: {
    label: 'DEBUGGER',
    value: 6,
    effect: 'Pull 2 from the Repository, keep 1, return the rest to the bottom.',
  },
  swap: {
    label: 'SWAP',
    value: 7,
    effect: "Exchange your process with another user's.",
  },
  encrypted_key: {
    label: 'ENCRYPTED_KEY',
    value: 8,
    effect: 'Must discard if you detect a REBOOT or SWAP nearby.',
  },
  root: {
    label: 'ROOT',
    value: 9,
    effect: 'If this process stops or is discarded, you lose connection.',
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

/** @package */
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

/** @package */
export { CARD_META }
