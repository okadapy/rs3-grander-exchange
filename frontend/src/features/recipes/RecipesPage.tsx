import { Alert, Box, Button, Stack, Typography } from '@mui/material';
import { DataGrid } from '@mui/x-data-grid';
import { useEffect, useMemo, useState } from 'react';
import { canonicalSkill } from '../../assets/skills';
import { useCalcBatch } from '../../api/queries/calc';
import { useLatestPrices, useLiquidityStats } from '../../api/queries/prices';
import { usePriceableItemIds, useSkillRecipes } from '../../api/queries/recipes';
import { usePlayerPrefs } from '../character/usePlayerPrefs';
import { AssumptionsBar } from './AssumptionsBar';
import { RecipeFilters } from './RecipeFilters';
import type { FiltersValue } from './RecipeFilters';
import { buildRows, firstAssumptions } from './buildRows';
import { recipeColumns } from './columns';

const PAGE_SIZE = 25;

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
  const prices = useLatestPrices(pageIds);
  const stats = useLiquidityStats(pageIds);

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

  return (
    <Stack spacing={2}>
      <Typography variant="h5" component="h2">Recipes</Typography>

      <RecipeFilters value={filters} onChange={handleFilters} />

      {player.name === '' && (
        <Alert severity="info" variant="outlined">
          Character name is not set, so recipe level eligibility is not checked. Enter it in the
          character column on the left.
        </Alert>
      )}

      <AssumptionsBar assumptions={assumptions} />

      {filters.includeIncomplete && (
        <Alert severity="warning" variant="outlined">
          Paths with unpriced inputs are included. Their money figures are an upper bound, not an
          estimate.
        </Alert>
      )}

      <Box sx={{ height: 640 }}>
        <DataGrid
          rows={rows}
          columns={recipeColumns}
          loading={recipes.isFetching || calc.isFetching || stats.isPending}
          hideFooter
          disableRowSelectionOnClick
          density="compact"
        />
      </Box>

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
