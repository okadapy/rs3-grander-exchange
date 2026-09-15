import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { DataGrid } from '@mui/x-data-grid';
import type { UseQueryResult } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { canonicalSkill } from '../../assets/skills';
import { useCalcBatch } from '../../api/queries/calc';
import { useLatestPrices, useLiquidityStats } from '../../api/queries/prices';
import { usePriceableItemIds, useSkillRecipes } from '../../api/queries/recipes';
import { QueryState } from '../../shared/QueryState';
import { usePlayerPrefs } from '../character/usePlayerPrefs';
import { useWs } from '../../ws/WsProvider';
import { AssumptionsBar } from './AssumptionsBar';
import { RecipeFilters } from './RecipeFilters';
import type { FiltersValue } from './RecipeFilters';
import { buildRows, firstAssumptions } from './buildRows';
import { recipeColumns } from './columns';
import { useLivePrices } from './useLivePrices';

const PAGE_SIZE = 25;

// The grid gives way to the notices above it instead of pushing the
// pagination row out of the viewport, but never below a height that would
// leave the DataGrid with no rows visible at all.
const GRID_MIN_HEIGHT = 320;

interface Props {
  skill: string;
  onSkillChange: (skill: string) => void;
}

export function RecipesPage({ skill, onSkillChange }: Props) {
  const player = usePlayerPrefs();

  // A skill the shell hands down that is not one of the 29 canonical names
  // (or none at all) falls back to "all skills" rather than querying for it.
  const normalizedSkill = canonicalSkill(skill) ?? '';

  const [localFilters, setLocalFilters] = useState<Omit<FiltersValue, 'skill'>>({
    minLevel: 1,
    maxLevel: 120,
    includeIncomplete: false,
    spreadPct: undefined,
  });
  const [page, setPage] = useState(0);

  // A deep page from a previous skill would otherwise survive into a
  // shorter result set once the shell switches skills.
  useEffect(() => setPage(0), [normalizedSkill]);

  const filters: FiltersValue = { ...localFilters, skill: normalizedSkill };
  const handleFilters = (next: FiltersValue) => {
    if (next.skill !== normalizedSkill) onSkillChange(next.skill);
    setLocalFilters({
      minLevel: next.minLevel,
      maxLevel: next.maxLevel,
      includeIncomplete: next.includeIncomplete,
      spreadPct: next.spreadPct,
    });
    setPage(0);
  };

  const recipes = useSkillRecipes(filters.skill, filters.maxLevel);
  const priceable = usePriceableItemIds(filters.skill, filters.minLevel, filters.maxLevel);

  // Unfiltered /recipes is a single ~4 MB body; it is fetched once, cached
  // for an hour, and paged client-side so only 25 rows ever reach /calc.
  const backbone = useMemo(() => {
    if (!recipes.data || !priceable.data) return [];
    const seen = new Set<number>();
    return recipes.data.filter((entry) => {
      const id = entry.output_item_id;
      if (!id || seen.has(id) || !priceable.data.has(id)) return false;
      seen.add(id);
      return true;
    });
  }, [recipes.data, priceable.data]);

  const pageRecipes = useMemo(
    () => backbone.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE),
    [backbone, page],
  );
  const pageIds = useMemo(
    () => pageRecipes.map((entry) => entry.output_item_id),
    [pageRecipes],
  );

  const calc = useCalcBatch(pageIds, {
    player: player.name || undefined,
    mode: player.mode,
    spreadPct: filters.spreadPct,
    includeIncomplete: filters.includeIncomplete,
  });
  const { status } = useWs();
  const prices = useLatestPrices(pageIds, { pollMs: status === 'open' ? undefined : 30_000 });
  const stats = useLiquidityStats(pageIds);
  const live = useLivePrices(pageIds);

  const rows = useMemo(
    () =>
      buildRows({
        recipes: pageRecipes,
        calc: calc.data ?? new Map(),
        prices: prices.data ?? new Map(),
        stats: stats.data,
      }),
    [pageRecipes, calc.data, prices.data, stats.data],
  );

  const assumptions = useMemo(
    () => (calc.data ? firstAssumptions(calc.data.values()) : null),
    [calc.data],
  );

  const lastPage = Math.max(0, Math.ceil(backbone.length / PAGE_SIZE) - 1);

  // The grid's backbone is two queries and QueryState takes one, so surface
  // whichever of them explains why there is nothing to draw. Without this a
  // gateway 502 rendered as the grid's bare "No rows", indistinguishable
  // from a skill that genuinely has nothing craftable.
  const backboneQuery: UseQueryResult<unknown> = recipes.isError
    ? recipes
    : priceable.isError
      ? priceable
      : recipes.isPending
        ? recipes
        : priceable;

  // A failed calculation or price read is not a failed page: the rows stay
  // on screen and the money columns explain themselves, so these are
  // reported next to the grid rather than in place of it.
  const moneyErrors = [
    calc.isError ? calc.error : null,
    prices.isError ? prices.error : null,
  ].filter((entry): entry is Error => entry !== null);

  const retryMoney = () => {
    if (calc.isError) void calc.refetch();
    if (prices.isError) void prices.refetch();
  };

  return (
    <Stack spacing={2} sx={{ flex: 1 }}>
      <Typography variant="h5" component="h2">Recipes</Typography>

      <RecipeFilters value={filters} onChange={handleFilters} />

      {player.name === '' && (
        <Alert severity="info" variant="outlined">
          Character name is not set, so recipe level eligibility is not checked. Enter it in the
          character column on the left.
        </Alert>
      )}

      <AssumptionsBar assumptions={assumptions} />

      {status !== 'open' && (
        <Alert severity="info" variant="outlined">
          Live prices require a signed-in token. Sign in from the chat to receive them instantly;
          otherwise prices refresh every 30 seconds.
        </Alert>
      )}

      {live.staleItemIds.size > 0 && (
        <Alert
          severity="warning"
          variant="outlined"
          action={
            <Button
              size="small"
              onClick={() => { live.clearStale(pageIds); void calc.refetch(); }}
            >
              Recalculate
            </Button>
          }
        >
          Prices changed for {live.staleItemIds.size} item(s). Margins are computed server-side,
          so they must be recalculated rather than re-derived in the browser.
        </Alert>
      )}

      {filters.includeIncomplete && (
        <Alert severity="warning" variant="outlined">
          Paths with unpriced inputs are included. Their money figures are an upper bound, not an
          estimate.
        </Alert>
      )}

      {moneyErrors.length > 0 && (
        <Alert
          severity="warning"
          variant="outlined"
          action={<Button size="small" onClick={retryMoney}>Retry</Button>}
        >
          {moneyErrors.map((entry) => entry.message).join('. ')}. The rows below are still listed,
          but their money columns cannot be filled in.
        </Alert>
      )}

      <QueryState query={backboneQuery}>
        {() => (
          <Box sx={{ flex: 1, minHeight: GRID_MIN_HEIGHT }}>
            <DataGrid
              rows={rows}
              columns={recipeColumns}
              loading={recipes.isFetching || calc.isFetching || stats.isPending}
              hideFooter
              disableRowSelectionOnClick
              density="compact"
            />
          </Box>
        )}
      </QueryState>

      <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        <Button size="small" disabled={page === 0} onClick={() => setPage(page - 1)}>
          Back
        </Button>
        <Typography variant="body2">
          Page {page + 1} of {lastPage + 1}, {backbone.length} items total
        </Typography>
        <Button size="small" disabled={page >= lastPage} onClick={() => setPage(page + 1)}>
          Next
        </Button>
      </Stack>

      <Typography variant="caption" color="text.secondary">
        Sorting applies within the loaded page: the server evaluates profitability one item at a
        time, so the whole skill is never computed at once.
      </Typography>
    </Stack>
  );
}
