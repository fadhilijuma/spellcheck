import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  workers: 1,
  retries: 0,
  timeout: 30000,
  globalTimeout: 300000,
  projects: [
    { name: 'chromium', grepInvert: /@touch/, use: { browserName: 'chromium' } },
    { name: 'firefox', grepInvert: /@touch|@contrast/, use: { browserName: 'firefox' } },
    { name: 'webkit', grepInvert: /@touch|@contrast/, use: { browserName: 'webkit' } },
    { name: 'touch', use: { browserName: 'chromium', hasTouch: true, viewport: { width: 390, height: 844 } } },
  ],
  use: {
    baseURL: 'http://127.0.0.1:3016',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: 'go run ./cmd/spellcheck --web-api-host=127.0.0.1:3015',
      cwd: '..',
      url: 'http://127.0.0.1:3015/v1/readiness',
      timeout: 120000,
      reuseExistingServer: false,
      gracefulShutdown: { signal: 'SIGTERM', timeout: 5000 },
    },
    {
      command: 'npm run dev -- --port 3016',
      env: { SPELL_API_PROXY: 'http://127.0.0.1:3015' },
      url: 'http://127.0.0.1:3016',
      reuseExistingServer: false,
    },
  ],
});
