import { Chip, Tooltip } from '@mui/material';
import { serviceName } from '../api/errors';
import { useGatewayHealth } from '../api/queries/health';

export function HealthIndicator() {
  const { data, isError } = useGatewayHealth();

  if (isError) return <Chip size="small" color="error" label="Шлюз недоступен" />;
  if (!data) return <Chip size="small" variant="outlined" label="Проверка" />;
  if (data.all_upstreams_ok) return <Chip size="small" color="success" label="Все сервисы в строю" />;

  const down = (data.upstreams ?? [])
    .filter((u) => !u.ok)
    .map((u) => serviceName(u.target) ?? u.target);
  return (
    <Tooltip title="Часть данных будет недоступна">
      <Chip size="small" color="warning" label={`Недоступны: ${down.join(', ')}`} />
    </Tooltip>
  );
}
