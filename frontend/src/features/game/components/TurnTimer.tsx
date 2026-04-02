import { useEffect, useState } from 'react'
import styles from '../Game.module.css'
import { testId } from '@/utils/testId'

interface TurnTimerProps {
  turnTimeoutSecs: number
  lastMoveAt: string
  isOver: boolean
  isSuspended: boolean
}

export function TurnTimer({ turnTimeoutSecs, lastMoveAt, isOver, isSuspended }: TurnTimerProps) {
  const [remaining, setRemaining] = useState(() => calcRemaining(turnTimeoutSecs, lastMoveAt))

  useEffect(() => {
    if (isOver || turnTimeoutSecs <= 0) {
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
    }, 1_000)

    return () => clearInterval(interval)
  }, [turnTimeoutSecs, lastMoveAt, isOver, isSuspended])

  if (isOver || turnTimeoutSecs <= 0) {
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
      {secs}s{paused ? ' (paused)' : ''}
    </span>
  )
}

function calcRemaining(timeoutSecs: number, lastMoveAt: string): number {
  const deadline = new Date(lastMoveAt).getTime() + timeoutSecs * 1000
  return Math.ceil((deadline - Date.now()) / 1000)
}
