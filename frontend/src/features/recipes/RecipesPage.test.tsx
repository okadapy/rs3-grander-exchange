import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { useState } from 'react';
import { beforeEach, expect, it, vi } from 'vitest';
import { batchEntry, liquidity, path, recipe, result, snapshot } from '../../test/fixtures';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { RecipesPage } from './RecipesPage';

function backend(over: { calcEntry?: unknown } = {}) {
  server.use(
    http.get('http://localhost:8080/recipes', () =>
      HttpResponse.json({ count: 1, recipes: [recipe()] }),
    ),
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: 'Crafting', min_level: 1, max_level: 99, count: 1, item_ids: [1603] }),
    ),
    http.get('http://localhost:8080/prices/latest', () =>
      HttpResponse.json({ count: 1, prices: [snapshot()] }),
    ),
    http.get('http://localhost:8080/prices/stats/1603', () => HttpResponse.json(liquidity())),
    http.get('http://localhost:8080/calc/batch', () =>
      HttpResponse.json({ count: 1, results: [over.calcEntry ?? batchEntry()] }),
    ),
  );
}

beforeEach(() => localStorage.clear());

// The shell owns the selected skill (Delta B); this harness stands in for it
// so a click on the in-page skill selector has somewhere to report to.
function Harness() {
  const [skill, setSkill] = useState('');
  return <RecipesPage skill={skill} onSkillChange={setSkill} />;
}

async function chooseCrafting() {
  renderWithProviders(<Harness />);
  await userEvent.click(screen.getByLabelText('Skill'));
  await userEvent.click(await screen.findByRole('option', { name: 'Crafting' }));
}

it('shows the profitability columns for the chosen skill', async () => {
  backend();
  await chooseCrafting();

  const row = await screen.findByRole('row', { name: /Ruby/ });
  expect(within(row).getByText('2.7K')).toBeInTheDocument();
  expect(within(row).getByText('2.4K')).toBeInTheDocument();
  expect(within(row).getByText('301')).toBeInTheDocument();
  expect(within(row).getByText('12.5%')).toBeInTheDocument();
  expect(within(row).getByText('85K')).toBeInTheDocument();
  expect(within(row).getByText('301K')).toBeInTheDocument();
  expect(within(row).getByText('752.5K')).toBeInTheDocument();
  expect(within(row).getByText(/77/)).toBeInTheDocument();
});

it('shows the assumptions the money figures rest on', async () => {
  backend();
  await chooseCrafting();

  expect(await screen.findByText(/Spread 2\.0%/)).toBeInTheDocument();
  expect(screen.getByText(/tax 1\.0%/)).toBeInTheDocument();
});

it('marks a path whose actions-per-hour is a house assumption', async () => {
  backend({
    calcEntry: batchEntry({ result: result({ paths: [path({ aph_source: 'default' })] }) }),
  });
  await chooseCrafting();

  const row = await screen.findByRole('row', { name: /Ruby/ });
  expect(within(row).getByTitle(/action rate assumed by the server/i)).toBeInTheDocument();
});

it('states the reason instead of a zero margin when the server could not price the item', async () => {
  backend({
    calcEntry: { item_id: 1603, error: 'no priceable production path for item 1603' },
  });
  await chooseCrafting();

  const row = await screen.findByRole('row', { name: /Ruby/ });
  expect(within(row).getByText(/no priceable production path/)).toBeInTheDocument();
  expect(within(row).queryByText('0')).not.toBeInTheDocument();
});

it('shows every craft when no skill is selected', async () => {
  server.use(
    http.get('http://localhost:8080/recipes', () =>
      HttpResponse.json({
        count: 2,
        recipes: [
          recipe({
            id: 1,
            name: 'Yew longbow',
            output_item_id: 2,
            output_item_name: 'Yew longbow',
            skill: 'Fletching',
            level_req: 70,
          }),
          recipe({
            id: 2,
            name: 'Rune platebody',
            output_item_id: 3,
            output_item_name: 'Rune platebody',
            skill: 'Smithing',
            level_req: 99,
          }),
        ],
      }),
    ),
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: '', min_level: 1, max_level: 120, count: 2, item_ids: [2, 3] }),
    ),
    http.get('http://localhost:8080/prices/latest', () => HttpResponse.json({ count: 0, prices: [] })),
    http.get('http://localhost:8080/prices/stats/:itemID', ({ params }) =>
      HttpResponse.json(liquidity({ item_id: Number(params.itemID) })),
    ),
    http.get('http://localhost:8080/calc/batch', () => HttpResponse.json({ count: 0, results: [] })),
  );

  renderWithProviders(<RecipesPage skill="" onSkillChange={() => {}} />);

  expect(await screen.findByText('Yew longbow')).toBeInTheDocument();
  expect(screen.getByText('Rune platebody')).toBeInTheDocument();
});

it('filters by the skill the shell selects and reports changes upward', async () => {
  const seenSkills: string[] = [];
  server.use(
    http.get('http://localhost:8080/recipes', ({ request }) => {
      seenSkills.push(new URL(request.url).searchParams.get('skill') ?? '');
      return HttpResponse.json({ count: 1, recipes: [recipe()] });
    }),
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: '', min_level: 1, max_level: 120, count: 1, item_ids: [1603] }),
    ),
    http.get('http://localhost:8080/prices/latest', () =>
      HttpResponse.json({ count: 1, prices: [snapshot()] }),
    ),
    http.get('http://localhost:8080/prices/stats/1603', () => HttpResponse.json(liquidity())),
    http.get('http://localhost:8080/calc/batch', () =>
      HttpResponse.json({ count: 1, results: [batchEntry()] }),
    ),
  );

  const onSkillChange = vi.fn();
  const { rerender } = renderWithProviders(<RecipesPage skill="" onSkillChange={onSkillChange} />);

  await screen.findByRole('row', { name: /Ruby/ });
  expect(seenSkills[0]).toBe('');

  rerender(<RecipesPage skill="Crafting" onSkillChange={onSkillChange} />);

  await waitFor(() => expect(seenSkills[seenSkills.length - 1]).toBe('Crafting'));
});

it('says that sorting only covers the loaded page', async () => {
  backend();
  await chooseCrafting();

  expect(
    await screen.findByText(/Sorting applies within the loaded page/),
  ).toBeInTheDocument();
});
