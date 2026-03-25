import { render, screen, waitFor, act } from '@testing-library/react'
import { vi } from 'vitest'
import { AuthProvider, useAuth } from '../use-auth'

// Mock API
vi.mock('@/lib/api', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '@/lib/api'
const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

// Helper component to expose context values
function TestConsumer() {
  const { user, isLoading, isAuthenticated, login, logout } = useAuth()
  return (
    <div>
      <div data-testid="loading">{String(isLoading)}</div>
      <div data-testid="authenticated">{String(isAuthenticated)}</div>
      <div data-testid="username">{user?.username ?? 'null'}</div>
      <button onClick={() => login('acc', 'ref', { id: '1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' })}>
        Login
      </button>
      <button onClick={logout}>Logout</button>
    </div>
  )
}

describe('AuthProvider', () => {
  beforeEach(() => {
    localStorage.clear()
    mockApiFetch.mockReset()
  })

  it('starts in loading state', () => {
    // No access token — resolves immediately
    render(<AuthProvider><TestConsumer /></AuthProvider>)
    // After effect: isLoading = false, no token
    // (effect runs synchronously in jsdom for the no-token path)
  })

  it('when no access token, sets isLoading=false and user=null', async () => {
    render(<AuthProvider><TestConsumer /></AuthProvider>)
    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
    expect(screen.getByTestId('username').textContent).toBe('null')
  })

  it('fetches user from /auth/me when access token exists', async () => {
    localStorage.setItem('aerodocs_access_token', 'valid-token')
    const mockUser = { id: '1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }
    mockApiFetch.mockResolvedValueOnce(mockUser)

    render(<AuthProvider><TestConsumer /></AuthProvider>)

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('username').textContent).toBe('admin')
    expect(screen.getByTestId('authenticated').textContent).toBe('true')
  })

  it('clears tokens and sets user=null when /auth/me fails', async () => {
    localStorage.setItem('aerodocs_access_token', 'bad-token')
    localStorage.setItem('aerodocs_refresh_token', 'bad-refresh')
    mockApiFetch.mockRejectedValueOnce(new Error('Unauthorized'))

    render(<AuthProvider><TestConsumer /></AuthProvider>)

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('username').textContent).toBe('null')
    expect(localStorage.getItem('aerodocs_access_token')).toBeNull()
  })

  it('login stores tokens and updates user', async () => {
    render(<AuthProvider><TestConsumer /></AuthProvider>)
    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })

    act(() => {
      screen.getByText('Login').click()
    })

    expect(screen.getByTestId('username').textContent).toBe('admin')
    expect(screen.getByTestId('authenticated').textContent).toBe('true')
    expect(localStorage.getItem('aerodocs_access_token')).toBe('acc')
    expect(localStorage.getItem('aerodocs_refresh_token')).toBe('ref')
  })

  it('logout clears tokens and sets user=null', async () => {
    localStorage.setItem('aerodocs_access_token', 'valid-token')
    const mockUser = { id: '1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }
    mockApiFetch.mockResolvedValueOnce(mockUser)

    render(<AuthProvider><TestConsumer /></AuthProvider>)
    await waitFor(() => {
      expect(screen.getByTestId('username').textContent).toBe('admin')
    })

    act(() => {
      screen.getByText('Logout').click()
    })

    expect(screen.getByTestId('username').textContent).toBe('null')
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
  })
})

describe('useAuth outside AuthProvider', () => {
  it('throws an error when used outside AuthProvider', () => {
    // Suppress console.error for this test
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<TestConsumer />)).toThrow('useAuth must be used within AuthProvider')
    spy.mockRestore()
  })
})
