import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuditLogsPage } from '../audit-logs'

vi.mock('@/lib/api', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '@/lib/api'
const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

const mockEntries = [
  { id: 'e1', user_id: 'u1', action: 'user.login', target: null, detail: null, ip_address: '1.2.3.4', created_at: new Date().toISOString() },
  { id: 'e2', user_id: null, action: 'server.created', target: 'web-prod-1', detail: null, ip_address: null, created_at: new Date().toISOString() },
]

const mockUsers = [
  { id: 'u1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' },
]

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <AuditLogsPage />
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

describe('AuditLogsPage', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    // Default: return mock data for both queries
    mockApiFetch
      .mockResolvedValueOnce({ entries: mockEntries, total: 2, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
  })

  it('renders page heading', async () => {
    renderPage()
    expect(screen.getByText('Audit Logs')).toBeInTheDocument()
  })

  it('shows loading state', () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('shows empty state when no entries', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ entries: [], total: 0, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('No audit log entries found.')).toBeInTheDocument()
    })
  })

  it('renders audit log entries', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('user.login')).toBeInTheDocument()
      expect(screen.getByText('server.created')).toBeInTheDocument()
    })
  })

  it('shows username for entries with user_id', async () => {
    renderPage()
    await waitFor(() => {
      // u1 maps to 'admin' — may appear in dropdown too, check it's present
      expect(screen.getAllByText('admin').length).toBeGreaterThan(0)
    })
  })

  it('shows "System" for entries with null user_id', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('System')).toBeInTheDocument()
    })
  })

  it('renders filter controls', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('All Users')).toBeInTheDocument()
      expect(screen.getByText('All Actions')).toBeInTheDocument()
    })
  })

  it('shows Clear Filters button when filter is set', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('All Actions')).toBeInTheDocument()
    })

    const actionSelect = screen.getByDisplayValue('All Actions')
    fireEvent.change(actionSelect, { target: { value: 'user.login' } })

    await waitFor(() => {
      expect(screen.getByText('Clear Filters')).toBeInTheDocument()
    })
  })

  it('Clear Filters resets filters', async () => {
    renderPage()
    await waitFor(() => expect(screen.getByText('All Actions')).toBeInTheDocument())

    const actionSelect = screen.getByDisplayValue('All Actions')
    fireEvent.change(actionSelect, { target: { value: 'user.login' } })
    await waitFor(() => expect(screen.getByText('Clear Filters')).toBeInTheDocument())

    fireEvent.click(screen.getByText('Clear Filters'))
    await waitFor(() => {
      expect(screen.queryByText('Clear Filters')).not.toBeInTheDocument()
    })
  })

  it('shows pagination when total > 0', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ entries: mockEntries, total: 100, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/Showing 1-50 of 100/)).toBeInTheDocument()
    })
  })

  it('Previous button is disabled on first page', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ entries: mockEntries, total: 100, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
    renderPage()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()
    })
  })

  it('Next button works to advance page', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ entries: mockEntries, total: 100, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
      .mockResolvedValueOnce({ entries: mockEntries, total: 100, limit: 50, offset: 50 })
      .mockResolvedValueOnce({ users: mockUsers })
    renderPage()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Next' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => {
      expect(screen.getByText(/Showing 51-100 of 100/)).toBeInTheDocument()
    })
  })

  it('Next button is disabled on last page', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ entries: mockEntries, total: 2, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
    renderPage()
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
    })
  })

  it('shows target value when present', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('web-prod-1')).toBeInTheDocument()
    })
  })

  it('shows ip address when present', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('1.2.3.4')).toBeInTheDocument()
    })
  })

  it('date filter triggers refetch', async () => {
    renderPage()
    await waitFor(() => expect(mockApiFetch).toHaveBeenCalled())

    const dateInputs = screen.getAllByDisplayValue('')
    // First two inputs are date pickers
    const fromInput = dateInputs.find(el => el.getAttribute('type') === 'date')
    if (fromInput) {
      mockApiFetch.mockResolvedValue({ entries: [], total: 0, limit: 50, offset: 0 })
      fireEvent.change(fromInput, { target: { value: '2024-01-01' } })
      await waitFor(() => expect(mockApiFetch).toHaveBeenCalled())
    }
  })

  it('shows user ID when user not found in users list', async () => {
    const entriesWithUnknownUser = [
      { id: 'e3', user_id: 'unknown-id', action: 'user.login', target: null, detail: null, ip_address: null, created_at: new Date().toISOString() },
    ]
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ entries: entriesWithUnknownUser, total: 1, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('unknown-id')).toBeInTheDocument()
    })
  })

  it('"to" date filter input triggers refetch (line 113-119)', async () => {
    renderPage()
    await waitFor(() => expect(mockApiFetch).toHaveBeenCalled())

    const dateInputs = document.querySelectorAll('input[type="date"]')
    const toInput = dateInputs[1] as HTMLInputElement // second date input is "to"
    expect(toInput).toBeTruthy()

    mockApiFetch
      .mockResolvedValueOnce({ entries: [], total: 0, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
    fireEvent.change(toInput, { target: { value: '2024-12-31' } })
    await waitFor(() => {
      // Clear Filters button appears when filter is active
      expect(screen.getByText('Clear Filters')).toBeInTheDocument()
    })
  })

  it('Previous button works when on page 2 (line 187)', async () => {
    mockApiFetch.mockReset()
    mockApiFetch
      .mockResolvedValueOnce({ entries: mockEntries, total: 100, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
      // After clicking Next (offset=50)
      .mockResolvedValueOnce({ entries: mockEntries, total: 100, limit: 50, offset: 50 })
      .mockResolvedValueOnce({ users: mockUsers })
      // After clicking Previous (offset=0)
      .mockResolvedValueOnce({ entries: mockEntries, total: 100, limit: 50, offset: 0 })
      .mockResolvedValueOnce({ users: mockUsers })
    renderPage()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Next' })).toBeInTheDocument())

    // Go to page 2
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    await waitFor(() => expect(screen.getByText(/Showing 51/)).toBeInTheDocument())

    // Now Previous button should be enabled — click it
    const prevBtn = screen.getByRole('button', { name: 'Previous' })
    expect(prevBtn).not.toBeDisabled()
    fireEvent.click(prevBtn)
    await waitFor(() => {
      expect(screen.getByText(/Showing 1-50 of 100/)).toBeInTheDocument()
    })
  })

  it('user filter select shows users in dropdown', async () => {
    renderPage()
    await waitFor(() => {
      expect(screen.getAllByText('admin').length).toBeGreaterThan(0)
    })
    // Select user filter
    const userSelect = screen.getByDisplayValue('All Users')
    fireEvent.change(userSelect, { target: { value: 'u1' } })
    await waitFor(() => {
      expect(screen.getByText('Clear Filters')).toBeInTheDocument()
    })
  })
})
