import { useClipboard } from '@/hooks/useClipboard'
import { testId } from '@/utils/testId'
import styles from './VersionBadge.module.css'

const VERSION = import.meta.env.VITE_APP_VERSION

export function VersionBadge() {
  const { copy, copied } = useClipboard(1200)

  return (
    <button
      type='button'
      className={`${styles.badge} ${copied ? styles.copied : ''}`}
      onClick={() => copy(VERSION)}
      aria-label={`Build version ${VERSION}. Click to copy.`}
      title={copied ? 'Copied!' : 'Click to copy version'}
      {...testId('version-badge')}
    >
      {copied ? 'copied' : VERSION}
    </button>
  )
}
