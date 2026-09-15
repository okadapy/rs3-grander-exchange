import { http, HttpResponse } from 'msw';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, expect, it } from 'vitest';
import App from './App';
import { batchEntry, liquidity, recipe, snapshot } from './test/fixtures';
import { server } from './test/msw/server';
import { renderWithProviders } from './test/renderWithProviders';

function healthHandler() {
  return http.get('http://localhost:8080/health/all', () =>
    HttpResponse.json({ gateway: 'ok', all_upstreams_ok: true, upstreams: [] }),
  );
}

// App always mounts RecipesPage, which queries /recipes and /recipes/ids
// unconditionally (Task 11, Delta C: no skill selected still shows every
// craft). ChatPopup's history fetch is gated on being expanded, so it does
// not fire on mount and needs no handler here.
beforeEach(() => {
  localStorage.clear();
  server.use(
    http.get('http://localhost:8080/recipes', () => HttpResponse.json({ count: 0, recipes: [] })),
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: '', min_level: 1, max_level: 120, count: 0, item_ids: [] }),
    ),
  );
});

it('renders the character column and the recipe area on one screen', async () => {
  server.use(healthHandler());
  renderWithProviders(<App />);

  expect(await screen.findByRole('heading', { name: 'Character' })).toBeInTheDocument();
  expect(screen.getByRole('heading', { name: 'Recipes' })).toBeInTheDocument();
  expect(screen.queryByRole('link', { name: 'Персонаж' })).not.toBeInTheDocument();
  expect(screen.getAllByRole('heading', { level: 1 })).toHaveLength(1);
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

  expect(
    await screen.findByLabelText(/Some services are not responding:.*recipe-service/),
  ).toBeInTheDocument();
});

it('shows a red dot when the gateway itself cannot be reached', async () => {
  server.use(http.get('http://localhost:8080/health/all', () => HttpResponse.error()));
  renderWithProviders(<App />);

  expect(await screen.findByLabelText('Services unavailable')).toBeInTheDocument();
});

// The character column and the recipe table read the player name from one
// provider; before that it was two independent useState copies and the name
// typed on the left never reached /calc/batch, so no row was ever checked
// against the player's levels. Observed through MSW rather than by spying on
// the hook, because the request is the behaviour that matters.
it('sends the name typed in the character column to the profitability calculation', async () => {
  const players: (string | null)[] = [];
  server.use(
    healthHandler(),
    http.get('http://localhost:8080/hiscore/Zezima', () =>
      HttpResponse.json({
        id: 2,
        name: 'Zezima',
        mode: 'normal',
        fetched_at: '2026-09-15T10:24:24.873Z',
        skills: [{ id: 421, player_id: 2, skill: 'Overall', level: 3232, xp: 5709998811, rank: 6520 }],
      }),
    ),
    http.get('http://localhost:8080/recipes', () =>
      HttpResponse.json({ count: 1, recipes: [recipe()] }),
    ),
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: '', min_level: 1, max_level: 120, count: 1, item_ids: [1603] }),
    ),
    http.get('http://localhost:8080/prices/latest', () =>
      HttpResponse.json({ count: 1, prices: [snapshot()] }),
    ),
    http.get('http://localhost:8080/prices/stats/1603', () => HttpResponse.json(liquidity())),
    http.get('http://localhost:8080/calc/batch', ({ request }) => {
      players.push(new URL(request.url).searchParams.get('player'));
      return HttpResponse.json({ count: 1, results: [batchEntry()] });
    }),
  );

  renderWithProviders(<App />);

  await screen.findByRole('row', { name: /Ruby/ });
  await waitFor(() => expect(players).toEqual([null]));
  expect(screen.getByText(/Character name is not set/)).toBeInTheDocument();

  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));

  await waitFor(() => expect(players.at(-1)).toBe('Zezima'));
  expect(screen.queryByText(/Character name is not set/)).not.toBeInTheDocument();
});

it('switches the main area between the recipe table and the item search', async () => {
  server.use(healthHandler());
  renderWithProviders(<App />);

  expect(await screen.findByRole('heading', { name: 'Recipes' })).toBeInTheDocument();

  await userEvent.click(screen.getByRole('tab', { name: 'Items' }));

  expect(await screen.findByRole('heading', { name: 'Items' })).toBeInTheDocument();
  // The recipe table is unmounted, not hidden: it polls prices and
  // recalculates margins, and a background tab doing that is waste.
  expect(screen.queryByRole('heading', { name: 'Recipes' })).not.toBeInTheDocument();
  expect(screen.getByLabelText('Item name')).toBeInTheDocument();

  await userEvent.click(screen.getByRole('tab', { name: 'Recipes' }));
  expect(await screen.findByRole('heading', { name: 'Recipes' })).toBeInTheDocument();
});
