import { Chip, Stack, Tooltip, Typography } from '@mui/material';
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid';
import { ItemIcon } from '../../shared/ItemIcon';
import { SkillIcon } from '../../shared/SkillIcon';
import { formatCompact, formatInt, formatPct } from '../../shared/format';
import type { RecipeRow, RowCaveat } from './buildRows';

const CAVEAT_TITLES: Record<RowCaveat, string> = {
  incomplete: 'Some inputs are unpriced and counted as zero — the margin is an upper bound',
  'default-aph': 'Action rate assumed by the server, not measured',
  'no-throughput': 'Buy limit unknown, throughput cap not computed',
};

const CAVEAT_LABELS: Record<RowCaveat, string> = {
  incomplete: 'incomplete',
  'default-aph': 'default APH',
  'no-throughput': 'uncapped',
};

// The reason for an unpriceable row is stated once, in the item column.
// Money columns render an em dash so the row never reads as a zero margin.
function Money({ row, value }: { row: RecipeRow; value: number | null }) {
  if (row.error) return <Typography variant="body2" color="text.secondary">—</Typography>;

  const dim = row.caveats.includes('incomplete');
  return (
    <Typography variant="body2" sx={{ opacity: dim ? 0.55 : 1 }}>
      {formatCompact(value)}
    </Typography>
  );
}

export const recipeColumns: GridColDef<RecipeRow>[] = [
  {
    field: 'skill',
    headerName: 'Skill',
    width: 110,
    renderCell: (params: GridRenderCellParams<RecipeRow, string>) => (
      <Stack direction="row" spacing={0.5} sx={{ alignItems: 'center' }}>
        <SkillIcon skill={params.row.skill} />
        <Typography variant="body2">{params.row.skill}</Typography>
      </Stack>
    ),
  },
  {
    field: 'itemName',
    headerName: 'Item',
    flex: 1,
    minWidth: 220,
    renderCell: (params: GridRenderCellParams<RecipeRow, string>) => (
      <Stack direction="row" spacing={1} sx={{ alignItems: 'center', minWidth: 0 }}>
        <ItemIcon itemId={params.row.itemId} name={params.row.itemName} />
        <Typography variant="body2" noWrap>{params.row.itemName}</Typography>

        {params.row.error && (
          <Typography variant="caption" color="text.secondary" noWrap title={params.row.error}>
            {params.row.error}
          </Typography>
        )}

        {params.row.caveats.map((caveat) => (
          <Chip
            key={caveat}
            size="small"
            variant="outlined"
            color={caveat === 'incomplete' ? 'warning' : 'default'}
            title={CAVEAT_TITLES[caveat]}
            label={CAVEAT_LABELS[caveat]}
          />
        ))}
      </Stack>
    ),
  },
  {
    field: 'levelReq',
    headerName: 'Level',
    width: 100,
    renderCell: (params: GridRenderCellParams<RecipeRow, number>) => (
      <Typography
        variant="body2"
        color={params.row.meetsRequirements === false ? 'text.disabled' : 'text.primary'}
      >
        {formatInt(params.row.levelReq)}
      </Typography>
    ),
  },
  {
    field: 'price',
    headerName: 'Price',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.price} />,
  },
  {
    field: 'componentsCost',
    headerName: 'Components',
    width: 120,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.componentsCost} />,
  },
  {
    field: 'margin',
    headerName: 'Margin',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => (
      <Tooltip
        title={
          p.row.unpricedInputs.length > 0
            ? `Unpriced: ${p.row.unpricedInputs.join(', ')}`
            : ''
        }
      >
        <Typography
          variant="body2"
          color={p.row.margin === null ? 'text.secondary' : p.row.margin >= 0 ? 'success.main' : 'error.main'}
        >
          {p.row.error ? '—' : formatCompact(p.row.margin)}
        </Typography>
      </Tooltip>
    ),
  },
  {
    field: 'roiPct',
    headerName: 'ROI',
    width: 100,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => (
      <Typography variant="body2">{p.row.error ? '—' : formatPct(p.row.roiPct)}</Typography>
    ),
  },
  {
    field: 'xpPerHour',
    headerName: 'XP/h',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.xpPerHour} />,
  },
  {
    field: 'gpPerXp',
    headerName: 'GP/XP',
    width: 100,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.gpPerXp} />,
  },
  {
    field: 'gpPerHour',
    headerName: 'GP/h',
    width: 110,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => <Money row={p.row} value={p.row.gpPerHour} />,
  },
  {
    field: 'throughputGpPerHour',
    headerName: 'GP/h capped',
    width: 150,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => (
      <Tooltip title={p.row.bindingItemName ? `Binding input: ${p.row.bindingItemName}` : 'Buy limit unknown'}>
        <Typography variant="body2">
          {p.row.error ? '—' : formatCompact(p.row.throughputGpPerHour)}
        </Typography>
      </Tooltip>
    ),
  },
  {
    field: 'liquidityScore',
    headerName: 'Liquidity',
    width: 180,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => {
      if (p.row.liquidityTier === null) {
        return <Typography variant="caption" color="text.secondary">no data</Typography>;
      }
      const color =
        p.row.liquidityTier === 'high' ? 'success'
          : p.row.liquidityTier === 'medium' ? 'warning'
          : 'default';
      return (
        <Tooltip title={`Observations: ${formatInt(p.row.observations)}, average volume ${formatCompact(p.row.volumeAvg)}`}>
          <Chip size="small" color={color} variant="outlined" label={`${p.row.liquidityScore} / 100`} />
        </Tooltip>
      );
    },
  },
];
