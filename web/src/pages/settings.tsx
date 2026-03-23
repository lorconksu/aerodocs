import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '@/hooks/use-auth'
import { apiFetch } from '@/lib/api'
import type { ChangePasswordRequest, User, Role, TOTPDisableRequest } from '@/types/api'
import { CreateUserModal } from '@/pages/create-user-modal'

function ProfileTab() {
  const { user } = useAuth()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordErrors, setPasswordErrors] = useState<string[]>([])
  const [success, setSuccess] = useState('')

  const validatePassword = (pw: string) => {
    const errors: string[] = []
    if (pw.length < 12) errors.push('At least 12 characters')
    if (!/[A-Z]/.test(pw)) errors.push('One uppercase letter')
    if (!/[a-z]/.test(pw)) errors.push('One lowercase letter')
    if (!/\d/.test(pw)) errors.push('One digit')
    if (!/[^a-zA-Z0-9]/.test(pw)) errors.push('One special character')
    setPasswordErrors(errors)
    return errors.length === 0
  }

  const mutation = useMutation({
    mutationFn: (data: ChangePasswordRequest) =>
      apiFetch<{ status: string }>('/auth/password', {
        method: 'PUT',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      setSuccess('Password updated successfully.')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setPasswordErrors([])
    },
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setSuccess('')

    if (!validatePassword(newPassword)) return
    if (newPassword !== confirmPassword) {
      mutation.reset()
      return
    }

    mutation.mutate({
      current_password: currentPassword,
      new_password: newPassword,
    })
  }

  return (
    <div className="max-w-lg space-y-6">
      {/* Account Info */}
      <div>
        <h3 className="text-sm font-semibold text-text-primary mb-3">Account Info</h3>
        <div className="bg-surface border border-border rounded p-4 space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-text-muted">Username</span>
            <span className="text-text-primary">{user?.username}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-text-muted">Email</span>
            <span className="text-text-primary">{user?.email}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-text-muted">Role</span>
            <span className="text-text-primary capitalize">{user?.role}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-text-muted">2FA</span>
            <span className="text-status-online">Enabled</span>
          </div>
        </div>
      </div>

      {/* Change Password */}
      <div>
        <h3 className="text-sm font-semibold text-text-primary mb-3">Change Password</h3>
        <form onSubmit={handleSubmit} className="space-y-3">
          {mutation.isError && (
            <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2">
              {mutation.error instanceof Error ? mutation.error.message : 'Failed to update password'}
            </div>
          )}
          {success && (
            <div className="bg-status-online/10 border border-status-online/20 text-status-online text-xs rounded px-3 py-2">
              {success}
            </div>
          )}

          <input
            type="password"
            placeholder="Current password"
            value={currentPassword}
            onChange={e => setCurrentPassword(e.target.value)}
            className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
            required
          />
          <div>
            <input
              type="password"
              placeholder="New password (min 12 chars)"
              value={newPassword}
              onChange={e => { setNewPassword(e.target.value); validatePassword(e.target.value) }}
              className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
              required
            />
            {newPassword && passwordErrors.length > 0 && (
              <div className="mt-2 space-y-1">
                {passwordErrors.map(err => (
                  <div key={err} className="text-status-warning text-[10px]">• {err}</div>
                ))}
              </div>
            )}
          </div>
          <input
            type="password"
            placeholder="Confirm new password"
            value={confirmPassword}
            onChange={e => setConfirmPassword(e.target.value)}
            className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
            required
          />
          {confirmPassword && newPassword !== confirmPassword && (
            <div className="text-status-warning text-[10px]">• Passwords do not match</div>
          )}
          <button
            type="submit"
            disabled={mutation.isPending || passwordErrors.length > 0 || newPassword !== confirmPassword}
            className="bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded px-4 py-2 transition-colors disabled:opacity-50"
          >
            {mutation.isPending ? 'Updating...' : 'Update Password'}
          </button>
        </form>
      </div>
    </div>
  )
}

function UsersTab() {
  const { user: currentUser } = useAuth()
  const queryClient = useQueryClient()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [disableTotpUserId, setDisableTotpUserId] = useState<string | null>(null)
  const [adminTotpCode, setAdminTotpCode] = useState('')

  const { data: users, isLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => apiFetch<User[]>('/users'),
  })

  const updateRoleMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: Role }) =>
      apiFetch<{ user: User }>(`/users/${userId}/role`, {
        method: 'PUT',
        body: JSON.stringify({ role }),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const deleteUserMutation = useMutation({
    mutationFn: (userId: string) =>
      apiFetch<{ status: string }>(`/users/${userId}`, { method: 'DELETE' }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const disableTotpMutation = useMutation({
    mutationFn: (data: TOTPDisableRequest) =>
      apiFetch<{ status: string }>('/auth/totp/disable', {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setDisableTotpUserId(null)
      setAdminTotpCode('')
    },
  })

  const handleDisableTotp = (e: React.FormEvent) => {
    e.preventDefault()
    if (!disableTotpUserId) return
    disableTotpMutation.mutate({
      user_id: disableTotpUserId,
      admin_totp_code: adminTotpCode,
    })
  }

  const formatDate = (iso: string) => {
    const date = new Date(iso)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
    if (diffDays === 0) return 'Today'
    if (diffDays === 1) return 'Yesterday'
    if (diffDays < 30) return `${diffDays} days ago`
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-semibold text-text-primary">User Management</h3>
        <button
          onClick={() => setShowCreateModal(true)}
          className="bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded px-4 py-1.5 transition-colors"
        >
          Create User
        </button>
      </div>

      {updateRoleMutation.isError && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-3">
          {updateRoleMutation.error instanceof Error ? updateRoleMutation.error.message : 'Failed to update role'}
        </div>
      )}

      <div className="border border-border rounded overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface border-b border-border">
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Username</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Email</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Role</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">2FA</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Created</th>
              <th className="text-left px-4 py-2 text-text-muted font-medium text-xs uppercase tracking-wider">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-text-muted">Loading...</td></tr>
            ) : !users || users.length === 0 ? (
              <tr><td colSpan={6} className="px-4 py-8 text-center text-text-muted">No users found.</td></tr>
            ) : (
              users.map(u => (
                <tr key={u.id} className="border-b border-border last:border-b-0 hover:bg-surface/50">
                  <td className="px-4 py-2 text-text-primary">{u.username}</td>
                  <td className="px-4 py-2 text-text-secondary">{u.email}</td>
                  <td className="px-4 py-2">
                    {u.id === currentUser?.id ? (
                      <span className={`text-xs px-2 py-0.5 rounded ${
                        u.role === 'admin' ? 'bg-accent/20 text-accent' : 'bg-elevated text-text-muted'
                      }`}>
                        {u.role === 'admin' ? 'Admin' : 'Viewer'}
                      </span>
                    ) : (
                      <select
                        value={u.role}
                        onChange={e => updateRoleMutation.mutate({ userId: u.id, role: e.target.value as Role })}
                        disabled={updateRoleMutation.isPending}
                        className="bg-elevated border border-border rounded px-2 py-0.5 text-xs text-text-primary focus:outline-none focus:border-accent"
                      >
                        <option value="admin">Admin</option>
                        <option value="viewer">Viewer</option>
                      </select>
                    )}
                  </td>
                  <td className="px-4 py-2">
                    {u.totp_enabled ? (
                      <span className="text-xs text-status-online">Enabled</span>
                    ) : (
                      <span className="text-xs text-status-warning">Not set up</span>
                    )}
                  </td>
                  <td className="px-4 py-2 text-text-muted text-xs">{formatDate(u.created_at)}</td>
                  <td className="px-4 py-2">
                    <div className="flex items-center gap-3">
                      {u.id !== currentUser?.id && u.totp_enabled && (
                        <button
                          onClick={() => setDisableTotpUserId(u.id)}
                          className="text-xs text-status-warning hover:text-status-error transition-colors"
                        >
                          Disable 2FA
                        </button>
                      )}
                      {u.id !== currentUser?.id && (
                        <button
                          onClick={() => {
                            if (confirm(`Delete user "${u.username}"? This cannot be undone.`)) {
                              deleteUserMutation.mutate(u.id)
                            }
                          }}
                          disabled={deleteUserMutation.isPending}
                          className="text-xs text-text-muted hover:text-status-error transition-colors"
                        >
                          Delete
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Create User Modal */}
      {showCreateModal && (
        <CreateUserModal onClose={() => setShowCreateModal(false)} />
      )}

      {/* Disable TOTP Confirmation Modal */}
      {disableTotpUserId && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-surface border border-border rounded-lg p-6 w-full max-w-sm">
            <h3 className="text-sm font-semibold text-text-primary mb-2">Disable 2FA</h3>
            <p className="text-text-muted text-xs mb-4">
              Enter your own TOTP code to confirm disabling 2FA for this user.
            </p>

            {disableTotpMutation.isError && (
              <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-3">
                {disableTotpMutation.error instanceof Error ? disableTotpMutation.error.message : 'Failed to disable 2FA'}
              </div>
            )}

            <form onSubmit={handleDisableTotp}>
              <input
                type="text"
                placeholder="Your 6-digit TOTP code"
                value={adminTotpCode}
                onChange={e => setAdminTotpCode(e.target.value)}
                className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent mb-4 font-mono text-center tracking-widest"
                maxLength={6}
                autoFocus
                required
              />
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => { setDisableTotpUserId(null); setAdminTotpCode('') }}
                  className="flex-1 border border-border rounded py-2 text-sm text-text-secondary hover:bg-elevated transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={disableTotpMutation.isPending || adminTotpCode.length !== 6}
                  className="flex-1 bg-status-error hover:bg-status-error/80 text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
                >
                  {disableTotpMutation.isPending ? 'Disabling...' : 'Disable 2FA'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  )
}

export function SettingsPage() {
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const [activeTab, setActiveTab] = useState<'profile' | 'users'>('profile')

  return (
    <div>
      <h2 className="text-lg font-semibold text-text-primary mb-4">Settings</h2>

      {/* Tab bar */}
      <div className="flex border-b border-border mb-6">
        <button
          onClick={() => setActiveTab('profile')}
          className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
            activeTab === 'profile'
              ? 'border-accent text-text-primary'
              : 'border-transparent text-text-muted hover:text-text-secondary'
          }`}
        >
          Profile
        </button>
        {isAdmin && (
          <button
            onClick={() => setActiveTab('users')}
            className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
              activeTab === 'users'
                ? 'border-accent text-text-primary'
                : 'border-transparent text-text-muted hover:text-text-secondary'
            }`}
          >
            Users
          </button>
        )}
      </div>

      {/* Tab content */}
      {activeTab === 'profile' && <ProfileTab />}
      {activeTab === 'users' && isAdmin && <UsersTab />}
    </div>
  )
}
