import { Box, ButtonBase, LinearProgress, Typography } from '@mui/material';
import type { PlayerSkill } from '../../api/queries/hiscore';
import { levelProgress } from '../../assets/experience';
import { formatCompact, formatInt } from '../../shared/format';
import { SkillIcon } from '../../shared/SkillIcon';

// Fixed pixel widths (not flex fractions) on every column except the name is
// what keeps level/xp/rank/bar aligned down the whole list: each row is its
// own flex container, so only identical literal widths guarantee the same
// column edges row to row, independent of how long a neighbour's skill name
// or numbers happen to be.
export function SkillRow({ skill, onSelect }: { skill: PlayerSkill; onSelect: (skill: string) => void }) {
  const progress = levelProgress(skill.skill, skill.level, skill.xp);

  return (
    <ButtonBase
      onClick={() => onSelect(skill.skill)}
      aria-label={`Filter recipes by ${skill.skill}`}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        px: 1.5,
        py: 1,
        width: '100%',
        textDecoration: 'none',
        color: 'inherit',
        borderBottom: 1,
        borderColor: 'divider',
        '&:hover': { bgcolor: 'action.hover' },
        '&:focus-visible': { outline: '2px solid', outlineColor: 'primary.main', outlineOffset: -2 },
      }}
    >
      <Box sx={{ width: 24, height: 24, flexShrink: 0 }}>
        <SkillIcon skill={skill.skill} size={24} />
      </Box>

      <Typography variant="body2" noWrap sx={{ flexGrow: 1, minWidth: 0 }}>
        {skill.skill}
      </Typography>

      <Typography variant="body2" noWrap sx={{ width: 48, flexShrink: 0, textAlign: 'right' }}>
        {formatInt(skill.level)}
      </Typography>

      <Typography
        variant="caption"
        color="text.secondary"
        noWrap
        sx={{ width: 72, flexShrink: 0, textAlign: 'right' }}
      >
        {formatCompact(skill.xp)}
      </Typography>

      <Typography
        variant="caption"
        color="text.secondary"
        noWrap
        sx={{ width: 88, flexShrink: 0, textAlign: 'right' }}
      >
        rank {formatInt(skill.rank)}
      </Typography>

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: 220, flexShrink: 0 }}>
        <LinearProgress
          variant="determinate"
          value={progress.fraction * 100}
          sx={{ flexGrow: 1, height: 6, borderRadius: 1 }}
        />
        <Typography
          variant="caption"
          color="text.secondary"
          noWrap
          sx={{ width: 120, flexShrink: 0, textAlign: 'right' }}
        >
          {progress.complete ? 'Max experience' : `${formatCompact(progress.remainingXp)} to go`}
        </Typography>
      </Box>
    </ButtonBase>
  );
}
