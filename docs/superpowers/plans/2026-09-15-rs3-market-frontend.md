# RS3 Market Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the RS3 Market web frontend — character skills, a crafting profitability table, and a live chat with realtime price updates — against the RS3 Market API.

**Architecture:** A Vite/React single-page app running inside a Node 22 container. All API types are generated from `combined.yaml` by `openapi-typescript`; nothing hand-declares a response shape. TanStack Query owns server state and caching; a single WebSocket owns realtime chat and price frames. The recipe table's row assembly is a pure function tested without React.

**Tech Stack:** Docker (`node:22-alpine`), Vite 8, React 19, TypeScript 7, MUI 9 + MUI X DataGrid 9, TanStack Query 5, React Router 7, openapi-typescript 7 + openapi-fetch, Vitest 5 + Testing Library + MSW 2.

**Spec:** `docs/superpowers/specs/2026-09-15-rs3-market-frontend-design.md`

## Global Constraints

- Every command runs inside the container: `docker compose run --rm frontend <command>`. Never run `npm` on the host — the host has Node 18.19.1 and Vite 8 requires Node >= 22.12.
- Exact dependency versions (verified against the npm registry on 2026-09-15): vite 8.3.0, @vitejs/plugin-react 6.1.1, typescript 7.0.2, react 19.3.0, react-dom 19.3.0, @mui/material 9.4.0, @mui/x-data-grid 9.13.0, @emotion/react 11.14.0, @emotion/styled 11.14.1, @tanstack/react-query 5.102.8, react-router-dom 7.18.3, openapi-typescript 7.13.0, openapi-fetch 0.17.0, vitest 5.0.1, jsdom 30.0.1, msw 2.15.0, @testing-library/react 16.3.3.
- If TypeScript 7.0.2 produces errors inside `node_modules` type definitions rather than in our own code, downgrade to `typescript@5.9.3` and note it in the commit message. Do not work around such errors with `any` or `@ts-ignore`.
- No emoji anywhere — not in UI copy, not in commit messages, not in code comments.
- **All user-facing UI text is in ENGLISH**: labels, buttons, column headers, empty states, error messages, accessible names. Russian remains only in project documentation and commit messages. This reverses an earlier constraint — Tasks 3, 5 and 6 shipped Russian copy and Task 13 retrofits them.
- There are no routes and no separate pages. The application is ONE screen: character in a fixed-width left column, recipes filling the centre, chat as a popup anchored bottom-right. No zone may steal width from the recipe table.
- MUI 9's `Stack` accepts only `children`, `component`, `direction`, `divider`, `spacing`, `sx` and `useFlexGap`. Layout properties such as `alignItems`, `justifyContent` and `flexWrap` are NOT forwarded as system props — passing them leaks a bogus DOM attribute and the CSS silently never applies. Put them in `sx`. Verified against the installed `@mui/material/Stack/Stack.js` propTypes on 2026-09-15.
- TypeScript runs in `strict` mode. No `any`, no `@ts-ignore`, no non-null assertion (`!`) to silence a real nullability case from the schema.
- No module outside `src/api/` imports `openapi-fetch` or calls `fetch` directly. No module anywhere hand-declares an API response interface — import it from `src/api/schema.d.ts`.
- Money, XP and percentage values reaching the screen go through `src/shared/format.ts`. No inline `toFixed` or `toLocaleString` in components.
- Every task ends with a commit. Commit messages are in Russian, imperative mood, no attribution lines, no mention of Claude or AI.
- The backend currently returns `price: 0` for every snapshot, so `/calc` answers `"no priceable production path"` for every item. This is a known backend defect (spec section 2.1). Tests use MSW fixtures with realistic numbers; the UI must render the empty case as a stated reason, never as a zero margin.

---

## File Structure

```
docker-compose.yml                      Container definition, dev server on 5173
frontend/
  Dockerfile                            Production image: build + nginx
  nginx.conf                            SPA fallback for the production image
  package.json  tsconfig.json  tsconfig.node.json  vite.config.ts  index.html
  scripts/fetch-skill-icons.mjs         One-off download of 29 skill icons
  public/icons/skills/*.png             Committed skill icons
  src/
    main.tsx                            Root render, providers
    App.tsx                             Routes and navigation shell
    api/
      schema.d.ts                       GENERATED from ../combined.yaml — never hand-edited
      client.ts                         The only openapi-fetch instance
      queryKeys.ts                      Query key factory
      queries/health.ts                 useGatewayHealth
      queries/hiscore.ts                usePlayer
      queries/recipes.ts                useSkillRecipes, usePriceableItemIds
      queries/prices.ts                 useLatestPrices, useLiquidityStats
      queries/calc.ts                   useCalcBatch
      queries/chat.ts                   useChatHistory
      queries/auth.ts                   useRegister, useLogin
    auth/AuthProvider.tsx               Token state, localStorage, context
    ws/connection.ts                    Socket lifecycle, reconnect, subscriptions
    ws/WsProvider.tsx                   Single app-wide socket, frame fan-out
    theme/theme.ts                      Dark MUI theme
    assets/skills.ts                    Canonical 29 skills, icon URL resolution
    shared/format.ts                    formatCompact, formatInt, formatPct
    shared/SkillIcon.tsx  shared/ItemIcon.tsx
    shared/QueryState.tsx               Loading / error / empty, three distinct states
    shared/HealthIndicator.tsx          Header gateway status
    features/character/CharacterPage.tsx  SkillTile.tsx  usePlayerPrefs.ts
    features/recipes/RecipesPage.tsx  RecipeFilters.tsx  AssumptionsBar.tsx
    features/recipes/buildRows.ts       Pure row assembly — the testable core
    features/recipes/columns.tsx        DataGrid column definitions
    features/chat/ChatPage.tsx  AuthPanel.tsx  MessageList.tsx  MessageComposer.tsx
    test/setup.ts  test/renderWithProviders.tsx  test/fixtures.ts
    test/msw/server.ts  test/msw/handlers.ts
```

Boundary rule: `features/*` import from `api/queries/*`, `shared/*`, `ws/*` and `auth/*`. They never import `api/client.ts`. `buildRows.ts` imports nothing from React.

---

## Task 1: Toolchain, container, generated API types

**Files:**
- Create: `docker-compose.yml`
- Create: `frontend/package.json`, `frontend/tsconfig.json`, `frontend/tsconfig.node.json`, `frontend/vite.config.ts`, `frontend/index.html`, `frontend/.dockerignore`, `.gitignore`
- Create: `frontend/src/api/client.ts`, `frontend/src/test/setup.ts`, `frontend/src/test/msw/server.ts`
- Test: `frontend/src/api/client.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `api` (an `openapi-fetch` client typed by `paths`), `API_URL: string`, `src/api/schema.d.ts` exporting `paths` and `components`. Later tasks import response types as `components['schemas']['Player']` and so on. `src/test/msw/server.ts` exports `server` (an MSW `SetupServerApi`) already wired into the global test lifecycle.

- [ ] **Step 1: Write the container and package manifest**

`docker-compose.yml` at the repository root:

```yaml
services:
  frontend:
    image: node:22-alpine
    working_dir: /app/frontend
    command: sh -c "npm install && npm run dev -- --host 0.0.0.0"
    environment:
      VITE_API_URL: http://localhost:8080
    ports:
      - "5173:5173"
    volumes:
      - .:/app
```

The whole repository is mounted, not just `frontend/`, so that `npm run gen:api` can read `../combined.yaml`.

`frontend/package.json`:

```json
{
  "name": "rs3-market-frontend",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview",
    "test": "vitest run",
    "test:watch": "vitest",
    "gen:api": "openapi-typescript ../combined.yaml -o src/api/schema.d.ts",
    "fetch:icons": "node scripts/fetch-skill-icons.mjs"
  },
  "dependencies": {
    "@emotion/react": "11.14.0",
    "@emotion/styled": "11.14.1",
    "@mui/material": "9.4.0",
    "@mui/x-data-grid": "9.13.0",
    "@tanstack/react-query": "5.102.8",
    "openapi-fetch": "0.17.0",
    "react": "19.3.0",
    "react-dom": "19.3.0",
    "react-router-dom": "7.18.3"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "6.9.1",
    "@testing-library/react": "16.3.3",
    "@testing-library/user-event": "14.6.1",
    "@types/react": "19.3.0",
    "@types/react-dom": "19.3.0",
    "@vitejs/plugin-react": "6.1.1",
    "jsdom": "30.0.1",
    "msw": "2.15.0",
    "openapi-typescript": "7.13.0",
    "typescript": "7.0.2",
    "vite": "8.3.0",
    "vitest": "5.0.1"
  }
}
```

If npm reports that an exact version of `@testing-library/jest-dom` or `@testing-library/user-event` does not exist, install the current release of that package and pin whatever version npm resolved.

`.gitignore` at the repository root:

```
node_modules/
frontend/dist/
frontend/.vite/
```

`frontend/.dockerignore`:

```
node_modules
dist
```

The repository already contains a stray `frontend/.vite/deps/` left over from
an earlier attempt. Delete it — it is not a Vite cache this project produced:

```bash
rm -rf frontend/.vite
```

- [ ] **Step 2: Write the TypeScript and Vite configuration**

`frontend/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "noEmit": true,
    "skipLibCheck": true,
    "types": ["vitest/globals"],
    "isolatedModules": true,
    "verbatimModuleSyntax": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

`frontend/tsconfig.node.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2023"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "noEmit": true,
    "composite": true,
    "skipLibCheck": true
  },
  "include": ["vite.config.ts", "scripts"]
}
```

`frontend/vite.config.ts`:

```ts
/// <reference types="vitest/config" />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: { host: '0.0.0.0', port: 5173 },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
});
```

`frontend/index.html`:

```html
<!doctype html>
<html lang="ru">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>RS3 Market</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 3: Install dependencies and generate the API types**

Run:

```bash
docker compose run --rm frontend npm install
docker compose run --rm frontend npm run gen:api
```

Expected: `frontend/src/api/schema.d.ts` exists and contains `export interface paths` with a `"/calc/batch"` key. Verify:

```bash
docker compose run --rm frontend grep -c '"/calc/batch"' src/api/schema.d.ts
```

Expected: at least `1`.

This file is generated but **is committed**, so that builds and tests never depend on the backend or the network.

- [ ] **Step 4: Write the test harness and the failing client test**

`frontend/src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
import { afterAll, afterEach, beforeAll } from 'vitest';
import { server } from './msw/server';

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
```

`frontend/src/test/msw/server.ts`:

```ts
import { setupServer } from 'msw/node';

export const server = setupServer();
```

`frontend/src/api/client.test.ts`:

```ts
import { http, HttpResponse } from 'msw';
import { expect, it } from 'vitest';
import { server } from '../test/msw/server';
import { api } from './client';

it('reads the gateway health endpoint through the generated types', async () => {
  server.use(
    http.get('http://localhost:8080/health', () =>
      HttpResponse.json({ status: 'ok', service: 'gateway' }),
    ),
  );

  const { data, error } = await api.GET('/health');

  expect(error).toBeUndefined();
  expect(data?.status).toBe('ok');
  expect(data?.service).toBe('gateway');
});
```

- [ ] **Step 5: Run the test to verify it fails**

Run: `docker compose run --rm frontend npm test`
Expected: FAIL — `Cannot find module './client'`.

- [ ] **Step 6: Write the client**

`frontend/src/api/client.ts`:

```ts
import createClient from 'openapi-fetch';
import type { paths } from './schema';

export const API_URL: string = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export const api = createClient<paths>({ baseUrl: API_URL });
```

Add the environment typing at `frontend/src/vite-env.d.ts`:

```ts
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
```

- [ ] **Step 7: Run the test to verify it passes**

Run: `docker compose run --rm frontend npm test`
Expected: PASS, 1 test.

Then confirm the type build is clean:

Run: `docker compose run --rm frontend npx tsc -b`
Expected: no output, exit code 0.

- [ ] **Step 8: Commit**

```bash
git add docker-compose.yml .gitignore frontend/
git commit -m "Поднять тулчейн фронтенда и генерацию типов из OpenAPI"
```

---

## Task 2: Number formatting

**Files:**
- Create: `frontend/src/shared/format.ts`
- Test: `frontend/src/shared/format.test.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `formatCompact(value: number | null | undefined): string`, `formatInt(value: number | null | undefined): string`, `formatPct(value: number | null | undefined, opts?: { sign?: boolean }): string`. All three render absent values as the em dash `—`. Every later task uses these for money, XP, counts and percentages.

- [ ] **Step 1: Write the failing tests**

`frontend/src/shared/format.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { formatCompact, formatInt, formatPct } from './format';

describe('formatCompact', () => {
  it('renders absent values as an em dash', () => {
    expect(formatCompact(null)).toBe('—');
    expect(formatCompact(undefined)).toBe('—');
    expect(formatCompact(Number.NaN)).toBe('—');
  });

  it('renders small values as whole numbers', () => {
    expect(formatCompact(0)).toBe('0');
    expect(formatCompact(307)).toBe('307');
    expect(formatCompact(999.6)).toBe('1000');
  });

  it('abbreviates thousands, millions and billions', () => {
    expect(formatCompact(1000)).toBe('1K');
    expect(formatCompact(12_345)).toBe('12.3K');
    expect(formatCompact(1_234_567)).toBe('1.23M');
    expect(formatCompact(343_000_000)).toBe('343M');
    expect(formatCompact(5_709_998_811)).toBe('5.71B');
  });

  it('keeps the sign on negative values', () => {
    expect(formatCompact(-250)).toBe('-250');
    expect(formatCompact(-12_345)).toBe('-12.3K');
  });
});

describe('formatInt', () => {
  it('groups thousands with a narrow space and handles absent values', () => {
    expect(formatInt(null)).toBe('—');
    expect(formatInt(99)).toBe('99');
    expect(formatInt(171_851)).toBe('171 851');
  });
});

describe('formatPct', () => {
  it('renders one decimal place', () => {
    expect(formatPct(null)).toBe('—');
    expect(formatPct(0)).toBe('0.0%');
    expect(formatPct(12.34)).toBe('12.3%');
    expect(formatPct(-4.5)).toBe('-4.5%');
  });

  it('adds an explicit plus sign when asked', () => {
    expect(formatPct(12.34, { sign: true })).toBe('+12.3%');
    expect(formatPct(-4.5, { sign: true })).toBe('-4.5%');
    expect(formatPct(0, { sign: true })).toBe('0.0%');
  });
});
```

Note on `formatInt`: the separator is U+202F NARROW NO-BREAK SPACE, which is what `Intl.NumberFormat('ru-RU')` produces in Node 22. Write the expectation with a literal `'171 851'` containing that character; if the assertion fails on a different runtime, read the actual separator from the failure output and use it — do not switch to string surgery.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker compose run --rm frontend npx vitest run src/shared/format.test.ts`
Expected: FAIL — `Cannot find module './format'`.

- [ ] **Step 3: Write the implementation**

`frontend/src/shared/format.ts`:

```ts
const ABSENT = '—';

function isAbsent(value: number | null | undefined): value is null | undefined {
  return value === null || value === undefined || Number.isNaN(value);
}

function trim(value: number, digits: number): string {
  return Number(value.toFixed(digits)).toString();
}

export function formatCompact(value: number | null | undefined): string {
  if (isAbsent(value)) return ABSENT;

  const abs = Math.abs(value);
  if (abs < 1_000) return Math.round(value).toString();
  if (abs < 1_000_000) return `${trim(value / 1_000, 1)}K`;
  if (abs < 1_000_000_000) return `${trim(value / 1_000_000, 2)}M`;
  return `${trim(value / 1_000_000_000, 2)}B`;
}

const integerFormat = new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 });

export function formatInt(value: number | null | undefined): string {
  if (isAbsent(value)) return ABSENT;
  return integerFormat.format(value);
}

