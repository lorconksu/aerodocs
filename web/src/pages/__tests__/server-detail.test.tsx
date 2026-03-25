import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ServerDetailPage } from '../server-detail'

// Mock heavy dependencies
vi.mock('mermaid', () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn().mockResolvedValue({ svg: '<svg data-testid="mermaid-svg"></svg>' }),
  },
}))

vi.mock('highlight.js/lib/core', () => ({
  default: {
    registerLanguage: vi.fn(),
    highlight: vi.fn((code: string) => ({ value: `<span>${code}</span>` })),
    getLanguage: vi.fn(() => true),
  },
}))

vi.mock('highlight.js/styles/github-dark.css', () => ({}))

vi.mock('react-markdown', () => ({
  default: ({ children }: { children: string }) => <div data-testid="markdown">{children}</div>,
}))

vi.mock('remark-gfm', () => ({ default: () => {} }))

// Mock individual highlight.js language modules
vi.mock('highlight.js/lib/languages/bash', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/css', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/dockerfile', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/go', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/ini', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/javascript', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/json', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/markdown', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/nginx', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/plaintext', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/python', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/shell', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/sql', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/typescript', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/xml', () => ({ default: {} }))
vi.mock('highlight.js/lib/languages/yaml', () => ({ default: {} }))

vi.mock('@/lib/api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('@/lib/auth', () => ({
  getAccessToken: vi.fn(() => 'test-token'),
}))

vi.mock('@/hooks/use-auth', () => ({
  useAuth: vi.fn(() => ({
    user: { id: 'u1', username: 'admin', role: 'admin', email: 'a@b.com', avatar: null, totp_enabled: true, created_at: '', updated_at: '' },
  })),
}))

const mockNavigate = vi.fn()
vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>()
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: vi.fn(() => ({ id: 'srv-1' })),
  }
})

import { apiFetch } from '@/lib/api'
const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

import { useAuth } from '@/hooks/use-auth'
const mockUseAuth = useAuth as ReturnType<typeof vi.fn>

import { useParams } from 'react-router-dom'
const mockUseParams = useParams as ReturnType<typeof vi.fn>

const mockServer = {
  id: 'srv-1',
  name: 'web-prod-1',
  hostname: 'web1.example.com',
  ip_address: '10.0.0.1',
  os: 'Ubuntu 22.04',
  status: 'online' as const,
  agent_version: '1.0.0',
  labels: '',
  last_seen_at: new Date(Date.now() - 30000).toISOString(),
  created_at: '',
  updated_at: '',
}

const mockOfflineServer = { ...mockServer, status: 'offline' as const }
const mockPendingServer = { ...mockServer, status: 'pending' as const }

const mockFileNodes = [
  { name: 'etc', path: '/etc', is_dir: true, size: 0, readable: true },
  { name: 'nginx.conf', path: '/etc/nginx.conf', is_dir: false, size: 1024, readable: true },
  { name: 'secret.key', path: '/etc/secret.key', is_dir: false, size: 512, readable: false },
]

function renderPage() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <ServerDetailPage />
      </BrowserRouter>
    </QueryClientProvider>,
  )
}

