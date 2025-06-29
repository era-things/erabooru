import createClient from 'openapi-fetch';
import type { paths } from '$lib/types/api';
import { apiBase } from '$lib/config';

export const api = createClient<paths>({
  baseUrl: apiBase,
});