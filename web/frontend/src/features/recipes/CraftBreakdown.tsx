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
import { useMemo, useState } from 'react';
import type { components } from '../../api/schema';
import { useRecipeTree } from '../../api/queries/recipes';
import type { RecipeInput } from '../../api/queries/recipes';
import { ABSENT, formatCompact, formatInt, formatPct } from '../../shared/format';
import { mostProfitable } from './buildRows';

type CalcPath = components['schemas']['CalcPath'];

interface Props {
  itemName: string;
  itemId: number;
  paths: CalcPath[];
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

function describe(inputs: RecipeInput[]): string {
  return inputs.map((entry) => `${entry.item_name} ×${entry.quantity}`).join(', ');
}

/**
 * The steps of one production path, each with the server's own figures for
 * that stage: what it buys, what its output is worth, and the margin between
 * them.
 *
 * The stages deliberately do not add up to the totals row. An intermediate is
 * consumed by the next stage rather than sold, so its revenue never reaches
 * the path — a rune bar chain whose stages read +856 in total nets −3463 once
 * only the finished item is sold. A running total over this column would
 * contradict the Margin figure fed by the very same response.
 */
export function CraftBreakdown({ itemName, itemId, paths }: Props) {
  // The server orders paths by GP/h, which is not the order of money made.
  // Opening on its first path would show a worse number than the row's own
  // Margin column, which is built on the most profitable one.
  const initial = useMemo(() => {
    const best = mostProfitable(paths);
    return best ? paths.indexOf(best) : 0;
  }, [paths]);
  const [chosen, setChosen] = useState(initial);

  const path = paths[chosen] ?? paths[initial];
  const tree = useRecipeTree(itemId);

  // A step names the recipe it performs, never what that recipe consumes. An
  // input counts as bought unless another stage of this same path produces
  // it, which is exactly what separates "buy the bars" from "smelt them from
  // ore". Matching is by name because a step gives no other handle.
  const crafted = useMemo(
    () => new Set((path?.steps ?? []).map((entry) => entry.recipe)),
    [path],
  );

  if (!path) {
    return (
      <Box sx={{ p: 1.5 }}>
        <Typography variant="caption" color="text.secondary">
          The server found no priceable way to produce {itemName}.
        </Typography>
      </Box>
    );
  }

  const { steps } = path;

  return (
    <Box sx={{ p: 1.5, minWidth: 560, maxWidth: 820 }}>
      <Typography variant="subtitle2" gutterBottom>{itemName}</Typography>

      {paths.length > 1 && (
        <Box sx={{ mb: 1 }}>
          <Typography variant="caption" color="text.secondary" component="p">
            {paths.length} ways to produce it — buying an input or crafting it changes the answer:
          </Typography>
          <Stack
            component="ul"
            sx={{ listStyle: 'none', m: 0, p: 0, maxHeight: 132, overflowY: 'auto' }}
          >
            {paths.map((option, index) => (
              <Box component="li" key={`${option.path.join('>')}-${index}`} sx={{ display: 'flex' }}>
                <Box
                  component="button"
                  type="button"
                  aria-pressed={index === chosen}
                  onClick={() => setChosen(index)}
                  sx={{
                    flex: 1,
                    display: 'flex',
                    gap: 1,
                    alignItems: 'baseline',
                    textAlign: 'left',
                    cursor: 'pointer',
                    border: 0,
                    borderRadius: 0.5,
                    px: 0.5,
                    py: 0.25,
                    font: 'inherit',
                    color: 'inherit',
                    bgcolor: index === chosen ? 'action.selected' : 'transparent',
                    '&:hover': { bgcolor: 'action.hover' },
                  }}
                >
                  <Typography variant="caption" sx={{ flex: 1 }}>
                    {option.path.join(' → ')}
                  </Typography>
                  <Signed value={option.profit_per_craft} />
                  <Typography variant="caption" color="text.secondary">
                    {formatCompact(option.gp_per_hour)} GP/h
                  </Typography>
                </Box>
              </Box>
            ))}
          </Stack>
        </Box>
      )}

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
            {steps.map((entry, index) => {
              const inputs = tree.data?.get(entry.recipe) ?? [];
              const bought = inputs.filter((item) => !crafted.has(item.item_name));
              const made = inputs.filter((item) => crafted.has(item.item_name));

              return (
                <TableRow key={`${entry.recipe}-${index}`}>
                  <TableCell>
                    <Typography variant="caption" color="text.secondary">{index + 1}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="caption" component="p">{entry.recipe}</Typography>
                    {bought.length > 0 && (
                      <Typography variant="caption" color="text.secondary" component="p">
                        buys {describe(bought)}
                      </Typography>
                    )}
                    {made.length > 0 && (
                      <Typography variant="caption" color="text.secondary" component="p">
                        uses {describe(made)} from an earlier step
                      </Typography>
                    )}
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
              );
            })}

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
          capped {path.throughput ? formatCompact(path.throughput.gp_per_hour) : ABSENT}
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