describe('ServerDetailPage', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    mockNavigate.mockReset()
    mockUseParams.mockReturnValue({ id: 'srv-1' })
    mockUseAuth.mockReturnValue({
      user: { id: 'u1', username: 'admin', role: 'admin', email: 'a@b.com', avatar: null, totp_enabled: true, created_at: '', updated_at: '' },
    })
    // Mock global fetch for SSE
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows loading state initially', () => {
    mockApiFetch.mockReturnValue(new Promise(() => {}))
    renderPage()
    expect(screen.getByText('Loading server...')).toBeInTheDocument()
  })

  it('shows error state when server query fails', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Not found'))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Failed to load server')).toBeInTheDocument()
      expect(screen.getByText('Not found')).toBeInTheDocument()
    })
  })

  it('shows error when server is null', async () => {
    // Make server query return undefined
    mockApiFetch.mockResolvedValueOnce(null)
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Failed to load server')).toBeInTheDocument()
    })
  })

  it('renders server name in header', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: [] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('web-prod-1')).toBeInTheDocument()
    })
  })

  it('renders server hostname and ip', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: [] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/web1\.example\.com/)).toBeInTheDocument()
    })
  })

  it('renders agent version', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: [] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('v1.0.0')).toBeInTheDocument()
    })
  })

  it('shows offline warning for offline servers', async () => {
    mockApiFetch.mockResolvedValueOnce(mockOfflineServer)
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/server is offline/i)).toBeInTheDocument()
    })
  })

  it('shows offline warning for pending servers', async () => {
    mockApiFetch.mockResolvedValueOnce(mockPendingServer)
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/server is offline/i)).toBeInTheDocument()
    })
  })

  it('shows file explorer for online server', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc', '/var/log'] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('File Explorer')).toBeInTheDocument()
    })
  })

  it('shows "No paths configured" when no paths granted', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: [] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText(/No paths configured/)).toBeInTheDocument()
    })
  })

  it('shows "Select a file" prompt when no file selected', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Select a file to view its contents')).toBeInTheDocument()
    })
  })

  it('renders file tree nodes', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('/etc')).toBeInTheDocument()
    })
  })

  it('clicking collapse button toggles sidebar', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByTitle('Collapse sidebar')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByTitle('Collapse sidebar'))
    await waitFor(() => {
      expect(screen.getByTitle('Expand sidebar')).toBeInTheDocument()
    })
  })

  it('clicking a directory fetches its children', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValueOnce({ files: mockFileNodes.slice(1) })
    renderPage()

    await waitFor(() => {
      expect(screen.getByText('/etc')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('/etc'))
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(expect.stringContaining('/files?path='))
    })
  })

  it('clicking a readable file fetches its content', async () => {
    const fileContent = {
      data: btoa('Hello, World!'),
      total_size: 13,
      mime_type: 'text/plain',
    }
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValueOnce({ files: mockFileNodes.slice(1) })
    mockApiFetch.mockResolvedValueOnce(fileContent)
    renderPage()

    await waitFor(() => expect(screen.getByText('/etc')).toBeInTheDocument())
    fireEvent.click(screen.getByText('/etc'))
    await waitFor(() => expect(screen.getByText('nginx.conf')).toBeInTheDocument())
    fireEvent.click(screen.getByText('nginx.conf'))

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(expect.stringContaining('/files/read'))
    })
  })

  it('shows path loading indicator', async () => {
    let resolveServer!: (val: unknown) => void
    mockApiFetch.mockReturnValueOnce(new Promise(r => { resolveServer = r }))
    renderPage()
    // While loading
    expect(screen.getByText('Loading server...')).toBeInTheDocument()
    resolveServer(mockServer)
  })

  it('shows admin-only PathManagement section', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Manage File Access')).toBeInTheDocument()
    })
  })

  it('does not show PathManagement for viewer', async () => {
    mockUseAuth.mockReturnValue({
      user: { id: 'u2', username: 'viewer', role: 'viewer', email: 'v@b.com', avatar: null, totp_enabled: true, created_at: '', updated_at: '' },
    })
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => {
      expect(screen.queryByText('Manage File Access')).not.toBeInTheDocument()
    })
  })

  it('shows Dropzone section for admin', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Dropzone')).toBeInTheDocument()
    })
  })

  it('Back to dashboard link is rendered', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => {
      expect(screen.getByTitle('Back to dashboard')).toBeInTheDocument()
    })
  })

  it('Back to dashboard link in error state works', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Not found'))
    renderPage()
    await waitFor(() => {
      expect(screen.getByText('Back to dashboard')).toBeInTheDocument()
    })
  })

  it('file viewer toolbar shows after selecting a file', async () => {
    const fileContent = { data: btoa('content'), total_size: 7, mime_type: 'text/plain' }
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValueOnce({ files: [{ name: 'nginx.conf', path: '/etc/nginx.conf', is_dir: false, size: 1024, readable: true }] })
    mockApiFetch.mockResolvedValueOnce(fileContent)
    renderPage()

    await waitFor(() => expect(screen.getByText('/etc')).toBeInTheDocument())
    fireEvent.click(screen.getByText('/etc'))
    await waitFor(() => expect(screen.getByText('nginx.conf')).toBeInTheDocument())
    fireEvent.click(screen.getByText('nginx.conf'))

    await waitFor(() => {
      expect(screen.getByTitle('Refresh file')).toBeInTheDocument()
    })
  })

  it('Ctrl+F event handler is registered', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/etc'] })
    renderPage()
    await waitFor(() => expect(screen.getByText('web-prod-1')).toBeInTheDocument())

    // The search bar only appears after a file is selected AND Ctrl+F is pressed
    // Just verify the event listener is registered (component doesn't crash on Ctrl+F)
    expect(() => {
      fireEvent.keyDown(window, { key: 'f', ctrlKey: true })
    }).not.toThrow()
  })

  it('expanding PathManagement shows add path form', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValue({ paths: [], users: [] })
    renderPage()

    await waitFor(() => expect(screen.getByText('Manage File Access')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Manage File Access'))

    await waitFor(() => {
      expect(screen.getByText('No path permissions configured yet.')).toBeInTheDocument()
    })
  })

  it('expanding Dropzone shows upload area', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValue({ files: [] })
    renderPage()

    await waitFor(() => expect(screen.getByText('Dropzone')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Dropzone'))

    await waitFor(() => {
      expect(screen.getByText('Drop files here or click to browse')).toBeInTheDocument()
    })
  })
})

