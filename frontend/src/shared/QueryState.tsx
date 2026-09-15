import { Alert, Box, Button, CircularProgress, Typography } from '@mui/material';
import type { UseQueryResult } from '@tanstack/react-query';
import type { ReactNode } from 'react';

interface Props<T> {
  query: UseQueryResult<T>;
  empty?: (data: T) => ReactNode;
  children: (data: T) => ReactNode;
}

export function QueryState<T>({ query, empty, children }: Props<T>) {
  if (query.isPending) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
        <CircularProgress size={28} />
      </Box>
    );
  }

  if (query.isError) {
    return (
      <Alert
        severity="error"
        sx={{ my: 2 }}
        action={<Button size="small" onClick={() => void query.refetch()}>Повторить</Button>}
      >
        <Typography variant="body2">{query.error.message}</Typography>
      </Alert>
    );
  }

  const emptyNode = empty?.(query.data);
  return <>{emptyNode ?? children(query.data)}</>;
}
