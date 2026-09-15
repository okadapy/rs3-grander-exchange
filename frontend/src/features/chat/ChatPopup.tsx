import { Box, Paper, Typography } from '@mui/material';

// Minimal placeholder for the bottom-right chat popup. Task 8 gives it real
// behaviour (open/close, message list, sending); this only claims the slot
// so the layout does not shift once chat becomes functional.
export function ChatPopup() {
  return (
    <Box sx={{ position: 'fixed', right: 16, bottom: 16 }}>
      <Paper elevation={3} sx={{ px: 2, py: 1 }}>
        <Typography variant="body2">Chat</Typography>
      </Paper>
    </Box>
  );
}
