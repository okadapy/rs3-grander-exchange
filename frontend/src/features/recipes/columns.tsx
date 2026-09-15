import { Chip, Stack, Tooltip, Typography } from '@mui/material';
import type { GridColDef, GridRenderCellParams } from '@mui/x-data-grid';
import { ItemIcon } from '../../shared/ItemIcon';
import { SkillIcon } from '../../shared/SkillIcon';
import { ABSENT, formatCompact, formatInt, formatPct } from '../../shared/format';
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

// The reason for an unpriceable row is stated once, in the item column, so
// every value column here renders an em dash instead and the row never reads
// as a zero margin. Both cell components share the same dimming, so an
// incomplete path can never show a greyed-out margin next to a
// full-contrast ROI derived from the very same path.
function dimming(row: RecipeRow) {
  return { opacity: row.caveats.includes('incomplete') ? 0.55 : 1 };
}

function Absent() {
  return <Typography variant="body2" color="text.secondary">{ABSENT}</Typography>;
}

function Money({
  row,
  value,
  signed = false,
}: {
  row: RecipeRow;
  value: number | null;
  signed?: boolean;
}) {
  if (row.error) return <Absent />;

  const color = !signed
    ? undefined
    : value === null
      ? 'text.secondary'
      : value >= 0
        ? 'success.main'
        : 'error.main';

  return (
    <Typography variant="body2" color={color} sx={dimming(row)}>
      {formatCompact(value)}
    </Typography>
  );
}

function Pct({ row, value }: { row: RecipeRow; value: number | null }) {
  if (row.error) return <Absent />;

  return (
    <Typography variant="body2" sx={dimming(row)}>
      {formatPct(value)}
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
        <span>
          <Money row={p.row} value={p.row.margin} signed />
        </span>
      </Tooltip>
    ),
  },
  {
    field: 'roiPct',
    headerName: 'ROI',
    width: 100,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => (
      <Pct row={p.row} value={p.row.roiPct} />
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
        <span>
          <Money row={p.row} value={p.row.throughputGpPerHour} />
        </span>
      </Tooltip>
    ),
  },
  {
    field: 'liquidityScore',
    headerName: 'Liquidity',
    width: 180,
    renderCell: (p: GridRenderCellParams<RecipeRow, number>) => {
      // A tier without a score used to render "null / 100"; both halves of
      // the chip have to be there for it to say anything.
      if (p.row.liquidityTier === null || p.row.liquidityScore === null) {
        return <Typography variant="caption" color="text.secondary">no data</Typography>;
      }
      const color =
        p.row.liquidityTier === 'high' ? 'success'
          : p.row.liquidityTier === 'medium' ? 'warning'
          : 'default';
      return (
        <Tooltip title={`Observations: ${formatInt(p.row.observations)}, average volume ${formatCompact(p.row.volumeAvg)}`}>
          <Chip
            size="small"
            color={color}
            variant="outlined"
            label={`${formatInt(p.row.liquidityScore)} / 100`}
          />
        </Tooltip>
      );
    },
  },
];
