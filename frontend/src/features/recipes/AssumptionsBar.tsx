import { Alert, Typography } from '@mui/material';
import type { components } from '../../api/schema';
import { formatCompact, formatInt, formatPct } from '../../shared/format';

type CalcAssumptions = components['schemas']['CalcAssumptions'];

export function AssumptionsBar({ assumptions }: { assumptions: CalcAssumptions | null }) {
  if (!assumptions) return null;

  return (
    <Alert severity="info" variant="outlined">
      <Typography variant="body2">
        Price is the Grand Exchange guide price. Spread {formatPct(assumptions.spread_pct)},
        tax {formatPct(assumptions.tax_pct)}, tax cap{' '}
        {formatCompact(assumptions.tax_cap_per_item)}, exempt below{' '}
        {formatCompact(assumptions.tax_exempt_below)}. Rates derived from tick costs assume{' '}
        {formatInt(assumptions.inventory_slots)} inventory slots and{' '}
        {formatInt(assumptions.bank_trip_ticks)} ticks per bank trip
        {assumptions.boosts ? `, with boosts ${assumptions.boosts}` : ''}. GP/h is meaningless
        without these assumptions.
      </Typography>
    </Alert>
  );
}
