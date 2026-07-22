import { expect } from '@playwright/test';

// Thin wrapper around Playwright's built-in snapshot matcher so callers compare
// serialized data (mock payloads, captured requests, etc.) the same way
// everywhere, instead of each test hand-rolling JSON.stringify + toMatchSnapshot.
export function compareSnapshot(data: unknown, snapshotName: string): void {
  expect(JSON.stringify(data, null, 2)).toMatchSnapshot(snapshotName);
}
