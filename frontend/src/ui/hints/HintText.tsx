import styles from './Hints.module.css'

interface Props {
  text: string
}

export function HintText({ text }: Props) {
  return <span className={styles.hintText}>{text}</span>
}
