import { defineConfig } from '@playwright/test';
import * as dotenv from 'dotenv';

// 手动加载 .env
dotenv.config({ path: __dirname + '/.env' });

export default defineConfig({
  testDir: './e2e',
  timeout: 60000,
  expect: { timeout: 10000 },
  fullyParallel: false,
  retries: 1,
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: process.env.APP_URL || 'http://localhost:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 10000,
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium', locale: 'zh-CN' },
    },
  ],
});
