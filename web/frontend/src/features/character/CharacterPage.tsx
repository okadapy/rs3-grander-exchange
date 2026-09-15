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
  { value: 'normal', label: 'Normal' },
  { value: 'ironman', label: 'Ironman' },
  { value: 'hardcore', label: 'Hardcore ironman' },
];

interface Props {
  onSelectSkill: (skill: string) => void;
}

export function CharacterColumn({ onSelectSkill }: Props) {
  const prefs = usePlayerPrefs();
  const [draft, setDraft] = useState(prefs.name);
  const query = usePlayer(prefs.name, prefs.mode);

  return (
    <Stack spacing={3}>
      <Typography variant="h5" component="h2">Character</Typography>

      {/* This MUI version dropped Stack's legacy alignItems/justifyContent
          passthrough props; they must go through sx or the DOM never gets
          the align-items rule and the value leaks as a bogus attribute. */}
      <Stack spacing={2} sx={{ alignItems: 'flex-start' }}>
        <TextField
          label="Player name"
          size="small"
          fullWidth
          value={draft}
          onChange={(event) => setDraft(event.target.value)}
        />
        <TextField
          select
          label="Mode"
          size="small"
          fullWidth
          value={prefs.mode}
          onChange={(event) => prefs.setMode(event.target.value as HiscoreMode)}
        >
          {MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>{mode.label}</MenuItem>
          ))}
        </TextField>
        <Button variant="contained" fullWidth onClick={() => prefs.setName(draft.trim())}>
          Show
        </Button>
      </Stack>

      {prefs.name === '' ? (
        <Typography color="text.secondary">
          Enter a player name to see their levels. It is also used for the recipe calculations.
        </Typography>
      ) : (
        <QueryState query={query}>
          {(player) => {
            const overall = player.skills.find((entry) => entry.skill === 'Overall');
            const skills = player.skills.filter((entry) => entry.skill !== 'Overall');

            return (
              <Stack spacing={2}>
                <Paper variant="outlined" sx={{ p: 2 }}>
                  <Stack direction="row" spacing={1} sx={{ alignItems: 'baseline' }} data-testid="player-summary">
                    <Typography variant="h6" component="p">{player.name}</Typography>
                    <Typography variant="body2" color="text.secondary">total level</Typography>
                    <Typography variant="h6" component="p">{formatInt(overall?.level)}</Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    {formatCompact(overall?.xp)} xp, rank {formatInt(overall?.rank)}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" data-testid="fetched-at">
                    {/* No explicit locale: the UI copy is English, but the date format itself
                        is left to the visitor's own browser locale on purpose. */}
                    Fetched {new Date(player.fetched_at).toLocaleString()} — may be a cached copy
                  </Typography>
                </Paper>

                <Box
                  sx={{
                    maxHeight: SKILL_LIST_MAX_HEIGHT,
                    overflowY: 'auto',
                    // overflow-x: visible is not allowed next to an overflowing
                    // y-axis, so the browser silently promotes it to auto and the
                    // list gains a horizontal scrollbar the moment a row is too
                    // wide. SkillRow is built to fit, and this keeps a future
                    // regression from turning the column into a scrolling strip.
                    overflowX: 'hidden',
                    border: 1,
                    borderColor: 'divider',
                    borderRadius: 1,
                  }}
                >
                  {skills.map((skill) => (
                    <SkillRow key={skill.skill} skill={skill} onSelect={onSelectSkill} />
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