export function formatPct(
  value: number | null | undefined,
  opts?: { sign?: boolean },
): string {
  if (isAbsent(value)) return ABSENT;
  const body = `${value.toFixed(1)}%`;
  return opts?.sign && value > 0 ? `+${body}` : body;
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/shared/format.test.ts`
Expected: PASS, 8 tests.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/shared/format.ts frontend/src/shared/format.test.ts
git commit -m "Добавить форматирование чисел"
```

---

## Task 3: Dark theme, application shell, routing, gateway health

**Files:**
- Create: `frontend/src/theme/theme.ts`, `frontend/src/main.tsx`, `frontend/src/App.tsx`
- Create: `frontend/src/api/queryKeys.ts`, `frontend/src/api/queries/health.ts`
- Create: `frontend/src/shared/HealthIndicator.tsx`, `frontend/src/shared/QueryState.tsx`
- Create: `frontend/src/api/errors.ts`
- Create: `frontend/src/test/renderWithProviders.tsx`
- Test: `frontend/src/App.test.tsx`, `frontend/src/api/errors.test.ts`

**Interfaces:**
- Consumes: `api` from Task 1, `formatInt` is not needed here.
- Produces:
  - `theme` — a MUI `Theme` with `palette.mode: 'dark'`.
  - `queryKeys` — an object of key factories: `queryKeys.health()`, `queryKeys.player(name, mode)`, `queryKeys.skillRecipes(skill, maxLevel)`, `queryKeys.priceableIds(skill, minLevel, maxLevel)`, `queryKeys.latestPrices(ids)`, `queryKeys.liquidity(itemId)`, `queryKeys.calcBatch(ids, params)`, `queryKeys.chatHistory()`. Every key starts with a literal string tag equal to the factory name.
  - `useGatewayHealth(): UseQueryResult<components['schemas']['GatewayHealth']>`.
  - `describeFailure(status: number, body: unknown): string` in `src/api/errors.ts` — turns an API failure into the message the user sees, distinguishing the three cases spec section 11 requires: a transport failure, a gateway `502` naming the downed service from its `target` field, and a domain refusal carrying the server's own `error` string.
  - `<QueryState query={...} empty={...}>{(data) => ...}</QueryState>` — renders a spinner while pending, the mapped failure message with a retry button, `empty` when the render callback's guard says there is nothing, and otherwise the children.
  - `renderWithProviders(ui: ReactElement, opts?: { route?: string })` — test helper wrapping in theme, QueryClient and MemoryRouter.

- [ ] **Step 1: Write the failing test**

`frontend/src/App.test.tsx`:

```ts
import { http, HttpResponse } from 'msw';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it } from 'vitest';
import App from './App';
import { server } from './test/msw/server';
import { renderWithProviders } from './test/renderWithProviders';

function healthHandler() {
  return http.get('http://localhost:8080/health/all', () =>
    HttpResponse.json({ gateway: 'ok', all_upstreams_ok: true, upstreams: [] }),
  );
}

it('shows the three sections and opens the character page by default', async () => {
  server.use(healthHandler());

  renderWithProviders(<App />, { route: '/' });

  expect(screen.getByRole('link', { name: 'Персонаж' })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Рецепты' })).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Чат' })).toBeInTheDocument();
  expect(await screen.findByRole('heading', { name: 'Персонаж' })).toBeInTheDocument();
});

it('navigates to the recipes section', async () => {
  server.use(healthHandler());

  renderWithProviders(<App />, { route: '/' });
  await userEvent.click(screen.getByRole('link', { name: 'Рецепты' }));

  expect(await screen.findByRole('heading', { name: 'Рецепты' })).toBeInTheDocument();
});

it('reports the failing upstream when the gateway is degraded', async () => {
  server.use(
    http.get('http://localhost:8080/health/all', () =>
      HttpResponse.json({
        gateway: 'ok',
        all_upstreams_ok: false,
        upstreams: [
          { target: 'http://recipe-service:8082', ok: false, status: 0, error: 'connection refused', ms: 1 },
        ],
      }),
    ),
  );

  renderWithProviders(<App />, { route: '/' });

  expect(await screen.findByText(/recipe-service/)).toBeInTheDocument();
});
```

The file must be named `App.test.tsx`, not `.ts`, because it contains JSX.

`frontend/src/api/errors.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { describeFailure } from './errors';

describe('describeFailure', () => {
  it('names the downed service when the gateway answers 502', () => {
    const message = describeFailure(
      502,
      {
        error: 'upstream unavailable',
        target: 'http://recipe-service:8082',
        details: 'dial tcp 172.18.0.5:8082: connect: connection refused',
      },
      'Запрос не выполнен',
    );

    expect(message).toBe('Сервис recipe-service недоступен');
  });

  it('stays useful when a 502 carries no target', () => {
    expect(describeFailure(502, {}, 'Запрос не выполнен')).toBe('Один из сервисов недоступен');
  });

  it('passes a domain refusal through unchanged', () => {
    expect(
      describeFailure(404, { error: 'no recipe for item 1513' }, 'Запрос не выполнен'),
    ).toBe('no recipe for item 1513');
  });

  it('reports a transport failure separately from an HTTP answer', () => {
    expect(describeFailure(0, undefined, 'Запрос не выполнен')).toBe('Нет связи со шлюзом');
  });

  it('uses the caller fallback when the body explains nothing', () => {
    expect(describeFailure(500, 'plain text', 'Запрос не выполнен')).toBe('Запрос не выполнен');
  });
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker compose run --rm frontend npx vitest run src/App.test.tsx src/api/errors.test.ts`
Expected: FAIL — `Cannot find module './App'` and `Cannot find module './errors'`.

- [ ] **Step 3: Write the theme**

`frontend/src/theme/theme.ts`:

```ts
import { createTheme } from '@mui/material/styles';

export const theme = createTheme({
  palette: {
    mode: 'dark',
    background: { default: '#14171c', paper: '#1b1f26' },
    primary: { main: '#c8a24a' },
    success: { main: '#5fb87a' },
    error: { main: '#d1697a' },
    divider: '#2c323c',
    text: { primary: '#e6e8ec', secondary: '#9aa3b0' },
  },
  shape: { borderRadius: 6 },
  typography: {
    fontSize: 13,
    fontFamily: '"Inter", system-ui, -apple-system, "Segoe UI", sans-serif',
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        // Digits line up across table columns only with tabular figures.
        body: { fontVariantNumeric: 'tabular-nums' },
      },
    },
    MuiTableCell: { styleOverrides: { root: { borderColor: '#2c323c' } } },
  },
});
```

- [ ] **Step 4: Write the failure taxonomy**

Spec section 11 asks for three failure states that must not collapse into one
word. `frontend/src/api/errors.ts`:

```ts
interface GatewayFailure {
  error?: unknown;
  target?: unknown;
  details?: unknown;
}

function asRecord(value: unknown): GatewayFailure | null {
  return typeof value === 'object' && value !== null ? (value as GatewayFailure) : null;
}

function serviceName(target: unknown): string | null {
  if (typeof target !== 'string') return null;
  return target.replace(/^https?:\/\//, '').split(':')[0];
}

export function describeFailure(status: number, body: unknown, fallback: string): string {
  const record = asRecord(body);

  // The gateway reports a service it could not reach as 502 plus the target.
  if (status === 502) {
    const name = serviceName(record?.target);
    return name ? `Сервис ${name} недоступен` : 'Один из сервисов недоступен';
  }

  // A transport failure never carries an HTTP status.
  if (status === 0) return 'Нет связи со шлюзом';

  // A server-authored reason is a domain answer, not a breakage: show it as is.
  if (typeof record?.error === 'string') return record.error;

  return fallback;
}

export function failure(response: Response, body: unknown, fallback: string): Error {
  return new Error(describeFailure(response.status, body, fallback));
}
```

Every query function in Tasks 3, 6, 8 and 10 throws through `failure(response, error, '<fallback>')`
rather than `new Error('<fallback>')`, keeping the fallback wording each of them already has.
`openapi-fetch` returns `response` alongside `data` and `error`, so this needs no extra request.

- [ ] **Step 5: Write the query keys and the health query**

`frontend/src/api/queryKeys.ts`:

```ts
import type { components } from './schema';

type HiscoreMode = components['schemas']['HiscoreMode'];

export interface CalcParams {
  player?: string;
  mode?: HiscoreMode;
  spreadPct?: number;
  aph?: number;
  includeIncomplete: boolean;
}

export const queryKeys = {
  health: () => ['health'] as const,
  player: (name: string, mode: HiscoreMode) => ['player', name, mode] as const,
  skillRecipes: (skill: string, maxLevel: number) =>
    ['skillRecipes', skill, maxLevel] as const,
  priceableIds: (skill: string, minLevel: number, maxLevel: number) =>
    ['priceableIds', skill, minLevel, maxLevel] as const,
  latestPrices: (ids: number[]) => ['latestPrices', ids.join(',')] as const,
  liquidity: (itemId: number) => ['liquidity', itemId] as const,
  calcBatch: (ids: number[], params: CalcParams) =>
    ['calcBatch', ids.join(','), params] as const,
  chatHistory: () => ['chatHistory'] as const,
};
```

`frontend/src/api/queries/health.ts`:

```ts
import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type GatewayHealth = components['schemas']['GatewayHealth'];

export function useGatewayHealth() {
  return useQuery({
    queryKey: queryKeys.health(),
    refetchInterval: 60_000,
    queryFn: async (): Promise<GatewayHealth> => {
      const { data, error, response } = await api.GET('/health/all');
      if (error || !data) throw failure(response, error, 'Шлюз недоступен');
      return data;
    },
  });
}
```

- [ ] **Step 6: Write the shared state and health components**

`frontend/src/shared/QueryState.tsx`:

```tsx
import { Alert, Box, Button, CircularProgress, Typography } from '@mui/material';
import type { UseQueryResult } from '@tanstack/react-query';
import type { ReactNode } from 'react';

interface Props<T> {
  query: UseQueryResult<T>;
  empty?: (data: T) => ReactNode;
  children: (data: T) => ReactNode;
}

export function QueryState<T>({ query, empty, children }: Props<T>) {
  if (query.isPending) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
        <CircularProgress size={28} />
      </Box>
    );
  }

  if (query.isError) {
    return (
      <Alert
        severity="error"
        sx={{ my: 2 }}
        action={<Button size="small" onClick={() => void query.refetch()}>Повторить</Button>}
      >
        <Typography variant="body2">{query.error.message}</Typography>
      </Alert>
    );
  }

  const emptyNode = empty?.(query.data);
  return <>{emptyNode ?? children(query.data)}</>;
}
```

`frontend/src/shared/HealthIndicator.tsx`:

```tsx
import { Chip, Tooltip } from '@mui/material';
import { useGatewayHealth } from '../api/queries/health';

function shortName(target: string): string {
  return target.replace(/^https?:\/\//, '').split(':')[0];
}

export function HealthIndicator() {
  const { data, isError } = useGatewayHealth();

  if (isError) return <Chip size="small" color="error" label="Шлюз недоступен" />;
  if (!data) return <Chip size="small" variant="outlined" label="Проверка" />;
  if (data.all_upstreams_ok) return <Chip size="small" color="success" label="Все сервисы в строю" />;

  const down = (data.upstreams ?? []).filter((u) => !u.ok).map((u) => shortName(u.target));
  return (
    <Tooltip title="Часть данных будет недоступна">
      <Chip size="small" color="warning" label={`Недоступны: ${down.join(', ')}`} />
    </Tooltip>
  );
}
```

- [ ] **Step 7: Write the shell, routes and entry point**

`frontend/src/App.tsx`:

```tsx
import { AppBar, Box, Button, Container, Stack, Toolbar, Typography } from '@mui/material';
import { NavLink, Navigate, Route, Routes } from 'react-router-dom';
import { HealthIndicator } from './shared/HealthIndicator';
import { CharacterPage } from './features/character/CharacterPage';
import { RecipesPage } from './features/recipes/RecipesPage';
import { ChatPage } from './features/chat/ChatPage';

const NAV = [
  { to: '/character', label: 'Персонаж' },
  { to: '/recipes', label: 'Рецепты' },
  { to: '/chat', label: 'Чат' },
];

export default function App() {
  return (
    <Box sx={{ minHeight: '100vh' }}>
      <AppBar position="static" color="transparent" elevation={0}>
        <Toolbar sx={{ gap: 3, borderBottom: 1, borderColor: 'divider' }}>
          <Typography variant="h6" sx={{ color: 'primary.main', fontWeight: 700 }}>
            RS3 Market
          </Typography>
          <Stack direction="row" spacing={1} sx={{ flexGrow: 1 }}>
            {NAV.map((item) => (
              <Button key={item.to} component={NavLink} to={item.to} color="inherit" size="small">
                {item.label}
              </Button>
            ))}
          </Stack>
          <HealthIndicator />
        </Toolbar>
      </AppBar>

      <Container maxWidth={false} sx={{ py: 3 }}>
        <Routes>
          <Route path="/" element={<Navigate to="/character" replace />} />
          <Route path="/character" element={<CharacterPage />} />
          <Route path="/recipes" element={<RecipesPage />} />
          <Route path="/chat" element={<ChatPage />} />
        </Routes>
      </Container>
    </Box>
  );
}
```

Create the three pages as minimal headings for now; Tasks 6, 8 and 11 fill them in.

`frontend/src/features/character/CharacterPage.tsx`:

```tsx
import { Typography } from '@mui/material';

export function CharacterPage() {
  return <Typography variant="h5" component="h1">Персонаж</Typography>;
}
```

`frontend/src/features/recipes/RecipesPage.tsx`:

```tsx
import { Typography } from '@mui/material';

export function RecipesPage() {
  return <Typography variant="h5" component="h1">Рецепты</Typography>;
}
```

`frontend/src/features/chat/ChatPage.tsx`:

```tsx
import { Typography } from '@mui/material';

export function ChatPage() {
  return <Typography variant="h5" component="h1">Чат</Typography>;
}
```

`frontend/src/main.tsx`:

```tsx
import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import App from './App';
import { theme } from './theme/theme';

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30_000, refetchOnWindowFocus: false } },
});

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
```

The `!` on `getElementById` is the one permitted exception to the non-null rule: the element is in `index.html` and its absence is a build error, not a runtime case.

- [ ] **Step 8: Write the test helper**

`frontend/src/test/renderWithProviders.tsx`:

```tsx
import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactElement } from 'react';
import { MemoryRouter } from 'react-router-dom';
import { theme } from '../theme/theme';

export function renderWithProviders(ui: ReactElement, opts?: { route?: string }) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={theme}>
        <MemoryRouter initialEntries={[opts?.route ?? '/']}>{ui}</MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>,
  );
}
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/App.test.tsx`
Expected: PASS, 3 tests.

- [ ] **Step 10: Commit**

```bash
git add frontend/src frontend/index.html
git commit -m "Добавить тёмную тему, оболочку приложения и маршруты"
```

---

## Task 4: Skill catalogue, skill icons and item icons

**Files:**
- Create: `frontend/src/assets/skills.ts`, `frontend/scripts/fetch-skill-icons.mjs`
- Create: `frontend/src/shared/SkillIcon.tsx`, `frontend/src/shared/ItemIcon.tsx`
- Create: `frontend/public/icons/skills/*.png` (downloaded, committed)
- Test: `frontend/src/assets/skills.test.ts`, `frontend/src/shared/ItemIcon.test.tsx`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `SKILLS: readonly string[]` — the 29 canonical skill names in game order, excluding `Overall`.
  - `canonicalSkill(raw: string): string | null` — case-insensitive lookup; returns `null` for `No`, `no`, `None`, empty strings and anything not a skill.
  - `skillIconUrl(raw: string): string | null` — `/icons/skills/<Skill>-icon.png` or `null`.
  - `itemIconUrl(itemId: number): string`.
  - `<SkillIcon skill={string} size={number} />`, `<ItemIcon itemId={number} name={string} />`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/assets/skills.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { SKILLS, canonicalSkill, itemIconUrl, skillIconUrl } from './skills';

describe('SKILLS', () => {
  it('lists the 29 skills without Overall', () => {
    expect(SKILLS).toHaveLength(29);
    expect(SKILLS).toContain('Crafting');
    expect(SKILLS).toContain('Necromancy');
    expect(SKILLS).not.toContain('Overall');
  });
});

describe('canonicalSkill', () => {
  it('repairs the casing the recipe data ships with', () => {
    expect(canonicalSkill('Crafting')).toBe('Crafting');
    expect(canonicalSkill('crafting')).toBe('Crafting');
    expect(canonicalSkill('SMITHING')).toBe('Smithing');
  });

  it('rejects the placeholder values in the recipe data', () => {
    expect(canonicalSkill('No')).toBeNull();
    expect(canonicalSkill('no')).toBeNull();
    expect(canonicalSkill('None')).toBeNull();
    expect(canonicalSkill('')).toBeNull();
  });
});

describe('skillIconUrl', () => {
  it('points at the committed icon file', () => {
    expect(skillIconUrl('crafting')).toBe('/icons/skills/Crafting-icon.png');
  });

  it('returns null for a non-skill', () => {
    expect(skillIconUrl('No')).toBeNull();
  });
});

describe('itemIconUrl', () => {
  it('builds the official item database url', () => {
    expect(itemIconUrl(1603)).toBe(
      'https://secure.runescape.com/m=itemdb_rs/obj_big.gif?id=1603',
    );
  });
});
```

`frontend/src/shared/ItemIcon.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/dom';
import { expect, it } from 'vitest';
import { ItemIcon } from './ItemIcon';

it('renders the item image with the item name as its alternative text', () => {
  render(<ItemIcon itemId={1603} name="Ruby" />);

  const img = screen.getByAltText('Ruby');
  expect(img).toHaveAttribute('src', 'https://secure.runescape.com/m=itemdb_rs/obj_big.gif?id=1603');
  expect(img).toHaveAttribute('loading', 'lazy');
});

it('falls back to a neutral placeholder when the image fails to load', () => {
  render(<ItemIcon itemId={999999} name="Unknown" />);

  fireEvent.error(screen.getByAltText('Unknown'));

  expect(screen.queryByAltText('Unknown')).not.toBeInTheDocument();
  expect(screen.getByTestId('item-icon-fallback')).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker compose run --rm frontend npx vitest run src/assets/skills.test.ts src/shared/ItemIcon.test.tsx`
Expected: FAIL — `Cannot find module './skills'`.

- [ ] **Step 3: Write the skill catalogue**

`frontend/src/assets/skills.ts`:

```ts
// Game order, matching what /hiscore/{name} returns after the Overall entry.
export const SKILLS = [
  'Attack', 'Defence', 'Strength', 'Constitution', 'Ranged', 'Prayer',
  'Magic', 'Cooking', 'Woodcutting', 'Fletching', 'Fishing', 'Firemaking',
  'Crafting', 'Smithing', 'Mining', 'Herblore', 'Agility', 'Thieving',
  'Slayer', 'Farming', 'Runecrafting', 'Hunter', 'Construction', 'Summoning',
  'Dungeoneering', 'Divination', 'Invention', 'Archaeology', 'Necromancy',
] as const;

export type SkillName = (typeof SKILLS)[number];

const BY_LOWER = new Map<string, SkillName>(SKILLS.map((s) => [s.toLowerCase(), s]));

// The recipe table stores 'No', 'no' and 'None' where a skill is unknown,
// alongside real names in mixed case. Everything else is not a skill.
export function canonicalSkill(raw: string): SkillName | null {
  return BY_LOWER.get(raw.trim().toLowerCase()) ?? null;
}

export function skillIconUrl(raw: string): string | null {
  const skill = canonicalSkill(raw);
  return skill ? `/icons/skills/${skill}-icon.png` : null;
}

export function itemIconUrl(itemId: number): string {
  return `https://secure.runescape.com/m=itemdb_rs/obj_big.gif?id=${itemId}`;
}
```

- [ ] **Step 4: Write and run the icon download script**

`frontend/scripts/fetch-skill-icons.mjs`:

```js
// One-off download of the 29 skill icons from the RuneScape Wiki into
// public/icons/skills/. The icons are committed so the running app makes
// no third-party requests and offline development keeps its artwork.
import { mkdir, writeFile } from 'node:fs/promises';
import { SKILLS } from '../src/assets/skills.ts';

const OUT = new URL('../public/icons/skills/', import.meta.url);

await mkdir(OUT, { recursive: true });

for (const skill of SKILLS) {
  const file = `${skill}-icon.png`;
  const response = await fetch(`https://runescape.wiki/images/${file}`, {
    headers: { 'User-Agent': 'rs3-market-frontend/1.0 (asset fetch)' },
  });
  if (!response.ok) {
    throw new Error(`${file}: HTTP ${response.status}`);
  }
  await writeFile(new URL(file, OUT), Buffer.from(await response.arrayBuffer()));
  console.log(`saved ${file}`);
}
```

Node 22 cannot import a `.ts` file directly. Change the import to a literal list inside the script if the run fails:

```js
const SKILLS = ['Attack', 'Defence', 'Strength', 'Constitution', 'Ranged', 'Prayer', 'Magic', 'Cooking', 'Woodcutting', 'Fletching', 'Fishing', 'Firemaking', 'Crafting', 'Smithing', 'Mining', 'Herblore', 'Agility', 'Thieving', 'Slayer', 'Farming', 'Runecrafting', 'Hunter', 'Construction', 'Summoning', 'Dungeoneering', 'Divination', 'Invention', 'Archaeology', 'Necromancy'];
```

Run: `docker compose run --rm frontend npm run fetch:icons`
Expected: 29 lines of `saved <Skill>-icon.png`, no error. All 29 filenames were verified to return HTTP 200 on 2026-09-15.

Verify: `ls frontend/public/icons/skills | wc -l`
Expected: `29`.

`Overall` has no icon on the wiki. The character page renders the overall summary as a text card instead — do not invent a substitute image.

- [ ] **Step 5: Write the icon components**

`frontend/src/shared/SkillIcon.tsx`:

```tsx
import { Box } from '@mui/material';
import { canonicalSkill, skillIconUrl } from '../assets/skills';

interface Props {
  skill: string;
  size?: number;
}

export function SkillIcon({ skill, size = 20 }: Props) {
  const url = skillIconUrl(skill);
  const name = canonicalSkill(skill);

  if (!url || !name) {
    return <Box component="span" sx={{ width: size, height: size, display: 'inline-block' }} />;
  }

  return (
    <Box
      component="img"
      src={url}
      alt={name}
      title={name}
      loading="lazy"
      sx={{ width: size, height: size, objectFit: 'contain', verticalAlign: 'middle' }}
    />
  );
}
```

`frontend/src/shared/ItemIcon.tsx`:

```tsx
import { Box } from '@mui/material';
import { useState } from 'react';
import { itemIconUrl } from '../assets/skills';

interface Props {
  itemId: number;
  name: string;
  size?: number;
}

export function ItemIcon({ itemId, name, size = 28 }: Props) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <Box
        data-testid="item-icon-fallback"
        title={name}
        sx={{
          width: size,
          height: size,
          borderRadius: 1,
          border: 1,
          borderColor: 'divider',
          bgcolor: 'background.default',
        }}
      />
    );
  }

  return (
    <Box
      component="img"
      src={itemIconUrl(itemId)}
      alt={name}
      loading="lazy"
      onError={() => setFailed(true)}
      sx={{ width: size, height: size, objectFit: 'contain' }}
    />
  );
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/assets/skills.test.ts src/shared/ItemIcon.test.tsx`
Expected: PASS, 7 tests.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/assets frontend/src/shared/SkillIcon.tsx frontend/src/shared/ItemIcon.tsx frontend/src/shared/ItemIcon.test.tsx frontend/scripts frontend/public/icons
git commit -m "Добавить каталог скиллов и иконки предметов и скиллов"
```

---

## Task 5: Authentication

**Files:**
- Create: `frontend/src/api/queries/auth.ts`, `frontend/src/auth/AuthProvider.tsx`
- Test: `frontend/src/auth/AuthProvider.test.tsx`

**Interfaces:**
- Consumes: `api` from Task 1.
- Produces:
  - `login(credentials: Credentials): Promise<string>` and `register(credentials: Credentials): Promise<RegisteredUser>` in `api/queries/auth.ts`, where `Credentials = components['schemas']['Credentials']`.
  - `<AuthProvider>` and `useAuth(): AuthContextValue` where

    ```ts
    interface AuthContextValue {
      token: string | null;
      username: string | null;
      pending: boolean;
      error: string | null;
      signIn(credentials: Credentials): Promise<void>;
      signUp(credentials: Credentials): Promise<void>;
      signOut(): void;
    }
    ```
  - localStorage key `rs3.auth` holding `{"token": string, "username": string}`.

Tasks 7, 8 and 12 consume `useAuth()`.

- [ ] **Step 1: Write the failing test**

`frontend/src/auth/AuthProvider.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it } from 'vitest';
import { server } from '../test/msw/server';
import { AuthProvider, useAuth } from './AuthProvider';

function Probe() {
  const auth = useAuth();
  return (
    <div>
      <span data-testid="token">{auth.token ?? 'нет токена'}</span>
      <span data-testid="username">{auth.username ?? 'аноним'}</span>
      <span data-testid="error">{auth.error ?? ''}</span>
      <button onClick={() => void auth.signIn({ username: 'okadishe', password: 'secret123' })}>
        Войти
      </button>
      <button onClick={() => auth.signOut()}>Выйти</button>
    </div>
  );
}

beforeEach(() => localStorage.clear());

it('stores the token after a successful login and restores it on remount', async () => {
  server.use(
    http.post('http://localhost:8080/auth/login', () =>
      HttpResponse.json({ token: 'jwt-abc' }),
    ),
  );

  const { unmount } = render(<AuthProvider><Probe /></AuthProvider>);
  await userEvent.click(screen.getByRole('button', { name: 'Войти' }));

  await waitFor(() => expect(screen.getByTestId('token')).toHaveTextContent('jwt-abc'));
  expect(screen.getByTestId('username')).toHaveTextContent('okadishe');

  unmount();
  render(<AuthProvider><Probe /></AuthProvider>);
  expect(screen.getByTestId('token')).toHaveTextContent('jwt-abc');
});

it('surfaces invalid credentials without storing anything', async () => {
  server.use(
    http.post('http://localhost:8080/auth/login', () =>
      HttpResponse.json({ error: 'invalid credentials' }, { status: 401 }),
    ),
  );

  render(<AuthProvider><Probe /></AuthProvider>);
  await userEvent.click(screen.getByRole('button', { name: 'Войти' }));

  await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('invalid credentials'));
  expect(screen.getByTestId('token')).toHaveTextContent('нет токена');
  expect(localStorage.getItem('rs3.auth')).toBeNull();
});

it('clears the stored session on sign out', async () => {
  server.use(
    http.post('http://localhost:8080/auth/login', () => HttpResponse.json({ token: 'jwt-abc' })),
  );

  render(<AuthProvider><Probe /></AuthProvider>);
  await userEvent.click(screen.getByRole('button', { name: 'Войти' }));
  await waitFor(() => expect(screen.getByTestId('token')).toHaveTextContent('jwt-abc'));

  await userEvent.click(screen.getByRole('button', { name: 'Выйти' }));

  expect(screen.getByTestId('token')).toHaveTextContent('нет токена');
  expect(localStorage.getItem('rs3.auth')).toBeNull();
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker compose run --rm frontend npx vitest run src/auth/AuthProvider.test.tsx`
Expected: FAIL — `Cannot find module './AuthProvider'`.

- [ ] **Step 3: Write the auth requests**

`frontend/src/api/queries/auth.ts`:

```ts
import { api } from '../client';
import { failure } from '../errors';
import type { components } from '../schema';

export type Credentials = components['schemas']['Credentials'];
export type RegisteredUser = components['schemas']['RegisteredUser'];

export async function login(credentials: Credentials): Promise<string> {
  const { data, error, response } = await api.POST('/auth/login', { body: credentials });
  if (error || !data) throw failure(response, error, 'Не удалось войти');
  return data.token;
}

export async function register(credentials: Credentials): Promise<RegisteredUser> {
  const { data, error, response } = await api.POST('/auth/register', { body: credentials });
  if (error || !data) throw failure(response, error, 'Не удалось зарегистрироваться');
  return data;
}
```

- [ ] **Step 4: Write the provider**

`frontend/src/auth/AuthProvider.tsx`:

```tsx
import { createContext, useCallback, useContext, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { login, register } from '../api/queries/auth';
import type { Credentials } from '../api/queries/auth';

const STORAGE_KEY = 'rs3.auth';

interface Session {
  token: string;
  username: string;
}

export interface AuthContextValue {
  token: string | null;
  username: string | null;
  pending: boolean;
  error: string | null;
  signIn(credentials: Credentials): Promise<void>;
  signUp(credentials: Credentials): Promise<void>;
  signOut(): void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function readSession(): Session | null {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (
      parsed && typeof parsed === 'object' &&
      typeof (parsed as Session).token === 'string' &&
      typeof (parsed as Session).username === 'string'
    ) {
      return parsed as Session;
    }
  } catch {
    // A corrupted entry is indistinguishable from no session.
  }
  localStorage.removeItem(STORAGE_KEY);
  return null;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(readSession);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(async (work: () => Promise<Session>) => {
    setPending(true);
    setError(null);
    try {
      const next = await work();
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
      setSession(next);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Неизвестная ошибка');
    } finally {
      setPending(false);
    }
  }, []);

  const signIn = useCallback(
    (credentials: Credentials) =>
      run(async () => ({
        token: await login(credentials),
        username: credentials.username,
      })),
    [run],
  );

  const signUp = useCallback(
    (credentials: Credentials) =>
      run(async () => {
        await register(credentials);
        return { token: await login(credentials), username: credentials.username };
      }),
    [run],
  );

  const signOut = useCallback(() => {
    localStorage.removeItem(STORAGE_KEY);
    setSession(null);
    setError(null);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      token: session?.token ?? null,
      username: session?.username ?? null,
      pending,
      error,
      signIn,
      signUp,
      signOut,
    }),
    [session, pending, error, signIn, signUp, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth используется вне AuthProvider');
  return value;
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `docker compose run --rm frontend npx vitest run src/auth/AuthProvider.test.tsx`
Expected: PASS, 3 tests.

- [ ] **Step 6: Wire the provider into the application**

In `frontend/src/main.tsx`, wrap `<App />` with `<AuthProvider>` inside `<BrowserRouter>`:

```tsx
import { AuthProvider } from './auth/AuthProvider';
// ...
<BrowserRouter>
  <AuthProvider>
    <App />
  </AuthProvider>
</BrowserRouter>
```

Add the same wrapper to `renderWithProviders` in `frontend/src/test/renderWithProviders.tsx`, inside `MemoryRouter`, so page tests can call `useAuth()`.

Run: `docker compose run --rm frontend npm test`
Expected: PASS, all tests so far.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/auth frontend/src/api/queries/auth.ts frontend/src/main.tsx frontend/src/test/renderWithProviders.tsx
git commit -m "Добавить регистрацию, вход и хранение сессии"
```

---

## Task 6: Character page

**Files:**
- Create: `frontend/src/api/queries/hiscore.ts`
- Create: `frontend/src/features/character/usePlayerPrefs.ts`, `SkillTile.tsx`
- Modify: `frontend/src/features/character/CharacterPage.tsx` (replace the Task 3 placeholder)
- Test: `frontend/src/features/character/CharacterPage.test.tsx`

**Interfaces:**
- Consumes: `QueryState`, `SkillIcon`, `formatInt`, `formatCompact`, `queryKeys`, `api`.
- Produces:
  - `usePlayer(name: string, mode: HiscoreMode): UseQueryResult<Player>` — disabled while `name` is empty.
  - `usePlayerPrefs(): { name: string; mode: HiscoreMode; setName(v: string): void; setMode(v: HiscoreMode): void }`, persisted at localStorage key `rs3.player`. Task 10 reads the same hook to pass `player` and `mode` into `/calc/batch`.
  - `<SkillTile skill={PlayerSkill} />`.

- [ ] **Step 1: Write the failing test**

`frontend/src/features/character/CharacterPage.test.tsx`:

```tsx
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it } from 'vitest';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { CharacterPage } from './CharacterPage';

const PLAYER = {
  id: 2,
  name: 'Zezima',
  mode: 'normal',
  fetched_at: '2026-09-15T10:24:24.873Z',
  skills: [
    { id: 421, player_id: 2, skill: 'Overall', level: 3232, xp: 5709998811, rank: 6520 },
    { id: 422, player_id: 2, skill: 'Attack', level: 120, xp: 200000000, rank: 351 },
    { id: 435, player_id: 2, skill: 'Crafting', level: 99, xp: 13034431, rank: 1200 },
  ],
};

beforeEach(() => localStorage.clear());

it('shows the overall summary and one tile per skill', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));

  expect(await screen.findByText('3232')).toBeInTheDocument();
  expect(screen.getByRole('img', { name: 'Crafting' })).toBeInTheDocument();
  expect(screen.getByText('Crafting')).toBeInTheDocument();
  expect(screen.queryByText('Overall')).not.toBeInTheDocument();
});

it('always shows when the hiscore copy was fetched', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));

  expect(await screen.findByText(/Данные получены/)).toBeInTheDocument();
});

it('reports a missing player as the API describes it, not as a crash', async () => {
  server.use(
    http.get('http://localhost:8080/hiscore/Nobody', () =>
      HttpResponse.json({ error: 'player not found' }, { status: 404 }),
    ),
  );

  renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Nobody');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));

  expect(
    await screen.findByText('Игрок не найден или хайскоры недоступны'),
  ).toBeInTheDocument();
});

it('remembers the last name across mounts', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  const first = renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));
  await screen.findByText('3232');
  first.unmount();

  renderWithProviders(<CharacterPage />);
  expect(screen.getByLabelText('Имя персонажа')).toHaveValue('Zezima');
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker compose run --rm frontend npx vitest run src/features/character/CharacterPage.test.tsx`
Expected: FAIL — the placeholder page has no `Имя персонажа` field.

- [ ] **Step 3: Write the hiscore query**

`frontend/src/api/queries/hiscore.ts`:

```ts
import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type Player = components['schemas']['Player'];
export type PlayerSkill = components['schemas']['PlayerSkill'];
export type HiscoreMode = components['schemas']['HiscoreMode'];

export function usePlayer(name: string, mode: HiscoreMode) {
  return useQuery({
    queryKey: queryKeys.player(name, mode),
    enabled: name.trim().length > 0,
    staleTime: 5 * 60_000,
    queryFn: async (): Promise<Player> => {
      const { data, error, response } = await api.GET('/hiscore/{name}', {
        params: { path: { name }, query: { mode } },
      });
      if (response.status === 404) {
        throw new Error('Игрок не найден или хайскоры недоступны');
      }
      if (error || !data) throw failure(response, error, 'Не удалось прочитать хайскоры');
      return data;
    },
  });
}
```

- [ ] **Step 4: Write the preferences hook and the skill tile**

`frontend/src/features/character/usePlayerPrefs.ts`:

```ts
import { useCallback, useState } from 'react';
import type { HiscoreMode } from '../../api/queries/hiscore';

const STORAGE_KEY = 'rs3.player';

interface Prefs {
  name: string;
  mode: HiscoreMode;
}

const EMPTY: Prefs = { name: '', mode: 'normal' };

function read(): Prefs {
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return EMPTY;
  try {
    const parsed: unknown = JSON.parse(raw);
    if (parsed && typeof parsed === 'object' && typeof (parsed as Prefs).name === 'string') {
      return { name: (parsed as Prefs).name, mode: (parsed as Prefs).mode ?? 'normal' };
    }
  } catch {
    // Fall through to the empty preferences below.
  }
  return EMPTY;
}

export function usePlayerPrefs() {
  const [prefs, setPrefs] = useState<Prefs>(read);

  const save = useCallback((next: Prefs) => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
    setPrefs(next);
  }, []);

  return {
    name: prefs.name,
    mode: prefs.mode,
    setName: useCallback((name: string) => save({ ...prefs, name }), [prefs, save]),
    setMode: useCallback((mode: HiscoreMode) => save({ ...prefs, mode }), [prefs, save]),
  };
}
```

`frontend/src/features/character/SkillTile.tsx`:

```tsx
import { Paper, Stack, Typography } from '@mui/material';
import type { PlayerSkill } from '../../api/queries/hiscore';
import { SkillIcon } from '../../shared/SkillIcon';
import { formatCompact, formatInt } from '../../shared/format';

export function SkillTile({ skill }: { skill: PlayerSkill }) {
  return (
    <Paper variant="outlined" sx={{ p: 1.5 }}>
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center' }}>
        <SkillIcon skill={skill.skill} size={24} />
        <Stack sx={{ minWidth: 0 }}>
          <Typography variant="body2" noWrap>{skill.skill}</Typography>
          <Typography variant="h6" component="p" sx={{ lineHeight: 1.2 }}>
            {formatInt(skill.level)}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {formatCompact(skill.xp)} опыта, ранг {formatInt(skill.rank)}
          </Typography>
        </Stack>
      </Stack>
    </Paper>
  );
}
```

- [ ] **Step 5: Write the page**

`frontend/src/features/character/CharacterPage.tsx` (replacing the placeholder):

```tsx
import { Box, Button, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import { useState } from 'react';
import { usePlayer } from '../../api/queries/hiscore';
import type { HiscoreMode } from '../../api/queries/hiscore';
import { QueryState } from '../../shared/QueryState';
import { formatCompact, formatInt } from '../../shared/format';
import { SkillTile } from './SkillTile';
import { usePlayerPrefs } from './usePlayerPrefs';

const MODES: { value: HiscoreMode; label: string }[] = [
  { value: 'normal', label: 'Обычный' },
  { value: 'ironman', label: 'Ironman' },
  { value: 'hardcore', label: 'Hardcore ironman' },
];

export function CharacterPage() {
  const prefs = usePlayerPrefs();
  const [draft, setDraft] = useState(prefs.name);
  const query = usePlayer(prefs.name, prefs.mode);

  return (
    <Stack spacing={3}>
      <Typography variant="h5" component="h1">Персонаж</Typography>

      <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        <TextField
          label="Имя персонажа"
          size="small"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
        />
        <TextField
          select
          label="Режим"
          size="small"
          value={prefs.mode}
          onChange={(event) => prefs.setMode(event.target.value as HiscoreMode)}
          sx={{ minWidth: 200 }}
        >
          {MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>{mode.label}</MenuItem>
          ))}
        </TextField>
        <Button variant="contained" onClick={() => prefs.setName(draft.trim())}>
          Показать
        </Button>
      </Stack>

      {prefs.name === '' ? (
        <Typography color="text.secondary">
          Введите имя персонажа, чтобы увидеть уровни. Оно же подставится в расчёт рецептов.
        </Typography>
      ) : (
        <QueryState query={query}>
          {(player) => {
            const overall = player.skills.find((entry) => entry.skill === 'Overall');
            const skills = player.skills.filter((entry) => entry.skill !== 'Overall');

            return (
              <Stack spacing={2}>
                <Paper variant="outlined" sx={{ p: 2 }}>
                  <Typography variant="body2" color="text.secondary">
                    {player.name}, суммарный уровень
                  </Typography>
                  <Typography variant="h4" component="p">{formatInt(overall?.level)}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {formatCompact(overall?.xp)} опыта, ранг {formatInt(overall?.rank)}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Данные получены {new Date(player.fetched_at).toLocaleString('ru-RU')}.
                    Если хайскоры недоступны, сервер отдаёт последнюю сохранённую копию.
                  </Typography>
                </Paper>

                <Box
                  sx={{
                    display: 'grid',
                    gap: 1.5,
                    gridTemplateColumns: 'repeat(auto-fill, minmax(190px, 1fr))',
                  }}
                >
                  {skills.map((skill) => (
                    <SkillTile key={skill.skill} skill={skill} />
                  ))}
                </Box>
              </Stack>
            );
          }}
        </QueryState>
      )}
    </Stack>
  );
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/features/character/CharacterPage.test.tsx`
Expected: PASS, 4 tests.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/features/character frontend/src/api/queries/hiscore.ts
git commit -m "Добавить страницу персонажа с уровнями скиллов"
```

---

## Task 7: WebSocket connection

**Files:**
- Create: `frontend/src/ws/connection.ts`
- Test: `frontend/src/ws/connection.test.ts`

**Interfaces:**
- Consumes: `components['schemas']` types from Task 1.
- Produces:

  ```ts
  export type WsStatus = 'connecting' | 'open' | 'closed' | 'unauthorized';

  export interface WebSocketLike {
    send(data: string): void;
    close(code?: number): void;
    onopen: (() => void) | null;
    onmessage: ((event: { data: string }) => void) | null;
    onclose: ((event: { code: number }) => void) | null;
    onerror: (() => void) | null;
  }

  export interface WsHandlers {
    onChat(message: ChatMessage): void;
    onPrice(snapshot: PriceSnapshot): void;
    onStatus(status: WsStatus): void;
    onServerError(message: string): void;
  }

  export interface WsConnectionOptions {
    url: string;
    token: string;
    handlers: WsHandlers;
    socketFactory?: (url: string) => WebSocketLike;
    reconnectDelays?: number[];
    scheduleReconnect?: (run: () => void, ms: number) => void;
  }

  export interface WsConnection {
    subscribe(itemIds: number[]): void;
    unsubscribe(itemIds: number[]): void;
    sendChat(body: string): void;
    close(): void;
  }

  export function createWsConnection(opts: WsConnectionOptions): WsConnection;
  ```

  `ChatMessage` and `PriceSnapshot` are re-exported from this module as `components['schemas']['ChatMessage']` and `components['schemas']['PriceSnapshot']`. Task 8 and Task 12 consume them.

Behaviour contract, all of it tested below:

- Frames sent before the socket opens are buffered and flushed on open.
- The set of subscribed item IDs is remembered and re-sent after a reconnect. Reconnects do not duplicate subscriptions.
- Close code `1008` means the token was rejected: status becomes `unauthorized` and no reconnect is attempted. The spec documents a 401 on the upgrade; browsers surface that as an abnormal close, so the delay list is finite and exhausting it ends in `closed` rather than an endless retry loop.
- `close()` is deliberate and never reconnects.

- [ ] **Step 1: Write the failing tests**

`frontend/src/ws/connection.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createWsConnection } from './connection';
import type { WebSocketLike, WsHandlers, WsStatus } from './connection';

