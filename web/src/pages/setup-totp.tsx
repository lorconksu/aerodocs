import { useState, useEffect, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { Copy, Check } from 'lucide-react'
import { apiFetchWithToken } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import type { TOTPSetupResponse, TOTPEnableRequest, AuthResponse } from '@/types/api'

export function SetupTOTPPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login } = useAuth()
  const setupToken = (location.state as { setupToken?: string })?.setupToken

  const [totpData, setTotpData] = useState<TOTPSetupResponse | null>(null)
  const [digits, setDigits] = useState(['', '', '', '', '', ''])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])
  const hasFetched = useRef(false)

  useEffect(() => {
    if (!setupToken) { navigate('/login'); return }
    if (hasFetched.current) return
    hasFetched.current = true

    apiFetchWithToken<TOTPSetupResponse>('/auth/totp/setup', setupToken, { method: 'POST' })
      .then(setTotpData)
      .catch(() => setError('Failed to generate TOTP secret'))
  }, [setupToken, navigate])

  const handleDigitChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return
    const newDigits = [...digits]
    newDigits[index] = value.slice(-1)
    setDigits(newDigits)
    if (value && index < 5) inputRefs.current[index + 1]?.focus()
    if (newDigits.every(Boolean) && index === 5) submitCode(newDigits.join(''))
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
    if (!setupToken) return
    setError('')
    setLoading(true)

    try {
      const resp = await apiFetchWithToken<AuthResponse>('/auth/totp/enable', setupToken, {
        method: 'POST',
        body: JSON.stringify({ code } satisfies TOTPEnableRequest),
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
      <div className="text-text-muted text-[10px] uppercase tracking-widest mb-2">Set Up Two-Factor Authentication</div>
      <p className="text-text-faint text-xs mb-4">Scan this QR code with your authenticator app, then enter the 6-digit code</p>

      {error && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-4">
          {error}
        </div>
      )}

      {totpData && (
        <>
          <div className="bg-white rounded-lg p-4 mb-4 flex items-center justify-center">
            <img src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(totpData.qr_url)}`} alt="TOTP QR Code" className="w-48 h-48" />
          </div>

          <div className="bg-elevated border border-border rounded px-3 py-2 mb-4">
            <div className="flex items-center justify-between mb-1">
              <div className="text-text-muted text-[10px] uppercase tracking-widest">Manual Entry Key</div>
              <button
                type="button"
                onClick={() => {
                  navigator.clipboard.writeText(totpData.secret)
                  setCopied(true)
                  setTimeout(() => setCopied(false), 2000)
                }}
                className="flex items-center gap-1 text-text-muted hover:text-text-primary text-[10px] uppercase tracking-wider transition-colors"
              >
                {copied ? <Check className="w-3 h-3 text-status-online" /> : <Copy className="w-3 h-3" />}
                {copied ? 'Copied' : 'Copy'}
              </button>
            </div>
            <code className="text-text-primary text-xs font-mono break-all select-all">{totpData.secret}</code>
          </div>

          <div className="flex gap-2 justify-center mb-4">
            {digits.map((digit, i) => (
              <input
                key={`digit-${i}`}
                ref={el => { inputRefs.current[i] = el }}
                type="text" inputMode="numeric" maxLength={1}
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
            {loading ? 'Verifying...' : 'Verify & Enable 2FA'}
          </button>
        </>
      )}
    </div>
  )
}
