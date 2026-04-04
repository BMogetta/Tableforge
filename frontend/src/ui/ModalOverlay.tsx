import { useEffect } from 'react'
import styles from './ModalOverlay.module.css'

interface Props {
  onClose: () => void
  children: React.ReactNode
  className?: string
}

/**
 * Accessible modal backdrop. Renders:
 * - A visually-hidden button covering the backdrop to handle click-to-dismiss
 * - Escape key listener for keyboard dismiss
 *
 * Usage:
 *   <ModalOverlay onClose={handleClose}>
 *     <div className={styles.panel} role="dialog" aria-modal="true">
 *       ...
 *     </div>
 *   </ModalOverlay>
 */
export function ModalOverlay({ onClose, children, className }: Props) {
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  return (
    <div className={`${styles.overlay} ${className ?? ''}`}>
      <button
        type='button'
        className={styles.backdrop}
        onClick={onClose}
        aria-label='Close dialog'
        tabIndex={-1}
      />
      {children}
    </div>
  )
}
