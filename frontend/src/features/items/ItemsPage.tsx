import {
  Alert,
  Box,
  Button,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material';
import { DataGrid } from '@mui/x-data-grid';
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid';
import { useEffect, useMemo, useState } from 'react';
import { useCalcItem } from '../../api/queries/calc';
import { MIN_QUERY_LENGTH, useItemSearch } from '../../api/queries/items';
import type { ItemSource, ItemSummary } from '../../api/queries/items';
import { ItemIcon } from '../../shared/ItemIcon';
import { QueryState } from '../../shared/QueryState';
import { useDebounced } from '../../shared/useDebounced';
import { usePlayerPrefs } from '../character/usePlayerPrefs';
import { CraftBreakdown } from '../recipes/CraftBreakdown';

const PAGE_SIZE = 25;
const DEBOUNCE_MS = 300;
const GRID_MIN_HEIGHT = 280;

const SOURCE_LABELS: Record<ItemSource, string> = {
  all: 'Anywhere',
  output: 'Produced by a recipe',
  input: 'Consumed by a recipe',
};

const itemColumns: GridColDef<ItemSummary>[] = [
  {
    field: 'name',
    headerName: 'Item',
    flex: 1,
    minWidth: 240,
    renderCell: (params: GridRenderCellParams<ItemSummary, string>) => (
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', minWidth: 0 }}>
        <ItemIcon itemId={params.row.item_id} name={params.row.name} />
        <Typography variant="body2" noWrap>{params.row.name}</Typography>
      </Stack>
    ),
  },
  {
    field: 'sources',
    headerName: 'Found as',
    width: 180,
    sortable: false,
    renderCell: (params: GridRenderCellParams<ItemSummary, string[]>) => (
      <Typography variant="body2" color="text.secondary">
        {params.row.sources.join(', ')}
      </Typography>
    ),
  },
  { field: 'item_id', headerName: 'Item ID', width: 120 },
];

export function ItemsPage() {
  const player = usePlayerPrefs();
  const [query, setQuery] = useState('');
  const [source, setSource] = useState<ItemSource>('all');
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<ItemSummary | null>(null);

  const settledQuery = useDebounced(query.trim(), DEBOUNCE_MS);

  // A deep page and a selection from the previous query would otherwise
  // survive into a result set that no longer contains either.
  useEffect(() => {
    setPage(0);
    setSelected(null);
  }, [settledQuery, source]);

  const search = useItemSearch(settledQuery, source, PAGE_SIZE, page * PAGE_SIZE);

  const calc = useCalcItem(selected?.item_id ?? null, {
    player: player.name || undefined,
    mode: player.mode,
    includeIncomplete: false,
  });

  const lastPage = useMemo(
    () => Math.max(0, Math.ceil((search.data?.total ?? 0) / PAGE_SIZE) - 1),
    [search.data?.total],
  );

  return (
    <Stack spacing={2} sx={{ flex: 1 }}>
      <Typography variant="h5" component="h2">Items</Typography>

      <Stack direction="row" spacing={2} sx={{ flexWrap: 'wrap' }}>
        <TextField
          label="Item name"
          size="small"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          sx={{ minWidth: 260 }}
        />
        <TextField
          select
          label="Found as"
          size="small"
          value={source}
          onChange={(event) => setSource(event.target.value as ItemSource)}
          sx={{ minWidth: 220 }}
        >
          {(Object.keys(SOURCE_LABELS) as ItemSource[]).map((value) => (
            <MenuItem key={value} value={value}>{SOURCE_LABELS[value]}</MenuItem>
          ))}
        </TextField>
      </Stack>

      {settledQuery.length < MIN_QUERY_LENGTH ? (
        <Alert severity="info" variant="outlined">
          Type at least {MIN_QUERY_LENGTH} characters to search the catalogue of recipe inputs and
          outputs.
        </Alert>
      ) : (
        <QueryState
          query={search}
          empty={(data) =>
            data.items.length === 0 ? (
              <Alert severity="info" variant="outlined">
                Nothing matches “{settledQuery}”. Items appear here only when a recipe produces or
                consumes them.
              </Alert>
            ) : null
          }
        >
          {(data) => (
            <>
              <Box sx={{ flexShrink: 0, minHeight: GRID_MIN_HEIGHT }}>
                <DataGrid
                  rows={data.items}
                  getRowId={(row) => row.item_id}
                  columns={itemColumns}
                  onRowClick={(params) => setSelected(params.row)}
                  hideFooter
                  density="compact"
                />
              </Box>

              <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
                <Button size="small" disabled={page === 0} onClick={() => setPage(page - 1)}>
                  Back
                </Button>
                <Typography variant="body2">
                  Page {page + 1} of {lastPage + 1}, {data.total} items matched
                </Typography>
                <Button size="small" disabled={page >= lastPage} onClick={() => setPage(page + 1)}>
                  Next
                </Button>
              </Stack>
            </>
          )}
        </QueryState>
      )}

      {selected && (
        <Box sx={{ border: 1, borderColor: 'divider', borderRadius: 1 }}>
          <QueryState
            query={calc}
            empty={(data) =>
              data.paths.length === 0 ? (
                <Alert severity="info" variant="outlined" sx={{ m: 1 }}>
                  No priceable way to produce {selected.name} was found.
                </Alert>
              ) : null
            }
          >
            {(data) => (
              <CraftBreakdown
                itemName={selected.name}
                itemId={selected.item_id}
                paths={data.paths}
              />
            )}
          </QueryState>
        </Box>
      )}

      {!selected && (
        <Typography variant="caption" color="text.secondary">
          Pick an item to see how it is crafted and where the profit is made along the way.
        </Typography>
      )}
    </Stack>
  );
}
