import { describe, expect, it } from 'vitest';
import { batchEntry, liquidity, path, recipe, result, snapshot } from '../../test/fixtures';
import { buildRows, firstAssumptions } from './buildRows';

function input(over: Partial<Parameters<typeof buildRows>[0]> = {}) {
  return {
    recipes: [recipe()],
    calc: new Map([[1603, batchEntry()]]),
    prices: new Map([[1603, snapshot()]]),
    stats: new Map([[1603, liquidity()]]),
    ...over,
  };
}

describe('buildRows', () => {
  it('assembles one row from the four responses', () => {
    const [row] = buildRows(input());

    expect(row.id).toBe(1603);
    expect(row.itemName).toBe('Ruby');
    expect(row.skill).toBe('Crafting');
    expect(row.levelReq).toBe(34);
    expect(row.price).toBe(2_707);
    expect(row.componentsCost).toBe(2_400);
    expect(row.margin).toBe(301);
    expect(row.roiPct).toBe(12.5);
    expect(row.xpPerHour).toBe(85_000);
    expect(row.gpPerHour).toBe(301_000);
    expect(row.gpPerXp).toBe(3.54);
    expect(row.throughputGpPerHour).toBe(752_500);
    expect(row.bindingItemName).toBe('Uncut ruby');
    expect(row.liquidityTier).toBe('high');
    expect(row.liquidityScore).toBe(77);
    expect(row.observations).toBe(48);
    expect(row.buyLimit4h).toBe(10_000);
    expect(row.meetsRequirements).toBe(true);
    expect(row.caveats).toEqual([]);
    expect(row.error).toBeNull();
  });

  it('repairs the skill casing the recipe data ships with', () => {
    const [row] = buildRows(input({ recipes: [recipe({ skill: 'crafting' })] }));
    expect(row.skill).toBe('Crafting');
  });

  it('drops recipes whose output item was never resolved', () => {
    const rows = buildRows(input({ recipes: [recipe({ output_item_id: 0 })] }));
    expect(rows).toEqual([]);
  });

  it('keeps one row per output item when several recipes produce it', () => {
    const rows = buildRows(
      input({ recipes: [recipe({ id: 1, name: 'Ruby' }), recipe({ id: 2, name: 'Ruby (alt)' })] }),
    );
    expect(rows).toHaveLength(1);
    expect(rows[0].itemName).toBe('Ruby');
  });

  it('renders a row with no price and no statistics rather than dropping it', () => {
    const [row] = buildRows(input({ prices: new Map(), stats: new Map() }));

    expect(row.price).toBeNull();
    expect(row.liquidityTier).toBeNull();
    expect(row.liquidityScore).toBeNull();
    expect(row.margin).toBe(301);
  });

  it('carries a per-item calculation error instead of showing a zero margin', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, { item_id: 1603, error: 'no priceable production path for item 1603' }],
        ]),
      }),
    );

    expect(rows[0].error).toBe('no priceable production path for item 1603');
    expect(rows[0].margin).toBeNull();
    expect(rows[0].gpPerHour).toBeNull();
    expect(rows[0].roiPct).toBeNull();
  });

  it('states the reason when the calculation returned no paths at all', () => {
    const rows = buildRows(
      input({ calc: new Map([[1603, batchEntry({ result: result({ paths: [] }) })]]) }),
    );

    expect(rows[0].error).toBe('Server found no priceable path');
    expect(rows[0].margin).toBeNull();
  });

  it('states the reason when the item was not part of the batch', () => {
    const rows = buildRows(input({ calc: new Map() }));

    expect(rows[0].error).toBe('Calculation not performed');
    expect(rows[0].margin).toBeNull();
  });

  it('marks an incomplete path and lists the inputs that could not be priced', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({
            result: result({
              paths: [path({ complete: false, unpriced_inputs: ['Uncut ruby', 'Chisel'] })],
            }),
          })],
        ]),
      }),
    );

    expect(rows[0].caveats).toContain('incomplete');
    expect(rows[0].unpricedInputs).toEqual(['Uncut ruby', 'Chisel']);
  });

  it('marks a house-assumed actions-per-hour rate', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({ result: result({ paths: [path({ aph_source: 'default' })] }) })],
        ]),
      }),
    );

    expect(rows[0].caveats).toContain('default-aph');
  });

  it('marks a path with no known buy limit and leaves the throughput empty', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({ result: result({ paths: [path({ throughput: null })] }) })],
        ]),
      }),
    );

    expect(rows[0].caveats).toContain('no-throughput');
    expect(rows[0].throughputGpPerHour).toBeNull();
    expect(rows[0].bindingItemName).toBeNull();
  });

  it('reports a player who cannot perform the path', () => {
    const rows = buildRows(
      input({
        calc: new Map([
          [1603, batchEntry({
            result: result({
              paths: [path({
                meets_requirements: false,
                skill_requirements: [{ skill: 'Crafting', required: 34, actual: 12, met: false }],
              })],
            }),
          })],
        ]),
      }),
    );

    expect(rows[0].meetsRequirements).toBe(false);
  });
});

describe('firstAssumptions', () => {
  it('returns the first assumptions block present in the batch', () => {
    const assumptions = firstAssumptions([
      { item_id: 1, error: 'nope' },
      batchEntry(),
    ]);

    expect(assumptions?.spread_pct).toBe(2);
    expect(assumptions?.price_basis).toBe('ge_guide_price');
  });

  it('returns null when every entry failed', () => {
    expect(firstAssumptions([{ item_id: 1, error: 'nope' }])).toBeNull();
  });
});
