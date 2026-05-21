import { test as base, expect } from '@playwright/test';

export const test = base.extend({
  page: async ({ page }, use) => {
    const username = process.env.TEST_USERNAME || 'admin';
    const password = process.env.TEST_PASSWORD || 'HZWLsoft.com123';

    await page.goto('/sign-in', { waitUntil: 'networkidle' });
    await page.getByRole('textbox', { name: /用户名或电子邮件|username or email/i }).fill(username);
    await page.getByRole('textbox', { name: /密码|password/i }).fill(password);
    await page.getByRole('button', { name: /登录|login|sign.?in/i }).click();
    await page.waitForURL(/\/(dashboard|wallet|subscriptions|overview)/, { timeout: 15000 });

    await use(page);
  },
});

export { expect };
