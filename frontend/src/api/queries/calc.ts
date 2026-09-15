import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { CalcParams } from '../queryKeys';
import type { components } from '../schema';

export type CalcBatchEntry = components['schemas']['CalcBatchEntry'];
export type CalcResult = components['schemas']['CalcResult'];

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
      if (error || !data) throw failure(response, error, 'Could not calculate profitability');
      return new Map(data.results.map((entry) => [entry.item_id, entry]));
    },
  });
}

// The single-item sibling of useCalcBatch, for a row picked out of the item
// search: the batch endpoint is keyed on the page of IDs the recipe table is
// showing, and a searched item is rarely on it.
export function useCalcItem(itemId: number | null, params: CalcParams) {
  return useQuery({
    queryKey: queryKeys.calcItem(itemId ?? 0, params),
    enabled: itemId !== null,
    queryFn: async (): Promise<CalcResult> => {
      const { data, error, response } = await api.GET('/calc/{itemID}', {
        params: {
          path: { itemID: itemId as number },
          query: {
            include_incomplete: params.includeIncomplete,
            ...(params.player ? { player: params.player, mode: params.mode } : {}),
            ...(params.spreadPct === undefined ? {} : { spread_pct: params.spreadPct }),
            ...(params.aph === undefined ? {} : { aph: params.aph }),
          },
        },
      });
      if (error || !data) throw failure(response, error, 'Could not calculate profitability');
      return data;
    },
  });
}
