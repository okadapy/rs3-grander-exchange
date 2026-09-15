import { AppBar, Box, Button, Container, Stack, Toolbar, Typography } from '@mui/material';
import { NavLink, Navigate, Route, Routes } from 'react-router-dom';
import { HealthIndicator } from './shared/HealthIndicator';
import { CharacterPage } from './features/character/CharacterPage';
import { RecipesPage } from './features/recipes/RecipesPage';
import { ChatPage } from './features/chat/ChatPage';

const NAV = [
  { to: '/character', label: 'Персонаж' },
  { to: '/recipes', label: 'Рецепты' },
  { to: '/chat', label: 'Чат' },
];

export default function App() {
  return (
    <Box sx={{ minHeight: '100vh' }}>
      <AppBar position="static" color="transparent" elevation={0}>
        <Toolbar sx={{ gap: 3, borderBottom: 1, borderColor: 'divider' }}>
          <Typography variant="h6" sx={{ color: 'primary.main', fontWeight: 700 }}>
            RS3 Market
          </Typography>
          <Stack direction="row" spacing={1} sx={{ flexGrow: 1 }}>
            {NAV.map((item) => (
              <Button key={item.to} component={NavLink} to={item.to} color="inherit" size="small">
                {item.label}
              </Button>
            ))}
          </Stack>
          <HealthIndicator />
        </Toolbar>
      </AppBar>

      <Container maxWidth={false} sx={{ py: 3 }}>
        <Routes>
          <Route path="/" element={<Navigate to="/character" replace />} />
          <Route path="/character" element={<CharacterPage />} />
          <Route path="/recipes" element={<RecipesPage />} />
          <Route path="/chat" element={<ChatPage />} />
        </Routes>
      </Container>
    </Box>
  );
}
