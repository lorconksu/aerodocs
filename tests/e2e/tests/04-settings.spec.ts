import { test, expect } from '@playwright/test'
import { loginViaAPI } from './helpers'

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:18081'

test.describe('Settings', () => {
  test.beforeEach(async ({ page }) => {
    await loginViaAPI(page, BASE_URL)
  })

  test('Profile tab shows the logged-in username', async ({ page }) => {
    await page.goto(BASE_URL + '/settings')
    await expect(page).toHaveURL(/\/settings/, { timeout: 10000 })

    // Profile tab should be active by default
    await expect(page.getByRole('button', { name: 'Profile' })).toBeVisible()

    // Username should appear in the Account Info section
    await expect(page.getByText('admin').first()).toBeVisible()

    // Role should be "admin"
    await expect(page.getByText('admin').last()).toBeVisible()

    // 2FA status should show Enabled (set up in 01-setup-flow)
    await expect(page.getByText('Enabled').first()).toBeVisible()
  })

  test('Users tab shows admin user in table', async ({ page }) => {
    await page.goto(BASE_URL + '/settings?tab=users')
    await expect(page).toHaveURL(/\/settings\?tab=users/, { timeout: 10000 })

    // Users tab should be active
    await expect(page.getByRole('button', { name: 'Users' })).toBeVisible()

    // Wait for the users table to load
    await expect(page.getByText('User Management')).toBeVisible()

    // Admin user should appear in the table
    const userRow = page.locator('tr').filter({ hasText: 'admin' })
    await expect(userRow.first()).toBeVisible({ timeout: 10000 })

    // Create User button should be visible for admin
    await expect(page.getByRole('button', { name: 'Create User' })).toBeVisible()
  })
})
