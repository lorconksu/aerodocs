import { Page } from '@playwright/test'
import { authenticator } from 'otplib'
import fs from 'fs'
import path from 'path'

// Path for persisting TOTP secret across test specs
const STATE_FILE = path.resolve(__dirname, '../.e2e-state.json')

interface E2EState {
  totpSecret: string
}

export function saveState(state: E2EState) {
  fs.writeFileSync(STATE_FILE, JSON.stringify(state, null, 2))
}

export function loadState(): E2EState {
  if (!fs.existsSync(STATE_FILE)) {
    throw new Error('E2E state file not found — run 01-setup-flow.spec.ts first')
  }
  return JSON.parse(fs.readFileSync(STATE_FILE, 'utf-8')) as E2EState
}

export function getTOTPCode(secret: string): string {
  return authenticator.generate(secret)
}

export async function loginViaAPI(page: Page, baseURL: string) {
  const { totpSecret } = loadState()

  // Step 1: Password login
  const loginResp = await page.request.post(`${baseURL}/api/auth/login`, {
    data: { username: 'admin', password: 'E2eTestPass!2026' },
  })
  const loginData = await loginResp.json() as { totp_token: string }

  // Step 2: TOTP verification
  const code = authenticator.generate(totpSecret)
  const totpResp = await page.request.post(`${baseURL}/api/auth/login/totp`, {
    data: { totp_token: loginData.totp_token, code },
  })
  const authData = await totpResp.json() as { access_token: string; refresh_token: string }

  // Step 3: Inject tokens into localStorage so the SPA picks them up
  await page.goto(baseURL + '/')
  await page.evaluate((tokens) => {
    localStorage.setItem('aerodocs_access_token', tokens.access_token)
    localStorage.setItem('aerodocs_refresh_token', tokens.refresh_token)
  }, authData)

  await page.reload()
}
