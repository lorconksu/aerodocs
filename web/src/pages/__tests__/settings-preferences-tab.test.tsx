import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { PreferencesTab } from '../settings-preferences-tab'

vi.mock('@/lib/api', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '@/lib/api'
const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

const mockPreferences = [
  { event_type: 'agent.offline', label: 'Agent Offline', category: 'Agent', enabled: true },
  { event_type: 'agent.online', label: 'Agent Online', category: 'Agent', enabled: false },
  { event_type: 'user.login', label: 'User Login', category: 'Security', enabled: true },
  { event_type: 'user.created', label: 'User Created', category: 'Security', enabled: false },
  { event_type: 'smtp.error', label: 'SMTP Error', category: 'System', enabled: true },
]

function renderTab() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <PreferencesTab />
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

describe('PreferencesTab', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    mockApiFetch.mockResolvedValue({ preferences: mockPreferences })
  })

  it('renders heading', () => {
    renderTab()
    expect(screen.getByText('Email Notification Preferences')).toBeInTheDocument()
  })

  it('shows loading state initially', () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}))
    renderTab()
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('renders preferences grouped by category', async () => {
    renderTab()
    await waitFor(() => {
      expect(screen.getByText('Agent')).toBeInTheDocument()
      expect(screen.getByText('Security')).toBeInTheDocument()
      expect(screen.getByText('System')).toBeInTheDocument()
    })
  })

  it('renders preference labels', async () => {
    renderTab()
    await waitFor(() => {
      expect(screen.getByText('Agent Offline')).toBeInTheDocument()
      expect(screen.getByText('Agent Online')).toBeInTheDocument()
      expect(screen.getByText('User Login')).toBeInTheDocument()
      expect(screen.getByText('User Created')).toBeInTheDocument()
      expect(screen.getByText('SMTP Error')).toBeInTheDocument()
    })
  })

  it('renders event type in mono', async () => {
    renderTab()
    await waitFor(() => {
      expect(screen.getByText('agent.offline')).toBeInTheDocument()
    })
  })

  it('renders checkboxes for each preference', async () => {
    renderTab()
    await waitFor(() => {
      const checkboxes = screen.getAllByRole('checkbox')
      expect(checkboxes).toHaveLength(5)
    })
  })

  it('renders enabled preferences as checked', async () => {
    renderTab()
    await waitFor(() => {
      const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[]
      // agent.offline is enabled, agent.online is not — but order depends on rendering
      const enabledCount = checkboxes.filter(c => c.checked).length
      expect(enabledCount).toBe(3) // agent.offline, user.login, smtp.error
    })
  })

  it('toggling a checkbox updates local state', async () => {
    renderTab()
    await waitFor(() => {
      expect(screen.getByText('Agent Offline')).toBeInTheDocument()
    })

    const checkboxes = screen.getAllByRole('checkbox') as HTMLInputElement[]
    // Find the agent.offline checkbox (should be checked)
    const agentOfflineCheckbox = checkboxes.find(c => c.checked)
    expect(agentOfflineCheckbox).toBeTruthy()

    fireEvent.click(agentOfflineCheckbox!)

    await waitFor(() => {
      expect(agentOfflineCheckbox!.checked).toBe(false)
    })
  })

  it('renders Save Preferences button', async () => {
    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save Preferences' })).toBeInTheDocument()
    })
  })

  it('calls PUT /notifications/preferences on save', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ preferences: mockPreferences })
      .mockResolvedValueOnce({ status: 'ok' })

    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save Preferences' })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save Preferences' }))

    await waitFor(() => {
      const calls = mockApiFetch.mock.calls
      const putCall = calls.find(c => c[0] === '/notifications/preferences' && c[1]?.method === 'PUT')
      expect(putCall).toBeTruthy()
    })
  })

  it('PUT payload includes event_type and enabled fields', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ preferences: mockPreferences })
      .mockResolvedValueOnce({ status: 'ok' })

    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save Preferences' })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save Preferences' }))

    await waitFor(() => {
      const calls = mockApiFetch.mock.calls
      const putCall = calls.find(c => c[0] === '/notifications/preferences' && c[1]?.method === 'PUT')
      expect(putCall).toBeTruthy()
      const body = JSON.parse(putCall![1].body)
      expect(body.preferences).toBeInstanceOf(Array)
      expect(body.preferences[0]).toHaveProperty('event_type')
      expect(body.preferences[0]).toHaveProperty('enabled')
    })
  })

  it('shows success message after save', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ preferences: mockPreferences })
      .mockResolvedValueOnce({ status: 'ok' })

    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save Preferences' })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save Preferences' }))

    await waitFor(() => {
      expect(screen.getByText('Preferences saved.')).toBeInTheDocument()
    })
  })

  it('shows error message when save fails', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ preferences: mockPreferences })
      .mockRejectedValueOnce(new Error('Server error'))

    renderTab()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save Preferences' })).toBeInTheDocument()
    })

    fireEvent.click(screen.getByRole('button', { name: 'Save Preferences' }))

    await waitFor(() => {
      expect(screen.getByText('Server error')).toBeInTheDocument()
    })
  })

  it('shows empty state when no preferences', async () => {
    mockApiFetch.mockReset()
    mockApiFetch.mockResolvedValue({ preferences: [] })

    renderTab()
    await waitFor(() => {
      expect(screen.getByText('No notification preferences available.')).toBeInTheDocument()
    })
  })
})
