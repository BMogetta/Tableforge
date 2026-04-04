import { useTranslation } from 'react-i18next'
import styles from './Rules.module.css'

/** @package */
export function TicTacToeRules() {
  const { t } = useTranslation()

  return (
    <div className={styles.root}>
      <section className={styles.section}>
        <h3 className={styles.heading}>{t('ttt.objective')}</h3>
        <p className={styles.text}>{t('ttt.objectiveDesc')}</p>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('ttt.turnFlow')}</h3>
        <ol className={styles.list}>
          <li>{t('ttt.turnFlowAlt')}</li>
          <li>{t('ttt.turnFlowClick')}</li>
          <li>{t('ttt.turnFlowPermanent')}</li>
        </ol>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>{t('ttt.winConditions')}</h3>
        <ul className={styles.list}>
          <li>{t('ttt.winThree')}</li>
          <li>{t('ttt.winDraw')}</li>
        </ul>
      </section>
    </div>
  )
}
