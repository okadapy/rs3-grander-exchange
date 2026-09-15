import { Box, Button, MenuItem, Paper, Stack, TextField, Typography } from '@mui/material';
import { useState } from 'react';
import { usePlayer } from '../../api/queries/hiscore';
import type { HiscoreMode } from '../../api/queries/hiscore';
import { QueryState } from '../../shared/QueryState';
import { formatCompact, formatInt } from '../../shared/format';
import { SkillRow } from './SkillRow';
import { usePlayerPrefs } from './usePlayerPrefs';

// Bounded so the header, the name input and the Overall card stay in view
// above the list; ~8 rows show before the list itself starts scrolling.
const SKILL_LIST_MAX_HEIGHT = 440;

const MODES: { value: HiscoreMode; label: string }[] = [
  { value: 'normal', label: 'Обычный' },
  { value: 'ironman', label: 'Ironman' },
  { value: 'hardcore', label: 'Hardcore ironman' },
];

export function CharacterPage() {
  const prefs = usePlayerPrefs();
  const [draft, setDraft] = useState(prefs.name);
  const query = usePlayer(prefs.name, prefs.mode);

  return (
    <Stack spacing={3}>
      <Typography variant="h5" component="h1">Персонаж</Typography>

      {/* This MUI version dropped Stack's legacy alignItems/justifyContent
          passthrough props; they must go through sx or the DOM never gets
          the align-items rule and the value leaks as a bogus attribute. */}
      <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
        <TextField
          label="Имя персонажа"
          size="small"
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
        />
        <TextField
          select
          label="Режим"
          size="small"
          value={prefs.mode}
          onChange={(event) => prefs.setMode(event.target.value as HiscoreMode)}
          sx={{ minWidth: 200 }}
        >
          {MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>{mode.label}</MenuItem>
          ))}
        </TextField>
        <Button variant="contained" onClick={() => prefs.setName(draft.trim())}>
          Показать
        </Button>
      </Stack>

      {prefs.name === '' ? (
        <Typography color="text.secondary">
          Введите имя персонажа, чтобы увидеть уровни. Оно же подставится в расчёт рецептов.
        </Typography>
      ) : (
        <QueryState query={query}>
          {(player) => {
            const overall = player.skills.find((entry) => entry.skill === 'Overall');
            const skills = player.skills.filter((entry) => entry.skill !== 'Overall');

            return (
              <Stack spacing={2}>
                <Paper variant="outlined" sx={{ p: 2 }}>
                  <Typography variant="body2" color="text.secondary">
                    {player.name}, суммарный уровень
                  </Typography>
                  <Typography variant="h4" component="p">{formatInt(overall?.level)}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {formatCompact(overall?.xp)} опыта, ранг {formatInt(overall?.rank)}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    Данные получены {new Date(player.fetched_at).toLocaleString('ru-RU')}.
                    Если хайскоры недоступны, сервер отдаёт последнюю сохранённую копию.
                  </Typography>
                </Paper>

                <Box
                  sx={{
                    maxHeight: SKILL_LIST_MAX_HEIGHT,
                    overflowY: 'auto',
                    border: 1,
                    borderColor: 'divider',
                    borderRadius: 1,
                  }}
                >
                  {skills.map((skill) => (
                    <SkillRow key={skill.skill} skill={skill} />
                  ))}
                </Box>
              </Stack>
            );
          }}
        </QueryState>
      )}
    </Stack>
  );
}
