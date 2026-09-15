interface GatewayFailure {
  error?: unknown;
  target?: unknown;
  details?: unknown;
}

function asRecord(value: unknown): GatewayFailure | null {
  return typeof value === 'object' && value !== null ? (value as GatewayFailure) : null;
}

function serviceName(target: unknown): string | null {
  if (typeof target !== 'string') return null;
  return target.replace(/^https?:\/\//, '').split(':')[0];
}

export function describeFailure(status: number, body: unknown, fallback: string): string {
  const record = asRecord(body);

  // The gateway reports a service it could not reach as 502 plus the target.
  if (status === 502) {
    const name = serviceName(record?.target);
    return name ? `Сервис ${name} недоступен` : 'Один из сервисов недоступен';
  }

  // A transport failure never carries an HTTP status.
  if (status === 0) return 'Нет связи со шлюзом';

  // A server-authored reason is a domain answer, not a breakage: show it as is.
  if (typeof record?.error === 'string') return record.error;

  return fallback;
}

export function failure(response: Response, body: unknown, fallback: string): Error {
  return new Error(describeFailure(response.status, body, fallback));
}
