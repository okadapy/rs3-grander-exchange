import { Box, Stack, Typography } from '@mui/material';
import type { ChatMessage } from '../../api/queries/chat';

export function MessageList({ messages }: { messages: ChatMessage[] }) {
  if (messages.length === 0) {
    return <Typography color="text.secondary">No messages yet.</Typography>;
  }

  return (
    <Stack spacing={1.5}>
      {messages.map((message) => (
        <Box key={message.id}>
          <Stack direction="row" spacing={1} sx={{ alignItems: 'baseline' }}>
            <Typography variant="body2" sx={{ color: 'primary.main', fontWeight: 600 }}>
              {message.username}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {new Date(message.created_at).toLocaleTimeString()}
            </Typography>
          </Stack>
          <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap' }}>
            {message.body}
          </Typography>
        </Box>
      ))}
    </Stack>
  );
}