class FakeSocket implements WebSocketLike {
  static instances: FakeSocket[] = [];

  sent: string[] = [];
  closedWith: number | undefined;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) {
    FakeSocket.instances.push(this);
  }

  send(data: string) { this.sent.push(data); }
  close(code?: number) { this.closedWith = code; }

  open() { this.onopen?.(); }
  deliver(frame: unknown) { this.onmessage?.({ data: JSON.stringify(frame) }); }
  drop(code: number) { this.onclose?.({ code }); }

  get parsed(): unknown[] { return this.sent.map((raw) => JSON.parse(raw)); }
}

function harness(overrides: Partial<WsHandlers> = {}) {
  const chats: unknown[] = [];
  const prices: unknown[] = [];
  const statuses: WsStatus[] = [];
  const errors: string[] = [];
  const pending: (() => void)[] = [];

  const connection = createWsConnection({
    url: 'ws://localhost:8080/ws',
    token: 'jwt-abc',
    socketFactory: (url) => new FakeSocket(url),
    reconnectDelays: [10, 20],
    scheduleReconnect: (run) => { pending.push(run); },
    handlers: {
      onChat: (m) => chats.push(m),
      onPrice: (p) => prices.push(p),
      onStatus: (s) => statuses.push(s),
      onServerError: (m) => errors.push(m),
      ...overrides,
    },
  });

  return {
    connection,
    chats,
    prices,
    statuses,
    errors,
    runReconnect: () => pending.shift()?.(),
    socket: (index = 0) => FakeSocket.instances[index],
    socketCount: () => FakeSocket.instances.length,
  };
}

