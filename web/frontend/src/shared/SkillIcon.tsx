import { Box } from '@mui/material';
import { canonicalSkill, skillIconUrl } from '../assets/skills';

interface Props {
  skill: string;
  size?: number;
}

export function SkillIcon({ skill, size = 20 }: Props) {
  const url = skillIconUrl(skill);
  const name = canonicalSkill(skill);

  if (!url || !name) {
    return <Box component="span" sx={{ width: size, height: size, display: 'inline-block' }} />;
  }

  return (
    <Box
      component="img"
      src={url}
      alt={name}
      title={name}
      loading="lazy"
      sx={{ width: size, height: size, objectFit: 'contain', verticalAlign: 'middle' }}
    />
  );
}