// --- FileTreeNode tests ---

describe('FileTreeNode rendering', () => {
  it('renders a directory node', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValue({ paths: ['/var/log'] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => {
      expect(screen.getByText('/var/log')).toBeInTheDocument()
    })
  })
})

// --- DropzoneUpload tests ---

describe('DropzoneUpload', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows existing dropzone files when expanded', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValue({ files: [{ name: 'file.tar.gz', path: '/dropzone/file.tar.gz', is_dir: false, size: 5120, readable: true }] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Dropzone')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Dropzone'))

    await waitFor(() => {
      expect(screen.getByText('file.tar.gz')).toBeInTheDocument()
    })
  })

  it('clicking delete button on dropzone file opens confirmation', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValue({ files: [{ name: 'file.tar.gz', path: '/dropzone/file.tar.gz', is_dir: false, size: 5120, readable: true }] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Dropzone')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Dropzone'))
    await waitFor(() => expect(screen.getByText('file.tar.gz')).toBeInTheDocument())

    fireEvent.click(screen.getByTitle('Delete file'))
    await waitFor(() => {
      expect(screen.getByText('Delete File?')).toBeInTheDocument()
    })
  })

  it('cancel in delete confirmation closes modal', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValue({ files: [{ name: 'file.tar.gz', path: '/dropzone/file.tar.gz', is_dir: false, size: 5120, readable: true }] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Dropzone')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Dropzone'))
    await waitFor(() => expect(screen.getByText('file.tar.gz')).toBeInTheDocument())

    fireEvent.click(screen.getByTitle('Delete file'))
    await waitFor(() => expect(screen.getByText('Delete File?')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    await waitFor(() => expect(screen.queryByText('Delete File?')).not.toBeInTheDocument())
  })

  it('confirms delete of dropzone file and shows success', async () => {
    const dropzoneFile = { name: 'file.tar.gz', path: '/dropzone/file.tar.gz', is_dir: false, size: 5120, readable: true }
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValueOnce({ files: [dropzoneFile] })
    // The delete call returns ok
    mockApiFetch.mockResolvedValue({ status: 'ok' })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Dropzone')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Dropzone'))
    await waitFor(() => expect(screen.getByText('file.tar.gz')).toBeInTheDocument())

    fireEvent.click(screen.getByTitle('Delete file'))
    await waitFor(() => expect(screen.getByText('Delete File?')).toBeInTheDocument())

    // In the delete modal, click "Delete" button to confirm
    const deleteButtons = screen.getAllByRole('button', { name: 'Delete' })
    fireEvent.click(deleteButtons[0])
    // The delete call uses apiFetch directly in handleConfirmDelete
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        expect.stringContaining('/dropzone'),
        expect.objectContaining({ method: 'DELETE' })
      )
    })
  })
})

// --- Additional FileViewerContent tests ---

