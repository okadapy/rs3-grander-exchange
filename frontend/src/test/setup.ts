import '@testing-library/jest-dom/vitest';
import { afterAll, afterEach } from 'vitest';
import { server } from './msw/server';

// Deliberately not `beforeAll(() => server.listen(...))`. openapi-fetch's
// createClient() captures `globalThis.fetch` at call time (module-import
// time for our singleton `api`), so MSW must already be listening before
// `client.ts` is imported. Vitest evaluates setup files before test files,
// so calling listen() here at the top level runs early enough; inside
// beforeAll it would run too late and requests would hit the real network.
server.listen({ onUnhandledRequest: 'error' });
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
