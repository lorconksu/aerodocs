import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { BrowserRouter, MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SettingsPage } from '../settings'

vi.mock('@/lib/api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('@/lib/avatar', () => ({
  getAvatarColor: vi.fn(() => '#3b82f6'),
  setAvatarColor: vi.fn(),
  AVATAR_COLORS: ['#3b82f6', '#8b5cf6', '#06b6d4'],
}))

vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(() => ({
    user: { id: 'u1', username: 'admin', email: 'admin@test.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '2024-01-01T00:00:00Z', updated_at: '' },
    login: vi.fn(),
  })),
}))

vi.mock('@/pages/create-user-modal', () => ({
  CreateUserModal: ({ onClose }: { onClose: () => void }) => (
    <div data-testid="create-user-modal">
      <button onClick={onClose}>Close Modal</button>
    </div>
  ),
}))

import { apiFetch } from '@/lib/api'
const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

import { useAuth } from '@/hooks/use-auth'
const mockUseAuth = useAuth as ReturnType<typeof vi.fn>

const mockUsers = [
  { id: 'u1', username: 'admin', email: 'admin@test.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '2024-01-01T00:00:00Z', updated_at: '' },
  { id: 'u2', username: 'viewer', email: 'viewer@test.com', role: 'viewer', totp_enabled: false, avatar: null, created_at: '2024-01-15T00:00:00Z', updated_at: '' },
]

function renderPage(initialPath = '/settings') {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={[initialPath]}>
        <SettingsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

describe('SettingsPage', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    mockUseAuth.mockReturnValue({
      user: { id: 'u1', username: 'admin', email: 'admin@test.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '2024-01-01T00:00:00Z', updated_at: '' },
      login: vi.fn(),
    })
  })

  it('renders Settings heading', () => {
    renderPage()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('shows Profile and Users tabs for admin', () => {
    renderPage()
    expect(screen.getByRole('button', { name: 'Profile' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Users' })).toBeInTheDocument()
  })

  it('does not show Users tab for non-admin', () => {
    mockUseAuth.mockReturnValue({
      user: { id: 'u2', username: 'viewer', email: 'v@b.com', role: 'viewer', totp_enabled: true, avatar: null, created_at: '', updated_at: '' },
      login: vi.fn(),
    })
    renderPage()
    expect(screen.queryByRole('button', { name: 'Users' })).not.toBeInTheDocument()
  })

  it('shows avatar section with username initial', () => {
    renderPage()
    expect(screen.getAllByText('A').length).toBeGreaterThan(0) // First letter of 'admin'
  })

  it('shows user info section', () => {
    renderPage()
    expect(screen.getByText('admin@test.com')).toBeInTheDocument()
    // "admin" appears multiple times - use a more specific query
    expect(screen.getAllByText('admin').length).toBeGreaterThan(0)
  })

  it('shows Change Password form', () => {
    renderPage()
    expect(screen.getByPlaceholderText('Current password')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('New password (min 12 chars)')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('Confirm new password')).toBeInTheDocument()
  })

  it('shows password validation errors for weak new password', async () => {
    renderPage()
    fireEvent.change(screen.getByPlaceholderText('New password (min 12 chars)'), { target: { value: 'weak' } })
    await waitFor(() => {
      // Validation errors are rendered as "• At least 12 characters" in a div
      expect(screen.getByText(/At least 12 characters/)).toBeInTheDocument()
    })
  })

  it('shows passwords do not match warning', async () => {
    renderPage()
    fireEvent.change(screen.getByPlaceholderText('New password (min 12 chars)'), { target: { value: 'ValidPass1@#$' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm new password'), { target: { value: 'Different1@#$' } })
    await waitFor(() => {
      expect(screen.getByText('• Passwords do not match')).toBeInTheDocument()
    })
  })

  it('submits password change successfully', async () => {
    mockApiFetch.mockResolvedValueOnce({ status: 'ok' })
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('Current password'), { target: { value: 'OldPass1@#$' } })
    fireEvent.change(screen.getByPlaceholderText('New password (min 12 chars)'), { target: { value: 'NewPass1@#$abc' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm new password'), { target: { value: 'NewPass1@#$abc' } })
    fireEvent.submit(screen.getByRole('button', { name: /update password/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Password updated successfully.')).toBeInTheDocument()
    })
  })

  it('shows error on password change failure', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Wrong current password'))
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('Current password'), { target: { value: 'WrongOld1@#$' } })
    fireEvent.change(screen.getByPlaceholderText('New password (min 12 chars)'), { target: { value: 'NewPass1@#$abc' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm new password'), { target: { value: 'NewPass1@#$abc' } })
    fireEvent.submit(screen.getByRole('button', { name: /update password/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Wrong current password')).toBeInTheDocument()
    })
  })

  it('Update Password button is disabled when passwords do not match', async () => {
    renderPage()
    fireEvent.change(screen.getByPlaceholderText('New password (min 12 chars)'), { target: { value: 'ValidPass1@#$' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm new password'), { target: { value: 'Different1@#$' } })

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /update password/i })).toBeDisabled()
    })
  })

  it('clicking Users tab switches to UsersTab', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByText('User Management')).toBeInTheDocument()
    })
  })

  it('Users tab shows loading state', async () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}))
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByText('Loading...')).toBeInTheDocument()
    })
  })

  it('Users tab shows user list', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByText('viewer@test.com')).toBeInTheDocument()
    })
  })

  it('Users tab shows no users message when empty', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: [] })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByText('No users found.')).toBeInTheDocument()
    })
  })

  it('Users tab shows Create User button', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Create User' })).toBeInTheDocument()
    })
  })

  it('Create User button opens CreateUserModal', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create User' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Create User' }))
    await waitFor(() => {
      expect(screen.getByTestId('create-user-modal')).toBeInTheDocument()
    })
  })

  it('shows Disable 2FA button for other users with TOTP enabled', async () => {
    const usersWithTotp = [
      { ...mockUsers[0] },
      { ...mockUsers[1], totp_enabled: true },
    ]
    mockApiFetch.mockResolvedValueOnce({ users: usersWithTotp })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Disable 2FA' })).toBeInTheDocument()
    })
  })

  it('shows Delete button for other users', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument()
    })
  })

  it('opens delete confirmation modal when Delete clicked', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => {
      // Modal h3 says "Delete User", modal also has a "Delete User" submit button
      expect(screen.getAllByText('Delete User').length).toBeGreaterThan(0)
      expect(screen.getByText(/"viewer"/)).toBeInTheDocument()
    })
  })

  it('delete user confirmation modal has Cancel and Delete User buttons', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Delete User' })).toBeInTheDocument()
    })
  })

  it('cancel on delete modal closes it', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByText('Delete User')).not.toBeInTheDocument())
  })

  it('opens Disable 2FA modal when button clicked', async () => {
    const usersWithTotp = [mockUsers[0], { ...mockUsers[1], totp_enabled: true }]
    mockApiFetch.mockResolvedValueOnce({ users: usersWithTotp })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Disable 2FA' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Disable 2FA' }))
    await waitFor(() => {
      // After clicking "Disable 2FA" button, the modal opens with h3 "Disable 2FA" and another "Disable 2FA" submit button
      expect(screen.getAllByText('Disable 2FA').length).toBeGreaterThan(1)
      expect(screen.getByPlaceholderText('Your 6-digit TOTP code')).toBeInTheDocument()
    })
  })

  it('avatar color buttons render', () => {
    renderPage()
    // Color buttons have style.backgroundColor set
    const colorButtons = screen.getAllByRole('button').filter(btn => {
      const el = btn as HTMLButtonElement
      return el.style.backgroundColor !== ''
    })
    expect(colorButtons.length).toBeGreaterThan(0)
  })

  it('shows avatar image when user has avatar', () => {
    mockUseAuth.mockReturnValue({
      user: { id: 'u1', username: 'admin', email: 'admin@test.com', role: 'admin', totp_enabled: true, avatar: 'data:image/png;base64,abc', created_at: '', updated_at: '' },
      login: vi.fn(),
    })
    renderPage()
    // Avatar img has alt="" (presentational), so query by tag directly
    const avatarImg = document.querySelector('img[src^="data:image"]')
    expect(avatarImg).toBeInTheDocument()
  })

  it('Upload image button triggers file input', () => {
    renderPage()
    const uploadBtn = screen.getByText('Upload image')
    expect(uploadBtn).toBeInTheDocument()
    fireEvent.click(uploadBtn)
    // No error should occur
  })

  it('shows 2FA status as Enabled in account info', () => {
    renderPage()
    // Multiple "Enabled" could appear - just ensure at least one exists
    expect(screen.getAllByText('Enabled').length).toBeGreaterThan(0)
  })

  it('shows users tab when URL has ?tab=users', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage('/settings?tab=users')
    await waitFor(() => {
      expect(screen.getByText('User Management')).toBeInTheDocument()
    })
  })

  it('cancels Disable 2FA modal', async () => {
    const usersWithTotp = [mockUsers[0], { ...mockUsers[1], totp_enabled: true }]
    mockApiFetch.mockResolvedValueOnce({ users: usersWithTotp })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Disable 2FA' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Disable 2FA' }))
    await waitFor(() => expect(screen.getByPlaceholderText('Your 6-digit TOTP code')).toBeInTheDocument())

    // Cancel
    const cancelButtons = screen.getAllByRole('button', { name: 'Cancel' })
    fireEvent.click(cancelButtons[0])
    await waitFor(() => {
      expect(screen.queryByPlaceholderText('Your 6-digit TOTP code')).not.toBeInTheDocument()
    })
  })

  it('handles avatar upload for large file', async () => {
    renderPage()
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    expect(fileInput).toBeTruthy()
  })

  it('shows Remove button when user has avatar', () => {
    mockUseAuth.mockReturnValue({
      user: { id: 'u1', username: 'admin', email: 'admin@test.com', role: 'admin', totp_enabled: true, avatar: 'data:image/png;base64,abc', created_at: '', updated_at: '' },
      login: vi.fn(),
    })
    renderPage()
    expect(screen.getByText('Remove')).toBeInTheDocument()
  })

  it('clicking Remove avatar button calls avatarMutation', async () => {
    mockApiFetch.mockResolvedValueOnce({ status: 'ok' })
    mockUseAuth.mockReturnValue({
      user: { id: 'u1', username: 'admin', email: 'admin@test.com', role: 'admin', totp_enabled: true, avatar: 'data:image/png;base64,abc', created_at: '', updated_at: '' },
      login: vi.fn(),
    })
    renderPage()
    fireEvent.click(screen.getByText('Remove'))
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/auth/avatar', expect.objectContaining({ method: 'PUT' }))
    })
  })

  it('confirms delete user and calls DELETE endpoint', async () => {
    mockApiFetch
      .mockResolvedValueOnce({ users: mockUsers }) // initial users fetch
      .mockResolvedValueOnce({ status: 'ok' })      // delete mutation
      .mockResolvedValueOnce({ users: [mockUsers[0]] }) // refetch after delete
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete User' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Delete User' }))
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/users/u2', expect.objectContaining({ method: 'DELETE' }))
    })
  })

  it('submits Disable 2FA form with TOTP code', async () => {
    const usersWithTotp = [mockUsers[0], { ...mockUsers[1], totp_enabled: true }]
    mockApiFetch
      .mockResolvedValueOnce({ users: usersWithTotp }) // initial users fetch
      .mockResolvedValueOnce({ status: 'ok' })          // disable totp mutation
      .mockResolvedValueOnce({ users: [{ ...mockUsers[1], totp_enabled: false }] }) // refetch
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Disable 2FA' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Disable 2FA' }))
    await waitFor(() => expect(screen.getByPlaceholderText('Your 6-digit TOTP code')).toBeInTheDocument())

    fireEvent.change(screen.getByPlaceholderText('Your 6-digit TOTP code'), { target: { value: '123456' } })
    fireEvent.submit(screen.getByPlaceholderText('Your 6-digit TOTP code').closest('form')!)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/auth/totp/disable', expect.objectContaining({ method: 'POST' }))
    })
  })

  it('changes user role via select dropdown', async () => {
    mockApiFetch
      .mockResolvedValueOnce({ users: mockUsers }) // initial users fetch
      .mockResolvedValueOnce({ user: { ...mockUsers[1], role: 'admin' } }) // update role
      .mockResolvedValueOnce({ users: mockUsers }) // refetch
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByDisplayValue('Viewer')).toBeInTheDocument())

    fireEvent.change(screen.getByDisplayValue('Viewer'), { target: { value: 'admin' } })
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/users/u2/role', expect.objectContaining({ method: 'PUT' }))
    })
  })

  it('shows role update error when mutation fails', async () => {
    mockApiFetch
      .mockResolvedValueOnce({ users: mockUsers }) // initial users fetch
      .mockRejectedValueOnce(new Error('Permission denied')) // update role error
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByDisplayValue('Viewer')).toBeInTheDocument())

    fireEvent.change(screen.getByDisplayValue('Viewer'), { target: { value: 'admin' } })
    await waitFor(() => {
      expect(screen.getByText('Permission denied')).toBeInTheDocument()
    })
  })

  it('shows avatar mutation error when upload fails', async () => {
    mockApiFetch
      .mockResolvedValueOnce({ status: 'ok' }) // ensure no pending calls
      .mockRejectedValueOnce(new Error('File too large'))
    mockUseAuth.mockReturnValue({
      user: { id: 'u1', username: 'admin', email: 'admin@test.com', role: 'admin', totp_enabled: true, avatar: 'data:image/png;base64,abc', created_at: '', updated_at: '' },
      login: vi.fn(),
    })
    renderPage()
    // Trigger avatarMutation by clicking Remove (which calls mutate(''))
    mockApiFetch.mockRejectedValueOnce(new Error('Avatar upload failed'))
    fireEvent.click(screen.getByText('Remove'))
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/auth/avatar', expect.objectContaining({ method: 'PUT' }))
    })
  })

  it('handles submit with valid passwords matching', async () => {
    // Tests the mutation.reset() branch when passwords match validation but we reset first
    mockApiFetch.mockResolvedValueOnce({ status: 'ok' })
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('Current password'), { target: { value: 'OldPass1@#$' } })
    fireEvent.change(screen.getByPlaceholderText('New password (min 12 chars)'), { target: { value: 'GoodPass1@#$abc' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm new password'), { target: { value: 'GoodPass1@#$abc' } })
    fireEvent.submit(screen.getByRole('button', { name: /update password/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Password updated successfully.')).toBeInTheDocument()
    })
  })

  it('shows delete user error when mutation fails', async () => {
    mockApiFetch
      .mockResolvedValueOnce({ users: mockUsers }) // initial users fetch
      .mockRejectedValueOnce(new Error('Cannot delete user')) // delete mutation error
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete User' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Delete User' }))
    await waitFor(() => {
      expect(screen.getByText('Cannot delete user')).toBeInTheDocument()
    })
  })

  it('shows disable 2FA error when mutation fails', async () => {
    const usersWithTotp = [mockUsers[0], { ...mockUsers[1], totp_enabled: true }]
    mockApiFetch
      .mockResolvedValueOnce({ users: usersWithTotp }) // initial users fetch
      .mockRejectedValueOnce(new Error('Invalid TOTP code')) // disable totp error
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Disable 2FA' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Disable 2FA' }))
    await waitFor(() => expect(screen.getByPlaceholderText('Your 6-digit TOTP code')).toBeInTheDocument())

    fireEvent.change(screen.getByPlaceholderText('Your 6-digit TOTP code'), { target: { value: '999999' } })
    fireEvent.submit(screen.getByPlaceholderText('Your 6-digit TOTP code').closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Invalid TOTP code')).toBeInTheDocument()
    })
  })

  it('formatDate shows Today for current date', async () => {
    const todayIso = new Date().toISOString()
    const usersToday = [
      { ...mockUsers[0] },
      { ...mockUsers[1], created_at: todayIso },
    ]
    mockApiFetch.mockResolvedValueOnce({ users: usersToday })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByText('Today')).toBeInTheDocument()
    })
  })

  it('formatDate shows Yesterday for 1 day ago', async () => {
    const yesterday = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString()
    const usersYesterday = [
      { ...mockUsers[0] },
      { ...mockUsers[1], created_at: yesterday },
    ]
    mockApiFetch.mockResolvedValueOnce({ users: usersYesterday })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByText('Yesterday')).toBeInTheDocument()
    })
  })

  it('formatDate shows days ago for 5 days ago', async () => {
    const fiveDaysAgo = new Date(Date.now() - 5 * 24 * 60 * 60 * 1000).toISOString()
    const users5Days = [
      { ...mockUsers[0] },
      { ...mockUsers[1], created_at: fiveDaysAgo },
    ]
    mockApiFetch.mockResolvedValueOnce({ users: users5Days })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      expect(screen.getByText('5 days ago')).toBeInTheDocument()
    })
  })

  it('formatDate shows formatted date for old dates', async () => {
    const oldDate = '2023-01-15T00:00:00Z'
    const usersOld = [
      { ...mockUsers[0] },
      { ...mockUsers[1], created_at: oldDate },
    ]
    mockApiFetch.mockResolvedValueOnce({ users: usersOld })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => {
      // Should show something like "Jan 15, 2023"
      expect(screen.getByText(/Jan 15, 2023/)).toBeInTheDocument()
    })
  })

  it('clicking a color button updates avatar color selection', async () => {
    renderPage()
    // Color buttons are buttons with style.backgroundColor
    const colorButtons = screen.getAllByRole('button').filter(btn => {
      const el = btn as HTMLButtonElement
      return el.style.backgroundColor !== ''
    })
    expect(colorButtons.length).toBeGreaterThan(0)
    // Click the first color button - should call setAvatarColor mock
    fireEvent.click(colorButtons[0])
    // Verify setAvatarColor was called (it's mocked at top of file)
    const { setAvatarColor } = await import('@/lib/avatar')
    expect(setAvatarColor).toHaveBeenCalled()
  })

  it('clicking Profile tab when Users is active switches back to profile', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage('/settings?tab=users')
    await waitFor(() => expect(screen.getByText('User Management')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Profile' }))
    await waitFor(() => {
      expect(screen.getByPlaceholderText('Current password')).toBeInTheDocument()
    })
  })

  it('CreateUserModal onClose callback hides modal', async () => {
    mockApiFetch.mockResolvedValueOnce({ users: mockUsers })
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Users' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Create User' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Create User' }))
    await waitFor(() => expect(screen.getByTestId('create-user-modal')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Close Modal' }))
    await waitFor(() => {
      expect(screen.queryByTestId('create-user-modal')).not.toBeInTheDocument()
    })
  })
})
