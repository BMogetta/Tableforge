import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { friends, players, presence } from '@/features/friends/api'
import { useAppStore } from '@/stores/store'
import { keys } from '@/lib/queryClient'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { FriendItem } from './FriendItem'
import { PendingRequestItem } from './PendingRequestItem'
import styles from './FriendsPanel.module.css'

type Tab = 'friends' | 'pending'

interface FriendsPanelProps {
  onClose: () => void
  onOpenDM: (friendId: string) => void
}

export function FriendsPanel({ onClose, onOpenDM }: FriendsPanelProps) {
  const player = useAppStore(s => s.player)!
  const toast = useToast()
  const qc = useQueryClient()
  const [tab, setTab] = useState<Tab>('friends')
  const [pendingId, setPendingId] = useState<string | null>(null)
  const [addUsername, setAddUsername] = useState('')
  const [addPending, setAddPending] = useState(false)

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: keys.friends(player.id) })
    qc.invalidateQueries({ queryKey: keys.friendsPending(player.id) })
  }

  const { data: friendsList = [] } = useQuery({
    queryKey: keys.friends(player.id),
    queryFn: () => friends.list(player.id),
  })

  const { data: pendingList = [] } = useQuery({
    queryKey: keys.friendsPending(player.id),
    queryFn: () => friends.pending(player.id),
  })

  // Fetch online status for all friends
  const friendIds = (friendsList ?? []).map(f => f.friend_id)
  const { data: onlineMap = {} } = useQuery({
    queryKey: ['presence', ...friendIds],
    queryFn: () => presence.check(friendIds),
    enabled: friendIds.length > 0,
    refetchInterval: 15_000,
  })

  const safeFriends = friendsList ?? []
  const safePending = pendingList ?? []
  const pendingCount = safePending.length

  const acceptMut = useMutation({
    mutationFn: (requesterId: string) => friends.acceptRequest(player.id, requesterId),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => { invalidate(); setPendingId(null) },
  })

  const declineMut = useMutation({
    mutationFn: (requesterId: string) => friends.declineRequest(player.id, requesterId),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => { invalidate(); setPendingId(null) },
  })

  const removeMut = useMutation({
    mutationFn: (friendId: string) => friends.remove(player.id, friendId),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => { invalidate(); setPendingId(null) },
  })

  async function handleAddFriend(e: React.FormEvent) {
    e.preventDefault()
    const username = addUsername.trim()
    if (!username) {
      return
    }
    setAddPending(true)
    try {
      const found = await players.search(username)
      if (found.id === player.id) {
        toast.showWarning("You can't add yourself as a friend.")
        return
      }
      await friends.sendRequest(player.id, found.id)
      toast.showInfo(`Friend request sent to ${found.username}!`)
      setAddUsername('')
      invalidate()
    } catch (err) {
      const appErr = catchToAppError(err)
      if (appErr.reason === 'NOT_FOUND') {
        toast.showWarning(`Player "${username}" not found.`)
      } else {
        toast.showError(appErr)
      }
    } finally {
      setAddPending(false)
    }
  }

  return (
    <div className={styles.overlay} onClick={e => e.target === e.currentTarget && onClose()}>
      <div className={styles.panel} data-testid='friends-panel'>
        <div className={styles.header}>
          <h2 className={styles.title}>Friends</h2>
          <button className={styles.closeBtn} onClick={onClose}>x</button>
        </div>

        <form className={styles.addFriendRow} onSubmit={handleAddFriend}>
          <input
            className={styles.addFriendInput}
            value={addUsername}
            onChange={e => setAddUsername(e.target.value)}
            placeholder='Add by username...'
            disabled={addPending}
          />
          <button
            className='btn btn-primary'
            type='submit'
            disabled={addPending || !addUsername.trim()}
            style={{ padding: '4px 12px', fontSize: 11 }}
          >
            {addPending ? '...' : 'Add'}
          </button>
        </form>

        <div className={styles.tabs}>
          <button
            className={`${styles.tab} ${tab === 'friends' ? styles.tabActive : ''}`}
            onClick={() => setTab('friends')}
          >
            Friends ({safeFriends.length})
          </button>
          <button
            className={`${styles.tab} ${tab === 'pending' ? styles.tabActive : ''}`}
            onClick={() => setTab('pending')}
          >
            Pending
            {pendingCount > 0 && <span className={styles.tabBadge}>{pendingCount}</span>}
          </button>
        </div>

        <div className={styles.list}>
          {tab === 'friends' && (
            safeFriends.length === 0 ? (
              <p className={styles.empty}>No friends yet.</p>
            ) : (
              safeFriends.map(f => (
                <FriendItem
                  key={f.friend_id}
                  friendId={f.friend_id}
                  username={f.friend_username}
                  avatarUrl={f.friend_avatar_url}
                  online={onlineMap[f.friend_id] ?? false}
                  onDM={onOpenDM}
                  onRemove={id => { setPendingId(id); removeMut.mutate(id) }}
                  removePending={pendingId === f.friend_id}
                />
              ))
            )
          )}

          {tab === 'pending' && (
            safePending.length === 0 ? (
              <p className={styles.empty}>No pending requests.</p>
            ) : (
              safePending.map(f => (
                <PendingRequestItem
                  key={f.friend_id}
                  requesterId={f.friend_id}
                  username={f.friend_username}
                  avatarUrl={f.friend_avatar_url}
                  onAccept={id => { setPendingId(id); acceptMut.mutate(id) }}
                  onDecline={id => { setPendingId(id); declineMut.mutate(id) }}
                  pending={pendingId === f.friend_id}
                />
              ))
            )
          )}
        </div>
      </div>
    </div>
  )
}

// Floating button rendered globally
interface FriendsButtonProps {
  pendingCount: number
  onClick: () => void
}

export function FriendsButton({ pendingCount, onClick }: FriendsButtonProps) {
  return (
    <button className={styles.floatingBtn} onClick={onClick} data-testid='friends-btn'>
      <svg width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='currentColor' strokeWidth='1.5'>
        <path d='M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2' />
        <circle cx='9' cy='7' r='4' />
        <path d='M22 21v-2a4 4 0 0 0-3-3.87' />
        <path d='M16 3.13a4 4 0 0 1 0 7.75' />
      </svg>
      Friends
      {pendingCount > 0 && <span className={styles.floatingBadge}>{pendingCount}</span>}
    </button>
  )
}
