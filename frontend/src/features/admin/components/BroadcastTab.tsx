import { useState } from 'react'
import { admin } from '@/features/admin/api'
import { useToast } from '@/ui/Toast'
import { catchToAppError } from '@/utils/errors'
import { testId } from '@/utils/testId'
import styles from '../Admin.module.css'

type BroadcastType = 'info' | 'warning'

export function BroadcastTab() {
  const toast = useToast()
  const [message, setMessage] = useState('')
  const [type, setType] = useState<BroadcastType>('info')
  const [sending, setSending] = useState(false)

  async function handleSend() {
    const trimmed = message.trim()
    if (!trimmed) return

    setSending(true)
    try {
      await admin.sendBroadcast(trimmed, type)
      toast.showInfo('Broadcast sent.')
      setMessage('')
    } catch (e) {
      toast.showError(catchToAppError(e))
    } finally {
      setSending(false)
    }
  }

  return (
    <div className={styles.panel} {...testId('broadcast-panel')}>
      <div className={styles.detailCard}>
        <h3>Send Broadcast</h3>
        <p className={styles.muted} style={{ fontSize: 'var(--text-sm)' }}>
          This message will be delivered to all connected players in real time.
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
          <div style={{ display: 'flex', gap: 'var(--space-3)', alignItems: 'center' }}>
            <label
              htmlFor='broadcast-type'
              className={styles.muted}
              style={{ fontSize: 'var(--text-sm)' }}
            >
              Type
            </label>
            <select
              id='broadcast-type'
              className='input input-sm'
              value={type}
              onChange={e => setType(e.target.value as BroadcastType)}
              style={{ maxWidth: 140 }}
              {...testId('broadcast-type')}
            >
              <option value='info'>Info</option>
              <option value='warning'>Warning</option>
            </select>
          </div>

          <textarea
            className='input'
            rows={4}
            placeholder='Type your message...'
            value={message}
            onChange={e => setMessage(e.target.value)}
            onKeyDown={e => {
              if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleSend()
            }}
            {...testId('broadcast-message')}
          />

          <div className={styles.actionRow}>
            <button
              type='button'
              className={`btn btn-sm ${type === 'warning' ? 'btn-danger' : 'btn-primary'}`}
              disabled={!message.trim() || sending}
              onClick={handleSend}
              {...testId('broadcast-send')}
            >
              {sending ? 'Sending...' : `Send ${type}`}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
