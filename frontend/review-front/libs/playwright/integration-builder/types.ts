import type { Request, Route } from '@playwright/test';

export interface Interceptor {
  name: string;
  match(request: Request): boolean;
  handle(route: Route): Promise<void> | void;
}
