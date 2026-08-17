import type { Page } from '@playwright/test';
import type { Interceptor } from './types';
import { fallbackInterceptor } from './fallbackInterceptor';

export class IntegrationBuilder {
  private readonly interceptors: Interceptor[] = [];

  add(interceptor: Interceptor): this {
    this.interceptors.push(interceptor);
    return this;
  }

  addMany(interceptors: Interceptor[]): this {
    interceptors.forEach((interceptor) => this.add(interceptor));
    return this;
  }

  async apply(page: Page): Promise<void> {
    const chain = [...this.interceptors, fallbackInterceptor];

    await page.route('**/*', async (route) => {
      const matched = chain.find((interceptor) => interceptor.match(route.request()));
      await matched!.handle(route);
    });
  }
}
