import { Box } from '@mui/material';
import { useState } from 'react';
import { itemIconUrl } from '../assets/skills';

interface Props {
  itemId: number;
  name: string;
  size?: number;
}

export function ItemIcon({ itemId, name, size = 28 }: Props) {
  const [failed, setFailed] = useState(false);

  if (failed) {
    return (
      <Box
        data-testid="item-icon-fallback"
        title={name}
        sx={{
          width: size,
          height: size,
          borderRadius: 1,
          border: 1,
          borderColor: 'divider',
          bgcolor: 'background.default',
        }}
      />
    );
  }

  return (
    <Box
      component="img"
      src={itemIconUrl(itemId)}
      alt={name}
      loading="lazy"
      onError={() => setFailed(true)}
      sx={{ width: size, height: size, objectFit: 'contain' }}
    />
  );
}
