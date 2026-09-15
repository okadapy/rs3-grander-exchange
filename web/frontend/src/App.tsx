import { AppBar, Box, Tab, Tabs, Toolbar, Typography } from '@mui/material';
import { useState } from 'react';
import { HealthIndicator } from './shared/HealthIndicator';
import { CharacterColumn } from './features/character/CharacterPage';
import { ItemsPage } from './features/items/ItemsPage';
import { RecipesPage } from './features/recipes/RecipesPage';
import { ChatPopup } from './features/chat/ChatPopup';

export default function App() {
  const [selectedSkill, setSelectedSkill] = useState('');
  // No router in the app (it was removed once the shell stopped needing
  // one), so the open tab is plain shell state and never a URL.
  const [tab, setTab] = useState<'recipes' | 'items'>('recipes');

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <AppBar position="static" color="transparent" elevation={0}>
        <Toolbar sx={{ gap: 2, borderBottom: 1, borderColor: 'divider', minHeight: 52 }}>
          <Typography
            variant="h6"
            component="h1"
            sx={{ color: 'primary.main', fontWeight: 700, flexGrow: 1 }}
          >
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

        {/* overflowY, not hidden: the table, its notices and the pagination
            row together outgrow a 1080p viewport, and with the overflow
            clipped the Back/Next buttons were unreachable rather than merely
            below the fold. The column layout lets the grid inside take the
            remaining height instead of forcing that overflow. */}
        <Box
          component="main"
          sx={{
            flex: 1,
            minWidth: 0,
            overflowY: 'auto',
            p: 2,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <Tabs
            value={tab}
            onChange={(_, next: 'recipes' | 'items') => setTab(next)}
            sx={{ mb: 2, borderBottom: 1, borderColor: 'divider' }}
          >
            <Tab value="recipes" label="Recipes" />
            <Tab value="items" label="Items" />
          </Tabs>

          {/* Unmounted rather than hidden: the recipe table holds a live
              price subscription and a page of calculations, and keeping
              them running behind an invisible tab would poll for rows
              nobody is looking at. */}
          {tab === 'recipes' ? (
            <RecipesPage skill={selectedSkill} onSkillChange={setSelectedSkill} />
          ) : (
            <ItemsPage />
          )}
        </Box>
      </Box>

      <ChatPopup />
    </Box>
  );
}
