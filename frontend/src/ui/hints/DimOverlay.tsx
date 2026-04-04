import styles from './Hints.module.css'

interface Props {
  dimmed: boolean
  children: React.ReactNode
}

export function DimOverlay({ dimmed, children }: Props) {
  return <div className={dimmed ? styles.dimOverlay : undefined}>{children}</div>
}
