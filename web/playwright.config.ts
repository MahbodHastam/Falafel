import { defineConfig } from '@playwright/test'

const PORT = Number(process.env.FALAFEL_E2E_PORT ?? 8091)

export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.spec.ts',
  timeout: 90_000,
  expect: { timeout: 20_000 },
  fullyParallel: false,
  retries: process.env.CI ? 2 : 1,
  workers: 1,
  reporter: [['list']],
  use: {
    launchOptions: {
      args: [
        '--disable-background-timer-throttling',
        '--disable-backgrounding-occluded-windows',
        '--disable-renderer-backgrounding',
      ],
    },
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: 'retain-on-failure',
  },
  webServer: process.env.FALAFEL_E2E_NO_SERVER
    ? undefined
    : {
        command: 'bash scripts/e2e-server.sh',
        url: `http://127.0.0.1:${PORT}/healthz`,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
        env: {
          PORT: String(PORT),
          FALAFEL_OWNER_GRACE: '2s',
        },
      },
})
