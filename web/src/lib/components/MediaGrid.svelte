<script lang="ts">
	import { onMount } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // Masonry component
	import type { MediaItem } from '$lib/types/media';
	import { PAGE_SIZE } from '$lib/constants';

	const apiBase = 'http://localhost/api';

	let { query = '', page = 1, pageSize = Number(PAGE_SIZE), total = $bindable(0) } = $props();
	let lastQuery: string = $state('');
	let lastPage: number = $state(1);

	let media: MediaItem[] = $state([]);
	let innerWidth = $state(0);
	let mounted = $state(false);

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
			const url = query
				? `${apiBase}/media/previews?q=${encodeURIComponent(query)}&page=${page}&page_size=${pageSize}`
				: `${apiBase}/media/previews?page=${page}&page_size=${pageSize}`;
			const res = await fetch(url);
			if (res.ok) {
				const data = await res.json();
				media = data.media as MediaItem[];
				total = data.total as number;
			} else {
				console.error('media fetch error', res.status, res.statusText);
			}
		} catch (err) {
			console.error('network error', err);
		}
	}

	onMount(async () => {
		mounted = true;
		lastQuery = query;
		lastPage = page;
		await load();
	});
</script>

<svelte:window bind:innerWidth />

<Masonry items={media} {columnWidths} />
