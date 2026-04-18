import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { setMaintenanceHandler } from '@/lib/api'
import type { AppError } from '@/utils/errors'
import styles from './Toast.module.css'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type ToastVariant = 'error' | 'warning' | 'info'

interface ToastItem {
  id: string
  variant: ToastVariant
  message: string
  code?: string
}

interface ToastContextValue {
  showError: (err: AppError) => void
  showWarning: (message: string) => void
  showInfo: (message: string) => void
}

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

const ToastContext = createContext<ToastContextValue | null>(null)

const MAX_TOASTS = 3
const AUTO_DISMISS_MS = 4000

let idCounter = 0
function nextId() {
  return `toast-${++idCounter}`
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])

  const add = useCallback((item: Omit<ToastItem, 'id'>) => {
    setToasts(prev => {
      const next = [...prev, { ...item, id: nextId() }]
      // Keep only the last MAX_TOASTS items
      return next.slice(-MAX_TOASTS)
    })
  }, [])

  const remove = useCallback((id: string) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const showError = useCallback(
    (err: AppError) => {
      add({ variant: 'error', message: err.message, code: err.code })
    },
    [add],
  )

  const showWarning = useCallback(
    (message: string) => {
      add({ variant: 'warning', message })
    },
    [add],
  )

  const showInfo = useCallback(
    (message: string) => {
      add({ variant: 'info', message })
    },
    [add],
  )

  // Wire api.ts → toast for maintenance-mode notices. The api layer can't
  // own the toast context (it's not a component), so it exposes a setter
  // and we register the handler once per mount here.
  const { t } = useTranslation()
  useEffect(() => {
    setMaintenanceHandler(() => {
      add({ variant: 'warning', message: t('maintenance.actionBlocked') })
    })
    return () => setMaintenanceHandler(null)
  }, [add, t])

  return (
    <ToastContext.Provider value={{ showError, showWarning, showInfo }}>
      {children}
      <ToastContainer toasts={toasts} onDismiss={remove} />
    </ToastContext.Provider>
  )
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used inside ToastProvider')
  return ctx
}

// ---------------------------------------------------------------------------
// Container
// ---------------------------------------------------------------------------

function ToastContainer({
  toasts,
  onDismiss,
}: {
  toasts: ToastItem[]
  onDismiss: (id: string) => void
}) {
  if (toasts.length === 0) return null
  return (
    <div className={styles.container} aria-live='polite' aria-atomic='false'>
      {toasts.map(t => (
        <ToastCard key={t.id} toast={t} onDismiss={onDismiss} />
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

function ToastCard({ toast, onDismiss }: { toast: ToastItem; onDismiss: (id: string) => void }) {
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const dismiss = useCallback(() => {
    if (timerRef.current) clearTimeout(timerRef.current)
    onDismiss(toast.id)
  }, [toast.id, onDismiss])

  useEffect(() => {
    timerRef.current = setTimeout(dismiss, AUTO_DISMISS_MS)
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [dismiss])

  const icon = {
    error: '✕',
    warning: '⚠',
    info: 'ℹ',
  }[toast.variant]

  return (
    <div className={`${styles.toast} ${styles[toast.variant]}`} role='alert'>
      <span className={styles.icon}>{icon}</span>
      <div className={styles.body}>
        <span className={styles.message}>{toast.message}</span>
        {toast.code && <span className={styles.code}>{toast.code}</span>}
      </div>
      <button
        type='button'
        className={styles.dismissBtn}
        onClick={dismiss}
        aria-label='Dismiss notification'
      >
        ×
      </button>
      <div className={styles.progress}>
        <div className={styles.progressBar} style={{ animationDuration: `${AUTO_DISMISS_MS}ms` }} />
      </div>
    </div>
  )
}
