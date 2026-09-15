import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { http, HttpResponse } from 'msw';
import { beforeEach, expect, it, vi } from 'vitest';
import { server } from '../../test/msw/server';
import { renderWithProviders } from '../../test/renderWithProviders';
import { CharacterColumn } from './CharacterPage';

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

  renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));

  expect(await screen.findByText('3,232')).toBeInTheDocument();
  expect(screen.getByRole('img', { name: 'Crafting' })).toBeInTheDocument();
  expect(screen.getByText('Crafting')).toBeInTheDocument();
  expect(screen.queryByText('Overall')).not.toBeInTheDocument();
});

it('puts the total level next to the player name', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));

  const heading = await screen.findByTestId('player-summary');
  expect(heading).toHaveTextContent('Zezima');
  expect(heading).toHaveTextContent('3,232');
});

it('reports the fetch time quietly rather than prominently', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));

  const stamp = await screen.findByTestId('fetched-at');
  expect(stamp).toBeInTheDocument();
  expect(stamp).toHaveClass(/MuiTypography-caption/);
});

it('reports a missing player as the API describes it, not as a crash', async () => {
  server.use(
    http.get('http://localhost:8080/hiscore/Nobody', () =>
      HttpResponse.json({ error: 'player not found' }, { status: 404 }),
    ),
  );

  renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Nobody');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));

  // describeFailure passes a server-authored message through untouched, so
  // the column says what the API said rather than a message of its own.
  expect(await screen.findByText('player not found')).toBeInTheDocument();
});

it('remembers the last name across mounts', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));

  const first = renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));
  await screen.findByText('3,232');
  first.unmount();

  renderWithProviders(<CharacterColumn onSelectSkill={() => {}} />);
  expect(screen.getByLabelText('Player name')).toHaveValue('Zezima');
});

it('reports a selected skill to the shell instead of navigating', async () => {
  server.use(http.get('http://localhost:8080/hiscore/Zezima', () => HttpResponse.json(PLAYER)));
  const onSelectSkill = vi.fn();

  renderWithProviders(<CharacterColumn onSelectSkill={onSelectSkill} />);
  await userEvent.type(screen.getByLabelText('Player name'), 'Zezima');
  await userEvent.click(screen.getByRole('button', { name: 'Show' }));
  await userEvent.click(await screen.findByRole('button', { name: /Crafting/ }));

  expect(onSelectSkill).toHaveBeenCalledWith('Crafting');
});
