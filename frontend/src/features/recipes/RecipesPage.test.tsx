import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { useState } from 'react';
import { beforeEach, expect, it, vi } from 'vitest';
import type { WebSocketLike } from '../../ws/connection';
import { batchEntry, liquidity, path, recipe, result, snapshot } from '../../test/fixtures';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { RecipesPage } from './RecipesPage';

class FakeSocket implements WebSocketLike {
  static last: FakeSocket | null = null;

  sent: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: ((event: { code: number }) => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(readonly url: string) {
    FakeSocket.last = this;
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    /* nothing to do in the fake */
  }
}

function signedIn() {
  localStorage.setItem('rs3.auth', JSON.stringify({ token: 'jwt-abc', username: 'okadishe' }));
}

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

beforeEach(() => {
  localStorage.clear();
  FakeSocket.last = null;
});

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
  // Price, components, margin, ROI, xp/h, gp/xp, gp/h and gp/h capped all
  // state the absence the same way instead of inventing a number.
  expect(within(row).getAllByText('\u2014')).toHaveLength(8);
});

it('dims every money column alike when the path is incomplete', async () => {
  backend({
    calcEntry: batchEntry({ result: result({ paths: [path({ complete: false })] }) }),
  });
  await chooseCrafting();

  const row = await screen.findByRole('row', { name: /Ruby/ });
  for (const value of ['301', '12.5%', '301K', '752.5K']) {
    expect(within(row).getByText(value)).toHaveStyle({ opacity: '0.55' });
  }
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

it('asks a signed-out visitor to sign in for live prices', async () => {
  backend();
  renderWithProviders(<RecipesPage skill="Crafting" onSkillChange={() => {}} />);

  await screen.findByRole('row', { name: /Ruby/ });
  expect(screen.getByText(/Live prices require a signed-in token/)).toBeInTheDocument();
  expect(screen.queryByText(/Connection lost/)).not.toBeInTheDocument();
});

it('stays quiet while the socket is still connecting and once it is open', async () => {
  backend();
  signedIn();
  renderWithProviders(<RecipesPage skill="Crafting" onSkillChange={() => {}} />, {
    socketFactory: (url) => new FakeSocket(url),
  });

  await screen.findByRole('row', { name: /Ruby/ });
  expect(screen.queryByText(/Live prices require a signed-in token/)).not.toBeInTheDocument();
  expect(screen.queryByText(/Connection lost/)).not.toBeInTheDocument();

  act(() => {
    FakeSocket.last?.onopen?.();
  });

  await waitFor(() => expect(screen.queryByText(/Connection lost/)).not.toBeInTheDocument());
  expect(screen.queryByText(/Live prices require a signed-in token/)).not.toBeInTheDocument();
});

it('tells a signed-in visitor that the connection dropped, not to sign in', async () => {
  backend();
  signedIn();
  renderWithProviders(<RecipesPage skill="Crafting" onSkillChange={() => {}} />, {
    socketFactory: (url) => new FakeSocket(url),
  });

  await screen.findByRole('row', { name: /Ruby/ });

  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onclose?.({ code: 1008 });
  });

  expect(await screen.findByText(/Connection lost/)).toBeInTheDocument();
  expect(screen.queryByText(/Live prices require a signed-in token/)).not.toBeInTheDocument();
});

it('marks the row stale when a live price tick arrives for an item on the page', async () => {
  backend();
  signedIn();
  renderWithProviders(<RecipesPage skill="Crafting" onSkillChange={() => {}} />, {
    socketFactory: (url) => new FakeSocket(url),
  });

  await screen.findByRole('row', { name: /Ruby/ });

  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({ type: 'price', payload: snapshot({ price: 3_100 }) }),
    });
  });

  expect(await screen.findByText(/Prices changed for 1 item\(s\)/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Recalculate' })).toBeInTheDocument();
});

it('recalculates via a fresh request to the calc endpoint when Recalculate is clicked', async () => {
  let calcRequests = 0;
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
    http.get('http://localhost:8080/calc/batch', () => {
      calcRequests += 1;
      return HttpResponse.json({ count: 1, results: [batchEntry()] });
    }),
  );
  signedIn();
  renderWithProviders(<RecipesPage skill="Crafting" onSkillChange={() => {}} />, {
    socketFactory: (url) => new FakeSocket(url),
  });

  await screen.findByRole('row', { name: /Ruby/ });
  await waitFor(() => expect(calcRequests).toBe(1));

  act(() => {
    FakeSocket.last?.onopen?.();
    FakeSocket.last?.onmessage?.({
      data: JSON.stringify({ type: 'price', payload: snapshot({ price: 3_100 }) }),
    });
  });
  await screen.findByText(/Prices changed for 1 item\(s\)/);

  await userEvent.click(screen.getByRole('button', { name: 'Recalculate' }));

  await waitFor(() => expect(calcRequests).toBe(2));
  expect(screen.queryByText(/Prices changed for/)).not.toBeInTheDocument();
});

it('explains a failed backbone load instead of an empty grid', async () => {
  server.use(
    http.get('http://localhost:8080/recipes', () =>
      HttpResponse.json(
        { error: 'bad gateway', target: 'http://recipe-service:8082' },
        { status: 502 },
      ),
    ),
    http.get('http://localhost:8080/recipes/ids', () =>
      HttpResponse.json({ skill: 'Crafting', min_level: 1, max_level: 99, count: 0, item_ids: [] }),
    ),
  );

  renderWithProviders(<RecipesPage skill="Crafting" onSkillChange={() => {}} />);

  expect(await screen.findByText('Service recipe-service is unavailable')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  expect(screen.queryByRole('grid')).not.toBeInTheDocument();
});

it('keeps the rows on screen when only the calculation fails', async () => {
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
      HttpResponse.json({ error: 'bad gateway', target: 'http://calc-service:8083' }, { status: 502 }),
    ),
  );

  renderWithProviders(<RecipesPage skill="Crafting" onSkillChange={() => {}} />);

  expect(await screen.findByText(/Service calc-service is unavailable/)).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument();
  expect(await screen.findByRole('row', { name: /Ruby/ })).toBeInTheDocument();
});
