import { test, expect } from '@playwright/test'
import { authenticator } from 'otplib'
import { loadState } from './helpers'

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:18081'

test.describe('Login flow', () => {
  test('logs in via UI: credentials → TOTP → dashboard', async ({ page }) => {
    const { totpSecret } = loadState()

    // Navigate to /login
    await page.goto(BASE_URL + '/login')
    await expect(page).toHaveURL(/\/login$/)

    // Fill credentials
    await page.getByPlaceholder('username').fill('admin')
    await page.getByPlaceholder('password').fill('E2eTestPass!2026')
    await page.getByRole('button', { name: 'Sign In' }).click()

    // Should redirect to TOTP page
    await expect(page).toHaveURL(/\/login\/totp$/, { timeout: 10000 })

    // Generate a valid TOTP code and fill the digit inputs
    const code = authenticator.generate(totpSecret)
    const inputs = page.locator('input[inputmode="numeric"]')
    await expect(inputs).toHaveCount(6)
    for (let i = 0; i < 6; i++) {
      await inputs.nth(i).fill(code[i])
    }

    // Should redirect to dashboard after successful TOTP verification
    await expect(page).toHaveURL(new RegExp(`^${BASE_URL}/?$`), { timeout: 15000 })
  })

  test('rejects invalid credentials', async ({ page }) => {
    await page.goto(BASE_URL + '/login')

    await page.getByPlaceholder('username').fill('admin')
    await page.getByPlaceholder('password').fill('WrongPassword!999')
    await page.getByRole('button', { name: 'Sign In' }).click()

    // Should show an error message and stay on /login
    await expect(page.getByText('invalid credentials')).toBeVisible({ timeout: 5000 })
    await expect(page).toHaveURL(/\/login$/)
  })
})
