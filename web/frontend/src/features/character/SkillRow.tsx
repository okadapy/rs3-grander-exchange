import { Box, ButtonBase, LinearProgress, Tooltip, Typography } from '@mui/material';
import type { PlayerSkill } from '../../api/queries/hiscore';
import { levelProgress } from '../../assets/experience';
import { formatCompact, formatInt } from '../../shared/format';
import { SkillIcon } from '../../shared/SkillIcon';

// The character column is 320px wide, which leaves roughly 262px inside this
// row's own padding. Icon, name, level, xp, rank and a progress bar on one
// line needed over 500px of fixed widths, so every column past the name was
// pushed beyond the right edge: the name collapsed to nothing, the icon and
// level scrolled out of sight, and the list gained a horizontal scrollbar.
//
// Two lines hold the same information in the width available — identity on
// top, progress underneath. Keep it that way: every flexible element here
// carries minWidth: 0 so it ellipsizes instead of pushing its neighbours,
// and nothing may take a fixed width wide enough to bring the overflow back.
export function SkillRow({ skill, onSelect }: { skill: PlayerSkill; onSelect: (skill: string) => void }) {
  const progress = levelProgress(skill.skill, skill.level, skill.xp);
  const remaining = progress.complete
    ? 'Maximum experience reached'
    : `${formatCompact(progress.remainingXp)} xp to the next level`;

  return (
    <ButtonBase
      onClick={() => onSelect(skill.skill)}
      aria-label={`Filter recipes by ${skill.skill}`}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'stretch',
        gap: 0.5,
        px: 1.5,
        py: 1,
        width: '100%',
        textAlign: 'left',
        color: 'inherit',
        borderBottom: 1,
        borderColor: 'divider',
        '&:hover': { bgcolor: 'action.hover' },
        '&:focus-visible': { outline: '2px solid', outlineColor: 'primary.main', outlineOffset: -2 },
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%', minWidth: 0 }}>
        <Box sx={{ width: 20, height: 20, flexShrink: 0, display: 'flex' }}>
          <SkillIcon skill={skill.skill} size={20} />
        </Box>

        <Typography variant="body2" noWrap sx={{ flexGrow: 1, minWidth: 0 }}>
          {skill.skill}
        </Typography>

        <Typography variant="body2" noWrap sx={{ flexShrink: 0, fontWeight: 600 }}>
          {formatInt(skill.level)}
        </Typography>
      </Box>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%', minWidth: 0 }}>
        <Tooltip title={remaining}>
          <LinearProgress
            variant="determinate"
            value={progress.fraction * 100}
            sx={{ flexGrow: 1, minWidth: 0, height: 5, borderRadius: 1 }}
          />
        </Tooltip>

        <Typography variant="caption" color="text.secondary" noWrap sx={{ flexShrink: 0 }}>
          {formatCompact(skill.xp)} · rank {formatInt(skill.rank)}
        </Typography>
      </Box>
    </ButtonBase>
  );
}
