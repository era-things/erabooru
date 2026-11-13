<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // Masonry component
	import type { MediaPreviewItem, MediaItem } from '$lib/types/media';
	import { PAGE_SIZE } from '$lib/constants';
	import { fetchMediaPreviews } from '$lib/api';
	import type { MediaPreviewsResponse } from '$lib/api';
	import { createRequest } from '$lib/utils/request.svelte';

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
	const request = createRequest<MediaPreviewsResponse>();
	const loading = $derived(request.loading);
	const error = $derived(request.error?.message ?? '');

	let columnCount = $derived(Math.max(Math.floor(innerWidth / 300), 2));
	let columnWidths = $derived(Array(columnCount).fill('1fr'));
	const normalizedVectorQuery = $derived(typeof vectorQuery === 'string' ? vectorQuery : '');

	const usingProvidedItems = $derived(items !== undefined);

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
		void untrack(() => load(true, currentQuery, currentPageSize, currentVectorQuery));
	});

	function load(
		reset: boolean,
		currentQuery: string,
		currentPageSize: number,
		currentVectorQuery: string
	) {
		if (usingProvidedItems || (loading && !reset)) return;
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
					signal
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
