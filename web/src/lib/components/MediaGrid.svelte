<script lang="ts">
	import { onMount } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // the Masonry component from the previous example
	import type { MediaItem } from '$lib/types/media'; 

	const apiBase = 'http://localhost/api';

	let {query = ''} = $props();
	let lastQuery: string = $state('');

	let media: MediaItem[] = $state([]);
	let innerWidth = $state(0);
	let mounted = $state(false);

	let columnCount = $derived(Math.max(Math.floor(innerWidth/300), 2));
	let columnWidths = $derived(Array(columnCount).fill('1fr'));

	$effect(() => {
        if (mounted && query !== lastQuery) {
            lastQuery = query;
            load();
        }
    });

	async function load() {
		try {
			const url = query
				? `${apiBase}/media/previews?q=${encodeURIComponent(query)}`
				: `${apiBase}/media/previews`;
			const res = await fetch(url);
			if (res.ok) {
				const data = await res.json();
				media = data.media as MediaItem[];
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
		await load();
	});
</script>

<svelte:window bind:innerWidth />

<Masonry items={media} columnWidths={columnWidths} />

