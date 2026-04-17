import { type ReactNode, useEffect, useId, useRef, useState } from 'react'
import { testId as makeTestId } from '@/utils/testId'
import styles from './Dropdown.module.css'

type Align = 'start' | 'end'

interface Props {
  trigger: ReactNode
  children: ReactNode
  align?: Align
  className?: string
  triggerClassName?: string
  panelClassName?: string
  /** Omit the caret — useful when the trigger is a bare avatar or icon. */
  hideCaret?: boolean
  testId?: string
  ariaLabel?: string
}

export function Dropdown({
  trigger,
  children,
  align = 'start',
  className,
  triggerClassName,
  panelClassName,
  hideCaret = false,
  testId,
  ariaLabel,
}: Props) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)
  const panelId = useId()

  useEffect(() => {
    if (!open) return
    function onDown(e: MouseEvent) {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false)
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const triggerProps = testId ? makeTestId(`${testId}-trigger`) : {}
  const panelProps = testId ? makeTestId(`${testId}-panel`) : {}

  return (
    <div ref={rootRef} className={`${styles.root} ${className ?? ''}`}>
      <button
        type='button'
        className={`${styles.trigger} ${triggerClassName ?? ''}`}
        aria-haspopup='true'
        aria-expanded={open}
        aria-controls={panelId}
        aria-label={ariaLabel}
        onClick={() => setOpen(v => !v)}
        {...triggerProps}
      >
        {trigger}
        {!hideCaret && (
          <span aria-hidden='true' className={styles.caret}>
            ▾
          </span>
        )}
      </button>
      {open && (
        <div
          id={panelId}
          role='menu'
          className={`${styles.panel} ${align === 'end' ? styles.alignEnd : styles.alignStart} ${panelClassName ?? ''}`}
          {...panelProps}
        >
          {children}
        </div>
      )}
    </div>
  )
}