beforeEach(() => { FakeSocket.instances = []; });

describe('createWsConnection', () => {
  it('puts the token in the query string', () => {
    const h = harness();
    expect(h.socket().url).toBe('ws://localhost:8080/ws?token=jwt-abc');
  });

  it('buffers commands issued before the socket opens', () => {
    const h = harness();
    h.connection.subscribe([1603, 1605]);
    expect(h.socket().sent).toEqual([]);

    h.socket().open();

    expect(h.socket().parsed).toEqual([{ action: 'subscribe', item_ids: [1603, 1605] }]);
  });

  it('routes chat, price and error frames to their handlers', () => {
    const h = harness();
    h.socket().open();

    h.socket().deliver({ type: 'chat', payload: { id: 1, user_id: 2, username: 'okadishe', body: 'Hey', created_at: '2026-09-15T07:06:48.728Z' } });
    h.socket().deliver({ type: 'price', payload: { id: 9, item_id: 1603, ts: '2026-09-15T09:53:33.001Z', price: 307, volume: 171851 } });
    h.socket().deliver({ type: 'heartbeat', payload: { ts: '2026-09-15T09:54:00.000Z' } });
    h.socket().deliver({ type: 'error', payload: { message: 'unknown action' } });

    expect(h.chats).toHaveLength(1);
    expect(h.prices).toEqual([
      { id: 9, item_id: 1603, ts: '2026-09-15T09:53:33.001Z', price: 307, volume: 171851 },
    ]);
    expect(h.errors).toEqual(['unknown action']);
  });

  it('ignores malformed frames instead of throwing', () => {
    const h = harness();
    h.socket().open();

    h.socket().onmessage?.({ data: 'not json' });
    h.socket().deliver({ type: 'price' });

    expect(h.prices).toEqual([]);
    expect(h.errors).toEqual([]);
  });

  it('restores subscriptions after a reconnect without duplicating them', () => {
    const h = harness();
    h.socket().open();
    h.connection.subscribe([1603]);
    h.connection.subscribe([1605, 1603]);
    h.connection.unsubscribe([1603]);

    h.socket().drop(1006);
    h.runReconnect();
    h.socket(1).open();

    expect(h.socketCount()).toBe(2);
    expect(h.socket(1).parsed).toEqual([{ action: 'subscribe', item_ids: [1605] }]);
  });

  it('stops after the reconnect delays are exhausted', () => {
    const h = harness();
    h.socket().open();

    h.socket().drop(1006);
    h.runReconnect();
    h.socket(1).drop(1006);
    h.runReconnect();
    h.socket(2).drop(1006);

    expect(h.socketCount()).toBe(3);
    expect(h.statuses.at(-1)).toBe('closed');
  });

  it('treats a policy close as a rejected token and does not reconnect', () => {
    const h = harness();
    h.socket().open();

    h.socket().drop(1008);
    h.runReconnect();

    expect(h.socketCount()).toBe(1);
    expect(h.statuses.at(-1)).toBe('unauthorized');
  });

  it('does not reconnect after a deliberate close', () => {
    const h = harness();
    h.socket().open();

    h.connection.close();
    h.socket().drop(1000);
    h.runReconnect();

    expect(h.socketCount()).toBe(1);
  });

  it('sends chat messages as command frames', () => {
    const h = harness();
    h.socket().open();

    h.connection.sendChat('Привет');

    expect(h.socket().parsed).toEqual([{ action: 'chat', body: 'Привет' }]);
  });

  it('reports the open status once the socket connects', () => {
    const h = harness();
    expect(h.statuses).toEqual(['connecting']);
    h.socket().open();
    expect(h.statuses).toEqual(['connecting', 'open']);
  });

  it('is unused but kept honest about vi', () => {
    expect(vi).toBeDefined();
  });
});
```

Remove the final placeholder test and the `vi` import if the linter objects to the unused import; it exists only so the import list matches the harness conventions of the other suites.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker compose run --rm frontend npx vitest run src/ws/connection.test.ts`
Expected: FAIL — `Cannot find module './connection'`.

- [ ] **Step 3: Write the connection**

`frontend/src/ws/connection.ts`:

```ts
import type { components } from '../api/schema';

export type ChatMessage = components['schemas']['ChatMessage'];
export type PriceSnapshot = components['schemas']['PriceSnapshot'];
export type WsCommand = components['schemas']['WsCommand'];

export type WsStatus = 'connecting' | 'open' | 'closed' | 'unauthorized';

export interface WebSocketLike {
  send(data: string): void;
  close(code?: number): void;
  onopen: (() => void) | null;
  onmessage: ((event: { data: string }) => void) | null;
  onclose: ((event: { code: number }) => void) | null;
  onerror: (() => void) | null;
}

export interface WsHandlers {
  onChat(message: ChatMessage): void;
  onPrice(snapshot: PriceSnapshot): void;
  onStatus(status: WsStatus): void;
  onServerError(message: string): void;
}

export interface WsConnectionOptions {
  url: string;
  token: string;
  handlers: WsHandlers;
  socketFactory?: (url: string) => WebSocketLike;
  reconnectDelays?: number[];
  scheduleReconnect?: (run: () => void, ms: number) => void;
}

export interface WsConnection {
  subscribe(itemIds: number[]): void;
  unsubscribe(itemIds: number[]): void;
  sendChat(body: string): void;
  close(): void;
}

// The gateway rejects a bad token by refusing the upgrade, which browsers
// report as an abnormal close rather than a status code. A finite delay
// list is what keeps that from becoming an endless retry loop.
const DEFAULT_DELAYS = [1_000, 2_000, 5_000, 10_000, 30_000];
const POLICY_VIOLATION = 1008;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

export function createWsConnection(opts: WsConnectionOptions): WsConnection {
  const {
    url,
    token,
    handlers,
    socketFactory = (target) => new WebSocket(target) as unknown as WebSocketLike,
    reconnectDelays = DEFAULT_DELAYS,
    scheduleReconnect = (run, ms) => { setTimeout(run, ms); },
  } = opts;

  const subscriptions = new Set<number>();
  let socket: WebSocketLike | null = null;
  let outbox: WsCommand[] = [];
  let attempt = 0;
  let opened = false;
  let disposed = false;

  function transmit(command: WsCommand) {
    if (socket && opened) {
      socket.send(JSON.stringify(command));
    } else {
      outbox.push(command);
    }
  }

  function handleFrame(raw: string) {
    let frame: unknown;
    try {
      frame = JSON.parse(raw);
    } catch {
      return;
    }
    if (!isRecord(frame) || !isRecord(frame.payload)) return;

    switch (frame.type) {
      case 'chat':
        handlers.onChat(frame.payload as ChatMessage);
        return;
      case 'price':
        handlers.onPrice(frame.payload as PriceSnapshot);
        return;
      case 'error': {
        const message = frame.payload.message;
        if (typeof message === 'string') handlers.onServerError(message);
        return;
      }
      default:
        // heartbeat and anything the server adds later need no action.
        return;
    }
  }

  function connect() {
    opened = false;
    handlers.onStatus('connecting');

    const next = socketFactory(`${url}?token=${encodeURIComponent(token)}`);
    socket = next;

    next.onopen = () => {
      opened = true;
      attempt = 0;
      handlers.onStatus('open');

      if (subscriptions.size > 0) {
        next.send(JSON.stringify({ action: 'subscribe', item_ids: [...subscriptions] }));
      }
      const queued = outbox;
      outbox = [];
      for (const command of queued) next.send(JSON.stringify(command));
    };

    next.onmessage = (event) => handleFrame(event.data);

    next.onclose = (event) => {
      opened = false;
      if (disposed) return;

      if (event.code === POLICY_VIOLATION) {
        handlers.onStatus('unauthorized');
        return;
      }
      const delay = reconnectDelays[attempt];
      if (delay === undefined) {
        handlers.onStatus('closed');
        return;
      }
      attempt += 1;
      scheduleReconnect(connect, delay);
    };

    next.onerror = () => { /* onclose always follows; the status moves there. */ };
  }

  connect();

  return {
    subscribe(itemIds) {
      const added = itemIds.filter((id) => !subscriptions.has(id));
      for (const id of added) subscriptions.add(id);
      if (added.length > 0) transmit({ action: 'subscribe', item_ids: added });
    },

    unsubscribe(itemIds) {
      const removed = itemIds.filter((id) => subscriptions.delete(id));
      if (removed.length > 0) transmit({ action: 'unsubscribe', item_ids: removed });
    },

    sendChat(body) {
      transmit({ action: 'chat', body });
    },

    close() {
      disposed = true;
      socket?.close(1000);
      socket = null;
    },
  };
}
```

Note the subscription-restore test: `subscribe([1603])` then `subscribe([1605, 1603])` sends only the new ID, and `unsubscribe([1603])` leaves `{1605}`, which is what the second socket re-sends as one frame.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/ws/connection.test.ts`
Expected: PASS, 10 tests.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/ws
git commit -m "Добавить WebSocket-соединение с переподключением и подписками"
```

---

## Task 8: WebSocket provider and chat page

**Files:**
- Create: `frontend/src/ws/WsProvider.tsx`
- Create: `frontend/src/api/queries/chat.ts`
- Create: `frontend/src/features/chat/AuthPanel.tsx`, `MessageList.tsx`, `MessageComposer.tsx`
- Modify: `frontend/src/features/chat/ChatPage.tsx` (replace the Task 3 placeholder)
- Modify: `frontend/src/main.tsx`, `frontend/src/test/renderWithProviders.tsx`
- Test: `frontend/src/features/chat/ChatPage.test.tsx`

**Interfaces:**
- Consumes: `createWsConnection`, `useAuth`, `QueryState`, `formatInt` is not needed.
- Produces:

  ```ts
  interface WsContextValue {
    status: WsStatus;
    messages: ChatMessage[];
    serverError: string | null;
    sendChat(body: string): void;
    subscribe(itemIds: number[]): void;
    unsubscribe(itemIds: number[]): void;
    onPrice(listener: (snapshot: PriceSnapshot) => void): () => void;
  }
  export function useWs(): WsContextValue;
  ```

  `WsProvider` accepts an optional `socketFactory` prop so tests can inject a fake socket; production passes nothing. Task 12 consumes `subscribe`/`unsubscribe` from the recipes page.
- `useChatHistory(): UseQueryResult<ChatMessage[]>` fetching `/chat/history?limit=50`.

- [ ] **Step 1: Write the failing test**

`frontend/src/features/chat/ChatPage.test.tsx`:

```tsx
import { act, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it } from 'vitest';
import type { WebSocketLike } from '../../ws/connection';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { ChatPage } from './ChatPage';

class FakeSocket implements WebSocketLike {
  static last: FakeSocket | null = null;

  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) { FakeSocket.last = this; }
  send(data: string) { this.sent.push(data); }
  close() { /* nothing to do in the fake */ }
}

const HISTORY = {
  messages: [
    { id: 1, user_id: 1, username: 'okadishe', body: 'Hey!', created_at: '2026-09-15T07:06:48.728Z' },
  ],
};

function signedIn() {
  localStorage.setItem('rs3.auth', JSON.stringify({ token: 'jwt-abc', username: 'okadishe' }));
}

beforeEach(() => {
  localStorage.clear();
  FakeSocket.last = null;
  server.use(http.get('http://localhost:8080/chat/history', () => HttpResponse.json(HISTORY)));
});

it('invites the visitor to sign in and hides the composer', async () => {
  renderWithProviders(<ChatPage />, { socketFactory: (url) => new FakeSocket(url) });

  expect(await screen.findByText('Hey!')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Войти' })).toBeInTheDocument();
  expect(screen.queryByLabelText('Сообщение')).not.toBeInTheDocument();
});

it('shows a live message that arrives over the socket', async () => {
  signedIn();
  renderWithProviders(<ChatPage />, { socketFactory: (url) => new FakeSocket(url) });

  await screen.findByText('Hey!');
  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({
        type: 'chat',
        payload: { id: 2, user_id: 2, username: 'fe_probe', body: 'Живое сообщение', created_at: '2026-09-15T10:29:51.487Z' },
      }),
    });
  });

  expect(await screen.findByText('Живое сообщение')).toBeInTheDocument();
  expect(screen.getByText('fe_probe')).toBeInTheDocument();
});

it('sends a typed message as a command frame and clears the field', async () => {
  signedIn();
  renderWithProviders(<ChatPage />, { socketFactory: (url) => new FakeSocket(url) });

  await screen.findByText('Hey!');
  act(() => { FakeSocket.last?.onopen?.(); });

  const field = screen.getByLabelText('Сообщение');
  await userEvent.type(field, 'Привет');
  await userEvent.click(screen.getByRole('button', { name: 'Отправить' }));

  await waitFor(() =>
    expect(FakeSocket.last?.sent).toContain(JSON.stringify({ action: 'chat', body: 'Привет' })),
  );
  expect(field).toHaveValue('');
});

it('does not echo the sent message until the server returns it', async () => {
  signedIn();
  renderWithProviders(<ChatPage />, { socketFactory: (url) => new FakeSocket(url) });

  await screen.findByText('Hey!');
  act(() => { FakeSocket.last?.onopen?.(); });
  await userEvent.type(screen.getByLabelText('Сообщение'), 'Не должно появиться');
  await userEvent.click(screen.getByRole('button', { name: 'Отправить' }));

  expect(screen.queryByText('Не должно появиться')).not.toBeInTheDocument();
});

it('reports a rejected token and signs the user out', async () => {
  signedIn();
  renderWithProviders(<ChatPage />, { socketFactory: (url) => new FakeSocket(url) });

  await screen.findByText('Hey!');
  act(() => { FakeSocket.last?.onclose?.({ code: 1008 }); });

  expect(await screen.findByText(/Сессия недействительна/)).toBeInTheDocument();
});
```

`renderWithProviders` gains a `socketFactory` option in Step 5 below.

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker compose run --rm frontend npx vitest run src/features/chat/ChatPage.test.tsx`
Expected: FAIL — the placeholder page renders no history.

- [ ] **Step 3: Write the chat history query**

`frontend/src/api/queries/chat.ts`:

```ts
import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type ChatMessage = components['schemas']['ChatMessage'];

export function useChatHistory() {
  return useQuery({
    queryKey: queryKeys.chatHistory(),
    queryFn: async (): Promise<ChatMessage[]> => {
      const { data, error, response } = await api.GET('/chat/history', {
        params: { query: { limit: 50 } },
      });
      if (error || !data) throw failure(response, error, 'Не удалось загрузить историю чата');
      return data.messages;
    },
  });
}
```

- [ ] **Step 4: Write the provider**

`frontend/src/ws/WsProvider.tsx`:

```tsx
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { useAuth } from '../auth/AuthProvider';
import { API_URL } from '../api/client';
import { createWsConnection } from './connection';
import type { ChatMessage, PriceSnapshot, WebSocketLike, WsConnection, WsStatus } from './connection';

interface WsContextValue {
  status: WsStatus;
  messages: ChatMessage[];
  serverError: string | null;
  sendChat(body: string): void;
  subscribe(itemIds: number[]): void;
  unsubscribe(itemIds: number[]): void;
  onPrice(listener: (snapshot: PriceSnapshot) => void): () => void;
}

const WsContext = createContext<WsContextValue | null>(null);

interface Props {
  children: ReactNode;
  socketFactory?: (url: string) => WebSocketLike;
}

function socketUrl(): string {
  return `${API_URL.replace(/^http/, 'ws')}/ws`;
}

export function WsProvider({ children, socketFactory }: Props) {
  const { token } = useAuth();
  const [status, setStatus] = useState<WsStatus>('closed');
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [serverError, setServerError] = useState<string | null>(null);

  const connectionRef = useRef<WsConnection | null>(null);
  const priceListeners = useRef(new Set<(snapshot: PriceSnapshot) => void>());

  useEffect(() => {
    if (!token) {
      setStatus('closed');
      return;
    }

    const connection = createWsConnection({
      url: socketUrl(),
      token,
      socketFactory,
      handlers: {
        onChat: (message) =>
          setMessages((current) =>
            current.some((existing) => existing.id === message.id)
              ? current
              : [...current, message],
          ),
        onPrice: (snapshot) => {
          for (const listener of priceListeners.current) listener(snapshot);
        },
        onStatus: setStatus,
        onServerError: setServerError,
      },
    });

    connectionRef.current = connection;
    return () => {
      connection.close();
      connectionRef.current = null;
    };
  }, [token, socketFactory]);

  const value = useMemo<WsContextValue>(
    () => ({
      status,
      messages,
      serverError,
      sendChat: (body) => connectionRef.current?.sendChat(body),
      subscribe: (itemIds) => connectionRef.current?.subscribe(itemIds),
      unsubscribe: (itemIds) => connectionRef.current?.unsubscribe(itemIds),
      onPrice: (listener) => {
        priceListeners.current.add(listener);
        return () => { priceListeners.current.delete(listener); };
      },
    }),
    [status, messages, serverError],
  );

  return <WsContext.Provider value={value}>{children}</WsContext.Provider>;
}

export function useWs(): WsContextValue {
  const value = useContext(WsContext);
  if (!value) throw new Error('useWs используется вне WsProvider');
  return value;
}

export type { PriceSnapshot };

```

Do **not** add an effect that signs the user out automatically when the status
becomes `unauthorized`. Clearing the token tears the socket down, which flips
the status to `closed` and makes the warning disappear before it can be read.
The chat page shows the warning and lets the user act on it instead.

The `socketFactory` prop must be a stable reference in tests — pass it from a module-level constant or memoise it in `renderWithProviders`, otherwise the effect reconnects on every render.

- [ ] **Step 5: Wire the provider into the app and the test helper**

In `frontend/src/main.tsx`, wrap `<App />` with `<WsProvider>` **inside** `<AuthProvider>`, because the socket needs the token.

In `frontend/src/test/renderWithProviders.tsx`, add the option and the provider:

```tsx
import { useMemo } from 'react';
import { AuthProvider } from '../auth/AuthProvider';
import { WsProvider } from '../ws/WsProvider';
import type { WebSocketLike } from '../ws/connection';

export function renderWithProviders(
  ui: ReactElement,
  opts?: { route?: string; socketFactory?: (url: string) => WebSocketLike },
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });

  function Wrapper() {
    const factory = useMemo(() => opts?.socketFactory, []);
    return (
      <QueryClientProvider client={queryClient}>
        <ThemeProvider theme={theme}>
          <MemoryRouter initialEntries={[opts?.route ?? '/']}>
            <AuthProvider>
              <WsProvider socketFactory={factory}>{ui}</WsProvider>
            </AuthProvider>
          </MemoryRouter>
        </ThemeProvider>
      </QueryClientProvider>
    );
  }

  return render(<Wrapper />);
}
```

- [ ] **Step 6: Write the chat components**

`frontend/src/features/chat/MessageList.tsx`:

```tsx
import { Box, Stack, Typography } from '@mui/material';
import type { ChatMessage } from '../../api/queries/chat';

