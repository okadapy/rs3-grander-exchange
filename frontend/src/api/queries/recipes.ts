import { useQuery } from '@tanstack/react-query';
import { api } from '../client';
import { failure } from '../errors';
import { queryKeys } from '../queryKeys';
import type { components } from '../schema';

export type Recipe = components['schemas']['Recipe'];
export type RecipeTreeNode = components['schemas']['RecipeTreeNode'];
export type RecipeInput = components['schemas']['RecipeInput'];

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

/**
 * Flattens a recipe tree to recipe name -> its declared inputs.
 *
 * A `CalcStep` names the recipe it performs but never what that recipe
 * consumes, so the money a step spends has no names attached to it without
 * this. Keyed by name because that is the only handle a step gives.
 */
export function inputsByRecipe(root: RecipeTreeNode): Map<string, RecipeInput[]> {
  const found = new Map<string, RecipeInput[]>();

  const walk = (node: RecipeTreeNode) => {
    if (!found.has(node.recipe.name)) found.set(node.recipe.name, node.recipe.inputs ?? []);
    for (const child of node.children ?? []) walk(child);
  };
  walk(root);

  return found;
}

// Fetched only when a breakdown is actually looked at: the recipe table shows
// 25 rows at a time and pre-fetching a tree for each would be 25 requests for
// a panel that opens over one of them.
export function useRecipeTree(itemId: number | null) {
  return useQuery({
    queryKey: queryKeys.recipeTree(itemId ?? 0),
    enabled: itemId !== null,
    staleTime: ONE_HOUR,
    queryFn: async (): Promise<Map<string, RecipeInput[]>> => {
      const { data, error, response } = await api.GET('/recipes/{itemID}', {
        params: { path: { itemID: itemId as number } },
      });
      if (error || !data) throw failure(response, error, 'Could not load the recipe tree');
      return inputsByRecipe(data.root);
    },
  });
}
