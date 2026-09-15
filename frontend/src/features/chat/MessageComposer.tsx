import { Button, Stack, TextField } from '@mui/material';
import { useState } from 'react';

interface Props {
  disabled: boolean;
  onSend(body: string): void;
}

export function MessageComposer({ disabled, onSend }: Props) {
  const [body, setBody] = useState('');

  function submit() {
    const trimmed = body.trim();
    if (trimmed.length === 0) return;
    onSend(trimmed);
    setBody('');
  }

  return (
    <Stack direction="row" spacing={1}>
      <TextField
        label="Message"
        size="small"
        fullWidth
        // The server caps the body at 500 characters.
        slotProps={{ htmlInput: { maxLength: 500 } }}
        value={body}
        onChange={(event) => setBody(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            submit();
          }
        }}
      />
      <Button variant="contained" onClick={submit} disabled={disabled}>
        Send
      </Button>
    </Stack>
  );
}
