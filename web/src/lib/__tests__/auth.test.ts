import { getAccessToken, getRefreshToken, setTokens, clearTokens } from '../auth'

describe('auth', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  describe('getAccessToken', () => {
    it('returns null when no token stored', () => {
      expect(getAccessToken()).toBeNull()
    })

    it('returns the stored access token', () => {
      localStorage.setItem('aerodocs_access_token', 'my-access-token')
      expect(getAccessToken()).toBe('my-access-token')
    })
  })

  describe('getRefreshToken', () => {
    it('returns null when no token stored', () => {
      expect(getRefreshToken()).toBeNull()
    })

    it('returns the stored refresh token', () => {
      localStorage.setItem('aerodocs_refresh_token', 'my-refresh-token')
      expect(getRefreshToken()).toBe('my-refresh-token')
    })
  })

  describe('setTokens', () => {
    it('stores both access and refresh tokens', () => {
      setTokens('acc', 'ref')
      expect(localStorage.getItem('aerodocs_access_token')).toBe('acc')
      expect(localStorage.getItem('aerodocs_refresh_token')).toBe('ref')
    })

    it('overwrites existing tokens', () => {
      setTokens('old-acc', 'old-ref')
      setTokens('new-acc', 'new-ref')
      expect(localStorage.getItem('aerodocs_access_token')).toBe('new-acc')
      expect(localStorage.getItem('aerodocs_refresh_token')).toBe('new-ref')
    })
  })

  describe('clearTokens', () => {
    it('removes both tokens from localStorage', () => {
      setTokens('acc', 'ref')
      clearTokens()
      expect(localStorage.getItem('aerodocs_access_token')).toBeNull()
      expect(localStorage.getItem('aerodocs_refresh_token')).toBeNull()
    })

    it('does not throw when tokens are not set', () => {
      expect(() => clearTokens()).not.toThrow()
    })
  })
})
