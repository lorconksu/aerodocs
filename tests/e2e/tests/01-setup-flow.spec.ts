import { test, expect } from '@playwright/test'
import { authenticator } from 'otplib'
import { saveState, markTOTPCodeUsed } from './helpers'

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:18081'

test.describe('First-time setup flow', () => {
  test('completes setup: register → TOTP setup → dashboard', async ({ page }) => {
    // Navigate to root — should redirect to /setup (uninitialized system)
    await page.goto(BASE_URL + '/')
    await expect(page).toHaveURL(/\/setup$/)

    // Fill out registration form
    await page.getByPlaceholder('username').fill('admin')
    await page.getByPlaceholder('email').fill('admin@example.com')
    await page.getByPlaceholder('password (min 12 chars)').fill('E2eTestPass!2026')
    await page.getByRole('button', { name: 'Create Account' }).click()

    // Should navigate to TOTP setup page
    await expect(page).toHaveURL(/\/setup\/totp$/)

    // Wait for the TOTP secret to appear
    const secretEl = page.locator('code')
    await expect(secretEl).toBeVisible({ timeout: 10000 })
    const totpSecret = (await secretEl.textContent()) ?? ''
    expect(totpSecret.length).toBeGreaterThan(10)

    // Save the TOTP secret for subsequent test specs
    saveState({ totpSecret })

    // Generate a valid TOTP code and fill the 6 digit inputs
    const code = authenticator.generate(totpSecret)
    const inputs = page.locator('input[inputmode="numeric"]')
    await expect(inputs).toHaveCount(6)
    for (let i = 0; i < 6; i++) {
      await inputs.nth(i).fill(code[i])
    }

    // Mark the code as used so subsequent tests wait for a fresh one
    markTOTPCodeUsed(code)

    // After submitting all digits the app auto-submits; wait for redirect to dashboard
    await expect(page).toHaveURL(new RegExp(`^${BASE_URL}/?$`), { timeout: 15000 })
  })
})
