import styles from './Hints.module.css'

interface Props {
  highlighted: boolean
  children: React.ReactNode
}

export function HighlightBorder({ highlighted, children }: Props) {
  return (
    <div className={highlighted ? styles.highlightBorder : undefined}>
      {children}
    </div>
  )
}
