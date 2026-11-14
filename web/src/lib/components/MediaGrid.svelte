<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // Masonry component
	import type { MediaPreviewItem, MediaItem, MediaGridState } from '$lib/types/media';
	import { PAGE_SIZE } from '$lib/constants';
	import { fetchMediaPreviews } from '$lib/api';
	import type { MediaPreviewsResponse } from '$lib/api';
	import { createRequest } from '$lib/utils/request.svelte';

	let {
		query = '',
		vectorQuery = '',
		seed = '',
		pageSize = Number(PAGE_SIZE),
		items
	}: {
		query?: string;
		vectorQuery?: string;
		seed?: string;
		pageSize?: number;
		items?: MediaPreviewItem[];
	} = $props();
	let media: MediaPreviewItem[] = $state([]);
	let innerWidth = $state(0);
	let mounted = $state(false);
	let nextCursor = $state<string | null>(null);
	const request = createRequest<MediaPreviewsResponse>();
	const loading = $derived(request.loading);
	const error = $derived(request.error?.message ?? '');

	let columnCount = $derived(Math.max(Math.floor(innerWidth / 300), 2));
	let columnWidths = $derived(Array(columnCount).fill('1fr'));
	const normalizedVectorQuery = $derived(typeof vectorQuery === 'string' ? vectorQuery : '');

	const usingProvidedItems = $derived(items !== undefined);
	const queryKey = $derived(JSON.stringify([query, pageSize, normalizedVectorQuery, seed]));
	let loadedKey = '';

	export function captureState(): MediaGridState | null {
		return media.length ? { key: loadedKey, media: $state.snapshot(media), nextCursor } : null;
	}

	export function restoreState(saved: MediaGridState): boolean {
		if (usingProvidedItems || saved.key !== queryKey) return false;
		request.cancel();
		loadedKey = saved.key;
		media = saved.media;
		nextCursor = saved.nextCursor;
		return true;
	}

	$effect(() => {
		if (usingProvidedItems) {
			request.cancel();
			media = items ?? [];
			nextCursor = null;
			return;
		}

		if (!mounted) return;

		const currentQuery = query;
		const currentPageSize = pageSize;
		const currentVectorQuery = normalizedVectorQuery;
		const currentSeed = seed;
		const key = queryKey;
		untrack(() => {
			if (key !== loadedKey)
				void load(true, currentQuery, currentPageSize, currentVectorQuery, currentSeed);
		});
	});

	function load(
		reset: boolean,
		currentQuery: string,
		currentPageSize: number,
		currentVectorQuery: string,
		currentSeed: string
	) {
		if (usingProvidedItems || (loading && !reset)) return;
		if (currentVectorQuery.trim() === ':random' && !currentSeed) return;
		loadedKey = JSON.stringify([currentQuery, currentPageSize, currentVectorQuery, currentSeed]);
		if (reset) {
			media = [];
			nextCursor = null;
		}
		return request.run(
			(signal) =>
				fetchMediaPreviews(
					currentQuery,
					currentPageSize,
					currentVectorQuery,
					reset ? null : nextCursor,
					signal,
					currentSeed
				),
			(data) => {
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
			}
		);
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

<svelte:window bind:innerWidth />

{#if loading}
	<p role="status" class="my-4 text-sm text-gray-500">Loading results...</p>
{/if}
{#if error}
	<div role="alert" class="my-4 text-sm text-red-600">
		<p>Could not load results: {error}</p>
		<button
			type="button"
			class="underline"
			onclick={() => load(true, query, pageSize, normalizedVectorQuery, seed)}>Retry</button
		>
	</div>
{/if}

<Masonry items={media} {columnWidths} />

{#if !usingProvidedItems && nextCursor}
	<div class="my-4 flex justify-center">
		<button
			type="button"
			class="rounded border px-4 py-2"
			disabled={loading}
			onclick={() => load(false, query, pageSize, normalizedVectorQuery, seed)}
		>
			{loading ? 'Loading…' : 'Load more'}
		</button>
	</div>
{/if}
