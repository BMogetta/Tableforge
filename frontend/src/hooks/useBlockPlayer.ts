import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { type BlockedPlayer, blocks } from '@/features/friends/api'
import { keys } from '@/lib/queryClient'
import { useAppStore } from '@/stores/store'
import { useToast } from '@/ui/Toast'
import { catchToAppError } from '@/utils/errors'

/**
 * Hook for block/unblock player operations.
 *
 * Since the backend has no "list blocked" endpoint, blocked players are tracked
 * locally in query cache. The list is built incrementally: when you block a
 * player, their info is added to the cache; when you unblock, it's removed.
 *
 * On page refresh the list resets to empty — this is acceptable because the
 * block status is enforced server-side (DMs, matchmaking, etc. respect it).
 * The client-side list is only for the Settings > Blocked Players UI.
 */
export function useBlockPlayer() {
  const player = useAppStore(s => s.player)!
  const toast = useToast()
  const qc = useQueryClient()

  const { data: blockedPlayers = [] } = useQuery<BlockedPlayer[]>({
    queryKey: keys.blockedPlayers(player.id),
    queryFn: () => qc.getQueryData<BlockedPlayer[]>(keys.blockedPlayers(player.id)) ?? [],
    staleTime: Infinity,
  })

  const blockMut = useMutation({
    mutationFn: ({ targetId }: { targetId: string; username: string; avatarUrl?: string }) =>
      blocks.block(player.id, targetId),
    onSuccess: (_data, { targetId, username, avatarUrl }) => {
      // Add to local blocked list
      const current = qc.getQueryData<BlockedPlayer[]>(keys.blockedPlayers(player.id)) ?? []
      if (!current.some(b => b.id === targetId)) {
        qc.setQueryData<BlockedPlayer[]>(keys.blockedPlayers(player.id), [
          ...current,
          { id: targetId, username, avatar_url: avatarUrl, blocked_at: new Date().toISOString() },
        ])
      }
      // Remove from friends list since blocking removes friendship server-side
      qc.invalidateQueries({ queryKey: keys.friends(player.id) })
      qc.invalidateQueries({ queryKey: keys.friendsPending(player.id) })
      toast.showInfo(`${username} has been blocked.`)
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  const unblockMut = useMutation({
    mutationFn: (targetId: string) => blocks.unblock(player.id, targetId),
    onSuccess: (_data, targetId) => {
      // Remove from local blocked list
      const current = qc.getQueryData<BlockedPlayer[]>(keys.blockedPlayers(player.id)) ?? []
      const updated = current.filter(b => b.id !== targetId)
      qc.setQueryData<BlockedPlayer[]>(keys.blockedPlayers(player.id), updated)
      const removed = current.find(b => b.id === targetId)
      toast.showInfo(`${removed?.username ?? 'Player'} has been unblocked.`)
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  const isBlocked = (targetId: string) => blockedPlayers.some(b => b.id === targetId)

  return {
    blockedPlayers,
    block: blockMut.mutate,
    unblock: unblockMut.mutate,
    isBlocked,
    blockPending: blockMut.isPending,
    unblockPending: unblockMut.isPending,
  }
}
