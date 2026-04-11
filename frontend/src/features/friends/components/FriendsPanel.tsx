import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { friends, players, presence } from '@/features/friends/api'
import { useBlockPlayer } from '@/hooks/useBlockPlayer'
import { useFocusTrap } from '@/hooks/useFocusTrap'
import { keys } from '@/lib/queryClient'
import { useAppStore } from '@/stores/store'
import { ModalOverlay } from '@/ui/ModalOverlay'
import { useToast } from '@/ui/Toast'
import { catchToAppError } from '@/utils/errors'
import { testId } from '@/utils/testId'
import { FriendItem } from './FriendItem'
import styles from './FriendsPanel.module.css'
import { PendingRequestItem } from './PendingRequestItem'

type Tab = 'friends' | 'pending'

interface FriendsPanelProps {
  onClose: () => void
  onOpenDM: (friendId: string) => void
}

export function FriendsPanel({ onClose, onOpenDM }: FriendsPanelProps) {
  const { t } = useTranslation()
  const trapRef = useFocusTrap<HTMLDivElement>()
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
    onSettled: () => {
      invalidate()
      setPendingId(null)
    },
  })

  const declineMut = useMutation({
    mutationFn: (requesterId: string) => friends.declineRequest(player.id, requesterId),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => {
      invalidate()
      setPendingId(null)
    },
  })

  const removeMut = useMutation({
    mutationFn: (friendId: string) => friends.remove(player.id, friendId),
    onError: e => toast.showError(catchToAppError(e)),
    onSettled: () => {
      invalidate()
      setPendingId(null)
    },
  })

  const { block, blockPending } = useBlockPlayer()

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
        toast.showWarning(t('friends.cantAddSelf'))
        return
      }
      await friends.sendRequest(player.id, found.id)
      toast.showInfo(t('friends.requestSent', { username: found.username }))
      setAddUsername('')
      invalidate()
    } catch (err) {
      const appErr = catchToAppError(err)
      if (appErr.reason === 'NOT_FOUND') {
        toast.showWarning(t('friends.playerNotFound', { username }))
      } else {
        toast.showError(appErr)
      }
    } finally {
      setAddPending(false)
    }
  }

  return (
    <ModalOverlay onClose={onClose} className={styles.overlay}>
      <div
        ref={trapRef}
        className={styles.panel}
        {...testId('friends-panel')}
        role='dialog'
        aria-modal='true'
        aria-labelledby='friends-title'
      >
        <div className={styles.header}>
          <h2 className={styles.title} id='friends-title'>
            {t('friends.title')}
          </h2>
          <button type="button" className={styles.closeBtn} onClick={onClose}>
            x
          </button>
        </div>

        <form className={styles.addFriendRow} onSubmit={handleAddFriend}>
          <input
            className={styles.addFriendInput}
            {...testId('add-friend-input')}
            aria-label={t('friends.addFriend')}
            value={addUsername}
            onChange={e => setAddUsername(e.target.value)}
            placeholder={t('friends.addPlaceholder')}
            disabled={addPending}
          />
          <button
            className='btn btn-primary btn-sm'
            {...testId('add-friend-btn')}
            type='submit'
            disabled={addPending || !addUsername.trim()}
          >
            {addPending ? '...' : t('friends.add')}
          </button>
        </form>

        <div className={styles.tabs}>
          <button type="button"
            {...testId('friends-tab')}
            className={`${styles.tab} ${tab === 'friends' ? styles.tabActive : ''}`}
            onClick={() => setTab('friends')}
          >
            {t('friends.friendsCount', { count: safeFriends.length })}
          </button>
          <button type="button"
            {...testId('pending-tab')}
            className={`${styles.tab} ${tab === 'pending' ? styles.tabActive : ''}`}
            onClick={() => setTab('pending')}
          >
            {t('friends.pending')}
            {pendingCount > 0 && <span className={styles.tabBadge}>{pendingCount}</span>}
          </button>
        </div>

        <div className={styles.list}>
          {tab === 'friends' &&
            (safeFriends.length === 0 ? (
              <p className={styles.empty}>{t('friends.noFriends')}</p>
            ) : (
              safeFriends.map(f => (
                <FriendItem
                  key={f.friend_id}
                  friendId={f.friend_id}
                  username={f.friend_username}
                  avatarUrl={f.friend_avatar_url}
                  online={onlineMap[f.friend_id] ?? false}
                  onDM={onOpenDM}
                  onRemove={id => {
                    setPendingId(id)
                    removeMut.mutate(id)
                  }}
                  onBlock={(id, username, avatarUrl) =>
                    block({ targetId: id, username, avatarUrl })
                  }
                  removePending={pendingId === f.friend_id}
                  blockPending={blockPending}
                />
              ))
            ))}

          {tab === 'pending' &&
            (safePending.length === 0 ? (
              <p className={styles.empty}>{t('friends.noPending')}</p>
            ) : (
              safePending.map(f => (
                <PendingRequestItem
                  key={f.friend_id}
                  requesterId={f.friend_id}
                  username={f.friend_username}
                  avatarUrl={f.friend_avatar_url}
                  onAccept={id => {
                    setPendingId(id)
                    acceptMut.mutate(id)
                  }}
                  onDecline={id => {
                    setPendingId(id)
                    declineMut.mutate(id)
                  }}
                  pending={pendingId === f.friend_id}
                />
              ))
            ))}
        </div>
      </div>
    </ModalOverlay>
  )
}

// Floating button rendered globally
interface FriendsButtonProps {
  pendingCount: number
  onClick: () => void
}

export function FriendsButton({ pendingCount, onClick }: FriendsButtonProps) {
  const { t } = useTranslation()
  return (
    <button type="button" className={styles.floatingBtn} onClick={onClick} {...testId('friends-btn')}>
      <svg
        width='14'
        height='14'
        viewBox='0 0 24 24'
        fill='none'
        stroke='currentColor'
        strokeWidth='1.5'
      >
        <path d='M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2' />
        <circle cx='9' cy='7' r='4' />
        <path d='M22 21v-2a4 4 0 0 0-3-3.87' />
        <path d='M16 3.13a4 4 0 0 1 0 7.75' />
      </svg>
      {t('friends.title')}
      {pendingCount > 0 && <span className={styles.floatingBadge}>{pendingCount}</span>}
    </button>
  )
}
