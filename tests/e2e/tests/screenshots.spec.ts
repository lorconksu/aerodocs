/**
 * Screenshot capture spec — standalone, does NOT use global setup/teardown.
 *
 * Spins up its own fresh Docker container so we can capture the setup flow
 * (before any account exists) as well as all authenticated pages.
 *
 * Run standalone:
 *   cd tests/e2e && npx playwright test screenshots.spec.ts --config playwright-screenshots.config.ts
 */

import { test, expect, type Page } from '@playwright/test'
import { authenticator } from 'otplib'
import { spawnSync } from 'child_process'
import fs from 'fs'
import path from 'path'

// Container / network config for the screenshot run
const CONTAINER_NAME = 'aerodocs-screenshots'
const HTTP_PORT = '18082'
const BASE_URL = `http://localhost:${HTTP_PORT}`
const IMAGE = process.env.E2E_IMAGE || 'aerodocs:latest'

// Output directory — project root docs/screenshots/
const SCREENSHOT_DIR = path.resolve(__dirname, '../../../docs/screenshots')

// Credentials
const USERNAME = 'admin'
const PASSWORD = 'E2eTestPass!2026'
const EMAIL = 'admin@example.com'

// Shared state across tests (serial execution)
let totpSecret = ''
let lastUsedTOTPCode = ''

// Auth tokens
let authTokens: { access_token: string; refresh_token: string } | null = null

/**
 * Wait until the TOTP code changes from the last used one.
 * Prevents code-reuse rejections when multiple tests use TOTP in the same 30s window.
 */
async function waitForFreshTOTPCode(): Promise<string> {
  if (!lastUsedTOTPCode) return authenticator.generate(totpSecret)
  for (let i = 0; i < 35; i++) {
    const code = authenticator.generate(totpSecret)
    if (code !== lastUsedTOTPCode) return code
    await new Promise(r => setTimeout(r, 1000))
  }
  throw new Error('TOTP code did not rotate within 35s')
}

// ---------------------------------------------------------------------------
// Container lifecycle helpers
// ---------------------------------------------------------------------------
function startContainer() {
  spawnSync('docker', ['rm', '-f', CONTAINER_NAME], { stdio: 'pipe' })

  const result = spawnSync('docker', [
    'run', '-d',
    '--name', CONTAINER_NAME,
    '-p', `${HTTP_PORT}:8081`,
    IMAGE,
  ], { stdio: 'pipe' })

  if (result.status !== 0) {
    throw new Error(`Failed to start container: ${result.stderr?.toString()}`)
  }
}

async function waitForReady(timeoutSec = 30) {
  for (let i = 0; i < timeoutSec; i++) {
    try {
      const res = await fetch(`${BASE_URL}/api/auth/status`)
      if (res.ok) {
        console.log(`Screenshot container ready after ${i + 1}s`)
        return
      }
    } catch { /* not ready */ }
    await new Promise(r => setTimeout(r, 1000))
  }
  throw new Error(`Container not ready after ${timeoutSec}s`)
}

function stopContainer() {
  spawnSync('docker', ['rm', '-f', CONTAINER_NAME], { stdio: 'pipe' })
}

