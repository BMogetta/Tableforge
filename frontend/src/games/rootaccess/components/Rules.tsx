import { useTranslation } from 'react-i18next'
import { CARD_META, type CardName } from './CardDisplay'
import styles from './Rules.module.css'

interface Props {
  /**
   * Cards currently in the player's hand — highlighted in the reference table.
   * Accepts string[] (the generic contract with the rules modal) and narrows
   * to CardName internally at lookup time so consumers don't need to cast.
   */
  handCards?: string[]
}

const CARD_ORDER: CardName[] = [
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

const CARD_COUNTS: Record<CardName, number> = {
  backdoor: 2,
  ping: 6,
  sniffer: 2,
  buffer_overflow: 2,
  firewall: 2,
  reboot: 2,
  debugger: 2,
  swap: 1,
  encrypted_key: 1,
  root: 1,
}

/** @package */
export function RootAccessRules({ handCards = [] }: Props) {
  const { t } = useTranslation()

  return (
    <div className={styles.root}>
      <section className={styles.section}>
        <h3 className={styles.heading}>{t('rootaccess.objective')}</h3>
        <p className={styles.text}>{t('rootaccess.objectiveDesc')}</p>
        <table className={styles.tokensTable}>
          <thead>
            <tr>
              <th>Players</th>
              <th>Tokens to win</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>2</td>
              <td>7</td>
            </tr>
            <tr>
              <td>3</td>
              <td>5</td>
            </tr>
            <tr>
              <td>4</td>
              <td>4</td>
            </tr>
            <tr>
              <td>5</td>
              <td>3</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('rootaccess.turnFlow')}</h3>
        <ol className={styles.list}>
          <li>{t('rootaccess.turnFlowDraw')}</li>
          <li>{t('rootaccess.turnFlowPlay')}</li>
          <li>{t('rootaccess.turnFlowDebugger')}</li>
        </ol>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('rootaccess.winConditions')}</h3>
        <ul className={styles.list}>
          <li>{t('rootaccess.winLastStanding')}</li>
          <li>{t('rootaccess.winHighestCard')}</li>
          <li>{t('rootaccess.winBackdoorBonus')}</li>
        </ul>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('rootaccess.cardReference')}</h3>
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
        <h3 className={styles.heading}>{t('rootaccess.specialRules')}</h3>
        <ul className={styles.list}>
          <li>{t('rootaccess.ruleEncryptedKey')}</li>
          <li>{t('rootaccess.ruleFirewall')}</li>
          <li>{t('rootaccess.ruleRoot')}</li>
        </ul>
      </section>
    </div>
  )
}
