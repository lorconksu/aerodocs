import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { LoginPage } from '../login'

const mockNavigate = vi.fn()

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return { ...actual, useNavigate: () => mockNavigate }
})

vi.mock('@/lib/api', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '@/lib/api'
const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <LoginPage />
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

describe('LoginPage', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    mockNavigate.mockReset()
    // Default: system is initialized
    mockApiFetch.mockResolvedValueOnce({ initialized: true })
  })

  it('renders username and password inputs', () => {
    renderPage()
    expect(screen.getByPlaceholderText('username')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('password')).toBeInTheDocument()
  })

  it('renders the Sign In button', () => {
    renderPage()
    expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument()
  })

  it('redirects to /setup when system not initialized', async () => {
    mockApiFetch.mockReset()
    mockApiFetch.mockResolvedValueOnce({ initialized: false })
    renderPage()
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/setup', { replace: true })
    })
  })

  it('does not redirect when system is initialized', async () => {
    renderPage()
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/auth/status')
    })
    expect(mockNavigate).not.toHaveBeenCalledWith('/setup', expect.anything())
  })

  it('submits form and navigates to /login/totp when totp_token returned', async () => {
    mockApiFetch.mockResolvedValueOnce({ totp_token: 'totp-tok-123' })
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('username'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByPlaceholderText('password'), { target: { value: 'secret' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/login/totp', { state: { totpToken: 'totp-tok-123' } })
    })
  })

  it('navigates to /setup/totp when requires_totp_setup is true', async () => {
    mockApiFetch.mockResolvedValueOnce({ requires_totp_setup: true, setup_token: 'setup-tok' })
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('username'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByPlaceholderText('password'), { target: { value: 'secret' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/setup/totp', { state: { setupToken: 'setup-tok' } })
    })
  })

  it('shows error message when login fails', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Invalid credentials'))
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('username'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByPlaceholderText('password'), { target: { value: 'wrong' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Invalid credentials')).toBeInTheDocument()
    })
  })

  it('shows generic error when non-Error is thrown', async () => {
    mockApiFetch.mockRejectedValueOnce('oops')
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('username'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByPlaceholderText('password'), { target: { value: 'x' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Login failed')).toBeInTheDocument()
    })
  })

  it('shows loading state during submit', async () => {
    let resolve!: (val: unknown) => void
    mockApiFetch.mockReturnValueOnce(new Promise(r => { resolve = r }))
    renderPage()

    fireEvent.change(screen.getByPlaceholderText('username'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByPlaceholderText('password'), { target: { value: 'secret' } })
    fireEvent.submit(screen.getByRole('button', { name: /sign in/i }).closest('form')!)

    await waitFor(() => {
      expect(screen.getByText('Signing in...')).toBeInTheDocument()
    })
    resolve({ totp_token: 't' })
  })

  it('ignores status check errors', async () => {
    mockApiFetch.mockReset()
    mockApiFetch.mockRejectedValueOnce(new Error('Network error'))
    renderPage()
    // Should not throw or crash
    await waitFor(() => {
      expect(screen.getByPlaceholderText('username')).toBeInTheDocument()
    })
  })
})