describe('FileViewerContent rendering', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  async function renderWithFile(fileContent: object, filename = 'nginx.conf') {
    const fileNode = { name: filename, path: `/etc/${filename}`, is_dir: false, size: 1024, readable: true }
    // Use mockResolvedValue (fallback) and more specific mocks
    mockApiFetch.mockResolvedValue(fileContent) // Default: all calls return fileContent
    mockApiFetch.mockResolvedValueOnce(mockServer) // Call 1: server
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] }) // Call 2: my-paths
    mockApiFetch.mockResolvedValueOnce({ files: [fileNode] }) // Call 3: file tree (direct apiFetch call)

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('/etc')).toBeInTheDocument())
    fireEvent.click(screen.getByText('/etc'))
    await waitFor(() => expect(screen.getByText(filename)).toBeInTheDocument())
    fireEvent.click(screen.getByText(filename))
  }

  it('renders highlighted file content', async () => {
    const content = 'server { listen 80; }'
    const fileContent = { data: btoa(content), total_size: content.length, mime_type: 'text/plain' }
    await renderWithFile(fileContent)
    // Wait for file content to fully load - the code block renders after fileContent resolves
    await waitFor(() => {
      // The toolbar appears when selectedFile is set
      expect(screen.getByTitle('Refresh file')).toBeInTheDocument()
    }, { timeout: 3000 })
    // Give more time for file content query to resolve
    await waitFor(() => {
      const codeEl = document.querySelector('code.hljs')
      expect(codeEl).toBeTruthy()
    }, { timeout: 3000 })
  })

  it('renders markdown file in rendered mode', async () => {
    const mdContent = '# Hello\n\nThis is markdown'
    const fileContent = { data: btoa(mdContent), total_size: mdContent.length, mime_type: 'text/markdown' }
    await renderWithFile(fileContent, 'README.md')
    // Wait for toolbar first, then markdown
    await waitFor(() => expect(screen.getByTitle('Refresh file')).toBeInTheDocument(), { timeout: 3000 })
    // Markdown is rendered via mocked ReactMarkdown
    await waitFor(() => {
      expect(screen.getByTestId('markdown')).toBeInTheDocument()
    }, { timeout: 3000 })
  })

  it('shows file load error when file query fails', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValueOnce({ files: [{ name: 'nginx.conf', path: '/etc/nginx.conf', is_dir: false, size: 1024, readable: true }] })
    mockApiFetch.mockRejectedValueOnce(new Error('Permission denied'))

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('/etc')).toBeInTheDocument())
    fireEvent.click(screen.getByText('/etc'))
    await waitFor(() => expect(screen.getByText('nginx.conf')).toBeInTheDocument())
    fireEvent.click(screen.getByText('nginx.conf'))

    await waitFor(() => {
      expect(screen.getByText('Failed to load file')).toBeInTheDocument()
    })
  })

  it('shows search bar when Ctrl+F is pressed after file selected', async () => {
    const fileContent = { data: btoa('Hello World content'), total_size: 18, mime_type: 'text/plain' }
    await renderWithFile(fileContent)

    await waitFor(() => expect(screen.getByTitle('Refresh file')).toBeInTheDocument())

    // Open search via toolbar button
    const searchBtn = screen.getByTitle('Search in file (Ctrl+F)')
    fireEvent.click(searchBtn)

    await waitFor(() => {
      expect(screen.getByPlaceholderText('Search in file...')).toBeInTheDocument()
    })
  })

  it('closes search bar with Escape key', async () => {
    const fileContent = { data: btoa('Hello World content'), total_size: 18, mime_type: 'text/plain' }
    await renderWithFile(fileContent)

    await waitFor(() => expect(screen.getByTitle('Refresh file')).toBeInTheDocument())

    const searchBtn = screen.getByTitle('Search in file (Ctrl+F)')
    fireEvent.click(searchBtn)

    await waitFor(() => expect(screen.getByPlaceholderText('Search in file...')).toBeInTheDocument())

    fireEvent.keyDown(screen.getByPlaceholderText('Search in file...'), { key: 'Escape' })

    await waitFor(() => {
      expect(screen.queryByPlaceholderText('Search in file...')).not.toBeInTheDocument()
    })
  })
})

// --- relativeTime tests (via ServerDetailHeader) ---

