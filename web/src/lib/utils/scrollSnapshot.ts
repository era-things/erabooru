import { onDestroy, tick } from 'svelte';
import type { Snapshot } from '@sveltejs/kit';

/** Restore list content before scrolling; an empty remounted list clamps scroll to zero. */
export function createScrollSnapshot<T>(
	capture: () => T,
	restore: (state: T) => boolean
): Snapshot<{ state: T; x: number; y: number }> {
	let mounted = true;
	onDestroy(() => {
		mounted = false;
	});
	return {
		capture: () => ({ state: capture(), x: window.scrollX, y: window.scrollY }),
		restore: (saved) => {
			if (!restore(saved.state)) return;
			void tick().then(() => {
				if (mounted) window.scrollTo(saved.x, saved.y);
			});
		}
	};
}
