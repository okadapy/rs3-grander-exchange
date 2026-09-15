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
        value={value.minLevel}
        onChange={(event) => onChange({ ...value, minLevel: Number(event.target.value) })}
      />
      <TextField
        label="Max level"
        type="number"
        size="small"
        sx={{ width: 120 }}
        value={value.maxLevel}
        onChange={(event) => onChange({ ...value, maxLevel: Number(event.target.value) })}
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
