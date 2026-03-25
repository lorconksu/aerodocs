import { test, expect } from '@playwright/test'
import { loginViaAPI } from './helpers'

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:18081'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await loginViaAPI(page, BASE_URL)
  })

  test('shows Fleet Dashboard with empty state when no servers exist', async ({ page }) => {
    await expect(page).toHaveURL(new RegExp(`^${BASE_URL}/?$`), { timeout: 10000 })

    // Page heading
    await expect(page.getByRole('heading', { name: 'Fleet Dashboard' })).toBeVisible()

    // Empty state message
    await expect(page.getByText('No servers found.')).toBeVisible()

    // "Add your first server" link should be visible for admin
    await expect(page.getByRole('button', { name: 'Add your first server' })).toBeVisible()
  })

  test('opens Add Server modal with install command on submit', async ({ page }) => {
    // Click the Add Server button in the header
    await page.getByRole('button', { name: 'Add Server' }).click()

    // Modal should appear
    await expect(page.getByRole('heading', { name: 'Add Server' })).toBeVisible()

    // Fill in server name and generate
    await page.getByPlaceholder('e.g., web-prod-1').fill('test-server-e2e')
    await page.getByRole('button', { name: 'Generate' }).click()

    // Should show the install command
    await expect(page.locator('pre')).toBeVisible({ timeout: 10000 })
    const installCmd = await page.locator('pre').textContent()
    expect(installCmd).toContain('curl')

    // Close the modal (allowed since agent won't connect in E2E)
    // The close button becomes enabled after timeout (2 min) or when agent connects.
    // We can force-close by clicking the X button — it is enabled when result is shown
    // and agent hasn't connected yet (timedOut starts false, canClose = !result || isOnline || timedOut)
    // Actually canClose = false until timedOut. Close via keyboard ESC or wait.
    // Instead, verify the modal content is correct and skip closing.
    await expect(page.getByText('Waiting for agent to connect...')).toBeVisible()
  })
})