export function MessageList({ messages }: { messages: ChatMessage[] }) {
  if (messages.length === 0) {
    return <Typography color="text.secondary">Сообщений пока нет.</Typography>;
  }

  return (
    <Stack spacing={1.5}>
      {messages.map((message) => (
        <Box key={message.id}>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'baseline' }}>
            <Typography variant="body2" sx={{ color: 'primary.main', fontWeight: 600 }}>
              {message.username}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {new Date(message.created_at).toLocaleTimeString('ru-RU')}
            </Typography>
          </Stack>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>{message.body}</Typography>
        </Box>
      ))}
    </Stack>
  );
}
```

`frontend/src/features/chat/MessageComposer.tsx`:

```tsx
import { Button, Stack, TextField } from '@mui/material';
import { useState } from 'react';

interface Props {
  disabled: boolean;
  onSend(body: string): void;
}

export function MessageComposer({ disabled, onSend }: Props) {
  const [body, setBody] = useState('');

  function submit() {
    const trimmed = body.trim();
    if (trimmed.length === 0) return;
    onSend(trimmed);
    setBody('');
  }

  return (
    <Stack direction="row" spacing={1}>
      <TextField
        label="Сообщение"
        size="small"
        fullWidth
        // The server caps the body at 500 characters.
        slotProps={{ htmlInput: { maxLength: 500 } }}
        value={body}
        onChange={(event) => setBody(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            submit();
          }
        }}
      />
      <Button variant="contained" onClick={submit} disabled={disabled}>Отправить</Button>
    </Stack>
  );
}
```

If MUI 9 rejects `slotProps.htmlInput`, use `inputProps={{ maxLength: 500 }}` instead — check the installed version's TextField typing rather than guessing.

`frontend/src/features/chat/AuthPanel.tsx`:

```tsx
import { Alert, Button, Paper, Stack, TextField, Typography } from '@mui/material';
import { useState } from 'react';
import { useAuth } from '../../auth/AuthProvider';

export function AuthPanel() {
  const { username, error, pending, signIn, signUp, signOut } = useAuth();
  const [form, setForm] = useState({ username: '', password: '' });

  if (username) {
    return (
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Typography variant="body2">Вы вошли как {username}</Typography>
          <Button size="small" onClick={signOut}>Выйти</Button>
        </Stack>
      </Paper>
    );
  }

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack spacing={2}>
        <Typography variant="body2" color="text.secondary">
          Чат и живые цены работают по токену, поэтому требуется вход.
        </Typography>
        {error && <Alert severity="error">{error}</Alert>}
        <Stack direction="row" spacing={1}>
          <TextField
            label="Логин"
            size="small"
            value={form.username}
            onChange={(event) => setForm({ ...form, username: event.target.value })}
          />
          <TextField
            label="Пароль"
            type="password"
            size="small"
            value={form.password}
            onChange={(event) => setForm({ ...form, password: event.target.value })}
          />
          <Button variant="contained" disabled={pending} onClick={() => void signIn(form)}>
            Войти
          </Button>
          <Button disabled={pending} onClick={() => void signUp(form)}>
            Зарегистрироваться
          </Button>
        </Stack>
      </Stack>
    </Paper>
  );
}
```

- [ ] **Step 7: Write the chat page**

`frontend/src/features/chat/ChatPage.tsx` (replacing the placeholder):

```tsx
import { Alert, Button, Paper, Stack, Typography } from '@mui/material';
import { useMemo } from 'react';
import { useChatHistory } from '../../api/queries/chat';
import { useAuth } from '../../auth/AuthProvider';
import { QueryState } from '../../shared/QueryState';
import { useWs } from '../../ws/WsProvider';
import { AuthPanel } from './AuthPanel';
import { MessageComposer } from './MessageComposer';
import { MessageList } from './MessageList';

export function ChatPage() {
  const { token, signOut } = useAuth();
  const { status, messages: live, serverError, sendChat } = useWs();
  const history = useChatHistory();

  const combined = useMemo(() => {
    const seen = new Set((history.data ?? []).map((message) => message.id));
    return [...(history.data ?? []), ...live.filter((message) => !seen.has(message.id))];
  }, [history.data, live]);

  return (
    <Stack spacing={2} sx={{ maxWidth: 780 }}>
      <Typography variant="h5" component="h1">Чат</Typography>

      <AuthPanel />

      {status === 'unauthorized' && (
        <Alert
          severity="warning"
          action={<Button size="small" onClick={signOut}>Выйти</Button>}
        >
          Сессия недействительна, войдите заново.
        </Alert>
      )}
      {status === 'closed' && token && (
        <Alert severity="warning">Соединение потеряно, живые сообщения не приходят.</Alert>
      )}
      {serverError && <Alert severity="error">{serverError}</Alert>}

      <Paper variant="outlined" sx={{ p: 2, minHeight: 320 }}>
        <QueryState query={history}>{() => <MessageList messages={combined} />}</QueryState>
      </Paper>

      {token && <MessageComposer disabled={status !== 'open'} onSend={sendChat} />}
    </Stack>
  );
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/features/chat/ChatPage.test.tsx`
Expected: PASS, 5 tests.

Then the whole suite: `docker compose run --rm frontend npm test`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/ws frontend/src/features/chat frontend/src/api/queries/chat.ts frontend/src/main.tsx frontend/src/test/renderWithProviders.tsx
git commit -m "Добавить живой чат поверх WebSocket"
```

---

## Task 9: Recipe row assembly

This is the core of the recipe table and the only place where four independent
API responses meet. It is a pure function so that every partial-data case is
testable without React, MSW or a DOM.

**Files:**
- Create: `frontend/src/features/recipes/buildRows.ts`
- Create: `frontend/src/test/fixtures.ts`
- Test: `frontend/src/features/recipes/buildRows.test.ts`

**Interfaces:**
- Consumes: `canonicalSkill` from Task 4, schema types from Task 1.
- Produces:

  ```ts
  export type RowCaveat = 'incomplete' | 'default-aph' | 'no-throughput';

  export interface RecipeRow {
    id: number;                       // equals itemId; the DataGrid row id
    itemId: number;
    itemName: string;
    skill: string;
    levelReq: number;
    meetsRequirements: boolean | null;
    price: number | null;
    componentsCost: number | null;
    margin: number | null;
    roiPct: number | null;
    xpPerHour: number | null;
    gpPerHour: number | null;
    gpPerXp: number | null;
    throughputGpPerHour: number | null;
    bindingItemName: string | null;
    liquidityTier: LiquidityTier | null;
    liquidityScore: number | null;
    observations: number | null;
    volumeAvg: number | null;
    buyLimit4h: number | null;
    caveats: RowCaveat[];
    unpricedInputs: string[];
    pathNames: string[];
    error: string | null;
  }

  export interface BuildRowsInput {
    recipes: Recipe[];
    calc: Map<number, CalcBatchEntry>;
    prices: Map<number, PriceSnapshot>;
    stats: Map<number, LiquidityStats>;
  }

  export function buildRows(input: BuildRowsInput): RecipeRow[];
  export function firstAssumptions(entries: Iterable<CalcBatchEntry>): CalcAssumptions | null;
  ```

  Tasks 11 and 12 consume `RecipeRow` field names verbatim as DataGrid column fields.

- [ ] **Step 1: Write the fixtures**

`frontend/src/test/fixtures.ts`:

```ts
import type { components } from '../api/schema';

type Recipe = components['schemas']['Recipe'];
type CalcPath = components['schemas']['CalcPath'];
type CalcResult = components['schemas']['CalcResult'];
type CalcBatchEntry = components['schemas']['CalcBatchEntry'];
type PriceSnapshot = components['schemas']['PriceSnapshot'];
type LiquidityStats = components['schemas']['LiquidityStats'];

export function recipe(over: Partial<Recipe> = {}): Recipe {
  return {
    id: 9937,
    name: 'Ruby',
    output_item_id: 1603,
    output_item_name: 'Ruby',
    output_qty: 1,
    skill: 'Crafting',
    level_req: 34,
    xp_per_action: 85,
    actions_per_hour: 1000,
    aph_source: 'wiki',
    members: false,
    source: 'runescape.wiki',
    inputs: [],
    ...over,
  };
}

export function path(over: Partial<CalcPath> = {}): CalcPath {
  return {
    path: ['Uncut ruby', 'Ruby'],
    steps: [],
    profit_per_craft: 301,
    gp_per_hour: 301_000,
    xp_per_hour: 85_000,
    gp_per_xp: 3.54,
    roi_pct: 12.5,
    total_hours: 0.001,
    total_xp: 85,
    buy_cost: 2_400,
    sell_revenue: 2_707,
    tax_paid: 6,
    actions_per_hour: 1000,
    aph_source: 'wiki',
    complete: true,
    throughput: {
      binding_item_id: 1619,
      binding_item_name: 'Uncut ruby',
      binding_limit_4h: 10_000,
      max_crafts_per_4h: 10_000,
      crafts_per_hour: 2_500,
      gp_per_hour: 752_500,
    },
    meets_requirements: true,
    skill_requirements: [{ skill: 'Crafting', required: 34, actual: 99, met: true }],
    ...over,
  };
}

export function result(over: Partial<CalcResult> = {}): CalcResult {
  return {
    item_id: 1603,
    generated_at: '2026-09-15T10:00:00Z',
    cached: false,
    assumptions: {
      price_basis: 'ge_guide_price',
      spread_pct: 2,
      tax_pct: 1,
      tax_cap_per_item: 5_000_000,
      tax_exempt_below: 100,
    },
    paths: [path()],
    ...over,
  };
}

export function batchEntry(over: Partial<CalcBatchEntry> = {}): CalcBatchEntry {
  return { item_id: 1603, result: result(), ...over };
}

export function snapshot(over: Partial<PriceSnapshot> = {}): PriceSnapshot {
  return {
    id: 5207,
    item_id: 1603,
    ts: '2026-09-15T09:53:33.001Z',
    price: 2_707,
    volume: 171_851,
    ...over,
  };
}

export function liquidity(over: Partial<LiquidityStats> = {}): LiquidityStats {
  return {
    item_id: 1603,
    observations: 48,
    distinct_prices: 12,
    price_min: 2_600,
    price_max: 2_800,
    price_avg: 2_700,
    price_last: 2_707,
    volatility_pct: 7.4,
    volume_avg: 165_000,
    volume_last: 171_851,
    last_change_at: '2026-09-15T09:00:00Z',
    stale_for_hours: 0.9,
    score: 77,
    tier: 'high',
    buy_limit_4h: 10_000,
    ...over,
  };
}
```

- [ ] **Step 2: Write the failing tests**

`frontend/src/features/recipes/buildRows.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { batchEntry, liquidity, path, recipe, result, snapshot } from '../../test/fixtures';
import { buildRows, firstAssumptions } from './buildRows';

function input(over: Partial<Parameters<typeof buildRows>[0]> = {}) {
  return {
    recipes: [recipe()],
    calc: new Map([[1603, batchEntry()]]),
    prices: new Map([[1603, snapshot()]]),
    stats: new Map([[1603, liquidity()]]),
    ...over,
  };
}

describe('buildRows', () => {
  it('assembles one row from the four responses', () => {
    const [row] = buildRows(input());

    expect(row.id).toBe(1603);
    expect(row.itemName).toBe('Ruby');
    expect(row.skill).toBe('Crafting');
    expect(row.levelReq).toBe(34);
    expect(row.price).toBe(2_707);
    expect(row.componentsCost).toBe(2_400);
    expect(row.margin).toBe(301);
    expect(row.roiPct).toBe(12.5);
    expect(row.xpPerHour).toBe(85_000);
    expect(row.gpPerHour).toBe(301_000);
    expect(row.gpPerXp).toBe(3.54);
    expect(row.throughputGpPerHour).toBe(752_500);
    expect(row.bindingItemName).toBe('Uncut ruby');
    expect(row.liquidityTier).toBe('high');
    expect(row.liquidityScore).toBe(77);
    expect(row.observations).toBe(48);
    expect(row.buyLimit4h).toBe(10_000);
    expect(row.meetsRequirements).toBe(true);
    expect(row.caveats).toEqual([]);
    expect(row.error).toBeNull();
  });

  it('repairs the skill casing the recipe data ships with', () => {
    const [row] = buildRows(input({ recipes: [recipe({ skill: 'crafting' })] }));
    expect(row.skill).toBe('Crafting');
  });

  it('drops recipes whose output item was never resolved', () => {
    const rows = buildRows(input({ recipes: [recipe({ output_item_id: 0 })] }));
    expect(rows).toEqual([]);
  });

  it('keeps one row per output item when several recipes produce it', () => {
    const rows = buildRows(
      input({ recipes: [recipe({ id: 1, name: 'Ruby' }), recipe({ id: 2, name: 'Ruby (alt)' })] }),
    );
    expect(rows).toHaveLength(1);
    expect(rows[0].itemName).toBe('Ruby');
  });

  it('renders a row with no price and no statistics rather than dropping it', () => {
    const [row] = buildRows(input({ prices: new Map(), stats: new Map() }));

    expect(row.price).toBeNull();
    expect(row.liquidityTier).toBeNull();
    expect(row.liquidityScore).toBeNull();
    expect(row.margin).toBe(301);
  });

  it('carries a per-item calculation error instead of showing a zero margin', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, { item_id: 1603, error: 'no priceable production path for item 1603' }],
        ]),
      }),
    );

    expect(rows[0].error).toBe('no priceable production path for item 1603');
    expect(rows[0].margin).toBeNull();
    expect(rows[0].gpPerHour).toBeNull();
    expect(rows[0].roiPct).toBeNull();
  });

  it('states the reason when the calculation returned no paths at all', () => {
    const rows = buildRows(
      input({ calc: new Map([[1603, batchEntry({ result: result({ paths: [] }) })]]) }),
    );

    expect(rows[0].error).toBe('Сервер не нашёл оцениваемого пути');
    expect(rows[0].margin).toBeNull();
  });

  it('states the reason when the item was not part of the batch', () => {
    const rows = buildRows(input({ calc: new Map() }));

    expect(rows[0].error).toBe('Расчёт не выполнен');
    expect(rows[0].margin).toBeNull();
  });

  it('marks an incomplete path and lists the inputs that could not be priced', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({
            result: result({
              paths: [path({ complete: false, unpriced_inputs: ['Uncut ruby', 'Chisel'] })],
            }),
          })],
        ]),
      }),
    );

    expect(rows[0].caveats).toContain('incomplete');
    expect(rows[0].unpricedInputs).toEqual(['Uncut ruby', 'Chisel']);
  });

  it('marks a house-assumed actions-per-hour rate', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({ result: result({ paths: [path({ aph_source: 'default' })] }) })],
        ]),
      }),
    );

    expect(rows[0].caveats).toContain('default-aph');
  });

  it('marks a path with no known buy limit and leaves the throughput empty', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({ result: result({ paths: [path({ throughput: null })] }) })],
        ]),
      }),
    );

    expect(rows[0].caveats).toContain('no-throughput');
    expect(rows[0].throughputGpPerHour).toBeNull();
    expect(rows[0].bindingItemName).toBeNull();
  });

  it('reports a player who cannot perform the path', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({
            result: result({
              paths: [path({
                meets_requirements: false,
                skill_requirements: [{ skill: 'Crafting', required: 34, actual: 12, met: false }],
              })],
            }),
          })],
        ]),
      }),
    );

    expect(rows[0].meetsRequirements).toBe(false);
  });
});

describe('firstAssumptions', () => {
  it('returns the first assumptions block present in the batch', () => {
    const assumptions = firstAssumptions([
      { item_id: 1, error: 'nope' },
      batchEntry(),
    ]);

    expect(assumptions?.spread_pct).toBe(2);
    expect(assumptions?.price_basis).toBe('ge_guide_price');
  });

  it('returns null when every entry failed', () => {
    expect(firstAssumptions([{ item_id: 1, error: 'nope' }])).toBeNull();
  });
});
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `docker compose run --rm frontend npx vitest run src/features/recipes/buildRows.test.ts`
Expected: FAIL — `Cannot find module './buildRows'`.

- [ ] **Step 4: Write the implementation**

`frontend/src/features/recipes/buildRows.ts`:

```ts
import { canonicalSkill } from '../../assets/skills';
import type { components } from '../../api/schema';

type Recipe = components['schemas']['Recipe'];
type CalcBatchEntry = components['schemas']['CalcBatchEntry'];
type CalcAssumptions = components['schemas']['CalcAssumptions'];
type PriceSnapshot = components['schemas']['PriceSnapshot'];
type LiquidityStats = components['schemas']['LiquidityStats'];
type LiquidityTier = components['schemas']['LiquidityTier'];

export type RowCaveat = 'incomplete' | 'default-aph' | 'no-throughput';

export interface RecipeRow {
  id: number;
  itemId: number;
  itemName: string;
  skill: string;
  levelReq: number;
  meetsRequirements: boolean | null;
  price: number | null;
  componentsCost: number | null;
  margin: number | null;
  roiPct: number | null;
  xpPerHour: number | null;
  gpPerHour: number | null;
  gpPerXp: number | null;
  throughputGpPerHour: number | null;
  bindingItemName: string | null;
  liquidityTier: LiquidityTier | null;
  liquidityScore: number | null;
  observations: number | null;
  volumeAvg: number | null;
  buyLimit4h: number | null;
  caveats: RowCaveat[];
  unpricedInputs: string[];
  pathNames: string[];
  error: string | null;
}

export interface BuildRowsInput {
  recipes: Recipe[];
  calc: Map<number, CalcBatchEntry>;
  prices: Map<number, PriceSnapshot>;
  stats: Map<number, LiquidityStats>;
}

const NOT_CALCULATED = 'Расчёт не выполнен';
const NO_PATH = 'Сервер не нашёл оцениваемого пути';

export function buildRows({ recipes, calc, prices, stats }: BuildRowsInput): RecipeRow[] {
  const seen = new Set<number>();
  const rows: RecipeRow[] = [];

  for (const source of recipes) {
    const itemId = source.output_item_id;
    // output_item_id === 0 means the wiki name was never matched to a GE item,
    // so nothing about it can be priced.
    if (!itemId || seen.has(itemId)) continue;
    seen.add(itemId);

    const price = prices.get(itemId);
    const stat = stats.get(itemId);
    const entry = calc.get(itemId);
    const best = entry?.result?.paths?.[0] ?? null;

    let error: string | null = null;
    if (!entry) error = NOT_CALCULATED;
    else if (entry.error) error = entry.error;
    else if (!best) error = NO_PATH;

    const caveats: RowCaveat[] = [];
    if (best) {
      if (!best.complete) caveats.push('incomplete');
      if (best.aph_source === 'default') caveats.push('default-aph');
      if (!best.throughput) caveats.push('no-throughput');
    }

    rows.push({
      id: itemId,
      itemId,
      itemName: source.output_item_name,
      skill: canonicalSkill(source.skill) ?? source.skill,
      levelReq: source.level_req,
      meetsRequirements: best?.meets_requirements ?? null,
      price: price?.price ?? null,
      componentsCost: best?.buy_cost ?? null,
      margin: best?.profit_per_craft ?? null,
      roiPct: best?.roi_pct ?? null,
      xpPerHour: best?.xp_per_hour ?? null,
      gpPerHour: best?.gp_per_hour ?? null,
      gpPerXp: best?.gp_per_xp ?? null,
      throughputGpPerHour: best?.throughput?.gp_per_hour ?? null,
      bindingItemName: best?.throughput?.binding_item_name ?? null,
      liquidityTier: stat?.tier ?? null,
      liquidityScore: stat?.score ?? null,
      observations: stat?.observations ?? null,
      volumeAvg: stat?.volume_avg ?? null,
      buyLimit4h: stat?.buy_limit_4h ?? null,
      caveats,
      unpricedInputs: best?.unpriced_inputs ?? [],
      pathNames: best?.path ?? [],
      error,
    });
  }

  return rows;
}

export function firstAssumptions(entries: Iterable<CalcBatchEntry>): CalcAssumptions | null {
  for (const entry of entries) {
    if (entry.result) return entry.result.assumptions;
  }
  return null;
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/features/recipes/buildRows.test.ts`
Expected: PASS, 14 tests.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/features/recipes/buildRows.ts frontend/src/features/recipes/buildRows.test.ts frontend/src/test/fixtures.ts
git commit -m "Добавить сборку строк таблицы рецептов"
```

---

## Task 10: Recipe, price and calculation queries

**Files:**
- Create: `frontend/src/api/queries/recipes.ts`, `frontend/src/api/queries/prices.ts`, `frontend/src/api/queries/calc.ts`
- Test: `frontend/src/api/queries/recipes.test.tsx`

**Interfaces:**
- Consumes: `api`, `queryKeys`, `CalcParams` from Task 3.
- Produces:
  - `useSkillRecipes(skill: string, maxLevel: number): UseQueryResult<Recipe[]>` — hits `/recipes?skill&level`, `staleTime` one hour, disabled while `skill` is empty.
  - `usePriceableItemIds(skill: string, minLevel: number, maxLevel: number): UseQueryResult<Set<number>>` — hits `/recipes/ids`.
  - `useLatestPrices(ids: number[]): UseQueryResult<Map<number, PriceSnapshot>>` — hits `/prices/latest?ids=...`, disabled on an empty list.
  - `useLiquidityStats(ids: number[]): { data: Map<number, LiquidityStats>; isPending: boolean }` — one `/prices/stats/{id}` query per item via `useQueries`, so the cache survives paging.
  - `useCalcBatch(ids: number[], params: CalcParams): UseQueryResult<Map<number, CalcBatchEntry>>` — hits `/calc/batch`.

- [ ] **Step 1: Write the failing test**

`frontend/src/api/queries/recipes.test.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { expect, it } from 'vitest';
import type { ReactNode } from 'react';
import { recipe } from '../../test/fixtures';
import { server } from '../../test/msw/server';
import { useCalcBatch } from './calc';
import { useLatestPrices } from './prices';
import { usePriceableItemIds, useSkillRecipes } from './recipes';

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

it('reads recipes for one skill up to a level', async () => {
  let seen = '';
  server.use(
    http.get('http://localhost:8080/recipes', ({ request }) => {
      seen = new URL(request.url).search;
      return HttpResponse.json({ count: 1, recipes: [recipe()] });
    }),
  );

  const { result } = renderHook(() => useSkillRecipes('Crafting', 99), { wrapper });

  await waitFor(() => expect(result.current.data).toHaveLength(1));
  expect(seen).toContain('skill=Crafting');
  expect(seen).toContain('level=99');
});

it('does not query while no skill is chosen', () => {
  const { result } = renderHook(() => useSkillRecipes('', 99), { wrapper });
  expect(result.current.fetchStatus).toBe('idle');
});

it('turns the priceable id list into a set', async () => {
  server.use(
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: 'Crafting', min_level: 1, max_level: 99, count: 2, item_ids: [1603, 1605] }),
    ),
  );

  const { result } = renderHook(() => usePriceableItemIds('Crafting', 1, 99), { wrapper });

  await waitFor(() => expect(result.current.data?.has(1603)).toBe(true));
  expect(result.current.data?.size).toBe(2);
});

it('keys the latest prices by item id', async () => {
  server.use(
    http.get('http://localhost:8080/prices/latest', () =>
      HttpResponse.json({
        count: 1,
        prices: [{ id: 1, item_id: 1603, ts: '2026-09-15T09:53:33.001Z', price: 2707, volume: 171851 }],
      }),
    ),
  );

  const { result } = renderHook(() => useLatestPrices([1603]), { wrapper });

  await waitFor(() => expect(result.current.data?.get(1603)?.price).toBe(2707));
});

it('keys the batch calculation by item id and forwards the player', async () => {
  let seen = '';
  server.use(
    http.get('http://localhost:8080/calc/batch', ({ request }) => {
      seen = new URL(request.url).search;
      return HttpResponse.json({
        count: 1,
        results: [{ item_id: 1603, error: 'no priceable production path for item 1603' }],
      });
    }),
  );

  const { result } = renderHook(
    () => useCalcBatch([1603], { player: 'Zezima', mode: 'normal', includeIncomplete: false }),
    { wrapper },
  );

  await waitFor(() => expect(result.current.data?.get(1603)?.error).toContain('no priceable'));
  expect(seen).toContain('ids=1603');
  expect(seen).toContain('player=Zezima');
  expect(seen).toContain('include_incomplete=false');
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker compose run --rm frontend npx vitest run src/api/queries/recipes.test.tsx`
Expected: FAIL — `Cannot find module './calc'`.

- [ ] **Step 3: Write the recipe queries**

`frontend/src/api/queries/recipes.ts`:

```ts
import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type Recipe = components['schemas']['Recipe'];

const ONE_HOUR = 60 * 60_000;

export function useSkillRecipes(skill: string, maxLevel: number) {
  return useQuery({
    queryKey: queryKeys.skillRecipes(skill, maxLevel),
    enabled: skill.length > 0,
    staleTime: ONE_HOUR,
    queryFn: async (): Promise<Recipe[]> => {
      const { data, error, response } = await api.GET('/recipes', {
        params: { query: { skill, level: maxLevel } },
      });
      if (error || !data) throw failure(response, error, 'Не удалось загрузить рецепты');
      return data.recipes;
    },
  });
}

export function usePriceableItemIds(skill: string, minLevel: number, maxLevel: number) {
  return useQuery({
    queryKey: queryKeys.priceableIds(skill, minLevel, maxLevel),
    enabled: skill.length > 0,
    staleTime: ONE_HOUR,
    queryFn: async (): Promise<Set<number>> => {
      const { data, error, response } = await api.GET('/recipes/ids', {
        params: { query: { skill, min_level: minLevel, level: maxLevel } },
      });
      if (error || !data) throw failure(response, error, 'Не удалось загрузить список предметов');
      return new Set(data.item_ids);
    },
  });
}
```

- [ ] **Step 4: Write the price queries**

`frontend/src/api/queries/prices.ts`:

```ts
import { useQueries, useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type PriceSnapshot = components['schemas']['PriceSnapshot'];
export type LiquidityStats = components['schemas']['LiquidityStats'];

export function useLatestPrices(ids: number[]) {
  return useQuery({
    queryKey: queryKeys.latestPrices(ids),
    enabled: ids.length > 0,
    queryFn: async (): Promise<Map<number, PriceSnapshot>> => {
      const { data, error, response } = await api.GET('/prices/latest', {
        params: { query: { ids: ids.join(',') } },
      });
      if (error || !data) throw failure(response, error, 'Не удалось загрузить цены');
      return new Map(data.prices.map((price) => [price.item_id, price]));
    },
  });
}

// One query per item rather than one for the page: the cache then survives
// paging back and forth, and a single slow item does not block the rest.
export function useLiquidityStats(ids: number[]) {
  return useQueries({
    queries: ids.map((itemId) => ({
      queryKey: queryKeys.liquidity(itemId),
      staleTime: 5 * 60_000,
      queryFn: async (): Promise<LiquidityStats> => {
        const { data, error, response } = await api.GET('/prices/stats/{itemID}', {
          params: { path: { itemID: itemId }, query: { window: '24h' } },
        });
        if (error || !data) throw failure(response, error, 'Нет статистики ликвидности');
        return data;
      },
    })),
    combine: (results) => ({
      data: new Map(
        results.flatMap((entry) => (entry.data ? [[entry.data.item_id, entry.data] as const] : [])),
      ),
      isPending: results.some((entry) => entry.isPending),
    }),
  });
}
```

- [ ] **Step 5: Write the calculation query**

`frontend/src/api/queries/calc.ts`:

```ts
import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { CalcParams } from '../queryKeys';
import type { components } from '../schema';

export type CalcBatchEntry = components['schemas']['CalcBatchEntry'];

export function useCalcBatch(ids: number[], params: CalcParams) {
  return useQuery({
    queryKey: queryKeys.calcBatch(ids, params),
    enabled: ids.length > 0,
    queryFn: async (): Promise<Map<number, CalcBatchEntry>> => {
      const { data, error, response } = await api.GET('/calc/batch', {
        params: {
          query: {
            ids: ids.join(','),
            include_incomplete: params.includeIncomplete,
            ...(params.player ? { player: params.player, mode: params.mode } : {}),
            ...(params.spreadPct === undefined ? {} : { spread_pct: params.spreadPct }),
            ...(params.aph === undefined ? {} : { aph: params.aph }),
          },
        },
      });
      if (error || !data) throw failure(response, error, 'Не удалось посчитать выгодность');
      return new Map(data.results.map((entry) => [entry.item_id, entry]));
    },
  });
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/api/queries/recipes.test.tsx`
Expected: PASS, 5 tests.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/api/queries
git commit -m "Добавить запросы рецептов, цен и расчёта выгодности"
```

---

## Task 11: Recipe table

**Files:**
- Create: `frontend/src/features/recipes/columns.tsx`, `RecipeFilters.tsx`, `AssumptionsBar.tsx`
- Modify: `frontend/src/features/recipes/RecipesPage.tsx` (replace the Task 3 placeholder)
- Test: `frontend/src/features/recipes/RecipesPage.test.tsx`

**Interfaces:**
- Consumes: everything from Tasks 2, 4, 6, 9 and 10.
- Produces: `recipeColumns: GridColDef<RecipeRow>[]`, `<RecipeFilters>`, `<AssumptionsBar assumptions={CalcAssumptions | null} />`. Task 12 adds live price patching on top of this page without changing these signatures.

- [ ] **Step 1: Write the failing test**

`frontend/src/features/recipes/RecipesPage.test.tsx`:

```tsx
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it } from 'vitest';
import { batchEntry, liquidity, path, recipe, result, snapshot } from '../../test/fixtures';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { RecipesPage } from './RecipesPage';

function backend(over: { calcEntry?: unknown } = {}) {
  server.use(
    http.get('http://localhost:8080/recipes', () =>
      HttpResponse.json({ count: 1, recipes: [recipe()] }),
    ),
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: 'Crafting', min_level: 1, max_level: 99, count: 1, item_ids: [1603] }),
    ),
    http.get('http://localhost:8080/prices/latest', () =>
      HttpResponse.json({ count: 1, prices: [snapshot()] }),
    ),
    http.get('http://localhost:8080/prices/stats/1603', () => HttpResponse.json(liquidity())),
    http.get('http://localhost:8080/calc/batch', () =>
      HttpResponse.json({ count: 1, results: [over.calcEntry ?? batchEntry()] }),
    ),
  );
}

