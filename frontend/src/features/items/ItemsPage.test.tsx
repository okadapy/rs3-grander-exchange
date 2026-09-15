import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it, vi } from 'vitest';
import { path, result, step } from '../../test/fixtures';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { ItemsPage } from './ItemsPage';

const ITEMS = [
  { item_id: 2357, name: 'Gold bar', sources: ['input', 'output'] },
  { item_id: 1673, name: 'Gold amulet', sources: ['output'] },
];

function searchHandler(queries: URLSearchParams[], items = ITEMS) {
  return http.get('http://localhost:8080/search/items', ({ request }) => {
    const params = new URL(request.url).searchParams;
    queries.push(params);
    return HttpResponse.json({
      query: params.get('q') ?? '',
      source: params.get('source') ?? 'all',
      count: items.length,
      total: items.length,
      limit: 25,
      offset: 0,
      items,
    });
  });
}

beforeEach(() => {
  localStorage.clear();
});

it('waits for a long enough query before asking the server', async () => {
  const queries: URLSearchParams[] = [];
  server.use(searchHandler(queries));
  renderWithProviders(<ItemsPage />);

  await userEvent.type(screen.getByLabelText('Item name'), 'g');
  await new Promise((resolve) => setTimeout(resolve, 400));
  expect(queries).toHaveLength(0);
  expect(screen.getByText(/Type at least 2 characters/)).toBeInTheDocument();

  await userEvent.type(screen.getByLabelText('Item name'), 'old');
  await waitFor(() => expect(queries.length).toBeGreaterThan(0));
  expect(queries.at(-1)?.get('q')).toBe('gold');
});

it('lists the matches with the side of a recipe they were found on', async () => {
  server.use(searchHandler([]));
  renderWithProviders(<ItemsPage />);

  await userEvent.type(screen.getByLabelText('Item name'), 'gold');

  const row = await screen.findByRole('row', { name: /Gold bar/ });
  expect(within(row).getByText('input, output')).toBeInTheDocument();
  expect(within(row).getByText('2357')).toBeInTheDocument();
});

it('narrows the search to one side of a recipe', async () => {
  const queries: URLSearchParams[] = [];
  server.use(searchHandler(queries));
  renderWithProviders(<ItemsPage />);

  await userEvent.type(screen.getByLabelText('Item name'), 'gold');
  await waitFor(() => expect(queries.length).toBeGreaterThan(0));

  await userEvent.click(screen.getByLabelText('Found as'));
  await userEvent.click(await screen.findByRole('option', { name: 'Produced by a recipe' }));

  await waitFor(() => expect(queries.at(-1)?.get('source')).toBe('output'));
});

it('breaks the chosen item down step by step', async () => {
  const calls: string[] = [];
  server.use(
    searchHandler([]),
    http.get('http://localhost:8080/calc/1673', ({ request }) => {
      calls.push(new URL(request.url).pathname);
      return HttpResponse.json(
        result({
          item_id: 1673,
          paths: [
            path({
              steps: [
                step({ recipe: 'Gold bar', buy_cost: 1_541, sell_revenue: 2_392, profit: 804 }),
                step({ recipe: 'Gold amulet', buy_cost: 0, sell_revenue: 385, profit: 378 }),
              ],
            }),
          ],
        }),
      );
    }),
  );
  renderWithProviders(<ItemsPage />);

  await userEvent.type(screen.getByLabelText('Item name'), 'gold');
  await userEvent.click(await screen.findByRole('row', { name: /Gold amulet/ }));

  const breakdown = await screen.findByRole('table');
  expect(within(breakdown).getByText('Gold bar')).toBeInTheDocument();
  // Each stage carries the server's own margin for it, last cell of its row.
  const lastStep = within(breakdown).getAllByRole('row')[2];
  expect(within(lastStep).getAllByRole('cell').at(-1)).toHaveTextContent('378');
  expect(calls).toEqual(['/calc/1673']);
});

it('says an item has no priceable path rather than drawing an empty table', async () => {
  server.use(
    searchHandler([]),
    http.get('http://localhost:8080/calc/1673', () => HttpResponse.json(result({ paths: [] }))),
  );
  renderWithProviders(<ItemsPage />);

  await userEvent.type(screen.getByLabelText('Item name'), 'gold');
  await userEvent.click(await screen.findByRole('row', { name: /Gold amulet/ }));

  expect(await screen.findByText(/No priceable way to produce Gold amulet/)).toBeInTheDocument();
});

it('says nothing matched instead of showing an empty grid', async () => {
  server.use(searchHandler([], []));
  renderWithProviders(<ItemsPage />);

  await userEvent.type(screen.getByLabelText('Item name'), 'zzz');

  expect(await screen.findByText(/Nothing matches/)).toBeInTheDocument();
  expect(screen.queryByRole('grid')).not.toBeInTheDocument();
});

it('reports a failed search with a retry rather than an empty result', async () => {
  vi.spyOn(console, 'error').mockImplementation(() => {});
  server.use(
    http.get('http://localhost:8080/search/items', () =>
      HttpResponse.json({ error: 'search index rebuilding' }, { status: 503 }),
    ),
  );
  renderWithProviders(<ItemsPage />);

  await userEvent.type(screen.getByLabelText('Item name'), 'gold');

  expect(await screen.findByText('search index rebuilding')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
});
