import { createContext, useContext, useState, useEffect, useCallback, useMemo, type ReactNode } from 'react'
import { clearTokens } from '@/lib/auth'
import type { User } from '@/types/api'

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  login: (user: User) => void
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    // Try to fetch user — cookie is sent automatically.
    // Use raw fetch (not apiFetch) to avoid the 401 → redirect-to-login loop
    // on initial load when no session exists.
    fetch('/api/auth/me', { credentials: 'same-origin' })
      .then(res => res.ok ? res.json() as Promise<User> : Promise.reject())
      .then(setUser)
      .catch(() => {
        // Not authenticated or cookie expired — no cleanup needed
      })
      .finally(() => setIsLoading(false))
  }, [])

  const login = useCallback((user: User) => {
    setUser(user)
  }, [])

  const logout = useCallback(async () => {
    await clearTokens()
    setUser(null)
  }, [])

  const contextValue = useMemo(() => ({
    user,
    isLoading,
    isAuthenticated: !!user,
    login,
    logout,
  }), [user, isLoading, login, logout])

  return (
    <AuthContext.Provider value={contextValue}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