beforeEach(() => localStorage.clear());

async function chooseCrafting() {
  renderWithProviders(<RecipesPage />);
  await userEvent.click(screen.getByLabelText('Скилл'));
  await userEvent.click(await screen.findByRole('option', { name: 'Crafting' }));
}

it('shows the profitability columns for the chosen skill', async () => {
  backend();
  await chooseCrafting();

  const row = await screen.findByRole('row', { name: /Ruby/ });
  expect(within(row).getByText('2.7K')).toBeInTheDocument();
  expect(within(row).getByText('2.4K')).toBeInTheDocument();
  expect(within(row).getByText('301')).toBeInTheDocument();
  expect(within(row).getByText('12.5%')).toBeInTheDocument();
  expect(within(row).getByText('85K')).toBeInTheDocument();
  expect(within(row).getByText('301K')).toBeInTheDocument();
  expect(within(row).getByText('752.5K')).toBeInTheDocument();
  expect(within(row).getByText(/77/)).toBeInTheDocument();
});

it('shows the assumptions the money figures rest on', async () => {
  backend();
  await chooseCrafting();

  expect(await screen.findByText(/Спред 2\.0%/)).toBeInTheDocument();
  expect(screen.getByText(/налог 1\.0%/)).toBeInTheDocument();
});

it('marks a path whose actions-per-hour is a house assumption', async () => {
  backend({
    calcEntry: batchEntry({ result: result({ paths: [path({ aph_source: 'default' })] }) }),
  });
  await chooseCrafting();

  const row = await screen.findByRole('row', { name: /Ruby/ });
  expect(within(row).getByTitle(/скорость действий принята сервером/i)).toBeInTheDocument();
});

it('states the reason instead of a zero margin when the server could not price the item', async () => {
  backend({
    calcEntry: { item_id: 1603, error: 'no priceable production path for item 1603' },
  });
  await chooseCrafting();

  const row = await screen.findByRole('row', { name: /Ruby/ });
  expect(within(row).getByText(/no priceable production path/)).toBeInTheDocument();
  expect(within(row).queryByText('0')).not.toBeInTheDocument();
});

it('takes the initial skill from the url when the character page linked here', async () => {
  backend();
  renderWithProviders(<RecipesPage />, { route: '/recipes?skill=Crafting' });

  expect(await screen.findByRole('row', { name: /Ruby/ })).toBeInTheDocument();
});

it('ignores a skill parameter that is not a real skill', async () => {
  backend();
  renderWithProviders(<RecipesPage />, { route: '/recipes?skill=%3Cscript%3E' });

  expect(screen.getByText('Выберите скилл, чтобы увидеть рецепты.')).toBeInTheDocument();
});

