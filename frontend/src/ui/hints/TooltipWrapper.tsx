import { useState } from 'react'
import styles from './Hints.module.css'

interface Props {
  text: string
  children: React.ReactNode
}

export function TooltipWrapper({ text, children }: Props) {
  const [visible, setVisible] = useState(false)

  return (
    <div
      className={styles.tooltipWrapper}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
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
