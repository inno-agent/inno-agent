import type { Interceptor } from './types';

// Always matches. Registered last by IntegrationBuilder so any request that
// no domain/shared interceptor claimed is severed with a 500 instead of
// silently hitting the real network or hanging the test.
export const fallbackInterceptor: Interceptor = {
  name: 'fallback-500',
  match: () => true,
  handle: async (route) => {
    const request = route.request();
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({
        error: 'Unmocked request',
        method: request.method(),
        url: request.url(),
      }),
    });
  },
};
