import { CARD_META, type CardName } from './CardDisplay'
import styles from './Rules.module.css'

interface Props {
  /** Cards currently in the player's hand — highlighted in the reference table. */
  handCards?: CardName[]
}

const CARD_ORDER: CardName[] = [
  'backdoor', 'ping', 'sniffer', 'buffer_overflow', 'firewall',
  'reboot', 'debugger', 'swap', 'encrypted_key', 'root',
]

const CARD_COUNTS: Record<CardName, number> = {
  backdoor: 2, ping: 6, sniffer: 2, buffer_overflow: 2, firewall: 2,
  reboot: 2, debugger: 2, swap: 1, encrypted_key: 1, root: 1,
}

/** @package */
export function RootAccessRules({ handCards = [] }: Props) {
  return (
    <div className={styles.root}>
      <section className={styles.section}>
        <h3 className={styles.heading}>Objective</h3>
        <p className={styles.text}>
          Collect enough <strong>Access Tokens</strong> to win. Each round, try to hold
          the highest-value card or be the last player with an active connection.
        </p>
        <table className={styles.tokensTable}>
          <thead>
            <tr><th>Players</th><th>Tokens to win</th></tr>
          </thead>
          <tbody>
            <tr><td>2</td><td>7</td></tr>
            <tr><td>3</td><td>5</td></tr>
            <tr><td>4</td><td>4</td></tr>
            <tr><td>5</td><td>3</td></tr>
          </tbody>
        </table>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>Turn Flow</h3>
        <ol className={styles.list}>
          <li>Draw the top card from <strong>The Repository</strong> (you now hold 2 cards).</li>
          <li>Play one card face-up and resolve its effect.</li>
          <li>If you played <strong>DEBUGGER</strong>: pick 1 of 3 cards to keep, return 2 to the bottom.</li>
        </ol>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>Win Conditions</h3>
        <ul className={styles.list}>
          <li><strong>Last standing:</strong> all other players have their Connection Dropped.</li>
          <li><strong>Highest card:</strong> when The Repository is empty, highest value wins.</li>
          <li><strong>BACKDOOR bonus:</strong> if only one player executed BACKDOOR this round, +1 token.</li>
        </ul>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>Card Reference</h3>
        <table className={styles.cardTable}>
          <thead>
            <tr>
              <th>Val</th>
              <th>Name</th>
              <th>Qty</th>
              <th>Effect</th>
            </tr>
          </thead>
          <tbody>
            {CARD_ORDER.map(card => {
              const meta = CARD_META[card]
              const inHand = handCards.includes(card)
              return (
                <tr key={card} className={inHand ? styles.inHand : undefined}>
                  <td className={styles.cardValue}>{meta.value}</td>
                  <td className={styles.cardName}>{meta.label}</td>
                  <td className={styles.cardCount}>{CARD_COUNTS[card]}</td>
                  <td className={styles.cardEffect}>{meta.effect}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>Special Rules</h3>
        <ul className={styles.list}>
          <li><strong>ENCRYPTED_KEY:</strong> must be played if you also hold REBOOT or SWAP.</li>
          <li><strong>FIREWALL:</strong> protects you from all effects until your next turn.</li>
          <li><strong>ROOT:</strong> if discarded for any reason, you lose your connection.</li>
        </ul>
      </section>
    </div>
  )
}
