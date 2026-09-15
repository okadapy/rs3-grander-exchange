import { Typography } from '@mui/material';

interface Props {
  skill: string;
  onSkillChange: (skill: string) => void;
}

// Placeholder until Task 11 wires up the recipe table. It already accepts
// the shell's selected-skill state so CharacterColumn can filter it without
// navigation; canonicalSkill validation on whatever `skill` arrives is added
// alongside the real table.
export function RecipesPage({ skill }: Props) {
  return (
    <>
      <Typography variant="h5" component="h1">Recipes</Typography>
      {skill && (
        <Typography variant="body2" color="text.secondary">
          Filtered by {skill}
        </Typography>
      )}
    </>
  );
}
