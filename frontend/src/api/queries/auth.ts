import { api } from '../client';
import { failure } from '../errors';
import type { components } from '../schema';

export type Credentials = components['schemas']['Credentials'];
export type RegisteredUser = components['schemas']['RegisteredUser'];

export async function login(credentials: Credentials): Promise<string> {
  const { data, error, response } = await api.POST('/auth/login', { body: credentials });
  if (error || !data) throw failure(response, error, 'Failed to sign in');
  return data.token;
}

export async function register(credentials: Credentials): Promise<RegisteredUser> {
  const { data, error, response } = await api.POST('/auth/register', { body: credentials });
  if (error || !data) throw failure(response, error, 'Failed to register');
  return data;
}
