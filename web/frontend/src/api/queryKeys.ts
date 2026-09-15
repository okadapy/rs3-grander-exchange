import type { components } from './schema';

type HiscoreMode = components['schemas']['HiscoreMode'];
type ItemSource = components['schemas']['ItemSource'];

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
  calcItem: (itemId: number, params: CalcParams) =>
    ['calcItem', itemId, params] as const,
  recipeTree: (itemId: number) => ['recipeTree', itemId] as const,
  itemSearch: (query: string, source: ItemSource, limit: number, offset: number) =>
    ['itemSearch', query, source, limit, offset] as const,
  chatHistory: () => ['chatHistory'] as const,
};
