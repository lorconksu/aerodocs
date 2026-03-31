import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '@/lib/api'
import type { ServerListResponse } from '@/types/api'

/**
 * Shared hook for the full (unfiltered) server list.
 * Used by app-shell for fleet counts and by dashboard when no filters are active.
 * Both consumers share the same query key so React Query deduplicates the polling.
 *
 * @param options.refetchInterval - Override the polling interval (default 10s). Pass `false` to disable polling.
 */
export const ALL_SERVERS_KEY = ['servers', 'all'] as const

export function useAllServers(options?: { refetchInterval?: number | false }) {
  return useQuery({
    queryKey: ALL_SERVERS_KEY,
    queryFn: () => apiFetch<ServerListResponse>('/servers?limit=1000'),
    refetchInterval: options?.refetchInterval ?? 10_000,
  })
}
