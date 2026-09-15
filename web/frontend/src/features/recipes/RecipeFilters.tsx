import { FormControlLabel, MenuItem, Stack, Switch, TextField } from '@mui/material';
import { SKILLS } from '../../assets/skills';

export interface FiltersValue {
  skill: string;
  minLevel: number;
  maxLevel: number;
  includeIncomplete: boolean;
  spreadPct: number | undefined;
}

interface Props {
  value: FiltersValue;
  onChange(next: FiltersValue): void;
}

const MIN_LEVEL = 1;
// Virtual levels from the hiscores go past a skill's nominal cap, so the
// bound is the table's own ceiling rather than 99 or 120.
const MAX_LEVEL = 150;

// A cleared numeric field reads as '', and Number('') is 0: clearing "Max
// level" used to send level=0 and empty the table with no explanation. An
// empty field means that bound simply is not narrowing anything.
function levelFrom(raw: string, fallback: number): number {
  if (raw.trim() === '') return fallback;
  const parsed = Number(raw);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.min(MAX_LEVEL, Math.max(MIN_LEVEL, Math.round(parsed)));
}

export function RecipeFilters({ value, onChange }: Props) {
  return (
    <Stack direction="row" spacing={2} useFlexGap sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
      <TextField
        select
        label="Skill"
        size="small"
        sx={{ minWidth: 200 }}
        value={value.skill}
        onChange={(event) => onChange({ ...value, skill: event.target.value })}
      >
        <MenuItem value="">All skills</MenuItem>
        {SKILLS.map((skill) => (
          <MenuItem key={skill} value={skill}>{skill}</MenuItem>
        ))}
      </TextField>

      <TextField
        label="Min level"
        type="number"
        size="small"
        sx={{ width: 120 }}
        slotProps={{ htmlInput: { min: MIN_LEVEL, max: MAX_LEVEL } }}
        value={value.minLevel}
        onChange={(event) =>
          onChange({ ...value, minLevel: levelFrom(event.target.value, MIN_LEVEL) })
        }
      />
      <TextField
        label="Max level"
        type="number"
        size="small"
        sx={{ width: 120 }}
        slotProps={{ htmlInput: { min: MIN_LEVEL, max: MAX_LEVEL } }}
        value={value.maxLevel}
        onChange={(event) =>
          onChange({ ...value, maxLevel: levelFrom(event.target.value, MAX_LEVEL) })
        }
      />
      <TextField
        label="Spread, %"
        type="number"
        size="small"
        sx={{ width: 120 }}
        value={value.spreadPct ?? ''}
        onChange={(event) =>
          onChange({
            ...value,
            spreadPct: event.target.value === '' ? undefined : Number(event.target.value),
          })
        }
      />

      <FormControlLabel
        control={
          <Switch
            checked={value.includeIncomplete}
            onChange={(event) => onChange({ ...value, includeIncomplete: event.target.checked })}
          />
        }
        label="Include paths with unpriced inputs"
      />
    </Stack>
  );
}
