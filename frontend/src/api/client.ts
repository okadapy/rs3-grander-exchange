import createClient from 'openapi-fetch';
import { describeFailure } from './errors';
import type { paths } from './schema';

export const API_URL: string = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

export const api = createClient<paths>({ baseUrl: API_URL });

// When fetch() itself throws — connection refused, DNS failure, the gateway
// process being down — the request never reached the server and there is no
// Response to inspect. openapi-fetch rethrows that raw error by default, which
// would otherwise surface an untranslated browser message ("Failed to fetch")
// to the user instead of the section-11 "no connection" case. Translate it
// once here so every query function's failure(response, ...) path only has to
// handle resolved-but-failed responses.
api.use({
  onError() {
    return new Error(describeFailure(0, undefined, 'Запрос не выполнен'));
  },
});
