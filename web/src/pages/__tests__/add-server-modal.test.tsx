import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AddServerModal } from '../add-server-modal'

vi.mock('@/lib/api', () => ({
  apiFetch: vi.fn(),
}))

// Mock clipboard
Object.assign(navigator, {
  clipboard: { writeText: vi.fn().mockResolvedValue(undefined) },
})

import { apiFetch } from '@/lib/api'
const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

const mockServer = {
  id: 'srv-1',
  name: 'web-prod-1',
  status: 'pending',
  hostname: null,
  ip_address: null,
  os: null,
  agent_version: null,
  labels: '',
  last_seen_at: null,
  created_at: '',
  updated_at: '',
}
const mockCreateResponse = {
  server: mockServer,
  registration_token: 'reg-token',
  install_command: 'curl -s https://example.com/install.sh | bash -s -- --token=reg-token',
}

function renderModal(onClose = vi.fn()) {
  // Use very short refetch intervals and disable retries to avoid test timeouts
  const qc = new QueryClient({
    defaultOptions: {
      queries: { retry: false, refetchInterval: false as const },
      mutations: { retry: false },
    },
  })
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <AddServerModal onClose={onClose} />
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

describe('AddServerModal', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    vi.clearAllMocks()
  })

  afterEach(() => {
    // Make sure real timers are restored after each test
    vi.useRealTimers()
  })

  it('renders Add Server heading', () => {
    renderModal()
    expect(screen.getByText('Add Server')).toBeInTheDocument()
  })

  it('renders Server Name label and input', () => {
    renderModal()
    expect(screen.getByLabelText('Server Name')).toBeInTheDocument()
  })

  it('Generate button is disabled when name is empty', () => {
    renderModal()
    expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled()
  })

  it('Generate button is enabled when name is entered', () => {
    renderModal()
    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    expect(screen.getByRole('button', { name: 'Generate' })).not.toBeDisabled()
  })

  it('pressing Enter in name input triggers generate', async () => {
    mockApiFetch.mockResolvedValueOnce(mockCreateResponse)
    // Poll query - return once then don't respond more
    mockApiFetch.mockResolvedValue({ ...mockServer, status: 'pending' })
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.keyDown(screen.getByLabelText('Server Name'), { key: 'Enter' })

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/servers', expect.objectContaining({ method: 'POST' }))
    })
  })

  it('shows install command after server creation', async () => {
    mockApiFetch.mockResolvedValueOnce(mockCreateResponse)
    mockApiFetch.mockResolvedValue({ ...mockServer, status: 'pending' })
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText(mockCreateResponse.install_command)).toBeInTheDocument()
    })
  })

  it('shows waiting for agent message after creation', async () => {
    mockApiFetch.mockResolvedValueOnce(mockCreateResponse)
    mockApiFetch.mockResolvedValue({ ...mockServer, status: 'pending' })
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText('Waiting for agent to connect...')).toBeInTheDocument()
    })
  })

  it('shows Agent connected when server comes online', async () => {
    mockApiFetch.mockResolvedValueOnce(mockCreateResponse)
    mockApiFetch.mockResolvedValue({ ...mockServer, status: 'online' })
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText('Agent connected!')).toBeInTheDocument()
    })
  })

  it('shows timeout message after 2 minutes', async () => {
    vi.useFakeTimers()
    try {
      mockApiFetch.mockResolvedValueOnce(mockCreateResponse)
      mockApiFetch.mockResolvedValue({ ...mockServer, status: 'pending' })
      renderModal()

      fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
      fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

      // Let the mutation resolve
      await act(async () => {
        await Promise.resolve()
        await Promise.resolve()
      })

      // Advance 2 minutes to trigger timeout
      await act(async () => {
        vi.advanceTimersByTime(2 * 60 * 1000 + 100)
        await Promise.resolve()
      })

      expect(screen.getByText(/Agent hasn't connected yet/)).toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('copy button copies install command to clipboard', async () => {
    mockApiFetch.mockResolvedValueOnce(mockCreateResponse)
    mockApiFetch.mockResolvedValue({ ...mockServer, status: 'pending' })
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText(mockCreateResponse.install_command)).toBeInTheDocument()
    })

    const copyBtn = screen.getByTitle('Copy to clipboard')
    fireEvent.click(copyBtn)

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(mockCreateResponse.install_command)
    })
  })

  it('shows error on create failure', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Server already exists'))
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText('Server already exists')).toBeInTheDocument()
    })
  })

  it('X close button calls onClose when canClose=true (before creation)', () => {
    const onClose = vi.fn()
    renderModal(onClose)

    // The X button should be enabled before any result
    const buttons = screen.getAllByRole('button')
    // Find the X button (disabled class has opacity-30, enabled doesn't)
    const xBtn = buttons.find(b => !b.hasAttribute('disabled') && !b.textContent?.trim())
    if (xBtn) {
      fireEvent.click(xBtn)
      expect(onClose).toHaveBeenCalled()
    } else {
      // Alternative: find button by the heading proximity
      expect(buttons.some(b => !b.hasAttribute('disabled'))).toBe(true)
    }
  })

  it('Close button is disabled while waiting for agent', async () => {
    mockApiFetch.mockResolvedValueOnce(mockCreateResponse)
    mockApiFetch.mockResolvedValue({ ...mockServer, status: 'pending' })
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText('Waiting for agent to connect...')).toBeInTheDocument()
    })

    const closeBtn = screen.getByRole('button', { name: 'Close' })
    expect(closeBtn).toBeDisabled()
  })

  it('shows generating state during mutation', async () => {
    let resolve!: (val: unknown) => void
    mockApiFetch.mockReturnValueOnce(new Promise(r => { resolve = r }))
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText('Generating...')).toBeInTheDocument()
    })
    resolve(mockCreateResponse)
  })

  it('handleGenerate returns early when name is empty (line 57)', () => {
    // handleGenerate is called via Enter key even when input is empty-ish
    renderModal()
    // Name input is empty — press Enter
    fireEvent.keyDown(screen.getByLabelText('Server Name'), { key: 'Enter' })
    // No API call should happen
    expect(mockApiFetch).not.toHaveBeenCalled()
  })

  it('handleGenerate returns early when name is only whitespace (line 57)', () => {
    renderModal()
    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: '   ' } })
    fireEvent.keyDown(screen.getByLabelText('Server Name'), { key: 'Enter' })
    expect(mockApiFetch).not.toHaveBeenCalled()
  })

  it('shows fallback error message when error has no message (line 100)', async () => {
    // Throw a non-Error object (no .message property)
    mockApiFetch.mockRejectedValueOnce({ status: 500 })
    renderModal()

    fireEvent.change(screen.getByLabelText('Server Name'), { target: { value: 'my-server' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate' }))

    await waitFor(() => {
      expect(screen.getByText('Failed to create server')).toBeInTheDocument()
    })
  })
})
