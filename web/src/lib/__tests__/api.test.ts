import { vi, type MockedFunction } from 'vitest'
import { apiFetch, apiFetchWithToken } from '../api'

// We need to access module-level state — reset it between tests by re-importing
// via a factory approach won't work cleanly; instead we test the public surface.

const mockFetch = vi.fn() as MockedFunction<typeof fetch>

beforeEach(() => {
  vi.stubGlobal('fetch', mockFetch)
  localStorage.clear()
  mockFetch.mockReset()
  // Reset window.location.href stub
  Object.defineProperty(window, 'location', {
    value: { href: '' },
    writable: true,
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
})

function makeResponse(
  body: unknown,
  status = 200,
  headers: Record<string, string> = {},
): Response {
  const headersObj = new Headers({ 'Content-Type': 'application/json', ...headers })
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: headersObj,
    json: () => Promise.resolve(body),
    body: null,
  } as unknown as Response
}

describe('apiFetch', () => {
  it('calls fetch with the correct URL and JSON content-type', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ data: 'ok' }))
    const result = await apiFetch<{ data: string }>('/test')
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/test',
      expect.objectContaining({
        headers: expect.any(Headers),
      }),
    )
    expect(result).toEqual({ data: 'ok' })
  })

  it('includes Authorization header when access token exists', async () => {
    localStorage.setItem('aerodocs_access_token', 'my-token')
    mockFetch.mockResolvedValueOnce(makeResponse({ ok: true }))
    await apiFetch('/protected')
    const [, init] = mockFetch.mock.calls[0]
    const headers = init!.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer my-token')
  })

  it('does not include Authorization header when no access token', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ ok: true }))
    await apiFetch('/public')
    const [, init] = mockFetch.mock.calls[0]
    const headers = init!.headers as Headers
    expect(headers.get('Authorization')).toBeNull()
  })

  it('throws with the error message from JSON body on non-ok response', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ error: 'Forbidden' }, 403))
    await expect(apiFetch('/fail')).rejects.toThrow('Forbidden')
  })

  it('throws HTTP status message when JSON error field is missing', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({}, 500))
    await expect(apiFetch('/fail')).rejects.toThrow('HTTP 500')
  })

  it('throws Unknown error when JSON body cannot be parsed', async () => {
    const badResponse = {
      ok: false,
      status: 503,
      headers: new Headers(),
      json: () => Promise.reject(new Error('bad json')),
      body: null,
    } as unknown as Response
    mockFetch.mockResolvedValueOnce(badResponse)
    await expect(apiFetch('/fail')).rejects.toThrow('Unknown error')
  })

  it('returns undefined for 204 responses', async () => {
    const noContent = {
      ok: true,
      status: 204,
      headers: new Headers(),
      json: vi.fn(),
      body: null,
    } as unknown as Response
    mockFetch.mockResolvedValueOnce(noContent)
    const result = await apiFetch('/delete')
    expect(result).toBeUndefined()
  })

  it('returns undefined when content-length is 0', async () => {
    const emptyResponse = {
      ok: true,
      status: 200,
      headers: new Headers({ 'content-length': '0' }),
      json: vi.fn(),
      body: null,
    } as unknown as Response
    mockFetch.mockResolvedValueOnce(emptyResponse)
    const result = await apiFetch('/empty')
    expect(result).toBeUndefined()
  })

  it('on 401 with refresh token: refreshes and retries successfully', async () => {
    localStorage.setItem('aerodocs_access_token', 'old-token')
    localStorage.setItem('aerodocs_refresh_token', 'refresh-token')

    // First call returns 401
    mockFetch.mockResolvedValueOnce(makeResponse({ error: 'Unauthorized' }, 401))
    // Second call is the refresh request — returns new tokens
    mockFetch.mockResolvedValueOnce(
      makeResponse({ access_token: 'new-token', refresh_token: 'new-refresh' }),
    )
    // Third call is the retry with new token
    mockFetch.mockResolvedValueOnce(makeResponse({ data: 'success' }))

    const result = await apiFetch<{ data: string }>('/protected')
    expect(result).toEqual({ data: 'success' })
    // Check that new access token was stored
    expect(localStorage.getItem('aerodocs_access_token')).toBe('new-token')
  })

  it('on 401 with refresh token: clears tokens and redirects when refresh fails', async () => {
    localStorage.setItem('aerodocs_access_token', 'old-token')
    localStorage.setItem('aerodocs_refresh_token', 'refresh-token')

    // First call returns 401
    mockFetch.mockResolvedValueOnce(makeResponse({ error: 'Unauthorized' }, 401))
    // Refresh call fails (non-ok)
    mockFetch.mockResolvedValueOnce(makeResponse({ error: 'Invalid refresh' }, 401))

    await expect(apiFetch('/protected')).rejects.toThrow('Session expired')
    expect(localStorage.getItem('aerodocs_access_token')).toBeNull()
    expect(window.location.href).toBe('/login')
  })

  it('on 401 without refresh token: throws the error immediately', async () => {
    localStorage.setItem('aerodocs_access_token', 'bad-token')
    // No refresh token
    mockFetch.mockResolvedValueOnce(makeResponse({ error: 'Unauthorized' }, 401))
    await expect(apiFetch('/protected')).rejects.toThrow('Unauthorized')
  })

  it('passes custom options (method, body) to fetch', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ created: true }, 201))
    await apiFetch('/items', { method: 'POST', body: JSON.stringify({ name: 'test' }) })
    const [, init] = mockFetch.mock.calls[0]
    expect(init!.method).toBe('POST')
    expect(init!.body).toBe(JSON.stringify({ name: 'test' }))
  })
})

describe('apiFetchWithToken', () => {
  it('sets Authorization header with the provided token', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ ok: true }))
    await apiFetchWithToken('/setup', 'setup-token-123')
    const [, init] = mockFetch.mock.calls[0]
    const headers = init!.headers as Headers
    expect(headers.get('Authorization')).toBe('Bearer setup-token-123')
  })

  it('returns parsed JSON on success', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ secret: 'abc', qr_url: 'otpauth://...' }))
    const result = await apiFetchWithToken<{ secret: string }>('/auth/totp/setup', 'token')
    expect(result.secret).toBe('abc')
  })

  it('throws with error message on non-ok response', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ error: 'Not found' }, 404))
    await expect(apiFetchWithToken('/bad', 'tok')).rejects.toThrow('Not found')
  })

  it('throws HTTP status when JSON error field missing', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({}, 500))
    await expect(apiFetchWithToken('/bad', 'tok')).rejects.toThrow('HTTP 500')
  })

  it('throws Unknown error when JSON cannot be parsed', async () => {
    const badResponse = {
      ok: false,
      status: 400,
      headers: new Headers(),
      json: () => Promise.reject(new Error('parse error')),
      body: null,
    } as unknown as Response
    mockFetch.mockResolvedValueOnce(badResponse)
    await expect(apiFetchWithToken('/bad', 'tok')).rejects.toThrow('Unknown error')
  })

  it('passes additional options (method, body)', async () => {
    mockFetch.mockResolvedValueOnce(makeResponse({ status: 'ok' }))
    await apiFetchWithToken('/auth/totp/enable', 'setup-tok', {
      method: 'POST',
      body: JSON.stringify({ code: '123456' }),
    })
    const [url, init] = mockFetch.mock.calls[0]
    expect(url).toBe('/api/auth/totp/enable')
    expect(init!.method).toBe('POST')
  })
})
