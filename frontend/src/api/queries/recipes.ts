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
    staleTime: ONE_HOUR,
    queryFn: async (): Promise<Recipe[]> => {
      const { data, error, response } = await api.GET('/recipes', {
        params: { query: { ...(skill ? { skill } : {}), level: maxLevel } },
      });
      if (error || !data) throw failure(response, error, 'Could not load recipes');
      return data.recipes;
    },
  });
}

export function usePriceableItemIds(skill: string, minLevel: number, maxLevel: number) {
  return useQuery({
    queryKey: queryKeys.priceableIds(skill, minLevel, maxLevel),
    staleTime: ONE_HOUR,
    queryFn: async (): Promise<Set<number>> => {
      const { data, error, response } = await api.GET('/recipes/ids', {
        params: { query: { ...(skill ? { skill } : {}), min_level: minLevel, level: maxLevel } },
      });
      if (error || !data) throw failure(response, error, 'Could not load the item list');
      return new Set(data.item_ids);
    },
  });
}
