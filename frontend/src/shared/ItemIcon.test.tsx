import { render, screen } from '@testing-library/react';
import { fireEvent } from '@testing-library/dom';
import { expect, it } from 'vitest';
import { ItemIcon } from './ItemIcon';

it('renders the item image with the item name as its alternative text', () => {
  render(<ItemIcon itemId={1603} name="Ruby" />);

  const img = screen.getByAltText('Ruby');
  expect(img).toHaveAttribute('src', 'https://secure.runescape.com/m=itemdb_rs/obj_big.gif?id=1603');
  expect(img).toHaveAttribute('loading', 'lazy');
});

it('falls back to a neutral placeholder when the image fails to load', () => {
  render(<ItemIcon itemId={999999} name="Unknown" />);

  fireEvent.error(screen.getByAltText('Unknown'));

  expect(screen.queryByAltText('Unknown')).not.toBeInTheDocument();
  expect(screen.getByTestId('item-icon-fallback')).toBeInTheDocument();
});