it('says that sorting only covers the loaded page', async () => {
  backend();
  await chooseCrafting();

  expect(
    await screen.findByText(/Сортировка работает в пределах загруженной страницы/),
  ).toBeInTheDocument();
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker compose run --rm frontend npx vitest run src/features/recipes/RecipesPage.test.tsx`
Expected: FAIL — the placeholder page has no skill selector.

- [ ] **Step 3: Write the assumptions bar**

`frontend/src/features/recipes/AssumptionsBar.tsx`:

```tsx
import { Alert, Typography } from '@mui/material';
import type { components } from '../../api/schema';
import { formatCompact, formatPct } from '../../shared/format';

type CalcAssumptions = components['schemas']['CalcAssumptions'];

export function AssumptionsBar({ assumptions }: { assumptions: CalcAssumptions | null }) {
  if (!assumptions) return null;

  return (
    <Alert severity="info" variant="outlined">
      <Typography variant="body2">
        Цена — гайдовая цена Grand Exchange. Спред {formatPct(assumptions.spread_pct)},
        налог {formatPct(assumptions.tax_pct)}, потолок налога{' '}
        {formatCompact(assumptions.tax_cap_per_item)}, освобождение ниже{' '}
        {formatCompact(assumptions.tax_exempt_below)}. GP/ч без этих допущений не имеет смысла.
      </Typography>
    </Alert>
  );
}
```

- [ ] **Step 4: Write the filters**

`frontend/src/features/recipes/RecipeFilters.tsx`:

```tsx
import { FormControlLabel, MenuItem, Stack, Switch, TextField } from '@mui/material';
import { SKILLS } from '../../assets/skills';

export interface FiltersValue {
  skill: string;
  minLevel: number;
  maxLevel: number;
  includeIncomplete: boolean;
  spreadPct: number | undefined;
}

interface Props {
  value: FiltersValue;
  onChange(next: FiltersValue): void;
}

export function RecipeFilters({ value, onChange }: Props) {
  return (
    <Stack direction="row" spacing={2} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
      <TextField
        select
        label="Скилл"
        size="small"
        sx={{ minWidth: 200 }}
        value={value.skill}
        onChange={(event) => onChange({ ...value, skill: event.target.value })}
      >
        {SKILLS.map((skill) => (
          <MenuItem key={skill} value={skill}>{skill}</MenuItem>
        ))}
      </TextField>

      <TextField
        label="Уровень от"
        type="number"
        size="small"
        sx={{ width: 120 }}
        value={value.minLevel}
        onChange={(event) => onChange({ ...value, minLevel: Number(event.target.value) })}
      />
      <TextField
        label="Уровень до"
        type="number"
        size="small"
        sx={{ width: 120 }}
        value={value.maxLevel}
        onChange={(event) => onChange({ ...value, maxLevel: Number(event.target.value) })}
      />
      <TextField
        label="Спред, %"
        type="number"
        size="small"
        sx={{ width: 120 }}
        value={value.spreadPct ?? ''}
        onChange={(event) =>
          onChange({
            ...value,
            spreadPct: event.target.value === '' ? undefined : Number(event.target.value),
          })
        }
      />

      <FormControlLabel
        control={
          <Switch
            checked={value.includeIncomplete}
            onChange={(event) => onChange({ ...value, includeIncomplete: event.target.checked })}
          />
        }
        label="Показывать пути с неоценёнными входами"
      />
    </Stack>
  );
}
```

- [ ] **Step 5: Write the columns**

`frontend/src/features/recipes/columns.tsx`:

```tsx
import { Chip, Stack, Tooltip, Typography } from '@mui/material';
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid';
import { ItemIcon } from '../../shared/ItemIcon';
import { SkillIcon } from '../../shared/SkillIcon';
import { formatCompact, formatInt, formatPct } from '../../shared/format';
import type { RecipeRow, RowCaveat } from './buildRows';

const CAVEAT_TITLES: Record<RowCaveat, string> = {
  incomplete: 'Часть входов не оценена и посчитана по нулю — маржа является верхней границей',
  'default-aph': 'Скорость действий принята сервером, а не измерена',
  'no-throughput': 'Лимит покупки неизвестен, ограничение по обороту не посчитано',
};

const CAVEAT_LABELS: Record<RowCaveat, string> = {
  incomplete: 'неполно',
  'default-aph': 'APH по умолчанию',
  'no-throughput': 'без лимита',
};

// The reason for an unpriceable row is stated once, in the item column.
// Money columns render an em dash so the row never reads as a zero margin.
function Money({ row, value }: { row: RecipeRow; value: number | null }) {
  if (row.error) return <Typography variant="body2" color="text.secondary">—</Typography>;

  const dim = row.caveats.includes('incomplete');
  return (
    <Typography variant="body2" sx={{ opacity: dim ? 0.55 : 1 }}>
      {formatCompact(value)}
    </Typography>
  );
}

export const recipeColumns: GridColDef<RecipeRow>[] = [
  {
    field: 'skill',
    headerName: 'Скилл',
    width: 110,
    renderCell: (params: GridRenderCellParams<RecipeRow, string>) => (
      <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
        <SkillIcon skill={params.row.skill} />
        <Typography variant="body2">{params.row.skill}</Typography>
      </Stack>
    ),
  },
  {
    field: 'itemName',
    headerName: 'Предмет',
    flex: 1,
    minWidth: 220,
    renderCell: (params: GridRenderCellParams<RecipeRow, string>) => (
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', minWidth: 0 }}>
        <ItemIcon itemId={params.row.itemId} name={params.row.itemName} />
        <Typography variant="body2" noWrap>{params.row.itemName}</Typography>

        {params.row.error && (
          <Typography variant="caption" color="text.secondary" noWrap title={params.row.error}>
            {params.row.error}
          </Typography>
        )}

        {params.row.caveats.map((caveat) => (
          <Chip
            key={caveat}
            size="small"
            variant="outlined"
            color={caveat === 'incomplete' ? 'warning' : 'default'}
            title={CAVEAT_TITLES[caveat]}
            label={CAVEAT_LABELS[caveat]}
          />
        ))}
      </Stack>
    ),
  },
  {
    field: 'levelReq',
    headerName: 'Уровень',
    width: 100,
    renderCell: (params: GridRenderCellParams<RecipeRow, number>) => (
      <Typography
        variant="body2"
        color={params.row.meetsRequirements === false ? 'text.disabled' : 'text.primary'}
      >
        {formatInt(params.row.levelReq)}
      </Typography>
    ),
  },
  {
    field: 'price',
    headerName: 'Цена',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.price} />,
  },
  {
    field: 'componentsCost',
    headerName: 'Компоненты',
    width: 120,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.componentsCost} />,
  },
  {
    field: 'margin',
    headerName: 'Маржа',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => (
      <Tooltip
        title={
          p.row.unpricedInputs.length > 0
            ? `Не оценены: ${p.row.unpricedInputs.join(', ')}`
            : ''
        }
      >
        <Typography
          variant="body2"
          color={p.row.margin === null ? 'text.secondary' : p.row.margin >= 0 ? 'success.main' : 'error.main'}
        >
          {p.row.error ? '—' : formatCompact(p.row.margin)}
        </Typography>
      </Tooltip>
    ),
  },
  {
    field: 'roiPct',
    headerName: 'ROI',
    width: 100,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => (
      <Typography variant="body2">{p.row.error ? '—' : formatPct(p.row.roiPct)}</Typography>
    ),
  },
  {
    field: 'xpPerHour',
    headerName: 'XP/ч',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.xpPerHour} />,
  },
  {
    field: 'gpPerXp',
    headerName: 'GP/XP',
    width: 100,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.gpPerXp} />,
  },
  {
    field: 'gpPerHour',
    headerName: 'GP/ч',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.gpPerHour} />,
  },
  {
    field: 'throughputGpPerHour',
    headerName: 'GP/ч по лимиту',
    width: 150,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => (
      <Tooltip title={p.row.bindingItemName ? `Связывающий вход: ${p.row.bindingItemName}` : 'Лимит покупки неизвестен'}>
        <Typography variant="body2">
          {p.row.error ? '—' : formatCompact(p.row.throughputGpPerHour)}
        </Typography>
      </Tooltip>
    ),
  },
  {
    field: 'liquidityScore',
    headerName: 'Ликвидность',
    width: 180,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => {
      if (p.row.liquidityTier === null) {
        return <Typography variant="caption" color="text.secondary">нет данных</Typography>;
      }
      const color =
        p.row.liquidityTier === 'high' ? 'success'
          : p.row.liquidityTier === 'medium' ? 'warning'
          : 'default';
      return (
        <Tooltip title={`Наблюдений: ${formatInt(p.row.observations)}, средний объём ${formatCompact(p.row.volumeAvg)}`}>
          <Chip size="small" color={color} variant="outlined" label={`${p.row.liquidityScore} / 100`} />
        </Tooltip>
      );
    },
  },
];
```

- [ ] **Step 6: Write the page**

`frontend/src/features/recipes/RecipesPage.tsx` (replacing the placeholder):

```tsx
import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { DataGrid } from '@mui/x-data-grid';
import { useMemo, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { canonicalSkill } from '../../assets/skills';
import { useCalcBatch } from '../../api/queries/calc';
import { useLatestPrices, useLiquidityStats } from '../../api/queries/prices';
import { usePriceableItemIds, useSkillRecipes } from '../../api/queries/recipes';
import { usePlayerPrefs } from '../character/usePlayerPrefs';
import { AssumptionsBar } from './AssumptionsBar';
import { RecipeFilters } from './RecipeFilters';
import type { FiltersValue } from './RecipeFilters';
import { buildRows, firstAssumptions } from './buildRows';
import { recipeColumns } from './columns';

const PAGE_SIZE = 25;

export function RecipesPage() {
  const player = usePlayerPrefs();
  const [search] = useSearchParams();
  const [filters, setFilters] = useState<FiltersValue>({
    // Task 6's character page links here as /recipes?skill=Crafting when a
    // skill row is clicked, so the initial filter comes from the URL when
    // present. canonicalSkill rejects a tampered or unknown value.
    skill: canonicalSkill(search.get('skill') ?? '') ?? '',
    minLevel: 1,
    maxLevel: 120,
    includeIncomplete: false,
    spreadPct: undefined,
  });
  const [page, setPage] = useState(0);

  const recipes = useSkillRecipes(filters.skill, filters.maxLevel);
  const priceable = usePriceableItemIds(filters.skill, filters.minLevel, filters.maxLevel);

  const backbone = useMemo(() => {
    if (!recipes.data || !priceable.data) return [];
    const seen = new Set<number>();
    return recipes.data.filter((entry) => {
      const id = entry.output_item_id;
      if (!id || seen.has(id) || !priceable.data.has(id)) return false;
      seen.add(id);
      return true;
    });
  }, [recipes.data, priceable.data]);

  const pageRecipes = useMemo(
    () => backbone.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE),
    [backbone, page],
  );
  const pageIds = useMemo(
    () => pageRecipes.map((entry) => entry.output_item_id),
    [pageRecipes],
  );

  const calc = useCalcBatch(pageIds, {
    player: player.name || undefined,
    mode: player.mode,
    spreadPct: filters.spreadPct,
    includeIncomplete: filters.includeIncomplete,
  });
  const prices = useLatestPrices(pageIds);
  const stats = useLiquidityStats(pageIds);

  const rows = useMemo(
    () =>
      buildRows({
        recipes: pageRecipes,
        calc: calc.data ?? new Map(),
        prices: prices.data ?? new Map(),
        stats: stats.data,
      }),
    [pageRecipes, calc.data, prices.data, stats.data],
  );

  const assumptions = useMemo(
    () => (calc.data ? firstAssumptions(calc.data.values()) : null),
    [calc.data],
  );

  const lastPage = Math.max(0, Math.ceil(backbone.length / PAGE_SIZE) - 1);

  return (
    <Stack spacing={2}>
      <Typography variant="h5" component="h1">Рецепты</Typography>

      <RecipeFilters
        value={filters}
        onChange={(next) => { setFilters(next); setPage(0); }}
      />

      {player.name === '' && (
        <Alert severity="info" variant="outlined">
          Имя персонажа не задано, поэтому доступность рецептов по уровням не проверяется.
          Укажите его на странице «Персонаж».
        </Alert>
      )}

      <AssumptionsBar assumptions={assumptions} />

      {filters.includeIncomplete && (
        <Alert severity="warning" variant="outlined">
          Включены пути с неоценёнными входами. Их деньги — верхняя граница, а не оценка.
        </Alert>
      )}

      {filters.skill === '' ? (
        <Typography color="text.secondary">Выберите скилл, чтобы увидеть рецепты.</Typography>
      ) : (
        <>
          <Box sx={{ height: 640 }}>
            <DataGrid
              rows={rows}
              columns={recipeColumns}
              loading={recipes.isFetching || calc.isFetching || stats.isPending}
              hideFooter
              disableRowSelectionOnClick
              density="compact"
            />
          </Box>

          <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
            <Button size="small" disabled={page === 0} onClick={() => setPage(page - 1)}>
              Назад
            </Button>
            <Typography variant="body2">
              Страница {page + 1} из {lastPage + 1}, всего {backbone.length} предметов
            </Typography>
            <Button size="small" disabled={page >= lastPage} onClick={() => setPage(page + 1)}>
              Вперёд
            </Button>
          </Stack>

          <Typography variant="caption" color="text.secondary">
            Сортировка работает в пределах загруженной страницы: сервер считает выгодность
            по одному предмету, поэтому весь скилл сразу не рассчитывается.
          </Typography>
        </>
      )}
    </Stack>
  );
}
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npx vitest run src/features/recipes/RecipesPage.test.tsx`
Expected: PASS, 5 tests.

If a `getByText` assertion fails because the DataGrid virtualises the cell out of the DOM, set the grid height so all rows in the test fit, or query by the row's accessible name — do not weaken the assertion to `queryAllByText(...).length >= 0`.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/features/recipes
git commit -m "Добавить таблицу рецептов с маржой, ROI и ликвидностью"
```

---

## Task 12: Live price updates, production image, full verification

**Files:**
- Create: `frontend/src/features/recipes/useLivePrices.ts`
- Modify: `frontend/src/features/recipes/RecipesPage.tsx`
- Create: `frontend/Dockerfile`, `frontend/nginx.conf`
- Modify: `docker-compose.yml`
- Test: `frontend/src/features/recipes/useLivePrices.test.tsx`

**Interfaces:**
- Consumes: `useWs` from Task 8, `queryKeys` from Task 3.
- Produces: `useLivePrices(itemIds: number[]): { staleItemIds: Set<number>; clearStale(): void }` — subscribes the socket to the given IDs, patches the `latestPrices` cache entry on each price frame, and reports which rows moved since the last calculation.

Calculated columns are deliberately not recomputed from a price frame. The
margin comes from `/calc`; recomputing it in the browser would produce a
number that disagrees with the server's next answer. The row is marked stale
and the user re-runs the calculation instead.

- [ ] **Step 1: Write the failing test**

`frontend/src/features/recipes/useLivePrices.test.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import type { ReactNode } from 'react';
import { queryKeys } from '../../api/queryKeys';
import { snapshot } from '../../test/fixtures';
import { useLivePrices } from './useLivePrices';

const subscribe = vi.fn();
const unsubscribe = vi.fn();
let emitPrice: ((snapshot: ReturnType<typeof snapshot>) => void) | null = null;

vi.mock('../../ws/WsProvider', () => ({
  useWs: () => ({
    status: 'open',
    messages: [],
    serverError: null,
    sendChat: vi.fn(),
    subscribe,
    unsubscribe,
    onPrice: (listener: (s: ReturnType<typeof snapshot>) => void) => {
      emitPrice = listener;
      return () => { emitPrice = null; };
    },
  }),
}));

function makeWrapper(client: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
}

it('subscribes to the page item ids and unsubscribes on unmount', () => {
  const client = new QueryClient();
  const { unmount } = renderHook(() => useLivePrices([1603, 1605]), {
    wrapper: makeWrapper(client),
  });

  expect(subscribe).toHaveBeenCalledWith([1603, 1605]);
  unmount();
  expect(unsubscribe).toHaveBeenCalledWith([1603, 1605]);
});

it('patches the cached price and marks the row stale', async () => {
  const client = new QueryClient();
  client.setQueryData(
    queryKeys.latestPrices([1603]),
    new Map([[1603, snapshot({ price: 2707 })]]),
  );

  const { result } = renderHook(() => useLivePrices([1603]), { wrapper: makeWrapper(client) });

  act(() => { emitPrice?.(snapshot({ price: 3100 })); });

  await waitFor(() => expect(result.current.staleItemIds.has(1603)).toBe(true));
  const cached = client.getQueryData<Map<number, ReturnType<typeof snapshot>>>(
    queryKeys.latestPrices([1603]),
  );
  expect(cached?.get(1603)?.price).toBe(3100);
});

it('ignores a price frame for an item outside the current page', async () => {
  const client = new QueryClient();
  const { result } = renderHook(() => useLivePrices([1603]), { wrapper: makeWrapper(client) });

  act(() => { emitPrice?.(snapshot({ item_id: 9999, price: 10 })); });

  await waitFor(() => expect(result.current.staleItemIds.size).toBe(0));
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker compose run --rm frontend npx vitest run src/features/recipes/useLivePrices.test.tsx`
Expected: FAIL — `Cannot find module './useLivePrices'`.

- [ ] **Step 3: Write the hook**

`frontend/src/features/recipes/useLivePrices.ts`:

```ts
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useState } from 'react';
import { queryKeys } from '../../api/queryKeys';
import type { PriceSnapshot } from '../../api/queries/prices';
import { useWs } from '../../ws/WsProvider';

export function useLivePrices(itemIds: number[]) {
  const { subscribe, unsubscribe, onPrice } = useWs();
  const queryClient = useQueryClient();
  const [staleItemIds, setStaleItemIds] = useState<Set<number>>(new Set());

  const key = itemIds.join(',');

  useEffect(() => {
    if (itemIds.length === 0) return;
    const ids = key.split(',').map(Number);
    subscribe(ids);
    return () => unsubscribe(ids);
    // `key` is the stable identity of the id list; `itemIds` is a new array
    // on every render and would resubscribe endlessly.
  }, [key, subscribe, unsubscribe]);

  useEffect(() => {
    const ids = new Set(key.length > 0 ? key.split(',').map(Number) : []);

    return onPrice((snapshot: PriceSnapshot) => {
      if (!ids.has(snapshot.item_id)) return;

      queryClient.setQueryData<Map<number, PriceSnapshot>>(
        queryKeys.latestPrices([...ids]),
        (current) => {
          const next = new Map(current ?? []);
          next.set(snapshot.item_id, snapshot);
          return next;
        },
      );

      setStaleItemIds((current) => new Set(current).add(snapshot.item_id));
    });
  }, [key, onPrice, queryClient]);

  return {
    staleItemIds,
    clearStale: useCallback(() => setStaleItemIds(new Set()), []),
  };
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `docker compose run --rm frontend npx vitest run src/features/recipes/useLivePrices.test.tsx`
Expected: PASS, 3 tests.

- [ ] **Step 5: Wire live prices into the recipes page**

In `frontend/src/features/recipes/RecipesPage.tsx`, after the `stats` query:

```tsx
const live = useLivePrices(pageIds);
const { status } = useWs();
```

Add the imports this needs at the top of the file:

```tsx
import { useWs } from '../../ws/WsProvider';
import { useLivePrices } from './useLivePrices';
```

Below `<AssumptionsBar />`, add the degradation notice and the recalculation control:

```tsx
{status !== 'open' && (
  <Alert severity="info" variant="outlined">
    Живые цены приходят по токену. Войдите в чате, чтобы получать их сразу;
    без входа цены обновляются раз в 30 секунд.
  </Alert>
)}

{live.staleItemIds.size > 0 && (
  <Alert
    severity="warning"
    variant="outlined"
    action={
      <Button
        size="small"
        onClick={() => { live.clearStale(); void calc.refetch(); }}
      >
        Пересчитать
      </Button>
    }
  >
    Цены изменились у {live.staleItemIds.size} позиций. Маржа считается на сервере,
    поэтому её нужно пересчитать, а не пересобирать в браузере.
  </Alert>
)}
```

Add polling for the signed-out case by passing `refetchInterval` to `useLatestPrices`. Change its signature in `frontend/src/api/queries/prices.ts`:

```ts
export function useLatestPrices(ids: number[], opts?: { pollMs?: number }) {
  return useQuery({
    queryKey: queryKeys.latestPrices(ids),
    enabled: ids.length > 0,
    refetchInterval: opts?.pollMs ?? false,
    // ... unchanged body
  });
}
```

and call it from the page as:

```tsx
const prices = useLatestPrices(pageIds, { pollMs: status === 'open' ? undefined : 30_000 });
```

Run: `docker compose run --rm frontend npm test`
Expected: PASS, every suite.

- [ ] **Step 6: Write the production image**

`frontend/Dockerfile`:

```dockerfile
FROM node:22-alpine AS build
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY combined.yaml /app/combined.yaml
COPY frontend/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend/dist /usr/share/nginx/html
```

The build context is the repository root, not `frontend/`, because the image needs `combined.yaml` for a type check.

`frontend/nginx.conf`:

```nginx
server {
  listen 80;
  root /usr/share/nginx/html;

  location / {
    try_files $uri $uri/ /index.html;
  }
}
```

Add the production service to `docker-compose.yml`, leaving the dev service untouched:

```yaml
  frontend-prod:
    profiles: ["prod"]
    build:
      context: .
      dockerfile: frontend/Dockerfile
    ports:
      - "8090:80"
```

Verify the production build:

```bash
docker compose --profile prod build frontend-prod
```

Expected: the build completes and `npm run build` inside it reports no TypeScript errors.

- [ ] **Step 7: Verify the whole suite and the type build**

Run, and paste the real output into the commit discussion rather than summarising it:

```bash
docker compose run --rm frontend npm test
docker compose run --rm frontend npx tsc -b
```

Expected: all tests pass; `tsc` exits 0 with no output.

Then start the app and open it:

```bash
docker compose up -d frontend
```

Open `http://localhost:5173`. Confirm by eye:

1. The header shows the gateway status chip.
2. The character page returns levels for `Zezima`.
3. The recipes page lists Crafting items and — with the backend in its current state — shows `no priceable production path` per row rather than zeros. This is the known defect from spec section 2.1, not a frontend bug.
4. The chat page loads history, and after registering and logging in a message sent from one browser tab appears in a second tab.

If prices have been repopulated on the backend by this point, check that margin, ROI, GP/h and the liquidity chip carry real numbers, and note it.

- [ ] **Step 8: Commit**

```bash
git add frontend docker-compose.yml
git commit -m "Добавить живые цены в таблице и продакшен-образ"
```

---

## Verification Checklist

After the final task, confirm each spec requirement has a home:

- Character display with skill levels and icons — Task 6, icons from Task 4.
- Recipe table with skill icon, item icon, price, component cost, margin, ROI, XP/h, GP/h — Tasks 9 and 11.
- Liquidity in the table — Task 10 (`/prices/stats`) and Task 11 (the chip).
- Live chat with registration and login — Tasks 5 and 8.
- Live updates on prices — Tasks 7 and 12.
- Dark theme, no emoji — Task 3 and the global constraints.
- TypeScript, schema generated by an external tool, React — Task 1.
- Three distinct failure states, including the gateway 502 naming its target — Task 3 (`api/errors.ts`), consumed by every query.

---

## Task 13: Single screen — English copy, character left, recipes centre, chat popup, status dot

Run this task AFTER Task 6 and BEFORE Task 7. It is numbered 13 only because
it was added after the plan was first written; execution order is by the
ledger, not by number.

This task restructures what Tasks 3, 5 and 6 already shipped. It changes no
data fetching and adds no API call.

**Files:**
- Modify: `frontend/src/App.tsx` (routes removed, three-zone layout)
- Modify: `frontend/src/main.tsx` (router still needed for URL state, no `Routes`)
- Modify: `frontend/src/shared/HealthIndicator.tsx` (dot, no text)
- Modify: `frontend/src/features/character/CharacterPage.tsx` → becomes the left column
- Modify: `frontend/src/features/character/SkillRow.tsx` (English accessible name, click filters in place)
- Modify: `frontend/src/features/chat/ChatPage.tsx` → placeholder stays until Task 8 turns it into the popup
- Modify: every file carrying Russian UI copy from Tasks 3, 5 and 6
- Test: `frontend/src/App.test.tsx`, `frontend/src/features/character/CharacterPage.test.tsx`, `frontend/src/api/errors.test.ts`

**Interfaces:**
- Consumes: everything Tasks 3-6 produced.
- Produces:
  - `<AppLayout>` — the three-zone shell: fixed-width left column, fluid centre, popup slot bottom-right.
  - `selectedSkill` state lifted to the shell, so clicking a skill row in the left column filters the centre table without navigation. Task 11's `RecipesPage` receives it as a prop instead of reading a route.
  - `<StatusDot>` — green / amber / red, with an accessible label.

Heading levels: with both zones mounted at once there must be exactly ONE
`<h1>` on the screen. The app bar title is it. "Character" and "Recipes" are
`component="h2"`. Before this task they were both `h1`, which was fine only
because routing meant one was ever mounted at a time.

