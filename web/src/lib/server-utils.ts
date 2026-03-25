import type { ServerStatus } from '@/types/api'

/** Maps server status values to their CSS colour class. */
export const statusDot: Record<ServerStatus, string> = {
  online: 'text-status-online',
  offline: 'text-status-offline',
  pending: 'text-status-warning',
}
