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
