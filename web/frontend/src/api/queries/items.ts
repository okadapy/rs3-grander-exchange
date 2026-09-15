import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type ItemSummary = components['schemas']['ItemSummary'];
export type ItemSource = components['schemas']['ItemSource'];

export interface ItemSearchPage {
  items: ItemSummary[];
  total: number;
}

// Shorter queries match most of the catalogue, so the server would be asked
// to page through thousands of rows to answer a keystroke.
export const MIN_QUERY_LENGTH = 2;

export function useItemSearch(
  query: string,
  source: ItemSource,
  limit: number,
  offset: number,
) {
  return useQuery({
    queryKey: queryKeys.itemSearch(query, source, limit, offset),
    enabled: query.length >= MIN_QUERY_LENGTH,
    queryFn: async (): Promise<ItemSearchPage> => {
      const { data, error, response } = await api.GET('/search/items', {
        params: { query: { q: query, source, limit, offset } },
      });
      if (error || !data) throw failure(response, error, 'Could not search items');
      return { items: data.items, total: data.total };
    },
  });
}
