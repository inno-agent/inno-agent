import { test as base } from '@playwright/test';
import { TEST_USER } from '../constants/userInfo.constant';
import { MODELS_CATALOG } from '../constants/modelsCatalog.constant';

export const test = base.extend({
    page: async ({ page }, use) => {
        await page.addInitScript(
            (token) => {
                localStorage.setItem('aicore_token', token);
            },
            TEST_USER.accessToken,
        );
        await page.route('**/llm/v1/models', (route) =>
            route.fulfill({ json: MODELS_CATALOG }),
        );
        await page.route('**/api/v1/installations/me', (route) =>
            route.fulfill({ status: 404, json: {} }),
        );

        await use(page);
    },
});

export { expect } from '@playwright/test';