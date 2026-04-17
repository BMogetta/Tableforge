import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { testId } from '@/utils/testId'
import styles from '../Game.module.css'

interface TurnTimerProps {
  turnTimeoutSecs?: number
  lastMoveAt: string
  isOver: boolean
  isSuspended: boolean
}

export function TurnTimer({ turnTimeoutSecs, lastMoveAt, isOver, isSuspended }: TurnTimerProps) {
  const { t } = useTranslation()
  const [remaining, setRemaining] = useState(() => calcRemaining(turnTimeoutSecs ?? 0, lastMoveAt))

  useEffect(() => {
    if (isOver || !turnTimeoutSecs || turnTimeoutSecs <= 0) {
      return
    }

    // While suspended, freeze — don't tick.
    if (isSuspended) {
      return
    }

    // Recalculate from lastMoveAt (backend is source of truth).
    setRemaining(calcRemaining(turnTimeoutSecs, lastMoveAt))

    const interval = setInterval(() => {
      setRemaining(calcRemaining(turnTimeoutSecs, lastMoveAt))
    }, 1000)

    return () => clearInterval(interval)
  }, [turnTimeoutSecs, lastMoveAt, isOver, isSuspended])

  if (isOver || !turnTimeoutSecs || turnTimeoutSecs <= 0) {
    return null
  }

  const secs = Math.max(0, remaining)
  const paused = isSuspended
  const urgent = !paused && secs <= 10

  return (
    <span
      className={`${styles.turnTimer} ${urgent ? styles.turnTimerUrgent : ''} ${paused ? styles.turnTimerPaused : ''}`}
      {...testId('turn-timer')}
    >
      {paused ? t('game.timerPaused', { seconds: secs }) : `${secs}s`}
    </span>
  )
}

function calcRemaining(timeoutSecs: number, lastMoveAt: string): number {
  const deadline = new Date(lastMoveAt).getTime() + timeoutSecs * 1000
  return Math.ceil((deadline - Date.now()) / 1000)
}
