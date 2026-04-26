import { useQuery } from '@tanstack/react-query'
import { validatedRequest } from '@/lib/api'
import { keys } from '@/lib/queryClient'
import {
  type GetMeCapabilitiesResponse,
  getMeCapabilitiesResponseSchema,
} from '@/lib/schema-generated.zod'

const EMPTY_CAPABILITIES: GetMeCapabilitiesResponse = {
  canSeeDevtools: false,
}

/**
 * useCapability fetches the current user's server-computed capability flags
 * from `/auth/me/capabilities`. The hook returns a `GetMeCapabilitiesResponse`
 * always — if the user is unauthenticated, the request 401s and the hook
 * falls back to a zero-capability object so consumers can render safely.
 *
 * The server pairs JWT role with live Unleash flag state, so client-side code
 * should **only** read these booleans; reproducing the logic in the browser
 * would let a malicious user flip a local flag and unlock gated features.
 *
 * Cache policy: fresh for 5 minutes (capabilities change rarely — role
 * changes are already full page reloads via logout/login, and flag flips
 * propagate in ~15s on the server side anyway, so we don't need aggressive
 * refetches here).
 */
export function useCapability() {
  const query = useQuery({
    queryKey: keys.capabilities(),
    queryFn: () => validatedRequest(getMeCapabilitiesResponseSchema, '/auth/me/capabilities'),
    staleTime: 5 * 60_000,
    retry: false,
  })

  return {
    capabilities: query.data ?? EMPTY_CAPABILITIES,
    isLoading: query.isPending,
    isError: query.isError,
  }
}
