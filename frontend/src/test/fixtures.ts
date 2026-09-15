import type { components } from '../api/schema';

type Recipe = components['schemas']['Recipe'];
type CalcPath = components['schemas']['CalcPath'];
type CalcStep = components['schemas']['CalcStep'];
type CalcResult = components['schemas']['CalcResult'];
type CalcBatchEntry = components['schemas']['CalcBatchEntry'];
type PriceSnapshot = components['schemas']['PriceSnapshot'];
type LiquidityStats = components['schemas']['LiquidityStats'];

export function recipe(over: Partial<Recipe> = {}): Recipe {
  return {
    id: 9937,
    name: 'Ruby',
    output_item_id: 1603,
    output_item_name: 'Ruby',
    output_qty: 1,
    skill: 'Crafting',
    level_req: 34,
    xp_per_action: 85,
    actions_per_hour: 1000,
    aph_source: 'wiki',
    ticks: 3,
    members: false,
    source: 'runescape.wiki',
    inputs: [],
    ...over,
  };
}

export function step(over: Partial<CalcStep> = {}): CalcStep {
  return {
    recipe: 'Ruby',
    skill: 'Crafting',
    level_req: 34,
    xp: 85,
    aph: 1000,
    aph_source: 'wiki',
    runs: 1,
    buy_cost: 2_400,
    sell_revenue: 2_707,
    profit: 301,
    ...over,
  };
}

export function path(over: Partial<CalcPath> = {}): CalcPath {
  return {
    path: ['Uncut ruby', 'Ruby'],
    steps: [step()],
    profit_per_craft: 301,
    gp_per_hour: 301_000,
    xp_per_hour: 85_000,
    gp_per_xp: 3.54,
    roi_pct: 12.5,
    total_hours: 0.001,
    total_xp: 85,
    buy_cost: 2_400,
    sell_revenue: 2_707,
    tax_paid: 6,
    actions_per_hour: 1000,
    aph_source: 'wiki',
    complete: true,
    throughput: {
      binding_item_id: 1619,
      binding_item_name: 'Uncut ruby',
      binding_limit_4h: 10_000,
      max_crafts_per_4h: 10_000,
      crafts_per_hour: 2_500,
      gp_per_hour: 752_500,
    },
    meets_requirements: true,
    skill_requirements: [{ skill: 'Crafting', required: 34, actual: 99, met: true }],
    ...over,
  };
}

export function result(over: Partial<CalcResult> = {}): CalcResult {
  return {
    item_id: 1603,
    generated_at: '2026-09-15T10:00:00Z',
    cached: false,
    assumptions: {
      price_basis: 'ge_guide_price',
      spread_pct: 2,
      tax_pct: 1,
      tax_cap_per_item: 5_000_000,
      tax_exempt_below: 100,
      inventory_slots: 28,
      bank_trip_ticks: 10,
    },
    paths: [path()],
    ...over,
  };
}

export function batchEntry(over: Partial<CalcBatchEntry> = {}): CalcBatchEntry {
  return { item_id: 1603, result: result(), ...over };
}

export function snapshot(over: Partial<PriceSnapshot> = {}): PriceSnapshot {
  return {
    id: 5207,
    item_id: 1603,
    ts: '2026-09-15T09:53:33.001Z',
    price: 2_707,
    volume: 171_851,
    ...over,
  };
}

export function liquidity(over: Partial<LiquidityStats> = {}): LiquidityStats {
  return {
    item_id: 1603,
    observations: 48,
    distinct_prices: 12,
    price_min: 2_600,
    price_max: 2_800,
    price_avg: 2_700,
    price_last: 2_707,
    volatility_pct: 7.4,
    volume_avg: 165_000,
    volume_last: 171_851,
    last_change_at: '2026-09-15T09:00:00Z',
    stale_for_hours: 0.9,
    score: 77,
    tier: 'high',
    buy_limit_4h: 10_000,
    ...over,
  };
}
