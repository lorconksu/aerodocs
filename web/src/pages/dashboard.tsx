import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, Trash2, X, Search } from 'lucide-react'
import { apiFetch } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import { AddServerModal } from '@/pages/add-server-modal'
import type { ServerListResponse, ServerStatus } from '@/types/api'

function relativeTime(dateStr: string | null): string {
  if (!dateStr) return '—'
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin} min ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay}d ago`
}

const statusDot: Record<ServerStatus, string> = {
  online: 'text-status-online',
  offline: 'text-status-offline',
  pending: 'text-status-warning',
}

export function DashboardPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const isAdmin = user?.role === 'admin'

  const [statusFilter, setStatusFilter] = useState<string | undefined>()
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [showAddModal, setShowAddModal] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['servers', statusFilter, searchTerm],
    queryFn: () => {
      const params = new URLSearchParams()
      if (statusFilter) params.set('status', statusFilter)
      if (searchTerm) params.set('search', searchTerm)
      const qs = params.toString()
      return apiFetch<ServerListResponse>(`/servers${qs ? `?${qs}` : ''}`)
    },
    refetchInterval: 10_000,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiFetch(`/servers/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['servers'] }),
  })

  const batchDeleteMutation = useMutation({
    mutationFn: (ids: string[]) =>
      apiFetch('/servers/batch-delete', {
        method: 'POST',
        body: JSON.stringify({ ids }),
      }),
    onSuccess: () => {
      setSelectedIds(new Set())
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  const servers = data?.servers ?? []
  const total = data?.total ?? 0

  const allSelected = servers.length > 0 && servers.every((s) => selectedIds.has(s.id))

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(servers.map((s) => s.id)))
    }
  }

  const statusFilters: { label: string; value: string | undefined }[] = [
    { label: 'All', value: undefined },
    { label: 'Online', value: 'online' },
    { label: 'Offline', value: 'offline' },
    { label: 'Pending', value: 'pending' },
  ]

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-semibold text-text-primary">Fleet Dashboard</h2>
          <p className="text-text-muted text-sm">{total} server{total !== 1 ? 's' : ''}</p>
        </div>
        {isAdmin && (
          <button
            onClick={() => setShowAddModal(true)}
            className="flex items-center gap-2 px-3 py-1.5 bg-accent hover:bg-accent-hover text-white text-sm rounded transition-colors"
          >
            <Plus className="w-4 h-4" />
            Add Server
          </button>
        )}
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4 mb-4">
        <div className="flex gap-1">
          {statusFilters.map(({ label, value }) => (
            <button
              key={label}
              onClick={() => setStatusFilter(value)}
              className={`px-3 py-1 text-xs rounded transition-colors ${
                statusFilter === value
                  ? 'bg-accent text-white'
                  : 'bg-elevated text-text-muted hover:text-text-secondary'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-text-faint" />
          <input
            type="text"
            placeholder="Search servers..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-8 pr-3 py-1.5 bg-elevated border border-border rounded text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      {/* Mass action bar */}
      {selectedIds.size > 0 && (
        <div className="flex items-center gap-3 mb-4 px-3 py-2 bg-elevated border border-border rounded text-sm">
          <span className="text-text-secondary">{selectedIds.size} selected</span>
          <button
            onClick={() => batchDeleteMutation.mutate([...selectedIds])}
            disabled={batchDeleteMutation.isPending}
            className="flex items-center gap-1 px-2 py-1 text-status-offline hover:bg-surface rounded transition-colors text-xs"
          >
            <Trash2 className="w-3.5 h-3.5" />
            Delete Selected
          </button>
          <button
            onClick={() => setSelectedIds(new Set())}
            className="flex items-center gap-1 px-2 py-1 text-text-muted hover:text-text-secondary text-xs"
          >
            <X className="w-3.5 h-3.5" />
            Clear
          </button>
        </div>
      )}

      {/* Table */}
      {isLoading ? (
        <div className="text-text-muted text-sm py-8 text-center">Loading servers...</div>
      ) : servers.length === 0 ? (
        <div className="text-text-muted text-sm py-8 text-center">
          No servers found.{' '}
          {isAdmin && (
            <button onClick={() => setShowAddModal(true)} className="text-accent hover:underline">
              Add your first server
            </button>
          )}
        </div>
      ) : (
        <div className="border border-border rounded overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-elevated text-text-muted text-xs uppercase tracking-wider">
                {isAdmin && (
                  <th className="px-3 py-2 w-8">
                    <input
                      type="checkbox"
                      checked={allSelected}
                      onChange={toggleSelectAll}
                      className="rounded"
                    />
                  </th>
                )}
                <th className="px-3 py-2 w-8">Status</th>
                <th className="px-3 py-2 text-left">Name</th>
                <th className="px-3 py-2 text-left">Hostname / IP</th>
                <th className="px-3 py-2 text-left">OS</th>
                <th className="px-3 py-2 text-left">Last Seen</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {servers.map((srv) => (
                <tr key={srv.id} className="hover:bg-elevated/50 transition-colors">
                  {isAdmin && (
                    <td className="px-3 py-2">
                      <input
                        type="checkbox"
                        checked={selectedIds.has(srv.id)}
                        onChange={() => toggleSelect(srv.id)}
                        className="rounded"
                      />
                    </td>
                  )}
                  <td className="px-3 py-2 text-center">
                    <span className={statusDot[srv.status]}>●</span>
                  </td>
                  <td className="px-3 py-2">
                    <Link
                      to={`/servers/${srv.id}`}
                      className="text-text-primary hover:text-accent transition-colors"
                    >
                      {srv.name}
                    </Link>
                  </td>
                  <td className="px-3 py-2 font-mono text-text-secondary text-xs">
                    {srv.hostname || srv.ip_address ? `${srv.hostname ?? '—'} / ${srv.ip_address ?? '—'}` : '—'}
                  </td>
                  <td className="px-3 py-2 text-text-secondary">{srv.os ?? '—'}</td>
                  <td className="px-3 py-2 text-text-muted">
                    {srv.status === 'pending' ? (
                      <span className="text-status-warning">Pending agent</span>
                    ) : (
                      relativeTime(srv.last_seen_at)
                    )}
                  </td>
                  <td className="px-3 py-2 text-right">
                    {isAdmin && (
                      <button
                        onClick={() => deleteMutation.mutate(srv.id)}
                        disabled={deleteMutation.isPending}
                        className="text-text-muted hover:text-status-offline transition-colors text-xs"
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Server Modal */}
      {showAddModal && <AddServerModal onClose={() => setShowAddModal(false)} />}
    </div>
  )
}