// ---------------------------------------------------------------------------
// Auth helper — obtains fresh tokens via Node.js fetch, injects into page
// ---------------------------------------------------------------------------
async function obtainFreshTokens(): Promise<{ access_token: string; refresh_token: string }> {
  const code = await waitForFreshTOTPCode()

  const loginResp = await fetch(`${BASE_URL}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username: USERNAME, password: PASSWORD }),
  })
  if (!loginResp.ok) {
    throw new Error(`Login API failed: ${loginResp.status} ${await loginResp.text()}`)
  }
  const loginData = (await loginResp.json()) as { totp_token: string }

  const totpResp = await fetch(`${BASE_URL}/api/auth/login/totp`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ totp_token: loginData.totp_token, code }),
  })
  if (!totpResp.ok) {
    throw new Error(`TOTP API failed: ${totpResp.status} ${await totpResp.text()}`)
  }

  lastUsedTOTPCode = code
  const tokens = (await totpResp.json()) as { access_token: string; refresh_token: string }
  authTokens = tokens
  return tokens
}

async function injectAuth(page: Page, targetPath = '/') {
  // Obtain fresh tokens if we don't have any yet
  if (!authTokens) {
    await obtainFreshTokens()
  }

  // Navigate to the target page — SPA will redirect to /login since no tokens yet
  await page.goto(BASE_URL + targetPath)
  // Set tokens in localStorage (works on any same-origin page including /login)
  await page.evaluate((tokens) => {
    localStorage.setItem('aerodocs_access_token', tokens.access_token)
    localStorage.setItem('aerodocs_refresh_token', tokens.refresh_token)
  }, authTokens!)
  // Reload — SPA reads tokens on init, calls /auth/me, authenticates, shows target page
  await page.reload()
  await page.waitForLoadState('networkidle')
}

// ---------------------------------------------------------------------------
// Screenshot helper
// ---------------------------------------------------------------------------
function screenshotPath(name: string) {
  return path.join(SCREENSHOT_DIR, name)
}

// ---------------------------------------------------------------------------
// Tests — serial execution
// ---------------------------------------------------------------------------
test.describe.configure({ mode: 'serial' })

test.describe('Screenshot capture', () => {
  test.beforeAll(async () => {
    fs.mkdirSync(SCREENSHOT_DIR, { recursive: true })
    startContainer()
    await waitForReady()
  })

  test.afterAll(async () => {
    stopContainer()
  })

  // 01 — Setup page (fresh system, no account)
  test('01-setup-page', async ({ page }) => {
    await page.goto(`${BASE_URL}/`)
    await expect(page).toHaveURL(/\/setup$/, { timeout: 10000 })
    await expect(page.getByRole('button', { name: 'Create Account' })).toBeVisible()

    await page.screenshot({
      path: screenshotPath('01-setup-page.png'),
      fullPage: true,
    })
  })

  // 02 — TOTP setup screen (after account creation)
  test('02-totp-setup', async ({ page }) => {
    await page.goto(`${BASE_URL}/setup`)
    await expect(page.getByRole('button', { name: 'Create Account' })).toBeVisible()

    await page.getByPlaceholder('username').fill(USERNAME)
    await page.getByPlaceholder('email').fill(EMAIL)
    await page.getByPlaceholder('password (min 12 chars)').fill(PASSWORD)
    await page.getByRole('button', { name: 'Create Account' }).click()

    await expect(page).toHaveURL(/\/setup\/totp$/, { timeout: 10000 })

    const secretEl = page.locator('code')
    await expect(secretEl).toBeVisible({ timeout: 10000 })
    totpSecret = (await secretEl.textContent()) ?? ''
    expect(totpSecret.length).toBeGreaterThan(10)

    await page.screenshot({
      path: screenshotPath('02-totp-setup.png'),
      fullPage: true,
    })

    // Complete TOTP setup
    const code = authenticator.generate(totpSecret)
    const inputs = page.locator('input[inputmode="numeric"]')
    await expect(inputs).toHaveCount(6)
    for (let i = 0; i < 6; i++) {
      await inputs.nth(i).fill(code[i])
    }
    lastUsedTOTPCode = code
    await expect(page).toHaveURL(new RegExp(`^${BASE_URL}/?$`), { timeout: 15000 })
  })

  // 03 — Login page
  test('03-login-page', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`)
    await expect(page).toHaveURL(/\/login$/, { timeout: 10000 })
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible()

    await page.screenshot({
      path: screenshotPath('03-login-page.png'),
      fullPage: true,
    })
  })

  // 04 — TOTP login verification page + obtain auth tokens for later tests
  test('04-totp-login', async ({ page }) => {
    await page.goto(`${BASE_URL}/login`)
    await page.getByPlaceholder('username').fill(USERNAME)
    await page.getByPlaceholder('password').fill(PASSWORD)
    await page.getByRole('button', { name: 'Sign In' }).click()

    await expect(page).toHaveURL(/\/login\/totp$/, { timeout: 10000 })

    await page.screenshot({
      path: screenshotPath('04-totp-login.png'),
      fullPage: true,
    })

    // Complete the login
    const code = await waitForFreshTOTPCode()
    const inputs = page.locator('input[inputmode="numeric"]')
    await expect(inputs).toHaveCount(6)
    for (let i = 0; i < 6; i++) {
      await inputs.nth(i).fill(code[i])
    }
    lastUsedTOTPCode = code
    await expect(page).toHaveURL(new RegExp(`^${BASE_URL}/?$`), { timeout: 15000 })

    // Obtain API tokens for subsequent authenticated screenshot tests
    await obtainFreshTokens()
  })

  // 05 — Fleet dashboard (empty state)
  test('05-fleet-dashboard', async ({ page }) => {
    await injectAuth(page)
    await expect(page.getByRole('heading', { name: 'Fleet Dashboard' })).toBeVisible({ timeout: 10000 })

    await page.screenshot({
      path: screenshotPath('05-fleet-dashboard.png'),
      fullPage: true,
    })
  })

  // 06 — Add Server modal
  test('06-add-server-modal', async ({ page }) => {
    await injectAuth(page)
    await expect(page.getByRole('heading', { name: 'Fleet Dashboard' })).toBeVisible({ timeout: 10000 })

    await page.getByRole('button', { name: 'Add Server' }).click()
    await expect(page.getByRole('heading', { name: 'Add Server' })).toBeVisible()

    await page.getByPlaceholder('e.g., web-prod-1').fill('web-prod-1')
    await page.getByRole('button', { name: 'Generate' }).click()

    await expect(page.locator('pre')).toBeVisible({ timeout: 10000 })

    await page.screenshot({
      path: screenshotPath('06-add-server-modal.png'),
      fullPage: true,
    })
  })

  // 07 — Audit logs
  test('07-audit-logs', async ({ page }) => {
    await injectAuth(page, '/audit-logs')
    await expect(page).toHaveURL(/\/audit-logs/, { timeout: 10000 })

    await expect(page.getByRole('heading', { name: 'Audit Logs' })).toBeVisible({ timeout: 10000 })

    // Wait for entries to load
    await expect(
      page.locator('table tbody tr').first()
    ).toBeVisible({ timeout: 10000 })

    await page.screenshot({
      path: screenshotPath('07-audit-logs.png'),
      fullPage: true,
    })
  })

  // 08 — Settings profile tab
  test('08-settings-profile', async ({ page }) => {
    await injectAuth(page, '/settings')
    await expect(page).toHaveURL(/\/settings/, { timeout: 10000 })
    await expect(page.getByText('admin').first()).toBeVisible({ timeout: 10000 })

    await page.screenshot({
      path: screenshotPath('08-settings-profile.png'),
      fullPage: true,
    })
  })

  // 09 — Settings users tab
  test('09-settings-users', async ({ page }) => {
    await injectAuth(page, '/settings?tab=users')
    await expect(page).toHaveURL(/\/settings/, { timeout: 10000 })
    await expect(page.getByText('User Management')).toBeVisible({ timeout: 10000 })
    await expect(page.locator('tr').filter({ hasText: 'admin' }).first()).toBeVisible({ timeout: 10000 })

    await page.screenshot({
      path: screenshotPath('09-settings-users.png'),
      fullPage: true,
    })
  })

  // 10 — Server detail file tree (requires connected agent — skip)
  test.skip('10-server-detail-filetree', async ({ page }) => {
    // Requires a connected agent — not feasible in standalone E2E
  })

  // 11 — Server detail dropzone (requires connected agent — skip)
  test.skip('11-server-detail-dropzone', async ({ page }) => {
    // Requires a connected agent — not feasible in standalone E2E
  })
})