- [ ] **Step 1: Write the failing tests**

Replace the routing assertions in `frontend/src/App.test.tsx`. The nav links are gone; there is one screen.

```tsx
it('renders the character column and the recipe area on one screen', async () => {
  server.use(healthHandler());
  renderWithProviders(<App />);

  expect(await screen.findByRole('heading', { name: 'Character' })).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: 'Recipes' })).toBeInTheDocument();
  expect(screen.queryByRole('link', { name: 'Персонаж' })).not.toBeInTheDocument();
});

it('shows a green status dot when every upstream is healthy', async () => {
  server.use(healthHandler());
  renderWithProviders(<App />);

  expect(await screen.findByLabelText('All services operational')).toBeInTheDocument();
  expect(screen.queryByText(/Все сервисы/)).not.toBeInTheDocument();
});

it('shows an amber dot when some upstreams are down', async () => {
  server.use(
    http.get('http://localhost:8080/health/all', () =>
      HttpResponse.json({
        gateway: 'ok',
        all_upstreams_ok: false,
        upstreams: [{ target: 'http://recipe-service:8082', ok: false, status: 0, error: 'refused', ms: 1 }],
      }),
    ),
  );
  renderWithProviders(<App />);

  // Naming the downed service is the whole point of the amber state, so the
  // assertion has to reach the service name, not just the prefix.
  expect(
    await screen.findByLabelText(/Some services are not responding:.*recipe-service/),
  ).toBeInTheDocument();
});

it('shows a red dot when the gateway itself cannot be reached', async () => {
  server.use(http.get('http://localhost:8080/health/all', () => HttpResponse.error()));
  renderWithProviders(<App />);

  expect(await screen.findByLabelText('Services unavailable')).toBeInTheDocument();
});
```

In `CharacterPage.test.tsx`, retarget the existing assertions to English copy
(`Player name`, `Show`, `Mode`) and replace the navigation test: clicking a
skill no longer changes the URL, it sets the selected skill in the shell.

```tsx
it('puts the total level next to the player name', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));

  const heading = await screen.findByTestId('player-summary');
  expect(heading).toHaveTextContent('Zezima');
  expect(heading).toHaveTextContent(/3\s*232/);
});

it('reports the fetch time quietly rather than prominently', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));

  const stamp = await screen.findByTestId('fetched-at');
  expect(stamp).toBeInTheDocument();
  expect(stamp).toHaveClass(/MuiTypography-caption/);
});

it('reports a selected skill to the shell instead of navigating', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));
  const onSelectSkill = vi.fn();

  renderWithProviders(<CharacterColumn onSelectSkill={onSelectSkill} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));
  await userEvent.click(await screen.findByRole('button', { name: /Crafting/ }));

  expect(onSelectSkill).toHaveBeenCalledWith('Crafting');
});
```

Also retarget `errors.test.ts`: the three taxonomy messages become English —
`No connection to the gateway`, `Service <name> is unavailable`, `One of the
services is unavailable`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker compose run --rm frontend npm test`
Expected: FAIL — Russian copy still present, `CharacterColumn` does not exist, no status dot label.

- [ ] **Step 3: Translate every user-facing string to English**

Go file by file through what Tasks 3, 5 and 6 shipped and translate the UI
copy. This is mechanical but must be complete — a single Russian label left
behind is a failure of this task. The files carrying copy are
`api/errors.ts`, `api/queries/*.ts` (the fallback messages), `api/client.ts`
(the middleware fallback), `auth/AuthProvider.tsx`, `shared/QueryState.tsx`,
`shared/HealthIndicator.tsx`, `App.tsx`, `features/character/*`,
`features/chat/ChatPage.tsx`, `features/recipes/RecipesPage.tsx`.

Do NOT translate: code comments (leave them as they are), commit messages,
anything under `docs/`, or the RuneScape proper nouns (skill names, mode
names like `Ironman`).

- [ ] **Step 4: Write the status dot**

`frontend/src/shared/HealthIndicator.tsx` becomes a coloured dot with no
visible text. The accessible label carries the meaning:

```tsx
import { Box, Tooltip } from '@mui/material';
import { useGatewayHealth } from '../api/queries/health';
import { serviceName } from '../api/errors';

export function HealthIndicator() {
  const { data, isError } = useGatewayHealth();

  const state = isError
    ? { color: 'error.main', label: 'Services unavailable' }
    : !data
      ? { color: 'text.disabled', label: 'Checking services' }
      : data.all_upstreams_ok
        ? { color: 'success.main', label: 'All services operational' }
        : {
            color: 'warning.main',
            label: `Some services are not responding: ${data.upstreams
              .filter((u) => !u.ok)
              .map((u) => serviceName(u.target) ?? u.target)
              .join(', ')}`,
          };

  return (
    <Tooltip title={state.label}>
      <Box
        role="status"
        aria-label={state.label}
        sx={{ width: 10, height: 10, borderRadius: '50%', bgcolor: state.color }}
      />
    </Tooltip>
  );
}
```

- [ ] **Step 5: Build the three-zone shell**

`frontend/src/App.tsx` drops `Routes`, `Route` and the nav buttons entirely.
The character column and the recipe area sit side by side; the chat popup
occupies a fixed slot at the bottom right.

```tsx
export default function App() {
  const [selectedSkill, setSelectedSkill] = useState('');

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <AppBar position="static" color="transparent" elevation={0}>
        <Toolbar sx={{ gap: 2, borderBottom: 1, borderColor: 'divider', minHeight: 52 }}>
          <Typography variant="h6" sx={{ color: 'primary.main', fontWeight: 700, flexGrow: 1 }}>
            RS3 Market
          </Typography>
          <HealthIndicator />
        </Toolbar>
      </AppBar>

      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Box
          component="aside"
          sx={{
            width: 320,
            flexShrink: 0,
            borderRight: 1,
            borderColor: 'divider',
            overflowY: 'auto',
            p: 2,
          }}
        >
          <CharacterColumn onSelectSkill={setSelectedSkill} />
        </Box>

        <Box component="main" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', p: 2 }}>
          <RecipesPage skill={selectedSkill} onSkillChange={setSelectedSkill} />
        </Box>
      </Box>

      <ChatPopup />
    </Box>
  );
}
```

`minWidth: 0` on the main zone is load-bearing: without it a wide DataGrid
forces the flex container wider than the viewport and pushes the left column
off screen.

Until Task 8 builds the real popup, `ChatPopup` is a minimal placeholder in
`features/chat/ChatPopup.tsx` rendering a collapsed bar at the bottom right —
no chat behaviour yet.

- [ ] **Step 6: Rework the character column**

Rename `CharacterPage` to `CharacterColumn` and change its contract: it takes
`onSelectSkill(skill: string): void` and no longer navigates. The name and
the total level share one line. `fetched_at` stays but as
`Typography variant="caption" color="text.secondary"` with
`data-testid="fetched-at"`, visually quiet.

`SkillRow` becomes a `ButtonBase`-backed button rather than a router `Link`,
calling `onSelect(skill)`. Its accessible name is
`Filter recipes by <Skill>`.

`RecipesPage` takes `skill` and `onSkillChange` props instead of reading the
URL. Remove the `useSearchParams` wiring the plan added for Task 11 — the two
zones are on one screen now, so there is nothing to navigate to. Keep
`canonicalSkill` validation on whatever value arrives.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `docker compose run --rm frontend npm test`
Expected: PASS, every suite. Then `docker compose run --rm frontend npx tsc -b`, exit 0.

Then grep for leftovers — this must return nothing:

```bash
docker compose run --rm frontend sh -c "grep -rnP '[\\x{0400}-\\x{04FF}]' src --include='*.tsx' --include='*.ts' | grep -v '^src/.*://' | grep -vP '^\\S+:\\d+:\\s*(//|\\*)'"
```

Any hit that is a string literal rather than a comment is a missed translation.

- [ ] **Step 8: Commit**

```bash
git add frontend/src
git commit -m "Собрать единый экран с английским интерфейсом и индикатором статуса"
```

---

---

## Task 8 amendment: chat is a popup, not a page

This overrides Task 8 wherever the two disagree. Everything Task 8 says about
`useChatHistory`, `WsProvider`, the auth panel, the message list and the
composer still holds; what changes is the container and the copy.

- `features/chat/ChatPage.tsx` does not exist. The component is
  `features/chat/ChatPopup.tsx`, and Task 13 already created it as an inert
  collapsed bar. You are giving that bar its behaviour.
- All user-facing copy is ENGLISH. The Russian strings in Task 8's code
  samples are stale — translate them. `Сообщение` becomes `Message`,
  `Отправить` becomes `Send`, `Войти` becomes `Sign in`,
  `Зарегистрироваться` becomes `Register`, `Выйти` becomes `Sign out`,
  `Логин` becomes `Username`, `Пароль` becomes `Password`,
  `Сообщений пока нет.` becomes `No messages yet.`,
  `Вы вошли как {username}` becomes `Signed in as {username}`,
  `Сессия недействительна, войдите заново.` becomes
  `Session is no longer valid. Sign in again.`,
  `Соединение потеряно, живые сообщения не приходят.` becomes
  `Connection lost. Live messages are not arriving.`,
  `Чат и живые цены работают по токену, поэтому требуется вход.` becomes
  `Chat and live prices require a token, so you need to sign in.`
- The popup is anchored bottom-right, `position: fixed`, above the rest of
  the content. Collapsed it is a narrow bar showing `Chat` and, when messages
  arrived while collapsed, an unread count. Expanded it is a panel roughly
  360px wide and 480px tall holding the auth panel, the message list and the
  composer.
- It must not steal width from the recipe table. It overlays; it never
  participates in the flex row that `App.tsx` builds.
- There are no routes. `App.tsx` already mounts `<ChatPopup />` as the last
  child of the shell.

Additional tests for the popup shell, on top of Task 8's own:

```tsx
it('starts collapsed and expands when clicked', async () => {
  server.use(historyHandler());
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });

  expect(screen.queryByText('Hey!')).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Chat' }));

  expect(await screen.findByText('Hey!')).toBeInTheDocument();
});

it('counts messages that arrive while collapsed', async () => {
  signedIn();
  server.use(historyHandler());
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });

  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({
        type: 'chat',
        payload: { id: 2, user_id: 2, username: 'fe_probe', body: 'While away', created_at: '2026-09-15T10:29:51.487Z' },
      }),
    });
  });

  expect(await screen.findByLabelText('Chat, 1 unread message')).toBeInTheDocument();
});

it('clears the unread count once expanded', async () => {
  signedIn();
  server.use(historyHandler());
  renderWithProviders(<ChatPopup />, { socketFactory: (url) => new FakeSocket(url) });

  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({
        type: 'chat',
        payload: { id: 2, user_id: 2, username: 'fe_probe', body: 'While away', created_at: '2026-09-15T10:29:51.487Z' },
      }),
    });
  });
  await userEvent.click(await screen.findByLabelText('Chat, 1 unread message'));

  expect(screen.getByRole('button', { name: 'Chat' })).toBeInTheDocument();
});
```

---

---

## Tasks 9-11 amendment: English copy, skill as a prop, all crafts by default

The bodies of `task-9-brief.md`, `task-10-brief.md` and `task-11-brief.md`
were written before the single-screen redesign. Their **structure, file
layout, function signatures, row-building logic, column set and test cases
remain correct** and are your requirements. Three things in them are stale.
Where this amendment and a brief disagree, **this amendment wins**.

Task 9's brief needs no change; it is listed here only because you build it
in the same dispatch and `RecipeRow` is what Tasks 10 and 11 consume.

---

## Delta A — every user-facing string is English

The application has no Russian UI text left; the other twelve tasks are
already translated and `App.test.tsx` asserts the Russian copy is gone. The
briefs' Russian strings are the last of it. Translate **every** user-visible
string you write — labels, headers, tooltips, chips, empty states, error
fallbacks, and the strings inside test assertions.

Use exactly these, so the tests and the UI agree:

| Brief (Russian) | Use this |
|---|---|
| `Скилл` (filter label and column header) | `Skill` |
| `Уровень от` | `Min level` |
| `Уровень до` | `Max level` |
| `Спред, %` | `Spread, %` |
| `Показывать пути с неоценёнными входами` | `Include paths with unpriced inputs` |
| `Предмет` | `Item` |
| `Уровень` (column header) | `Level` |
| `Цена` | `Price` |
| `Компоненты` | `Components` |
| `Маржа` | `Margin` |
| `XP/ч` | `XP/h` |
| `GP/ч` | `GP/h` |
| `GP/XP` | `GP/XP` (unchanged) |
| `GP/ч по лимиту` | `GP/h capped` |
| `Ликвидность` | `Liquidity` |
| `нет данных` | `no data` |
| `Не оценены: {list}` | `Unpriced: {list}` |
| `Связывающий вход: {name}` | `Binding input: {name}` |
| `Лимит покупки неизвестен` | `Buy limit unknown` |
| `Наблюдений: {n}, средний объём {v}` | `Observations: {n}, average volume {v}` |
| caveat title `incomplete` | `Some inputs are unpriced and counted as zero — the margin is an upper bound` |
| caveat title `default-aph` | `Action rate assumed by the server, not measured` |
| caveat title `no-throughput` | `Buy limit unknown, throughput cap not computed` |
| caveat label `incomplete` | `incomplete` |
| caveat label `default-aph` | `default APH` |
| caveat label `no-throughput` | `uncapped` |
| `Сортировка работает в пределах загруженной страницы` | `Sorting applies within the loaded page` |
| `Цена — гайдовая цена Grand Exchange. Спред {s}, налог {t}, потолок налога {c}, освобождение ниже {e}. GP/ч без этих допущений не имеет смысла.` | `Price is the Grand Exchange guide price. Spread {s}, tax {t}, tax cap {c}, exempt below {e}. GP/h is meaningless without these assumptions.` |
| `Не удалось загрузить рецепты` | `Could not load recipes` |
| `Не удалось загрузить список предметов` | `Could not load the item list` |
| any other query-failure fallback | translate literally, sentence case, no trailing period |

Test assertions in the briefs quote the Russian strings (e.g.
`getByLabelText('Скилл')`, `findByText(/Спред 2\.0%/)`). Update each to its
English counterpart from this table. Keep the assertion's shape — do not
weaken a regex into a substring match while translating it.

## Delta B — the selected skill is a prop, not a URL parameter

There are no routes and no separate pages. The application is one screen: a
character column on the left, the recipe table in the centre, a chat popup
overlay. `App.tsx` owns the selected skill as state and passes it down;
clicking a skill in the character column sets it.

`RecipesPage.tsx` already exists as a placeholder with the correct
signature — keep it:

```tsx
interface Props {
  skill: string;
  onSkillChange: (skill: string) => void;
}

export function RecipesPage({ skill, onSkillChange }: Props)
```

So, in Task 11 Step 6:

- **Delete** `import { useSearchParams } from 'react-router-dom';` and the
  `const [search] = useSearchParams();` line. `RecipesPage` must not import
  from `react-router-dom` at all.
- The filter state's `skill` is no longer local. Derive it from the prop and
  push changes up:

```tsx
const filters: FiltersValue = { ...localFilters, skill };
const handleFilters = (next: FiltersValue) => {
  if (next.skill !== skill) onSkillChange(next.skill);
  setLocalFilters(next);
};
```

  where `localFilters` is the `useState` holding only `minLevel`, `maxLevel`,
  `includeIncomplete` and `spreadPct`. `FiltersValue` keeps all five fields —
  `RecipeFilters` is unchanged and still renders the skill selector.
- Validate the incoming prop with `canonicalSkill`, exactly as the brief
  already does for the URL value: `canonicalSkill(skill) ?? ''`. A skill the
  shell hands down that is not one of the 29 canonical names falls back to
  "all skills" rather than querying for it.
- Reset `page` to 0 when `skill` changes, or a deep page from a previous
  skill survives into a shorter result set.

Add this test to `RecipesPage.test.tsx`:

```tsx
it('filters by the skill the shell selects and reports changes upward', async () => {
  const onSkillChange = vi.fn();
  const { rerender } = renderWithProviders(
    <RecipesPage skill="" onSkillChange={onSkillChange} />,
  );
  // ...assert an unfiltered request went out, then:
  rerender(<RecipesPage skill="Crafting" onSkillChange={onSkillChange} />);
  // ...assert the next request carries skill=Crafting.
});
```

## Delta C — no skill selected means every craft, not an empty table

The brief's `Выберите скилл, чтобы увидеть рецепты.` empty state is removed
entirely. With no skill selected the table shows all crafts.

This is implementable against the current API: `skill` is **optional** on
both `GET /recipes` and `GET /recipes/ids` (verified in `combined.yaml`
lines 108-138 and 139-178). Omitting it returns every recipe.

In Task 10:

- Drop `enabled: skill.length > 0` from **both** `useSkillRecipes` and
  `usePriceableItemIds`. They always run.
- Omit the `skill` query parameter when the skill is empty, rather than
  sending `skill=`:

```ts
params: { query: { ...(skill ? { skill } : {}), level: maxLevel } },
```

  and the same shape in `usePriceableItemIds`.
- Replace the brief's test `it('does not query while no skill is chosen')`
  — that behaviour is gone — with:

```ts
it('omits the skill parameter when no skill is chosen', async () => {
  let seen = '';
  server.use(
    http.get('http://localhost:8080/recipes', ({ request }) => {
      seen = new URL(request.url).search;
      return HttpResponse.json({ count: 1, recipes: [recipe()] });
    }),
  );

  const { result } = renderHook(() => useSkillRecipes('', 99), { wrapper });

  await waitFor(() => expect(result.current.data).toHaveLength(1));
  expect(seen).not.toContain('skill');
  expect(seen).toContain('level=99');
});
```

- The query keys already carry `skill` as their second element, so `''` and
  `'Crafting'` cache separately. No change needed there.

In Task 11:

- `RecipeFilters`' skill selector gains a first option for the unfiltered
  case, so the user can get back to it after choosing a skill:

```tsx
<MenuItem value="">All skills</MenuItem>
```

  placed before the `SKILLS.map(...)`. The `TextField` keeps
  `label="Skill"`; an empty `value` renders as `All skills`.
- Replace the brief's "select a skill" empty-state test with:

```tsx
it('shows every craft when no skill is selected', async () => {
  renderWithProviders(<RecipesPage skill="" onSkillChange={() => {}} />);
  expect(await screen.findByText('Yew longbow')).toBeInTheDocument();
  expect(screen.getByText('Rune platebody')).toBeInTheDocument();
});
```

  with MSW returning two recipes of **different skills** from an
  unfiltered `/recipes`, proving no client-side skill filter is applied.
- Note the cost so a later reader understands the trade-off: unfiltered
  `/recipes` is a single ~4 MB body. It is fetched once and cached for an
  hour by the existing `staleTime`, and the table pages client-side at 25
  rows, so only 25 rows are ever sent to `/calc`. Put that in a short
  comment above the `backbone` memo — one sentence, not an essay.

---

## Unchanged and still binding

- `RecipeRow`'s field names are consumed verbatim as DataGrid column
  fields by Task 12. Do not rename them.
- The reason an unpriceable row is unpriceable is stated **once**, in the
  item column. Money columns render an em dash so a row never reads as a
  zero margin.
- MUI 9's `Stack` accepts only `children`, `component`, `direction`,
  `divider`, `spacing`, `sx` and `useFlexGap`. `alignItems`,
  `justifyContent` and `flexWrap` go in `sx` — the briefs already do this
  correctly; keep it that way in anything you add.
- The recipe table is the centre zone of the screen. It must not set a
  fixed width or overflow its column; `App.tsx` gives it `flex: 1` and
  `minWidth: 0`.
