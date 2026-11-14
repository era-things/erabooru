<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { page } from '$app/state';
	import { PAGE_SIZE } from '$lib/constants';
	import { goto } from '$app/navigation';
	import { createRandomSeed } from '$lib/utils/searchParams';
	import { createScrollSnapshot } from '$lib/utils/scrollSnapshot';

	let grid = $state<MediaGrid>();
	export const snapshot = createScrollSnapshot(
		() => grid?.captureState() ?? null,
		(saved) => saved !== null && (grid?.restoreState(saved) ?? false)
	);

	const rawQuery = $derived(page.url.searchParams.get('q') ?? '');
	const vectorFlag = $derived(page.url.searchParams.get('vector') === '1');
	const vectorParam = $derived(page.url.searchParams.get('vector_q'));
	const hasVectorParam = $derived(page.url.searchParams.has('vector_q'));
	const randomSort = $derived(page.url.searchParams.get('sort') === 'random');
	const vectorQuery = $derived(
		randomSort ? ':random' : hasVectorParam ? (vectorParam ?? '') : vectorFlag ? rawQuery : ''
	);
	const seed = $derived(page.url.searchParams.get('seed') ?? '');
	const q = $derived(!randomSort && !hasVectorParam && vectorFlag ? '' : rawQuery);
	const pageSize = $derived(Number(page.url.searchParams.get('page_size') || PAGE_SIZE));

	$effect(() => {
		// Canonicalize direct random-sort links too, without adding a history entry.
		if (vectorQuery.trim() === ':random' && !seed) {
			const url = new URL(page.url);
			url.searchParams.set('seed', createRandomSeed());
			void goto(url, { replaceState: true, noScroll: true, keepFocus: true });
		}
	});
</script>

<div class="h-screen">
	<TabNav active="media" />
	<MediaGrid bind:this={grid} query={q} {vectorQuery} {pageSize} {seed} />
</div>
