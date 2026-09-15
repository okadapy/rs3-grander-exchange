import { Box, Tooltip } from '@mui/material';
import { useGatewayHealth } from '../api/queries/health';
import { serviceName } from '../api/errors';

export function HealthIndicator() {
  const { data, isError } = useGatewayHealth();

  const state = isError
    ? { color: 'error.main', label: 'Services unavailable' }
    : !data
      ? { color: 'text.disabled', label: 'Checking services' }
      : data.all_upstreams_ok
        ? { color: 'success.main', label: 'All services operational' }
        : {
            color: 'warning.main',
            label: `Some services are not responding: ${data.upstreams
              .filter((u) => !u.ok)
              .map((u) => serviceName(u.target) ?? u.target)
              .join(', ')}`,
          };

  return (
    <Tooltip title={state.label}>
      <Box
        role="status"
        aria-label={state.label}
        sx={{ width: 10, height: 10, borderRadius: '50%', bgcolor: state.color }}
      />
    </Tooltip>
  );
}
