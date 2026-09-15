import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { path, step } from '../../test/fixtures';
import { CraftBreakdown } from './CraftBreakdown';

// Two steps with opposite signs: the first buys inputs and sells nothing,
// the second sells the finished item. A running total is the only way to
// see that the path is under water until the last step pays for it.
const chained = path({
  path: ['Gold bar', 'Gold amulet'],
  steps: [
    step({
      recipe: 'Gold bar',
      skill: 'Smithing',
      level_req: 40,
      xp: 22.5,
      runs: 2,
      buy_cost: 900,
      sell_revenue: 0,
      profit: -900,
    }),
    step({
      recipe: 'Gold amulet',
      skill: 'Crafting',
      level_req: 8,
      xp: 30,
      runs: 1,
      buy_cost: 0,
      sell_revenue: 1_800,
      profit: 1_800,
    }),
  ],
  buy_cost: 900,
  sell_revenue: 1_800,
  profit_per_craft: 882,
  tax_paid: 18,
  total_xp: 75,
  roi_pct: 98,
});

function cells(row: HTMLElement) {
  return within(row).getAllByRole('cell').map((cell) => cell.textContent);
}

describe('CraftBreakdown', () => {
  it('renders one row per step, in execution order', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={chained} />);

    const rows = screen.getAllByRole('row');
    // Header, two steps, totals.
    expect(rows).toHaveLength(4);
    expect(cells(rows[1])[1]).toBe('Gold bar');
    expect(cells(rows[2])[1]).toBe('Gold amulet');
  });

  it('accumulates profit across the steps', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={chained} />);

    const rows = screen.getAllByRole('row');
    // Per-step profit, then the running total: −900 alone, then +900 once
    // the second step sells.
    expect(cells(rows[1]).slice(-2)).toEqual(['-900', '-900']);
    expect(cells(rows[2]).slice(-2)).toEqual(['1.8K', '900']);
  });

  it('takes the totals from the server rather than summing the steps', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={chained} />);

    // The steps add up to 900; the path nets 882 because 18 GP of tax is
    // charged outside any single step. Showing the sum here would quietly
    // contradict the Margin column.
    // Cells are [Total, XP, Buy, Sell, Profit, note] — the label spans the
    // first four columns.
    const totals = cells(screen.getAllByRole('row')[3]);
    expect(totals[4]).toBe('882');
  });

  it('reports the headline rates under the table', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={chained} />);

    expect(screen.getByText(/ROI 98.0%/)).toBeInTheDocument();
    expect(screen.getByText(/tax 18/)).toBeInTheDocument();
  });

  it('names the unpriced inputs when the path is incomplete', () => {
    render(
      <CraftBreakdown
        itemName="Gold amulet"
        path={path({ complete: false, unpriced_inputs: ['Gold ore'] })}
      />,
    );

    expect(screen.getByText(/Gold ore/)).toBeInTheDocument();
  });

  it('says so rather than drawing an empty table when there are no steps', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={path({ steps: [] })} />);

    expect(screen.queryByRole('table')).not.toBeInTheDocument();
    expect(screen.getByText(/no step-by-step breakdown/i)).toBeInTheDocument();
  });
});
