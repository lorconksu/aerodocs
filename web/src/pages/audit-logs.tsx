import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { apiFetch } from '@/lib/api'
import type { AuditLogResponse, User } from '@/types/api'

const AUDIT_ACTIONS = [
  'user.login',
  'user.login_failed',
  'user.login_totp_failed',
  'user.registered',
  'user.created',
  'user.password_changed',
  'user.role_updated',
  'user.totp_setup',
  'user.totp_enabled',
  'user.totp_disabled',
  'user.totp_reset',
  'server.created',
  'server.updated',
  'server.deleted',
  'server.batch_deleted',
  'server.registered',
  'server.connected',
  'server.disconnected',
  'server.unregistered',
  'file.read',
  'file.uploaded',
  'path.granted',
  'path.revoked',
  'log.tail_started',
] as const

const PAGE_SIZE = 50

export function AuditLogsPage() {
  const [filters, setFilters] = useState<{
    action: string
    userId: string
    from: string
    to: string
  }>({ action: '', userId: '', from: '', to: '' })
  const [offset, setOffset] = useState(0)

  // Build query string from filters
  const buildParams = () => {
    const params = new URLSearchParams()
    params.set('limit', String(PAGE_SIZE))
    params.set('offset', String(offset))
    if (filters.action) params.set('action', filters.action)
    if (filters.userId) params.set('user_id', filters.userId)
    if (filters.from) params.set('from', new Date(filters.from).toISOString())
    if (filters.to) params.set('to', new Date(filters.to + 'T23:59:59').toISOString())
    return params.toString()
  }

  const { data, isLoading } = useQuery({
    queryKey: ['audit-logs', filters, offset],
    queryFn: () => apiFetch<AuditLogResponse>(`/audit-logs?${buildParams()}`),
  })

  const { data: usersData } = useQuery({
    queryKey: ['users'],
    queryFn: () => apiFetch<{ users: User[] }>('/users'),
  })
  const users = usersData?.users

  const hasActiveFilters = filters.action || filters.userId || filters.from || filters.to

  const clearFilters = () => {
    setFilters({ action: '', userId: '', from: '', to: '' })
    setOffset(0)
  }

  const updateFilter = (key: keyof typeof filters, value: string) => {
    setFilters(prev => ({ ...prev, [key]: value }))
    setOffset(0)
  }

  const formatDate = (iso: string) => {
    return new Date(iso).toLocaleDateString('en-US', {
      month: 'short', day: 'numeric', year: 'numeric',
      hour: 'numeric', minute: '2-digit',
    })
  }

  const getUsernameById = (userId: string | null) => {
    if (!userId) return 'System'
    const user = users?.find(u => u.id === userId)
    return user?.username ?? userId
  }

  const total = data?.total ?? 0
  const entries = data?.entries ?? []
  const showingFrom = total > 0 ? offset + 1 : 0
  const showingTo = Math.min(offset + PAGE_SIZE, total)

  return (
    <div className="p-6">
      <h2 className="text-lg font-semibold text-text-primary mb-4">Audit Logs</h2>

      {/* Filter Bar */}
      <div className="flex items-center gap-3 mb-4 flex-wrap">
        <input
          type="date"
          value={filters.from}
          onChange={e => updateFilter('from', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
          placeholder="From date"
        />
        <input
          type="date"
          value={filters.to}
          onChange={e => updateFilter('to', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
          placeholder="To date"
        />
        <select
          value={filters.userId}
          onChange={e => updateFilter('userId', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
        >
          <option value="">All Users</option>
          {users?.map(u => (
            <option key={u.id} value={u.id}>{u.username}</option>
          ))}
        </select>
        <select
          value={filters.action}
          onChange={e => updateFilter('action', e.target.value)}
          className="bg-elevated border border-border rounded px-3 py-1.5 text-sm text-text-primary focus:outline-none focus:border-accent"
        >
          <option value="">All Actions</option>
          {AUDIT_ACTIONS.map(a => (
            <option key={a} value={a}>{a}</option>
          ))}
        </select>
        {hasActiveFilters && (
          <button
            onClick={clearFilters}
            className="text-accent hover:text-accent-hover text-sm transition-colors"
          >
            Clear Filters
          </button>
        )}
      </div>

      {/* Table */}
      <div className="border border-border rounded overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface border-b border-border">
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Timestamp</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">User</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Action</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Target</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">IP Address</th>
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-text-muted">Loading...</td></tr>
            )}
            {!isLoading && entries.length === 0 && (
              <tr><td colSpan={5} className="px-4 py-8 text-center text-text-muted">No audit log entries found.</td></tr>
            )}
            {!isLoading && entries.length > 0 && entries.map(entry => (
              <tr key={entry.id} className="border-b border-border last:border-b-0 hover:bg-surface/50">
                <td className="px-4 py-2 text-text-secondary">{formatDate(entry.created_at)}</td>
                <td className="px-4 py-2 text-text-primary">{getUsernameById(entry.user_id)}</td>
                <td className="px-4 py-2">
                  <span className="font-mono text-xs bg-elevated px-2 py-0.5 rounded text-text-secondary">{entry.action}</span>
                </td>
                <td className="px-4 py-2 text-text-muted font-mono text-xs">{entry.target ?? '—'}</td>
                <td className="px-4 py-2 text-text-muted font-mono text-xs">{entry.ip_address ?? '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination Footer */}
      {total > 0 && (
        <div className="flex items-center justify-between mt-3 text-sm text-text-muted">
          <span>Showing {showingFrom}-{showingTo} of {total}</span>
          <div className="flex gap-2">
            <button
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              disabled={offset === 0}
              className="px-3 py-1 border border-border rounded text-text-secondary hover:bg-surface disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Previous
            </button>
            <button
              onClick={() => setOffset(offset + PAGE_SIZE)}
              disabled={offset + PAGE_SIZE >= total}
              className="px-3 py-1 border border-border rounded text-text-secondary hover:bg-surface disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
