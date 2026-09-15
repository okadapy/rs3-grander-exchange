import { describe, expect, it } from 'vitest';
import { describeFailure } from './errors';

describe('describeFailure', () => {
  it('names the downed service when the gateway answers 502', () => {
    const message = describeFailure(
      502,
      {
        error: 'upstream unavailable',
        target: 'http://recipe-service:8082',
        details: 'dial tcp 172.18.0.5:8082: connect: connection refused',
      },
      'Request failed',
    );

    expect(message).toBe('Service recipe-service is unavailable');
  });

  it('stays useful when a 502 carries no target', () => {
    expect(describeFailure(502, {}, 'Request failed')).toBe('One of the services is unavailable');
  });

  it('passes a domain refusal through unchanged', () => {
    expect(
      describeFailure(404, { error: 'no recipe for item 1513' }, 'Request failed'),
    ).toBe('no recipe for item 1513');
  });

  it('reports a transport failure separately from an HTTP answer', () => {
    expect(describeFailure(0, undefined, 'Request failed')).toBe('No connection to the gateway');
  });

  it('uses the caller fallback when the body explains nothing', () => {
    expect(describeFailure(500, 'plain text', 'Request failed')).toBe('Request failed');
  });
});
