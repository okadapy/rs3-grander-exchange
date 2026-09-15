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
      if (error || !data) throw failure(response, error, 'Could not load prices');
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
        if (error || !data) throw failure(response, error, 'No liquidity data');
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
