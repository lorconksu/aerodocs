import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  retries: 1,
  fullyParallel: false,
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:18081',
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
  },
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
})
