import styles from '../Room.module.css'
import { testId } from '@/utils/testId'

interface InviteCodeProps {
  code: string
  isPrivate: boolean
}

export function InviteCode({ code, isPrivate }: InviteCodeProps) {
  return (
    <div className={styles.shareSection}>
      {isPrivate ? (
        <>
          <p className='label'>Private Room</p>
          <p className={styles.privateNote}>
            Share the room code privately — it won't appear in the public lobby.
          </p>
        </>
      ) : (
        <p className='label'>Invite Code</p>
      )}
      <div className={styles.codeBox}>
        <span {...testId('room-code-display')} className={styles.codeDisplay}>
          {code}
        </span>
        <button
          className='btn btn-ghost'
          style={{ padding: '4px 10px', fontSize: 11 }}
          onClick={() => navigator.clipboard.writeText(code)}
        >
          Copy
        </button>
      </div>
    </div>
  )
}
