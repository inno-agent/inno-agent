import type { Page } from '@playwright/test';

export interface PageObjectOptions {
  page: Page;
}

export abstract class BasePage {
  protected readonly page: Page;

  constructor({ page }: PageObjectOptions) {
    this.page = page;
  }
}
