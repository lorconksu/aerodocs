import { render, screen, waitFor, act } from '@testing-library/react'
import { vi } from 'vitest'
import { AuthProvider, useAuth } from '../use-auth'

// Mock auth
vi.mock('@/lib/auth', () => ({
  clearTokens: vi.fn().mockResolvedValue(undefined),
}))

import { clearTokens } from '@/lib/auth'
const mockClearTokens = clearTokens as ReturnType<typeof vi.fn>

// Helper component to expose context values
function TestConsumer() {
  const { user, isLoading, isAuthenticated, login, logout } = useAuth()
  return (
    <div>
      <div data-testid="loading">{String(isLoading)}</div>
      <div data-testid="authenticated">{String(isAuthenticated)}</div>
      <div data-testid="username">{user?.username ?? 'null'}</div>
      <button onClick={() => login({ id: '1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' })}>
        Login
      </button>
      <button onClick={logout}>Logout</button>
    </div>
  )
}

describe('AuthProvider', () => {
  beforeEach(() => {
    mockClearTokens.mockReset()
    mockClearTokens.mockResolvedValue(undefined)
    vi.restoreAllMocks()
  })

  it('starts in loading state and fetches /auth/me on mount', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(null, { status: 401 }))
    render(<AuthProvider><TestConsumer /></AuthProvider>)
    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
    expect(globalThis.fetch).toHaveBeenCalledWith('/api/auth/me', { credentials: 'same-origin' })
  })

  it('sets user when /auth/me succeeds (cookie is valid)', async () => {
    const mockUser = { id: '1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify(mockUser), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )

    render(<AuthProvider><TestConsumer /></AuthProvider>)

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('username').textContent).toBe('admin')
    expect(screen.getByTestId('authenticated').textContent).toBe('true')
  })

  it('sets user=null when /auth/me fails (no valid cookie)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(null, { status: 401 }))

    render(<AuthProvider><TestConsumer /></AuthProvider>)

    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })
    expect(screen.getByTestId('username').textContent).toBe('null')
  })

  it('login updates user state', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(new Response(null, { status: 401 }))
    render(<AuthProvider><TestConsumer /></AuthProvider>)
    await waitFor(() => {
      expect(screen.getByTestId('loading').textContent).toBe('false')
    })

    act(() => {
      screen.getByText('Login').click()
    })

    expect(screen.getByTestId('username').textContent).toBe('admin')
    expect(screen.getByTestId('authenticated').textContent).toBe('true')
  })

  it('logout calls clearTokens and sets user=null', async () => {
    const mockUser = { id: '1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }
    vi.spyOn(globalThis, 'fetch').mockResolvedValueOnce(
      new Response(JSON.stringify(mockUser), { status: 200, headers: { 'Content-Type': 'application/json' } })
    )

    render(<AuthProvider><TestConsumer /></AuthProvider>)
    await waitFor(() => {
      expect(screen.getByTestId('username').textContent).toBe('admin')
    })

    await act(async () => {
      screen.getByText('Logout').click()
    })

    expect(mockClearTokens).toHaveBeenCalled()
    expect(screen.getByTestId('username').textContent).toBe('null')
    expect(screen.getByTestId('authenticated').textContent).toBe('false')
  })
})

describe('useAuth outside AuthProvider', () => {
  it('throws an error when used outside AuthProvider', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<TestConsumer />)).toThrow('useAuth must be used within AuthProvider')
    spy.mockRestore()
  })
})
