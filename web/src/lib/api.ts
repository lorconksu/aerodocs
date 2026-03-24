import { getAccessToken, getRefreshToken, setTokens, clearTokens } from './auth'
import type { TokenPair } from '@/types/api'

const BASE_URL = '/api'

let isRefreshing = false
let refreshPromise: Promise<boolean> | null = null

async function refreshTokens(): Promise<boolean> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) return false

  try {
    const res = await fetch(`${BASE_URL}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })

    if (!res.ok) return false

    const data: TokenPair = await res.json()
    setTokens(data.access_token, data.refresh_token)
    return true
  } catch {
    return false
  }
}

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')

  const accessToken = getAccessToken()
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`)
  }

  let res = await fetch(`${BASE_URL}${path}`, { ...options, headers })

  // If 401 and we have a refresh token, try refreshing
  if (res.status === 401 && getRefreshToken()) {
    if (!isRefreshing) {
      isRefreshing = true
      refreshPromise = refreshTokens().finally(() => {
        isRefreshing = false
        refreshPromise = null
      })
    }

    const refreshed = await refreshPromise
    if (refreshed) {
      // Retry with new token
      const retryHeaders = new Headers(options.headers)
      retryHeaders.set('Content-Type', 'application/json')
      retryHeaders.set('Authorization', `Bearer ${getAccessToken()}`)
      res = await fetch(`${BASE_URL}${path}`, { ...options, headers: retryHeaders })
    } else {
      clearTokens()
      window.location.href = '/login'
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
export async function apiFetchWithToken<T>(
  path: string,
  token: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error((error as { error?: string }).error || `HTTP ${res.status}`)
  }

  return res.json() as Promise<T>
}