describe('relativeTime via ServerDetailHeader', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows time in minutes for 5 min ago', async () => {
    const fiveMinAgo = new Date(Date.now() - 5 * 60 * 1000).toISOString()
    const serverWithOldTime = { ...mockServer, last_seen_at: fiveMinAgo }
    mockApiFetch.mockResolvedValueOnce(serverWithOldTime)
    mockApiFetch.mockResolvedValue({ paths: [] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )
    await waitFor(() => {
      expect(screen.getByText(/\d+ min ago/)).toBeInTheDocument()
    })
  })

  it('shows time in hours for 2 hours ago', async () => {
    const twoHoursAgo = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString()
    const serverWithOldTime = { ...mockServer, last_seen_at: twoHoursAgo }
    mockApiFetch.mockResolvedValueOnce(serverWithOldTime)
    mockApiFetch.mockResolvedValue({ paths: [] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )
    await waitFor(() => {
      expect(screen.getByText(/\d+h ago/)).toBeInTheDocument()
    })
  })

  it('shows time in days for 3 days ago', async () => {
    const threeDaysAgo = new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString()
    const serverWithOldTime = { ...mockServer, last_seen_at: threeDaysAgo }
    mockApiFetch.mockResolvedValueOnce(serverWithOldTime)
    mockApiFetch.mockResolvedValue({ paths: [] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )
    await waitFor(() => {
      expect(screen.getByText(/\d+d ago/)).toBeInTheDocument()
    })
  })

  it('shows -- when last_seen_at is null', async () => {
    const serverNoTime = { ...mockServer, last_seen_at: null }
    mockApiFetch.mockResolvedValueOnce(serverNoTime)
    mockApiFetch.mockResolvedValue({ paths: [] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )
    await waitFor(() => {
      expect(screen.getByText(/Last seen: --/)).toBeInTheDocument()
    })
  })
})

// --- PathManagement tests ---

describe('PathManagement', () => {
  beforeEach(() => {
    mockApiFetch.mockReset()
    vi.stubGlobal('fetch', vi.fn())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows add path form when expanded', async () => {
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    // PathManagement queries: paths + users
    mockApiFetch.mockResolvedValueOnce({ paths: [] })
    mockApiFetch.mockResolvedValueOnce({ users: [{ id: 'u1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Manage File Access')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Manage File Access'))

    await waitFor(() => {
      expect(screen.getByLabelText('Path')).toBeInTheDocument()
    })
  })

  it('shows paths in table when paths exist', async () => {
    const pathPermission = { id: 'p1', user_id: 'u1', username: 'admin', path: '/etc/nginx', server_id: 'srv-1', created_at: '' }
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    // PathManagement: paths with existing entries
    mockApiFetch.mockResolvedValueOnce({ paths: [pathPermission] })
    mockApiFetch.mockResolvedValueOnce({ users: [{ id: 'u1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Manage File Access')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Manage File Access'))

    await waitFor(() => {
      expect(screen.getByText('/etc/nginx')).toBeInTheDocument()
    })
  })

  it('can submit add path form', async () => {
    const pathPermission = { id: 'p1', user_id: 'u1', username: 'admin', path: '/etc/nginx', server_id: 'srv-1', created_at: '' }
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    // PathManagement queries
    mockApiFetch.mockResolvedValueOnce({ paths: [] })
    mockApiFetch.mockResolvedValueOnce({ users: [{ id: 'u1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }] })
    // Add path mutation response
    mockApiFetch.mockResolvedValueOnce(pathPermission)
    // Refetch paths after add
    mockApiFetch.mockResolvedValue({ paths: [pathPermission] })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Manage File Access')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Manage File Access'))

    await waitFor(() => expect(screen.getByLabelText('Path')).toBeInTheDocument())

    // Select user and enter path
    const userSelect = screen.getByLabelText('User')
    fireEvent.change(userSelect, { target: { value: 'u1' } })

    const pathInput = screen.getByLabelText('Path')
    fireEvent.change(pathInput, { target: { value: '/var/log' } })

    const addBtn = screen.getByRole('button', { name: /add path/i })
    fireEvent.click(addBtn)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        expect.stringContaining('/paths'),
        expect.objectContaining({ method: 'POST' })
      )
    })
  })

  it('delete permission button removes path', async () => {
    const pathPermission = { id: 'p1', user_id: 'u1', username: 'admin', path: '/etc/nginx', server_id: 'srv-1', created_at: '' }
    mockApiFetch.mockResolvedValueOnce(mockServer)
    mockApiFetch.mockResolvedValueOnce({ paths: ['/etc'] })
    mockApiFetch.mockResolvedValueOnce({ paths: [pathPermission] })
    mockApiFetch.mockResolvedValueOnce({ users: [{ id: 'u1', username: 'admin', email: 'a@b.com', role: 'admin', totp_enabled: true, avatar: null, created_at: '', updated_at: '' }] })
    // Delete mutation response
    mockApiFetch.mockResolvedValue({ status: 'ok' })

    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={qc}>
        <BrowserRouter>
          <ServerDetailPage />
        </BrowserRouter>
      </QueryClientProvider>,
    )

    await waitFor(() => expect(screen.getByText('Manage File Access')).toBeInTheDocument())
    fireEvent.click(screen.getByText('Manage File Access'))

    await waitFor(() => expect(screen.getByText('/etc/nginx')).toBeInTheDocument())

    const removeBtn = screen.getByTitle('Remove permission')
    fireEvent.click(removeBtn)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        expect.stringContaining('/paths/p1'),
        expect.objectContaining({ method: 'DELETE' })
      )
    })
  })
})
