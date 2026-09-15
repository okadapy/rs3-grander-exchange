import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it } from 'vitest';
import { useLocation } from 'react-router-dom';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { CharacterPage } from './CharacterPage';

// Reflects the router's current location as text so a click that navigates
// can be asserted on without reaching into react-router internals.
function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname + location.search}</div>;
}

const PLAYER = {
  id: 2,
  name: 'Zezima',
  mode: 'normal',
  fetched_at: '2026-09-15T10:24:24.873Z',
  skills: [
    { id: 421, player_id: 2, skill: 'Overall', level: 3232, xp: 5709998811, rank: 6520 },
    { id: 422, player_id: 2, skill: 'Attack', level: 120, xp: 200000000, rank: 351 },
    { id: 435, player_id: 2, skill: 'Crafting', level: 99, xp: 13034431, rank: 1200 },
  ],
};

beforeEach(() => localStorage.clear());

it('shows the overall summary and one tile per skill', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));

  // formatInt (Task 2) groups thousands with a narrow no-break space, so the
  // rendered text is "3 232", not the literal digit string "3232".
  expect(
    await screen.findByText((content) => content.replace(/\s/g, '') === '3232'),
  ).toBeInTheDocument();
  expect(screen.getByRole('img', { name: 'Crafting' })).toBeInTheDocument();
  expect(screen.getByText('Crafting')).toBeInTheDocument();
  expect(screen.queryByText('Overall')).not.toBeInTheDocument();
});

it('always shows when the hiscore copy was fetched', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));

  expect(await screen.findByText(/Данные получены/)).toBeInTheDocument();
});

it('reports a missing player as the API describes it, not as a crash', async () => {
  server.use(
    http.get('http://localhost:8080/hiscore/Nobody', () =>
      HttpResponse.json({ error: 'player not found' }, { status: 404 }),
    ),
  );

  renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Nobody');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));

  expect(
    await screen.findByText('Игрок не найден или хайскоры недоступны'),
  ).toBeInTheDocument();
});

it('remembers the last name across mounts', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  const first = renderWithProviders(<CharacterPage />);
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));
  await screen.findByText((content) => content.replace(/\s/g, '') === '3232');
  first.unmount();

  renderWithProviders(<CharacterPage />);
  expect(screen.getByLabelText('Имя персонажа')).toHaveValue('Zezima');
});

it('clicking a skill navigates to the recipes page filtered to that skill', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(
    <>
      <CharacterPage />
      <LocationProbe />
    </>,
  );
  await userEvent.type(screen.getByLabelText('Имя персонажа'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Показать' }));
  await screen.findByText('Crafting');

  await userEvent.click(screen.getByRole('link', { name: /Crafting/ }));

  expect(screen.getByTestId('location')).toHaveTextContent('/recipes?skill=Crafting');
});
