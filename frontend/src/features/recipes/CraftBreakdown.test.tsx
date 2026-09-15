import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, describe, expect, it } from 'vitest';
import { path, recipe, step } from '../../test/fixtures';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
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

// Buying the bar instead of smelting it: one stage, and the better margin.
const shallow = path({
  path: ['Gold amulet'],
  steps: [step({ recipe: 'Gold amulet', buy_cost: 2_400, sell_revenue: 385, profit: -2_064 })],
  buy_cost: 2_400,
  sell_revenue: 385,
  profit_per_craft: -2_064,
  gp_per_hour: -3_302_400,
  roi_pct: -86,
});

// /recipes/{itemID} is the only place that names what a step consumes.
function treeHandler() {
  return http.get('http://localhost:8080/recipes/1673', () =>
    HttpResponse.json({
      root: {
        depth: 0,
        recipe: recipe({
          name: 'Gold amulet',
          inputs: [
            {
              id: 1, recipe_id: 1, item_id: 2357, item_name: 'Gold bar',
              quantity: 1, is_intermediate: true,
            },
            {
              id: 2, recipe_id: 1, item_id: 1759, item_name: 'Ball of wool',
              quantity: 1, is_intermediate: false,
            },
          ],
        }),
        children: [
          {
            depth: 1,
            recipe: recipe({
              name: 'Gold bar',
              inputs: [
                {
                  id: 3, recipe_id: 2, item_id: 444, item_name: 'Gold ore',
                  quantity: 1, is_intermediate: false,
                },
              ],
            }),
            children: [],
          },
        ],
      },
    }),
  );
}

function cells(row: HTMLElement) {
  return within(row).getAllByRole('cell').map((cell) => cell.textContent);
}

function render(paths = [chained]) {
  return renderWithProviders(
    <CraftBreakdown itemName="Gold amulet" itemId={1673} paths={paths} />,
  );
}

beforeEach(() => {
  localStorage.clear();
  server.use(treeHandler());
});

describe('CraftBreakdown', () => {
  it('renders one row per step, in execution order', async () => {
    render();

    const rows = await screen.findAllByRole('row');
    // Header, two steps, totals.
    expect(rows).toHaveLength(4);
    expect(cells(rows[1])[1]).toContain('Gold bar');
    expect(cells(rows[2])[1]).toContain('Gold amulet');
  });

  it('shows each stage priced on its own, without deriving a running total', async () => {
    render();

    const rows = await screen.findAllByRole('row');
    expect(cells(rows[1]).at(-1)).toBe('804');
    expect(cells(rows[2]).at(-1)).toBe('378');
    // 804 + 378 = 1182, a number that is true of nothing: the bar is consumed
    // by the next stage, never sold. It must appear nowhere.
    expect(screen.queryByText('1.2K')).not.toBeInTheDocument();
    expect(screen.getByText(/do not add up to the total/)).toBeInTheDocument();
  });

  it('takes the totals from the server rather than summing the steps', async () => {
    render();

    // The stages read +804 and +378; the path nets -1163, because only the
    // finished amulet is ever sold. Cells are [Total, XP, Buy, Sell, Profit]
    // — the label spans the first four columns.
    const rows = await screen.findAllByRole('row');
    expect(cells(rows[3])[4]).toBe('-1.2K');
  });

  it('names what each stage buys and what it takes from an earlier stage', async () => {
    render();

    // Neither name is anywhere in /calc — the step only carries a number.
    expect(await screen.findByText(/buys Gold ore ×1/)).toBeInTheDocument();
    expect(screen.getByText(/buys Ball of wool ×1/)).toBeInTheDocument();
    expect(screen.getByText(/uses Gold bar ×1 from an earlier step/)).toBeInTheDocument();
  });

  it('counts an input as bought when this path does not craft it', async () => {
    // Same item, but the path buys the bar instead of smelting it, so the bar
    // moves from "made earlier" to "bought".
    render([shallow]);

    expect(await screen.findByText(/buys Gold bar ×1, Ball of wool ×1/)).toBeInTheDocument();
    expect(screen.queryByText(/from an earlier step/)).not.toBeInTheDocument();
  });

  it('opens on the most profitable path, not the server order', async () => {
    // The server ranks by GP/h and would put the chained path first; -1163
    // beats -2064, so the breakdown opens on it regardless.
    render([chained, shallow]);

    const rows = await screen.findAllByRole('row');
    expect(cells(rows[3])[4]).toBe('-1.2K');
    expect(screen.getByRole('button', { name: /Gold bar → Gold amulet/ })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('switches to another path when one is picked', async () => {
    render([chained, shallow]);
    await screen.findAllByRole('row');

    await userEvent.click(screen.getByRole('button', { name: /^Gold amulet/ }));

    await waitFor(() => {
      const rows = screen.getAllByRole('row');
      // One stage now, and the totals follow the chosen path.
      expect(rows).toHaveLength(3);
      expect(cells(rows[2])[4]).toBe('-2.1K');
    });
  });

  it('reports the headline rates under the table', async () => {
    render();

    expect(await screen.findByText(/ROI -75.5%/)).toBeInTheDocument();
    expect(screen.getByText(/tax 7/)).toBeInTheDocument();
  });

  it('names the unpriced inputs when the path is incomplete', async () => {
    render([path({ complete: false, unpriced_inputs: ['Gold ore'] })]);

    expect(await screen.findByText(/Every money figure here is an upper bound/)).toBeInTheDocument();
    expect(screen.getByText(/Unpriced and counted as zero: Gold ore/)).toBeInTheDocument();
  });

  it('says so rather than drawing an empty table when there are no steps', async () => {
    render([path({ steps: [] })]);

    expect(await screen.findByText(/no step-by-step breakdown/i)).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('still tabulates the stages when the recipe tree cannot be loaded', async () => {
    server.use(
      http.get('http://localhost:8080/recipes/1673', () => HttpResponse.error()),
    );
    render();

    // The names are a garnish on the numbers, not a precondition for them.
    const rows = await screen.findAllByRole('row');
    expect(cells(rows[1]).at(-1)).toBe('804');
    expect(screen.queryByText(/buys /)).not.toBeInTheDocument();
  });
});
