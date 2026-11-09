import { defineConfig } from '@playwright/test';

// Hosted web UI end-to-end tests. They need a running server with test login:
//   (cd ../server && DEV_MEMORY_STORE=1 TEST_LOGIN_SECRET=local ... go run ./cmd/server)
// and the hosted build embedded (see .github/workflows/server.yml).
export default defineConfig({
  testDir: 'e2e',
  timeout: 30_000,
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:18080',
    screenshot: 'only-on-failure',
    trace: 'off',
  },
  reporter: [['list']],
});
