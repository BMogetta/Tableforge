import { useState } from 'react'
import styles from './Hints.module.css'

interface Props {
  text: string
  children: React.ReactNode
}

export function TooltipWrapper({ text, children }: Props) {
  const [visible, setVisible] = useState(false)

  return (
    // biome-ignore lint/a11y/noStaticElementInteractions: wrapper relays hover/focus to the inner child; the interactive element is the child itself
    <div
      className={styles.tooltipWrapper}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
      onFocus={() => setVisible(true)}
      onBlur={() => setVisible(false)}
    >
      {children}
      {visible && (
        <div className={styles.tooltipContent} role='tooltip'>
          {text}
        </div>
      )}
    </div>
  )
}
