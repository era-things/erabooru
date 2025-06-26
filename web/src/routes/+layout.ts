// Root layout-module — runs at build time, never in the browser
export const prerender = true;
export const ssr = false;

/*
 * ── optional extras you can add later ─────────────────────────
 * export const ssr = false;          // turn the whole site into a pure SPA
 * export const trailingSlash = 'always'; // write /foo/index.html instead of /foo.html
 *
 * // Typed load function stub (only if you need data in the layout)
 * import type { LayoutLoad } from './$types';
 * export const load: LayoutLoad = async () => {
 *   return {};
 * };
 */
