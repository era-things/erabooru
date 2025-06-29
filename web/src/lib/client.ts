import createClient from "openapi-fetch";
import type { paths } from "$lib/types/api";   // <- the .d.ts file you generated

export const api = createClient<paths>({
  //baseUrl: import.meta.env.VITE_API_BASE,
});