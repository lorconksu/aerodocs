import { useState, useRef, useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { apiFetch } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import type { LoginTOTPRequest, AuthResponse } from '@/types/api'

export function LoginTOTPPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login } = useAuth()
  const totpToken = (location.state as { totpToken?: string })?.totpToken

  const [digits, setDigits] = useState(['', '', '', '', '', ''])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])

  useEffect(() => {
    if (!totpToken) navigate('/login')
  }, [totpToken, navigate])

  const handleDigitChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return

    const newDigits = [...digits]
    newDigits[index] = value.slice(-1)
    setDigits(newDigits)

    if (value && index < 5) {
      inputRefs.current[index + 1]?.focus()
    }

    // Auto-submit when all 6 digits entered
    if (newDigits.every(Boolean) && index === 5) {
      submitCode(newDigits.join(''))
    }
  }

  const handlePaste = (e: React.ClipboardEvent) => {
    e.preventDefault()
    const pasted = e.clipboardData.getData('text').replaceAll(/\D/g, '').slice(0, 6)
    if (!pasted) return
    const newDigits = [...digits]
    for (let i = 0; i < pasted.length; i++) {
      newDigits[i] = pasted[i]
    }
    setDigits(newDigits)
    const nextEmpty = pasted.length < 6 ? pasted.length : 5
    inputRefs.current[nextEmpty]?.focus()
    if (newDigits.every(Boolean)) submitCode(newDigits.join(''))
  }

  const handleKeyDown = (index: number, e: React.KeyboardEvent) => {
    if (e.key === 'Backspace' && !digits[index] && index > 0) {
      inputRefs.current[index - 1]?.focus()
    }
  }

  const submitCode = async (code: string) => {
    if (!totpToken) return
    setError('')
    setLoading(true)

    try {
      const resp = await apiFetch<AuthResponse>('/auth/login/totp', {
        method: 'POST',
        body: JSON.stringify({ totp_token: totpToken, code } satisfies LoginTOTPRequest),
      })

      login(resp.access_token, resp.refresh_token, resp.user)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed')
      setDigits(['', '', '', '', '', ''])
      inputRefs.current[0]?.focus()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div className="text-text-muted text-[10px] uppercase tracking-widest mb-2">Two-Factor Authentication</div>
      <p className="text-text-faint text-xs mb-4">Enter the 6-digit code from your authenticator app</p>

      {error && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-4">
          {error}
        </div>
      )}

      <div className="flex gap-2 justify-center mb-4">
        {digits.map((digit, i) => (
          <input
            key={`digit-${i}`}
            ref={el => { inputRefs.current[i] = el }}
            type="text"
            inputMode="numeric"
            maxLength={1}
            value={digit}
            onChange={(e) => handleDigitChange(i, e.target.value)}
            onKeyDown={(e) => handleKeyDown(i, e)}
            onPaste={handlePaste}
            className="w-10 h-12 bg-elevated border border-border rounded text-center text-lg font-mono text-text-primary focus:outline-none focus:border-accent"
            autoFocus={i === 0}
            disabled={loading}
          />
        ))}
      </div>

      <button
        onClick={() => submitCode(digits.join(''))}
        disabled={loading || digits.some(d => d === '')}
        className="w-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
      >
        {loading ? 'Verifying...' : 'Verify'}
      </button>
    </div>
  )
}
