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
      if (error || !data) throw failure(response, error, 'Could not calculate profitability');
      return new Map(data.results.map((entry) => [entry.item_id, entry]));
    },
  });
}
