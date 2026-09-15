import { AppBar, Box, Toolbar, Typography } from '@mui/material';
import { useState } from 'react';
import { HealthIndicator } from './shared/HealthIndicator';
import { CharacterColumn } from './features/character/CharacterPage';
import { RecipesPage } from './features/recipes/RecipesPage';
import { ChatPopup } from './features/chat/ChatPopup';

export default function App() {
  const [selectedSkill, setSelectedSkill] = useState('');

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <AppBar position="static" color="transparent" elevation={0}>
        <Toolbar sx={{ gap: 2, borderBottom: 1, borderColor: 'divider', minHeight: 52 }}>
          <Typography variant="h6" sx={{ color: 'primary.main', fontWeight: 700, flexGrow: 1 }}>
            RS3 Market
          </Typography>
          <HealthIndicator />
        </Toolbar>
      </AppBar>

      <Box sx={{ flex: 1, minHeight: 0, display: 'flex' }}>
        <Box
          component="aside"
          sx={{
            width: 320,
            flexShrink: 0,
            borderRight: 1,
            borderColor: 'divider',
            overflowY: 'auto',
            p: 2,
          }}
        >
          <CharacterColumn onSelectSkill={setSelectedSkill} />
        </Box>

        <Box component="main" sx={{ flex: 1, minWidth: 0, overflow: 'hidden', p: 2 }}>
          <RecipesPage skill={selectedSkill} onSkillChange={setSelectedSkill} />
        </Box>
      </Box>

      <ChatPopup />
    </Box>
  );
}
