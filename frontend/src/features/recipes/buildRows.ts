import { canonicalSkill } from '../../assets/skills';
import type { components } from '../../api/schema';

type Recipe = components['schemas']['Recipe'];
type CalcBatchEntry = components['schemas']['CalcBatchEntry'];
type CalcAssumptions = components['schemas']['CalcAssumptions'];
type PriceSnapshot = components['schemas']['PriceSnapshot'];
type LiquidityStats = components['schemas']['LiquidityStats'];
type LiquidityTier = components['schemas']['LiquidityTier'];

export type RowCaveat = 'incomplete' | 'default-aph' | 'no-throughput';

export interface RecipeRow {
  id: number;
  itemId: number;
  itemName: string;
  skill: string;
  levelReq: number;
  meetsRequirements: boolean | null;
  price: number | null;
  componentsCost: number | null;
  margin: number | null;
  roiPct: number | null;
  xpPerHour: number | null;
  gpPerHour: number | null;
  gpPerXp: number | null;
  throughputGpPerHour: number | null;
  bindingItemName: string | null;
  liquidityTier: LiquidityTier | null;
  liquidityScore: number | null;
  observations: number | null;
  volumeAvg: number | null;
  buyLimit4h: number | null;
  caveats: RowCaveat[];
  unpricedInputs: string[];
  pathNames: string[];
  error: string | null;
}

export interface BuildRowsInput {
  recipes: Recipe[];
  calc: Map<number, CalcBatchEntry>;
  prices: Map<number, PriceSnapshot>;
  stats: Map<number, LiquidityStats>;
}

const NOT_CALCULATED = 'Calculation not performed';
const NO_PATH = 'Server found no priceable path';

export function buildRows({ recipes, calc, prices, stats }: BuildRowsInput): RecipeRow[] {
  const seen = new Set<number>();
  const rows: RecipeRow[] = [];

  for (const source of recipes) {
    const itemId = source.output_item_id;
    // output_item_id === 0 means the wiki name was never matched to a GE item,
    // so nothing about it can be priced.
    if (!itemId || seen.has(itemId)) continue;
    seen.add(itemId);

    const price = prices.get(itemId);
    const stat = stats.get(itemId);
    const entry = calc.get(itemId);
    const best = entry?.result?.paths?.[0] ?? null;

    let error: string | null = null;
    if (!entry) error = NOT_CALCULATED;
    else if (entry.error) error = entry.error;
    else if (!best) error = NO_PATH;

    const caveats: RowCaveat[] = [];
    if (best) {
      if (!best.complete) caveats.push('incomplete');
      if (best.aph_source === 'default') caveats.push('default-aph');
      if (!best.throughput) caveats.push('no-throughput');
    }

    rows.push({
      id: itemId,
      itemId,
      itemName: source.output_item_name,
      skill: canonicalSkill(source.skill) ?? source.skill,
      levelReq: source.level_req,
      meetsRequirements: best?.meets_requirements ?? null,
      price: price?.price ?? null,
      componentsCost: best?.buy_cost ?? null,
      margin: best?.profit_per_craft ?? null,
      roiPct: best?.roi_pct ?? null,
      xpPerHour: best?.xp_per_hour ?? null,
      gpPerHour: best?.gp_per_hour ?? null,
      gpPerXp: best?.gp_per_xp ?? null,
      throughputGpPerHour: best?.throughput?.gp_per_hour ?? null,
      bindingItemName: best?.throughput?.binding_item_name ?? null,
      liquidityTier: stat?.tier ?? null,
      liquidityScore: stat?.score ?? null,
      observations: stat?.observations ?? null,
      volumeAvg: stat?.volume_avg ?? null,
      buyLimit4h: stat?.buy_limit_4h ?? null,
      caveats,
      unpricedInputs: best?.unpriced_inputs ?? [],
      pathNames: best?.path ?? [],
      error,
    });
  }

  return rows;
}

export function firstAssumptions(entries: Iterable<CalcBatchEntry>): CalcAssumptions | null {
  for (const entry of entries) {
    if (entry.result) return entry.result.assumptions;
  }
  return null;
}
