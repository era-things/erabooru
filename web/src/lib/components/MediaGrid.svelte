<script lang="ts">
	import { onMount } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // Masonry component
	import type { MediaItem } from '$lib/types/media';
	import { PAGE_SIZE } from '$lib/constants';
	import { fetchMediaPreviews } from '$lib/api';

	let { query = '', page = 1, pageSize = Number(PAGE_SIZE), total = $bindable(1) } = $props();
	let lastQuery: string = $state('');
	let lastPage: number = $state(1);

	let media: MediaItem[] = $state([]);
	let innerWidth = $state(0);
	let mounted = $state(false);
	let scrollY = $state(0);

	let columnCount = $derived(Math.max(Math.floor(innerWidth / 300), 2));
	let columnWidths = $derived(Array(columnCount).fill('1fr'));

	$effect(() => {
		if (mounted && (query !== lastQuery || page !== lastPage)) {
			lastQuery = query;
			lastPage = page;
			load();
		}
	});

	async function load() {
		try {
			const data = await fetchMediaPreviews(query, page, pageSize);
			media = data.media as MediaItem[];
			total = data.total ?? (1 as number);
		} catch (err) {
			console.error('media fetch error', err);
		}
	}

	onMount(async () => {
		mounted = true;
		lastQuery = query;
		lastPage = page;
		await load();
	});
</script>

<svelte:window bind:innerWidth bind:scrollY />

<Masonry items={media} {columnWidths} scrollPosition={scrollY} />
