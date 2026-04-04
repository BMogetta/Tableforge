import styles from './Rules.module.css'

/** @package */
export function TicTacToeRules() {
  return (
    <div className={styles.root}>
      <section className={styles.section}>
        <h3 className={styles.heading}>Objective</h3>
        <p className={styles.text}>
          Get three of your marks (X or O) in a row — horizontally, vertically, or diagonally.
        </p>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>Turn Flow</h3>
        <ol className={styles.list}>
          <li>Players alternate turns starting with X.</li>
          <li>On your turn, click an empty cell to place your mark.</li>
          <li>Once placed, marks cannot be moved or removed.</li>
        </ol>
      </section>

      <section className={styles.section}>
        <h3 className={styles.heading}>Win Conditions</h3>
        <ul className={styles.list}>
          <li>
            <strong>Three in a row:</strong> first player to align three marks wins.
          </li>
          <li>
            <strong>Draw:</strong> if all 9 cells are filled with no three in a row, the game is a
            draw.
          </li>
        </ul>
      </section>
    </div>
  )
}
