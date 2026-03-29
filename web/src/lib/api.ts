import { getCSRFToken } from './auth'

const BASE_URL = '/api'

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')

  const csrfToken = getCSRFToken()
  if (csrfToken) {
    headers.set('X-CSRF-Token', csrfToken)
  }

  let res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
    credentials: 'same-origin',
  })

  // Handle 401 with automatic refresh
  if (res.status === 401) {
    const refreshRes = await fetch(`${BASE_URL}/auth/refresh`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : undefined,
    })

    if (refreshRes.ok) {
      // Retry with new cookies (set by server)
      const newCsrf = getCSRFToken()
      if (newCsrf) {
        headers.set('X-CSRF-Token', newCsrf)
      }
      res = await fetch(`${BASE_URL}${path}`, {
        ...options,
        headers,
        credentials: 'same-origin',
      })
    } else {
      globalThis.location.href = '/login'
      throw new Error('Session expired')
    }
  }

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error((error as { error?: string }).error || `HTTP ${res.status}`)
  }

  if (res.status === 204 || res.headers.get('content-length') === '0') {
    return undefined as T
  }
  return res.json() as Promise<T>
}

// Convenience for requests that use a specific token (setup/totp)
// These tokens are NOT in cookies — they are passed via navigation state
export async function apiFetchWithToken<T>(
  path: string,
  token: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  headers.set('Authorization', `Bearer ${token}`)

  const csrfToken = getCSRFToken()
  if (csrfToken) {
    headers.set('X-CSRF-Token', csrfToken)
  }

  const res = await fetch(`${BASE_URL}${path}`, {
    ...options,
    headers,
    credentials: 'same-origin',
  })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error((error as { error?: string }).error || `HTTP ${res.status}`)
  }

  return res.json() as Promise<T>
}
