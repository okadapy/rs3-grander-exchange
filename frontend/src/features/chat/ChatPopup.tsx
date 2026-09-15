import { Alert, Box, Button, Paper, Stack } from '@mui/material';
import { useEffect, useMemo, useRef, useState } from 'react';
import type { ChatMessage } from '../../api/queries/chat';
import { useChatHistory } from '../../api/queries/chat';
import { useAuth } from '../../auth/AuthProvider';
import { QueryState } from '../../shared/QueryState';
import { useWs } from '../../ws/WsProvider';
import { AuthPanel } from './AuthPanel';
import { MessageComposer } from './MessageComposer';
import { MessageList } from './MessageList';

// The socket only ever adds messages that are missing from history (see
// WsProvider), but history can also load *after* a live message with the
// same id has already arrived (e.g. the user's own echo lands, then the
// `/chat/history` request that was already in flight resolves). Dedupe by
// id on every merge so neither ordering produces a duplicate row.
function mergeMessages(history: ChatMessage[], live: ChatMessage[]): ChatMessage[] {
  const seen = new Set(history.map((message) => message.id));
  return [...history, ...live.filter((message) => !seen.has(message.id))];
}

const PANEL_WIDTH = 360;
const PANEL_HEIGHT = 480;

export function ChatPopup() {
  const { token, signOut } = useAuth();
  const { status, messages: live, serverError, sendChat } = useWs();

  const [collapsed, setCollapsed] = useState(true);
  const [unread, setUnread] = useState(0);

  // Nobody sees the history while the popup is collapsed, so there is no
  // reason to fetch it before the visitor actually opens the panel.
  const history = useChatHistory({ enabled: !collapsed });
  const merged = useMemo(() => mergeMessages(history.data ?? [], live), [history.data, live]);
  // Count of live messages already accounted for, either by having been
  // shown (panel expanded) or already folded into the unread count. Only a
  // ref because it must not itself trigger a render.
  const seenCountRef = useRef(0);

  useEffect(() => {
    if (collapsed) {
      const arrived = live.length - seenCountRef.current;
      if (arrived > 0) {
        seenCountRef.current = live.length;
        setUnread((current) => current + arrived);
      }
    } else {
      seenCountRef.current = live.length;
      setUnread(0);
    }
  }, [live, collapsed]);

  const toggleLabel =
    collapsed && unread > 0 ? `Chat, ${unread} unread message${unread === 1 ? '' : 's'}` : 'Chat';

  return (
    <Box sx={{ position: 'fixed', right: 16, bottom: 16, zIndex: (t) => t.zIndex.snackbar }}>
      <Paper elevation={3} sx={{ width: collapsed ? 'auto' : PANEL_WIDTH, overflow: 'hidden' }}>
        <Button
          aria-label={toggleLabel}
          onClick={() => setCollapsed((current) => !current)}
          sx={{ width: '100%', justifyContent: 'flex-start', px: 2, py: 1 }}
        >
          Chat{unread > 0 && collapsed ? ` (${unread})` : ''}
        </Button>

        {!collapsed && (
          <Stack
            spacing={1.5}
            sx={{
              width: PANEL_WIDTH,
              height: PANEL_HEIGHT,
              boxSizing: 'border-box',
              p: 2,
              pt: 0,
            }}
          >
            <AuthPanel />

            {status === 'unauthorized' && (
              <Alert
                severity="warning"
                action={
                  <Button size="small" onClick={signOut}>
                    Sign out
                  </Button>
                }
              >
                Session is no longer valid. Sign in again.
              </Alert>
            )}
            {status === 'closed' && token && (
              <Alert severity="warning">Connection lost. Live messages are not arriving.</Alert>
            )}
            {serverError && <Alert severity="error">{serverError}</Alert>}

            <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto' }}>
              <QueryState query={history}>{() => <MessageList messages={merged} />}</QueryState>
            </Box>

            {token && <MessageComposer disabled={status !== 'open'} onSend={sendChat} />}
          </Stack>
        )}
      </Paper>
    </Box>
  );
}
