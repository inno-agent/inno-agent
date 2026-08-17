import { test, expect } from '../fixtures/auth.fixture';

test('link account page loads', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Link account' }).click();
    await expect(page.getByRole('heading', { name: 'Link GitFlame account' })).toBeVisible();
});