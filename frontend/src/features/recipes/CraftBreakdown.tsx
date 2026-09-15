import {
  Box,
  Divider,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import type { components } from '../../api/schema';
import { ABSENT, formatCompact, formatInt, formatPct } from '../../shared/format';

type CalcPath = components['schemas']['CalcPath'];

interface Props {
  itemName: string;
  path: CalcPath;
}

function Signed({ value }: { value: number }) {
  return (
    <Typography
      variant="caption"
      color={value >= 0 ? 'success.main' : 'error.main'}
      sx={{ fontVariantNumeric: 'tabular-nums' }}
    >
      {formatCompact(value)}
    </Typography>
  );
}

function Num({ value }: { value: number | null | undefined }) {
  return (
    <Typography variant="caption" sx={{ fontVariantNumeric: 'tabular-nums' }}>
      {formatCompact(value)}
    </Typography>
  );
}

/**
 * The steps of one production path, each with the server's own figures for
 * that stage: what it buys, what its output is worth, and the margin between
 * them. That is what says which stage of a chain carries the money and which
 * one destroys it.
 *
 * The stages deliberately do not add up to the totals row, and are not
 * presented as if they did. An intermediate is consumed by the next stage
 * rather than sold, so its revenue never reaches the path — a rune bar chain
 * whose stages read +856 in total nets −3463 once only the finished item is
 * sold. A running total over this column would contradict the Margin figure
 * fed by the very same response.
 */
export function CraftBreakdown({ itemName, path }: Props) {
  const { steps } = path;

  return (
    <Box sx={{ p: 1.5, minWidth: 520 }}>
      <Typography variant="subtitle2" gutterBottom>
        {itemName} — best path
      </Typography>

      {steps.length === 0 ? (
        <Typography variant="caption" color="text.secondary">
          The server returned no step-by-step breakdown for this path.
        </Typography>
      ) : (
        <Table size="small" padding="none" sx={{ '& td, & th': { px: 1, py: 0.25 } }}>
          <TableHead>
            <TableRow>
              <TableCell>#</TableCell>
              <TableCell>Step</TableCell>
              <TableCell align="right">Runs</TableCell>
              <TableCell>Skill</TableCell>
              <TableCell align="right">XP</TableCell>
              <TableCell align="right">Buy</TableCell>
              <TableCell align="right">Sell</TableCell>
              <TableCell align="right">Profit</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {steps.map((entry, index) => (
              <TableRow key={`${entry.recipe}-${index}`}>
                <TableCell>
                  <Typography variant="caption" color="text.secondary">{index + 1}</Typography>
                </TableCell>
                <TableCell>
                  <Typography variant="caption">{entry.recipe}</Typography>
                </TableCell>
                <TableCell align="right"><Num value={entry.runs} /></TableCell>
                <TableCell>
                  <Typography variant="caption" color="text.secondary">
                    {entry.skill} {formatInt(entry.level_req)}
                  </Typography>
                </TableCell>
                <TableCell align="right"><Num value={entry.xp} /></TableCell>
                <TableCell align="right"><Num value={entry.buy_cost} /></TableCell>
                <TableCell align="right"><Num value={entry.sell_revenue} /></TableCell>
                <TableCell align="right"><Signed value={entry.profit} /></TableCell>
              </TableRow>
            ))}

            <TableRow>
              <TableCell colSpan={4}>
                <Typography variant="caption" sx={{ fontWeight: 600 }}>Total</Typography>
              </TableCell>
              <TableCell align="right"><Num value={path.total_xp} /></TableCell>
              <TableCell align="right"><Num value={path.buy_cost} /></TableCell>
              <TableCell align="right"><Num value={path.sell_revenue} /></TableCell>
              <TableCell align="right"><Signed value={path.profit_per_craft} /></TableCell>
            </TableRow>
          </TableBody>
        </Table>
      )}

      {steps.length > 0 && (
        <Typography variant="caption" color="text.secondary" component="p" sx={{ mt: 0.5 }}>
          Each stage is priced on its own. They do not add up to the total: an intermediate is
          consumed by the next stage, not sold, and the total is the path's own figure after tax.
        </Typography>
      )}

      <Divider sx={{ my: 1 }} />

      <Stack direction="row" spacing={1.5} sx={{ flexWrap: 'wrap' }}>
        <Typography variant="caption" color="text.secondary">
          ROI {formatPct(path.roi_pct)}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          GP/h {formatCompact(path.gp_per_hour)}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          capped{' '}
          {path.throughput ? formatCompact(path.throughput.gp_per_hour) : ABSENT}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {formatCompact(path.total_hours)} h per craft
        </Typography>
        <Typography variant="caption" color="text.secondary">
          tax {formatCompact(path.tax_paid)}
        </Typography>
      </Stack>

      {!path.complete && (
        <Typography variant="caption" color="warning.main" component="p" sx={{ mt: 0.5 }}>
          Unpriced and counted as zero: {(path.unpriced_inputs ?? []).join(', ') || 'unknown input'}.
          Every money figure here is an upper bound.
        </Typography>
      )}
    </Box>
  );
}
