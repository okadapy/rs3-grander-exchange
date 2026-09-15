import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { path, step } from '../../test/fixtures';
import { CraftBreakdown } from './CraftBreakdown';

// Real /calc/1673 shape, rounded: the gateway prices each stage on its own,
// so the stages read +804 and +378 while the path itself nets -1163. Anything
// this component derives by adding the stages up would contradict the Margin
// column fed by the same response.
const chained = path({
  path: ['Gold bar', 'Gold amulet'],
  steps: [
    step({
      recipe: 'Gold bar',
      skill: 'Smithing',
      level_req: 40,
      xp: 7,
      runs: 1,
      buy_cost: 1_541,
      sell_revenue: 2_392,
      profit: 804,
    }),
    step({
      recipe: 'Gold amulet',
      skill: 'Crafting',
      level_req: 8,
      xp: 30,
      runs: 1,
      buy_cost: 0,
      sell_revenue: 385,
      profit: 378,
    }),
  ],
  buy_cost: 1_541,
  sell_revenue: 385,
  profit_per_craft: -1_163,
  tax_paid: 7,
  total_xp: 37,
  roi_pct: -75.5,
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

  it('shows each stage priced on its own, without deriving a running total', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={chained} />);

    const rows = screen.getAllByRole('row');
    expect(cells(rows[1]).at(-1)).toBe('804');
    expect(cells(rows[2]).at(-1)).toBe('378');
    // 804 + 378 = 1182, a number that is true of nothing: the bar is consumed
    // by the next stage, never sold. It must appear nowhere.
    expect(screen.queryByText('1.2K')).not.toBeInTheDocument();
    expect(screen.getByText(/do not add up to the total/)).toBeInTheDocument();
  });

  it('takes the totals from the server rather than summing the steps', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={chained} />);

    // The stages read +804 and +378; the path nets -1163, because only the
    // finished amulet is ever sold. Cells are [Total, XP, Buy, Sell, Profit]
    // — the label spans the first four columns.
    const totals = cells(screen.getAllByRole('row')[3]);
    expect(totals[4]).toBe('-1.2K');
  });

  it('reports the headline rates under the table', () => {
    render(<CraftBreakdown itemName="Gold amulet" path={chained} />);

    expect(screen.getByText(/ROI -75.5%/)).toBeInTheDocument();
    expect(screen.getByText(/tax 7/)).toBeInTheDocument();
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
