import { Alert, Typography } from '@mui/material';
import type { components } from '../../api/schema';
import { formatCompact, formatPct } from '../../shared/format';

type CalcAssumptions = components['schemas']['CalcAssumptions'];

export function AssumptionsBar({ assumptions }: { assumptions: CalcAssumptions | null }) {
  if (!assumptions) return null;

  return (
    <Alert severity="info" variant="outlined">
      <Typography variant="body2">
        Price is the Grand Exchange guide price. Spread {formatPct(assumptions.spread_pct)},
        tax {formatPct(assumptions.tax_pct)}, tax cap{' '}
        {formatCompact(assumptions.tax_cap_per_item)}, exempt below{' '}
        {formatCompact(assumptions.tax_exempt_below)}. GP/h is meaningless without these assumptions.
      </Typography>
    </Alert>
  );
}
