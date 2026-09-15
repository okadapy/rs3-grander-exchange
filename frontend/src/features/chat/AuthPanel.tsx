import { Alert, Button, Paper, Stack, TextField, Typography } from '@mui/material';
import { useState } from 'react';
import { useAuth } from '../../auth/AuthProvider';

export function AuthPanel() {
  const { username, error, pending, signIn, signUp, signOut } = useAuth();
  const [form, setForm] = useState({ username: '', password: '' });

  if (username) {
    return (
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Typography variant="body2">Signed in as {username}</Typography>
          <Button size="small" onClick={signOut}>
            Sign out
          </Button>
        </Stack>
      </Paper>
    );
  }

  return (
    <Paper variant="outlined" sx={{ p: 2 }}>
      <Stack spacing={2}>
        <Typography variant="body2" color="text.secondary">
          Chat and live prices require a token, so you need to sign in.
        </Typography>
        {error && <Alert severity="error">{error}</Alert>}
        <Stack spacing={1}>
          <TextField
            label="Username"
            size="small"
            value={form.username}
            onChange={(event) => setForm({ ...form, username: event.target.value })}
          />
          <TextField
            label="Password"
            type="password"
            size="small"
            value={form.password}
            onChange={(event) => setForm({ ...form, password: event.target.value })}
          />
          <Stack direction="row" spacing={1}>
            <Button variant="contained" disabled={pending} onClick={() => void signIn(form)}>
              Sign in
            </Button>
            <Button disabled={pending} onClick={() => void signUp(form)}>
              Register
            </Button>
          </Stack>
        </Stack>
      </Stack>
    </Paper>
  );
}
