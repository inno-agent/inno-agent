import { test, expect } from '../fixtures/auth.fixture';

test('home page loads', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle('PR Reviewer');
  page.on('request', r => console.log(r.method(), r.url()));
});
