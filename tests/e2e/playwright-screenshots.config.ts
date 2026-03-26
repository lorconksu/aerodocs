import { defineConfig } from '@playwright/test'

/**
 * Playwright config for the screenshot spec only.
 * No global setup/teardown — the spec manages its own container.
 */
export default defineConfig({
  testDir: './tests',
  testMatch: 'screenshots.spec.ts',
  timeout: 60000,
  retries: 0,
  fullyParallel: false,
  use: {
    baseURL: 'http://localhost:18082',
    screenshot: 'only-on-failure',
    trace: 'off',
    viewport: { width: 1280, height: 720 },
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
})
