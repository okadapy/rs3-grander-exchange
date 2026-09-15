import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { renderWithProviders } from '../../test/renderWithProviders';
import { RecipeFilters } from './RecipeFilters';
import type { FiltersValue } from './RecipeFilters';

const VALUE: FiltersValue = {
  skill: '',
  minLevel: 1,
  maxLevel: 120,
  includeIncomplete: false,
  spreadPct: undefined,
};

it('treats a cleared max level as the upper bound, not as zero', async () => {
  const onChange = vi.fn();
  renderWithProviders(<RecipeFilters value={VALUE} onChange={onChange} />);

  await userEvent.clear(screen.getByLabelText('Max level'));

  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ maxLevel: 150 }));
});

it('treats a cleared min level as the lower bound', async () => {
  const onChange = vi.fn();
  renderWithProviders(<RecipeFilters value={{ ...VALUE, minLevel: 40 }} onChange={onChange} />);

  await userEvent.clear(screen.getByLabelText('Min level'));

  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ minLevel: 1 }));
});

it('clamps a level typed above the table ceiling', async () => {
  const onChange = vi.fn();
  renderWithProviders(<RecipeFilters value={VALUE} onChange={onChange} />);

  await userEvent.type(screen.getByLabelText('Max level'), '9');

  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ maxLevel: 150 }));
});
