<script lang="ts">
	import { onMount, onDestroy, untrack } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // Masonry component
	import type { MediaPreviewItem, MediaItem } from '$lib/types/media';
	import { PAGE_SIZE } from '$lib/constants';
	import { fetchMediaPreviews } from '$lib/api';

	let {
		query = '',
		vectorQuery = '',
		pageSize = Number(PAGE_SIZE),
		items
	}: {
		query?: string;
		vectorQuery?: string;
		pageSize?: number;
		items?: MediaPreviewItem[];
	} = $props();
	let media: MediaPreviewItem[] = $state([]);
	let innerWidth = $state(0);
	let mounted = $state(false);
	let scrollY = $state(0);
	let nextCursor = $state<string | null>(null);
	let loading = $state(false);
	let error = $state('');

	let columnCount = $derived(Math.max(Math.floor(innerWidth / 300), 2));
	let columnWidths = $derived(Array(columnCount).fill('1fr'));
	const normalizedVectorQuery = $derived(typeof vectorQuery === 'string' ? vectorQuery : '');

	const usingProvidedItems = $derived(items !== undefined);

	let requestCounter = 0;
	let controller: AbortController | undefined;
	onDestroy(() => {
		requestCounter++;
		controller?.abort();
	});

	$effect(() => {
		if (usingProvidedItems) {
			requestCounter++;
			controller?.abort();
			loading = false;
			error = '';
			media = items ?? [];
			nextCursor = null;
			return;
		}

		if (!mounted) return;

		const currentQuery = query;
		const currentPageSize = pageSize;
		const currentVectorQuery = normalizedVectorQuery;
		void untrack(() => load(true, currentQuery, currentPageSize, currentVectorQuery));
	});

	async function load(
		reset: boolean,
		currentQuery: string,
		currentPageSize: number,
		currentVectorQuery: string
	) {
		if (usingProvidedItems || (loading && !reset)) return;
		const requestId = ++requestCounter;
		controller?.abort();
		controller = new AbortController();
		loading = true;
		error = '';
		if (reset) {
			media = [];
			nextCursor = null;
		}
		try {
			const data = await fetchMediaPreviews(
				currentQuery,
				currentPageSize,
				currentVectorQuery,
				reset ? null : nextCursor,
				controller.signal
			);
			if (requestId !== requestCounter) {
				return;
			}
			const items = data.media as MediaItem[];
			const previews = items.map((it) => {
				const displayHeight = Math.min(it.height, it.width * 3);
				return {
					...it,
					height: displayHeight,
					displayHeight,
					originalHeight: it.height,
					cropped: displayHeight < it.height
				} satisfies MediaPreviewItem;
			});
			media = reset ? previews : [...media, ...previews];
			nextCursor = data.next_cursor || null;
		} catch (err) {
			if (requestId === requestCounter) {
				error = err instanceof Error ? err.message : 'Search failed';
			}
		} finally {
			if (requestId === requestCounter) loading = false;
		}
	}

	onMount(() => {
		mounted = true;
		if (usingProvidedItems) {
			media = items ?? [];
			nextCursor = null;
			return;
		}
	});
</script>

<svelte:window bind:innerWidth bind:scrollY />

{#if loading}
	<p role="status" class="my-4 text-sm text-gray-500">Loading results...</p>
{/if}
{#if error}
	<div role="alert" class="my-4 text-sm text-red-600">
		<p>Could not load results: {error}</p>
		<button
			type="button"
			class="underline"
			onclick={() => load(true, query, pageSize, normalizedVectorQuery)}>Retry</button
		>
	</div>
{/if}

<Masonry items={media} {columnWidths} scrollPosition={scrollY} />

{#if !usingProvidedItems && nextCursor}
	<div class="my-4 flex justify-center">
		<button
			type="button"
			class="rounded border px-4 py-2"
			disabled={loading}
			onclick={() => load(false, query, pageSize, normalizedVectorQuery)}
		>
			{loading ? 'Loading…' : 'Load more'}
		</button>
	</div>
{/if}
