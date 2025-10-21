<script lang="ts">
	import { onMount } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // Masonry component
	import type { MediaPreviewItem, MediaItem } from '$lib/types/media';
	import { PAGE_SIZE } from '$lib/constants';
	import { fetchMediaPreviews } from '$lib/api';

	let {
		query = '',
		vectorQuery = '',
		page = 1,
		pageSize = Number(PAGE_SIZE),
		total = $bindable(1),
		items
	}: {
		query?: string;
		vectorQuery?: string;
		page?: number;
		pageSize?: number;
		total?: number;
		items?: MediaPreviewItem[];
	} = $props();
	let media: MediaPreviewItem[] = $state([]);
	let innerWidth = $state(0);
	let mounted = $state(false);
	let scrollY = $state(0);

	let columnCount = $derived(Math.max(Math.floor(innerWidth / 300), 2));
	let columnWidths = $derived(Array(columnCount).fill('1fr'));
	const normalizedVectorQuery = $derived(typeof vectorQuery === 'string' ? vectorQuery : '');

	const usingProvidedItems = $derived(items !== undefined);

	let requestCounter = 0;

	$effect(() => {
		if (usingProvidedItems) {
			media = items ?? [];
			total = media.length;
			return;
		}

		if (!mounted) return;

		void load(query, page, pageSize, normalizedVectorQuery);
	});

	async function load(
		currentQuery: string,
		currentPage: number,
		currentPageSize: number,
		currentVectorQuery: string
	) {
		const requestId = ++requestCounter;
		try {
			const data = await fetchMediaPreviews(
				currentQuery,
				currentPage,
				currentPageSize,
				currentVectorQuery
			);
			if (requestId !== requestCounter) {
				return;
			}
			const items = data.media as MediaItem[];
			media = items.map((it) => {
				const displayHeight = Math.min(it.height, it.width * 3);
				return {
					...it,
					height: displayHeight,
					displayHeight,
					originalHeight: it.height,
					cropped: displayHeight < it.height
				} satisfies MediaPreviewItem;
			});
			total = data.total ?? (1 as number);
		} catch (err) {
			console.error('media fetch error', err);
		}
	}

	onMount(() => {
		mounted = true;
		if (usingProvidedItems) {
			media = items ?? [];
			total = media.length;
			return;
		}
	});
</script>

<svelte:window bind:innerWidth bind:scrollY />

<Masonry items={media} {columnWidths} scrollPosition={scrollY} />
